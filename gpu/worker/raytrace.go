package worker

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"unsafe"
)

// BVHHandle is an opaque server-side handle to an uploaded BVH + triangle
// mesh.  Returned by AllocBVH; passed to TraceRays.
type BVHHandle uint64

// BVHNode matches the layout of gpu/raytrace/bvh.h BVHNode (24 bytes):
//
//	AABB  box          — lo float3 (12 B) + hi float3 (12 B) = 24 B
//	int32 leftOrFirst  — child index (internal) or first triangle (leaf)
//	int32 count        — 0 = internal, >0 = leaf
//
// Total: 24 + 4 + 4 = 32 bytes per node.
type BVHNode struct {
	LoX, LoY, LoZ float32
	HiX, HiY, HiZ float32
	LeftOrFirst   int32
	Count         int32
}

// Triangle matches gpu/raytrace/bvh.h Triangle (52 bytes):
//
//	float3 v0, v1, v2  (36 B)
//	int32  id          (4 B)
//
// Total: 40 bytes per triangle.
type Triangle struct {
	V0X, V0Y, V0Z float32
	V1X, V1Y, V1Z float32
	V2X, V2Y, V2Z float32
	ID            int32
}

// Ray matches gpu/raytrace/bvh.h Ray (32 bytes):
//
//	float3 origin (12 B)
//	float3 dir    (12 B)
//	float  tmin   (4 B)
//	float  tmax   (4 B)
type Ray struct {
	OriginX, OriginY, OriginZ float32
	DirX, DirY, DirZ          float32
	Tmin, Tmax                float32
}

// HitRecord matches gpu/raytrace/bvh.h HitRecord (8 bytes):
//
//	float t      — hit distance; FLT_MAX if no hit
//	int32 tri_id — triangle ID; -1 if no hit
type HitRecord struct {
	T     float32
	TriID int32
}

// AllocBVH uploads the BVH and triangle array to the GPU.  The mesh persists
// until FreeBVH is called.  BVH node layout must match BVHNode above.
func (w *Worker) AllocBVH(ctx context.Context, nodes []BVHNode, tris []Triangle) (BVHHandle, error) {
	// Pack nodes followed by triangles into a single shm segment.
	nodeBytes := len(nodes) * int(unsafe.Sizeof(BVHNode{}))
	triBytes := len(tris) * int(unsafe.Sizeof(Triangle{}))
	total := nodeBytes + triBytes

	shmName, shmData, err := createShm(total)
	if err != nil {
		return 0, fmt.Errorf("AllocBVH: create shm: %w", err)
	}
	defer closeShm(shmName, shmData) //nolint:errcheck

	// Write nodes.
	if len(nodes) > 0 {
		nb := unsafe.Slice((*byte)(unsafe.Pointer(&nodes[0])), nodeBytes)
		copy(shmData[:nodeBytes], nb)
	}
	// Write triangles.
	if len(tris) > 0 {
		tb := unsafe.Slice((*byte)(unsafe.Pointer(&tris[0])), triBytes)
		copy(shmData[nodeBytes:], tb)
	}

	req := allocBVHReq{
		NodeCount: uint32(len(nodes)), //nolint:gosec
		TriCount:  uint32(len(tris)),  //nolint:gosec
		ShmName:   shmNameOf(shmName),
	}

	var buf bytes.Buffer

	err = binary.Write(&buf, byteOrder, req)
	if err != nil {
		return 0, fmt.Errorf("encode AllocBVH: %w", err)
	}

	resp, err := w.do(ctx, MsgAllocBVH, buf.Bytes())
	if err != nil {
		return 0, fmt.Errorf("AllocBVH: %w", err)
	}

	var r allocBVHResp

	err = binary.Read(bytes.NewReader(resp), byteOrder, &r)
	if err != nil {
		return 0, fmt.Errorf("decode AllocBVH response: %w", err)
	}

	return BVHHandle(r.Handle), nil
}

// FreeBVH releases the GPU-side BVH allocation.
func (w *Worker) FreeBVH(ctx context.Context, h BVHHandle) error {
	req := freeBVHReq{Handle: uint64(h)}

	var buf bytes.Buffer

	err := binary.Write(&buf, byteOrder, req)
	if err != nil {
		return fmt.Errorf("encode FreeBVH: %w", err)
	}

	_, err = w.do(ctx, MsgFreeBVH, buf.Bytes())

	return err
}

// TraceRays submits a ray batch against BVH h and returns one HitRecord per
// ray.  Rays and hits are transferred via POSIX shared memory; the BVH stays
// on the GPU.
func (w *Worker) TraceRays(ctx context.Context, h BVHHandle, rays []Ray) ([]HitRecord, error) {
	if len(rays) == 0 {
		return nil, nil
	}

	// Shared memory for rays (input).
	rayBytes := len(rays) * int(unsafe.Sizeof(Ray{}))

	raysName, raysData, err := createShm(rayBytes)
	if err != nil {
		return nil, fmt.Errorf("TraceRays: create rays shm: %w", err)
	}
	defer closeShm(raysName, raysData) //nolint:errcheck

	rb := unsafe.Slice((*byte)(unsafe.Pointer(&rays[0])), rayBytes)
	copy(raysData, rb)

	// Shared memory for hits (output, pre-allocated by caller).
	hitBytes := len(rays) * int(unsafe.Sizeof(HitRecord{}))

	hitsName, hitsData, err := createShm(hitBytes)
	if err != nil {
		return nil, fmt.Errorf("TraceRays: create hits shm: %w", err)
	}
	defer closeShm(hitsName, hitsData) //nolint:errcheck

	req := traceRaysReq{
		Handle:   uint64(h),
		RayCount: uint32(len(rays)), //nolint:gosec
		RaysSHM:  shmNameOf(raysName),
		HitsSHM:  shmNameOf(hitsName),
	}

	var buf bytes.Buffer

	err = binary.Write(&buf, byteOrder, req)
	if err != nil {
		return nil, fmt.Errorf("encode TraceRays: %w", err)
	}

	_, err = w.do(ctx, MsgTraceRays, buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("TraceRays: %w", err)
	}

	// Copy hits out (mapping released on return via defer).
	hits := make([]HitRecord, len(rays))
	hb := unsafe.Slice((*byte)(unsafe.Pointer(&hits[0])), hitBytes)
	copy(hb, hitsData[:hitBytes])

	return hits, nil
}
