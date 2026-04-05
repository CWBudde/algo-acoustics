// Package worker manages a long-lived GPU server subprocess and provides a
// channel-based Go API for submitting GPU work.
//
// # Architecture: subprocess model
//
// The GPU server is a standalone CUDA binary (built separately with nvcc).
// Go starts it as a subprocess, communicates via a Unix domain socket for
// control messages, and uses POSIX shared memory for bulk data transfers
// (field arrays, ray buffers, hit records).
//
// This keeps the Go build entirely CUDA-free: `go build ./...` works on
// machines without a CUDA Toolkit.  The GPU binary is only required at
// runtime.
//
// # Wire protocol
//
// Every exchange is a request/response pair over the Unix socket.
// Requests and responses each have a fixed 8-byte header followed by a
// fixed-layout payload.  All integers are little-endian.
//
// Request header (8 bytes):
//
//	[2] type  — message type constant (see Msg* below)
//	[2] flags — reserved, must be 0
//	[4] len   — payload byte length
//
// Response header (8 bytes):
//
//	[4] status — 0 = OK, non-zero = error code (see Status* below)
//	[4] len    — response payload byte length
//
// Bulk data (field arrays, ray buffers) is never sent over the socket.
// The sender creates a POSIX shared memory segment, writes the data,
// sends the segment name (a 64-byte null-padded string) in the payload,
// and the receiver reads and unlinks when done.
//
// # Memory lifecycle
//
//	upload-once  geometry: BVH nodes + triangles (AllocBVH), IBM grid (AllocGrid)
//	upload-once  materials: packed into AllocGrid payload
//	persistent   FDTD field arrays stay in GPU VRAM across RunFDTD calls
//	per-batch    ray origins/directions uploaded each TraceRays call (~4 MB/64K rays)
//	download     receiver time series (tiny: N float32 samples) after RunFDTD
//	download     hit records per ray batch (32 bytes/ray) after TraceRays
//
// # Worker goroutine pattern
//
// Worker.Do serialises all socket I/O through a single internal goroutine.
// Callers block on a per-call response channel.  This prevents interleaved
// reads/writes on the socket from concurrent callers and allows safe use of
// Worker from multiple goroutines.
package worker

import (
	"encoding/binary"
	"fmt"
	"io"
)

// byteOrder is the wire byte order for all multi-byte integers.
var byteOrder = binary.LittleEndian

// -- Message types (uint16) -----------------------------------------------

const (
	MsgPing     uint16 = 0x0001
	MsgShutdown uint16 = 0x0002

	// MsgAllocGrid and related constants are FDTD messages (0x1xxx).
	MsgAllocGrid    uint16 = 0x1001
	MsgFreeGrid     uint16 = 0x1002
	MsgUploadFields uint16 = 0x1003
	MsgRunFDTD      uint16 = 0x1004

	// MsgAllocBVH and related constants are ray-tracing messages (0x2xxx).
	MsgAllocBVH  uint16 = 0x2001
	MsgFreeBVH   uint16 = 0x2002
	MsgTraceRays uint16 = 0x2003
)

// -- Status codes (uint32) -------------------------------------------------

const (
	StatusOK        uint32 = 0
	StatusNotImpl   uint32 = 1 // handler not yet implemented (14.4/14.5)
	StatusBadHandle uint32 = 2 // unknown or freed handle
	StatusOOM       uint32 = 3 // GPU out of memory
	StatusCUDA      uint32 = 4 // CUDA driver error (detail in server log)
	StatusBadMsg    uint32 = 5 // malformed request
)

// -- Fixed-layout payload structs -----------------------------------------
// All fields are little-endian.  Struct sizes must match the C counterparts
// in gpu/server/protocol.h exactly.

// ShmName is a 64-byte null-padded POSIX shared memory name, e.g. "/algo_gpu_deadbeef".
type ShmName [64]byte

