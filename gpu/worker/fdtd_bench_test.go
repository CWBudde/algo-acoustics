package worker

// End-to-end benchmark: full PDE simulation via GPU server vs CPU-only.
//
// Measures the complete round-trip: Go → Unix socket → GPU server → CUDA
// kernel execution (with CUDA streams + pinned memory) → result download → Go.
//
// Requires a compiled GPU server binary.  Set ALGO_GPU_SERVER to its path:
//
//	ALGO_GPU_SERVER=./gpu/server/algo-acoustics-gpu go test ./gpu/worker/ -bench BenchmarkFDTD -v
//
// Benchmarks are skipped automatically when the env var is absent.

import (
	"context"
	"math"
	"os"
	"testing"
)

// benchConfig defines one benchmark scenario (grid size + step count).
type benchConfig struct {
	name string
	n    int // grid dimension (cube: n×n×n)
	step int // number of FDTD timesteps
}

var benchConfigs = []benchConfig{
	{"32cube_100steps", 32, 100},
	{"64cube_100steps", 64, 100},
	{"128cube_50steps", 128, 50},
}

// BenchmarkFDTDEndToEndGPU benchmarks the full GPU round-trip for various
// grid sizes.  This includes Go→socket→server overhead, CUDA kernel execution,
// and result download through shared memory with pinned host buffers.
func BenchmarkFDTDEndToEndGPU(b *testing.B) {
	bin := os.Getenv("ALGO_GPU_SERVER")
	if bin == "" {
		b.Skip("ALGO_GPU_SERVER not set — skipping GPU benchmark")
	}

	ctx := context.Background()

	for _, cfg := range benchConfigs {
		b.Run(cfg.name, func(b *testing.B) {
			w, err := Start(ctx, bin)
			if err != nil {
				b.Fatalf("Start: %v", err)
			}

			defer w.Close() //nolint:contextcheck // Close uses internal timeout context

			const (
				ds = float32(0.025) // grid spacing (m)
				c  = float32(343.0) // speed of sound (m/s)
				dt = float32(40e-6) // timestep (s)
			)

			nx, ny, nz := cfg.n, cfg.n, cfg.n
			N := nx * ny * nz

			grid, err := w.AllocGrid(ctx, nx, ny, nz)
			if err != nil {
				b.Fatalf("AllocGrid: %v", err)
			}

			defer func() {
				_ = w.FreeGrid(ctx, grid)
			}()

			// Initial field: Gaussian pulse at centre.
			pCur := make([]float32, N)
			pPrev := make([]float32, N)
			cx, cy, cz := float64(nx/2), float64(ny/2), float64(nz/2)

			for ix := range nx {
				for iy := range ny {
					for iz := range nz {
						dx := float64(ix) - cx
						dy := float64(iy) - cy
						dz := float64(iz) - cz
						pCur[ix*ny*nz+iy*nz+iz] = float32(math.Exp(-(dx*dx + dy*dy + dz*dz) / 100))
					}
				}
			}

			srcIdx := uint32(nx/2*ny*nz + ny/2*nz + nz/2)
			rcvIdx := uint32(nx/2*ny*nz + ny/2*nz + nz/4)

			b.ResetTimer()

			for b.Loop() {
				// Upload fields each iteration (measures upload + run + download).
				err = w.UploadFields(ctx, grid, pCur, pPrev)
				if err != nil {
					b.Fatalf("UploadFields: %v", err)
				}

				_, err = w.RunFDTD(ctx, grid, FDTDParams{
					Steps:        uint32(cfg.step),
					SrcIdx:       srcIdx,
					RcvIdx:       rcvIdx,
					SpeedOfSound: c,
					Dt:           dt,
					Ds:           ds,
				})
				if err != nil {
					b.Fatalf("RunFDTD: %v", err)
				}
			}

			b.ReportMetric(float64(N)*float64(cfg.step)/1e6, "Mcells/op")
		})
	}
}

// BenchmarkFDTDEndToEndCPU benchmarks the CPU-only plain Cartesian FDTD for
// the same grid sizes, providing a direct comparison baseline.
func BenchmarkFDTDEndToEndCPU(b *testing.B) {
	for _, cfg := range benchConfigs {
		b.Run(cfg.name, func(b *testing.B) {
			nx, ny, nz := cfg.n, cfg.n, cfg.n
			N := nx * ny * nz

			pCur := make([]float64, N)
			pPrev := make([]float64, N)
			pNext := make([]float64, N)

			cx, cy, cz := float64(nx/2), float64(ny/2), float64(nz/2)

			for ix := range nx {
				for iy := range ny {
					for iz := range nz {
						dx := float64(ix) - cx
						dy := float64(iy) - cy
						dz := float64(iz) - cz
						pCur[ix*ny*nz+iy*nz+iz] = math.Exp(-(dx*dx + dy*dy + dz*dz) / 100)
					}
				}
			}

			copy(pPrev, pCur)

			c := 343.0
			h := 0.025
			dt := 0.95 * h / (c * math.Sqrt(3))
			lambda := (c * dt / h) * (c * dt / h)

			b.ResetTimer()

			for b.Loop() {
				for range cfg.step {
					cpuFDTDStep(pNext, pCur, pPrev, nx, ny, nz, lambda)
					pPrev, pCur, pNext = pCur, pNext, pPrev
				}
			}

			b.ReportMetric(float64(N)*float64(cfg.step)/1e6, "Mcells/op")
		})
	}
}

// cpuFDTDStep is a minimal 7-point Cartesian FDTD step (hardwall boundaries).
// Matches the GPU kernel's physics exactly for apples-to-apples comparison.
func cpuFDTDStep(pNext, pCur, pPrev []float64, nx, ny, nz int, lambda float64) {
	nyNz := ny * nz

	for ix := 1; ix < nx-1; ix++ {
		for iy := 1; iy < ny-1; iy++ {
			for iz := 1; iz < nz-1; iz++ {
				flat := ix*nyNz + iy*nz + iz
				cur := pCur[flat]
				lap := pCur[flat+nyNz] + pCur[flat-nyNz] +
					pCur[flat+nz] + pCur[flat-nz] +
					pCur[flat+1] + pCur[flat-1] -
					6*cur
				pNext[flat] = 2*cur - pPrev[flat] + lambda*lap
			}
		}
	}
}
