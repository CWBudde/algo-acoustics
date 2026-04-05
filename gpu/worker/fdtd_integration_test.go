package worker

// Integration test for the full FDTD path: AllocGrid → UploadFields → RunFDTD → FreeGrid.
//
// Requires a compiled GPU server binary.  Set ALGO_GPU_SERVER to its path:
//
//	ALGO_GPU_SERVER=./gpu/server/algo-acoustics-gpu go test ./gpu/worker/ -run TestFDTDIntegration -v
//
// The test is skipped automatically when the env var is absent, so `go test ./...` is safe.

import (
	"context"
	"math"
	"os"
	"testing"
)

// TestFDTDIntegration exercises the full FDTD round-trip against a real GPU server.
//
// Grid: 20×20×20 nodes, h = 0.025 m.
// Source: unit impulse at centre node (10,10,10) via UploadFields.
// Receiver: (10,10,15) — 5 nodes (0.125 m) away in the Z direction.
// Propagation time: 0.125 / 343 ≈ 364 µs → ~9 steps at dt = 40 µs.
// After 20 steps the receiver must have a non-zero response.
func TestFDTDIntegration(t *testing.T) {
	bin := os.Getenv("ALGO_GPU_SERVER")
	if bin == "" {
		t.Skip("ALGO_GPU_SERVER not set — skipping GPU integration test")
	}

	ctx := context.Background()
	w, err := Start(ctx, bin)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Close() //nolint:errcheck

	if err := w.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	const (
		nx, ny, nz = 20, 20, 20
		ds         = float32(0.025) // grid spacing (m)
		c          = float32(343.0) // speed of sound (m/s)
		dt         = float32(40e-6) // timestep (s); CFL = c·dt/h ≈ 0.549 < 1/√3
		steps      = 50
	)

	// Flat indices in row-major [NX][NY][NZ] layout.
	srcIdx := uint32(10*ny*nz + 10*nz + 10) // centre node
	rcvIdx := uint32(10*ny*nz + 10*nz + 15) // 5 nodes away in Z

	grid, err := w.AllocGrid(ctx, nx, ny, nz)
	if err != nil {
		t.Fatalf("AllocGrid: %v", err)
	}
	defer func() {
		if err := w.FreeGrid(ctx, grid); err != nil {
			t.Logf("FreeGrid: %v", err)
		}
	}()

	// Initial field: unit impulse at the source node; pPrev = 0 everywhere.
	N := nx * ny * nz
	pCur := make([]float32, N)
	pPrev := make([]float32, N)
	pCur[srcIdx] = 1.0

	if err := w.UploadFields(ctx, grid, pCur, pPrev); err != nil {
		t.Fatalf("UploadFields: %v", err)
	}

	samples, err := w.RunFDTD(ctx, grid, FDTDParams{
		Steps:        steps,
		SrcIdx:       srcIdx,
		RcvIdx:       rcvIdx,
		SpeedOfSound: c,
		Dt:           dt,
		Ds:           ds,
	})
	if err != nil {
		t.Fatalf("RunFDTD: %v", err)
	}

	if len(samples) != steps {
		t.Fatalf("got %d samples, want %d", len(samples), steps)
	}

	// No NaN or Inf anywhere.
	for i, s := range samples {
		if math.IsNaN(float64(s)) || math.IsInf(float64(s), 0) {
			t.Errorf("sample[%d] = %v (NaN or Inf)", i, s)
		}
	}

	// After step 5 the impulse must have reached the receiver.
	var maxAbs float32
	for _, s := range samples[5:] {
		if s < 0 {
			s = -s
		}
		if s > maxAbs {
			maxAbs = s
		}
	}
	if maxAbs < 1e-6 {
		t.Error("receiver time series is all-zero after step 5 — impulse did not propagate")
	}
	t.Logf("peak receiver amplitude (steps 5–%d): %e", steps, maxAbs)
}