// shmNameOf converts a string to a ShmName (truncated to 63 chars, always NUL-terminated).
func shmNameOf(s string) ShmName {
	var n ShmName

	copy(n[:63], s) // n[63] stays 0 (zero value = NUL terminator)

	return n
}

// String returns the name as a Go string (up to the first NUL byte).
func (n ShmName) String() string {
	for i, b := range n {
		if b == 0 {
			return string(n[:i])
		}
	}

	return string(n[:])
}

// -- Request header (8 bytes) ---------------------------------------------

type reqHeader struct {
	Type       uint16
	Flags      uint16
	PayloadLen uint32
}

func (h reqHeader) writeTo(w io.Writer) error {
	err := binary.Write(w, byteOrder, h)
	if err != nil {
		return fmt.Errorf("write request header: %w", err)
	}

	return nil
}

// -- Response header (8 bytes) --------------------------------------------

type respHeader struct {
	Status      uint32
	ResponseLen uint32
}

func readRespHeader(r io.Reader) (respHeader, error) {
	var h respHeader

	err := binary.Read(r, byteOrder, &h)
	if err != nil {
		return h, fmt.Errorf("read response header: %w", err)
	}

	return h, nil
}

// -- AllocGrid (0x1001) ---------------------------------------------------
// Request payload: 12 bytes.

type allocGridReq struct {
	Nx, Ny, Nz uint32 // 12 bytes
}

// Response payload: 8 bytes.
type allocGridResp struct {
	Handle uint64
}

// -- FreeGrid (0x1002) ----------------------------------------------------
// Request payload: 8 bytes.

type freeGridReq struct {
	Handle uint64
}

// -- UploadFields (0x1003) ------------------------------------------------
// Request payload: 72 bytes.
// The shared memory segment contains pCur followed by pPrev, each Nx·Ny·Nz
// float32 values (little-endian).

type uploadFieldsReq struct {
	Handle  uint64  // 8 bytes
	ShmName ShmName // 64 bytes
} // 72 bytes

// -- RunFDTD (0x1004) -----------------------------------------------------
// Request payload: 92 bytes.

type runFDTDReq struct {
	Handle       uint64  // 8
	Steps        uint32  // 4
	SrcIdx       uint32  // 4 — flat index of source injection node
	RcvIdx       uint32  // 4 — flat index of receiver extraction node
	SpeedOfSound float32 // 4 m/s
	Dt           float32 // 4 seconds
	Ds           float32 // 4 metres — grid spacing h; server computes λ = (c·Δt/h)²
	// ResultSHM is pre-created by the caller; the server writes Steps float32
	// pressure samples into it, then the caller reads and removes the segment.
	ResultSHM ShmName // 64 bytes
} // 96 bytes

// -- AllocBVH (0x2001) ----------------------------------------------------
// Request payload: 72 bytes.
// The shared memory segment contains all BVH nodes followed by all triangles
// (both using the same C struct layout as in gpu/raytrace/bvh.h).

type allocBVHReq struct {
	NodeCount uint32  // 4
	TriCount  uint32  // 4
	ShmName   ShmName // 64 bytes
} // 72 bytes

// Response payload: 8 bytes.
type allocBVHResp struct {
	Handle uint64
}

// -- FreeBVH (0x2002) -----------------------------------------------------

type freeBVHReq struct {
	Handle uint64
}

// -- TraceRays (0x2003) ---------------------------------------------------
// Request payload: 140 bytes.
// RaysSHM contains RayCount Ray structs (see gpu/raytrace/bvh.h).
// HitsSHM is pre-created by the caller; the server writes RayCount HitRecord
// structs into it.

type traceRaysReq struct {
	Handle   uint64  // 8
	RayCount uint32  // 4
	RaysSHM  ShmName // 64 bytes — caller writes, server reads
	HitsSHM  ShmName // 64 bytes — server writes, caller reads
} // 140 bytes
