package worker

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
)

// GridHandle is an opaque server-side handle to an allocated FDTD grid.
// Returned by AllocGrid; passed to UploadFields and RunFDTD.
type GridHandle uint64

// FDTDParams configures one RunFDTD call.
type FDTDParams struct {
	Steps        uint32  // number of FDTD timesteps to advance
	SrcIdx       uint32  // flat grid index of the source injection point
	RcvIdx       uint32  // flat grid index of the receiver extraction point
	SpeedOfSound float32 // m/s (typically 343.0)
	Dt           float32 // timestep in seconds (must satisfy CFL condition)
	Ds           float32 // grid spacing in metres (h); used to compute λ = (c·Δt/h)²
}

// AllocGrid allocates a Cartesian FDTD grid of dimensions nx × ny × nz on the
// GPU.  The grid persists until FreeGrid is called.  The caller is responsible
// for calling UploadFields before the first RunFDTD.
func (w *Worker) AllocGrid(ctx context.Context, nx, ny, nz int) (GridHandle, error) {
	req := allocGridReq{Nx: uint32(nx), Ny: uint32(ny), Nz: uint32(nz)} //nolint:gosec

	var buf bytes.Buffer

	err := binary.Write(&buf, byteOrder, req)
	if err != nil {
		return 0, fmt.Errorf("encode AllocGrid: %w", err)
	}

	resp, err := w.do(ctx, MsgAllocGrid, buf.Bytes())
	if err != nil {
		return 0, fmt.Errorf("AllocGrid: %w", err)
	}

	var r allocGridResp

	err = binary.Read(bytes.NewReader(resp), byteOrder, &r)
	if err != nil {
		return 0, fmt.Errorf("decode AllocGrid response: %w", err)
	}

	return GridHandle(r.Handle), nil
}

// FreeGrid releases the GPU-side grid allocation.
func (w *Worker) FreeGrid(ctx context.Context, h GridHandle) error {
	req := freeGridReq{Handle: uint64(h)}

	var buf bytes.Buffer

	err := binary.Write(&buf, byteOrder, req)
	if err != nil {
		return fmt.Errorf("encode FreeGrid: %w", err)
	}

	_, err = w.do(ctx, MsgFreeGrid, buf.Bytes())

	return err
}

// UploadFields copies the initial pressure fields for grid h to GPU VRAM.
// cur and prev must each have length nx·ny·nz (float32 values, host byte order).
// After this call the fields persist on the GPU until the next UploadFields
// or FreeGrid.
func (w *Worker) UploadFields(ctx context.Context, h GridHandle, cur, prev []float32) error {
	if len(cur) != len(prev) {
		return fmt.Errorf("UploadFields: cur len %d != prev len %d", len(cur), len(prev))
	}

	// Pack both arrays into a single shared memory segment: [cur..., prev...].
	total := len(cur) + len(prev)

	shmName, shmData, err := createShm(total * 4)
	if err != nil {
		return fmt.Errorf("UploadFields: create shm: %w", err)
	}
	defer closeShm(shmName, shmData) //nolint:errcheck

	copy(shmData[:len(cur)*4], float32sToBytes(cur))
	copy(shmData[len(cur)*4:], float32sToBytes(prev))

	req := uploadFieldsReq{Handle: uint64(h), ShmName: shmNameOf(shmName)}

	var buf bytes.Buffer

	err = binary.Write(&buf, byteOrder, req)
	if err != nil {
		return fmt.Errorf("encode UploadFields: %w", err)
	}

	_, err = w.do(ctx, MsgUploadFields, buf.Bytes())

	return err
}

// RunFDTD advances the simulation for p.Steps timesteps on the GPU and returns
// the receiver pressure time-series (len = p.Steps, float32).
// The field arrays stay on the GPU after this call; subsequent RunFDTD calls
// continue from where the previous left off.
func (w *Worker) RunFDTD(ctx context.Context, h GridHandle, p FDTDParams) ([]float32, error) {
	// Pre-allocate the result shm (server writes into it).
	resultName, resultData, err := createShm(int(p.Steps) * 4)
	if err != nil {
		return nil, fmt.Errorf("RunFDTD: create result shm: %w", err)
	}
	defer closeShm(resultName, resultData) //nolint:errcheck

	req := runFDTDReq{
		Handle:       uint64(h),
		Steps:        p.Steps,
		SrcIdx:       p.SrcIdx,
		RcvIdx:       p.RcvIdx,
		SpeedOfSound: p.SpeedOfSound,
		Dt:           p.Dt,
		Ds:           p.Ds,
		ResultSHM:    shmNameOf(resultName),
	}

	var buf bytes.Buffer

	err = binary.Write(&buf, byteOrder, req)
	if err != nil {
		return nil, fmt.Errorf("encode RunFDTD: %w", err)
	}

	_, err = w.do(ctx, MsgRunFDTD, buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("RunFDTD: %w", err)
	}

	// Copy result out of shared memory (mapping may be released on return).
	samples := make([]float32, p.Steps)
	copy(float32sToBytes(samples), resultData[:p.Steps*4])

	return samples, nil
}
