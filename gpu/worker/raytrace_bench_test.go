package worker

// End-to-end benchmark: ray-BVH traversal via GPU server vs CPU-only.
//
// Measures the complete round-trip: Go → Unix socket → GPU server → CUDA
// kernel → result download → Go, compared against the CPU geometry.BVH.
//
// Requires a compiled GPU server binary.  Set ALGO_GPU_SERVER to its path:
//
//	ALGO_GPU_SERVER=./gpu/server/algo-acoustics-gpu go test ./gpu/worker/ -bench BenchmarkRayTrace -v
//
// Benchmarks are skipped automatically when the env var is absent.

import (
	"context"
	"math"
	"math/rand/v2"
	"os"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// shoeboxMesh builds a closed 12-triangle mesh for a shoebox room
// of dimensions w × d × h (metres), centred at (w/2, d/2, h/2).
func shoeboxMesh(w, d, h float64) (*geometry.Mesh, []BVHNode, []Triangle) {
	// 8 corners of the box.
	v := [8]geometry.Vec3{
		{X: 0, Y: 0, Z: 0}, // 0: floor-front-left
		{X: w, Y: 0, Z: 0}, // 1: floor-front-right
		{X: w, Y: d, Z: 0}, // 2: floor-back-right
		{X: 0, Y: d, Z: 0}, // 3: floor-back-left
		{X: 0, Y: 0, Z: h}, // 4: ceil-front-left
		{X: w, Y: 0, Z: h}, // 5: ceil-front-right
		{X: w, Y: d, Z: h}, // 6: ceil-back-right
		{X: 0, Y: d, Z: h}, // 7: ceil-back-left
	}

	// 6 faces × 2 triangles = 12 triangles.
	faces := [][4]int{
		{0, 1, 2, 3}, // floor (Z=0)
		{4, 7, 6, 5}, // ceiling (Z=h)
		{0, 4, 5, 1}, // front (Y=0)
		{2, 6, 7, 3}, // back (Y=d)
		{0, 3, 7, 4}, // left (X=0)
		{1, 5, 6, 2}, // right (X=w)
	}

	tris := make([]geometry.Triangle, 0, 12)
	for _, f := range faces {
		tris = append(
			tris,
			geometry.Triangle{V0: v[f[0]], V1: v[f[1]], V2: v[f[2]]},
			geometry.Triangle{V0: v[f[0]], V1: v[f[2]], V2: v[f[3]]},
		)
	}

	mesh := &geometry.Mesh{Triangles: tris}

	// Build GPU-side flat BVH and triangle arrays.
	gpuTris := make([]Triangle, len(tris))
	for i, t := range tris {
		gpuTris[i] = Triangle{
			V0X: float32(t.V0.X), V0Y: float32(t.V0.Y), V0Z: float32(t.V0.Z),
			V1X: float32(t.V1.X), V1Y: float32(t.V1.Y), V1Z: float32(t.V1.Z),
			V2X: float32(t.V2.X), V2Y: float32(t.V2.Y), V2Z: float32(t.V2.Z),
			ID: int32(i),
		}
	}

	// Single leaf node covering all triangles (simple but sufficient for
	// measuring traversal + intersection throughput).
	lo := geometry.Vec3{X: 0, Y: 0, Z: 0}
	hi := geometry.Vec3{X: w, Y: d, Z: h}
	gpuNodes := []BVHNode{
		{
			LoX: float32(lo.X), LoY: float32(lo.Y), LoZ: float32(lo.Z),
			HiX: float32(hi.X), HiY: float32(hi.Y), HiZ: float32(hi.Z),
			LeftOrFirst: 0,
			Count:       int32(len(gpuTris)),
		},
	}

	return mesh, gpuNodes, gpuTris
}

// randomRaysInBox generates n random rays originating inside the box
// with uniformly distributed directions.
func randomRaysInBox(n int, w, d, h float64) ([]Ray, []geometry.Ray) {
	rng := rand.New(rand.NewPCG(42, 0))

	gpuRays := make([]Ray, n)
	cpuRays := make([]geometry.Ray, n)

	for i := range n {
		ox := rng.Float64()*w*0.8 + w*0.1
		oy := rng.Float64()*d*0.8 + d*0.1
		oz := rng.Float64()*h*0.8 + h*0.1

		// Random direction on unit sphere (rejection method).
		var dx, dy, dz float64
		for {
			dx = rng.Float64()*2 - 1
			dy = rng.Float64()*2 - 1
			dz = rng.Float64()*2 - 1

			l := math.Sqrt(dx*dx + dy*dy + dz*dz)
			if l > 0.01 && l <= 1.0 {
				dx /= l
				dy /= l
				dz /= l

				break
			}
		}

		gpuRays[i] = Ray{
			OriginX: float32(ox), OriginY: float32(oy), OriginZ: float32(oz),
			DirX: float32(dx), DirY: float32(dy), DirZ: float32(dz),
			Tmin: 0, Tmax: 1e6,
		}
		cpuRays[i] = geometry.Ray{
			Origin:    geometry.Vec3{X: ox, Y: oy, Z: oz},
			Direction: geometry.Vec3{X: dx, Y: dy, Z: dz},
		}
	}

	return gpuRays, cpuRays
}

type rayBenchConfig struct {
	name  string
	nRays int
}

var rayBenchConfigs = []rayBenchConfig{
	{"1K_rays", 1_000},
	{"10K_rays", 10_000},
	{"100K_rays", 100_000},
}

// BenchmarkRayTraceEndToEndGPU benchmarks the full GPU round-trip for
// BVH ray tracing: Go → socket → GPU server → kernel → result download → Go.
func BenchmarkRayTraceEndToEndGPU(b *testing.B) {
	bin := os.Getenv("ALGO_GPU_SERVER")
	if bin == "" {
		b.Skip("ALGO_GPU_SERVER not set — skipping GPU benchmark")
	}

	ctx := context.Background()
	_, gpuNodes, gpuTris := shoeboxMesh(8, 6, 3)

	for _, cfg := range rayBenchConfigs {
		b.Run(cfg.name, func(b *testing.B) {
			w, err := Start(ctx, bin)
			if err != nil {
				b.Fatalf("Start: %v", err)
			}

			defer w.Close() //nolint:contextcheck // Close uses internal timeout context

			bvh, err := w.AllocBVH(ctx, gpuNodes, gpuTris)
			if err != nil {
				b.Fatalf("AllocBVH: %v", err)
			}

			defer func() {
				_ = w.FreeBVH(ctx, bvh)
			}()

			gpuRays, _ := randomRaysInBox(cfg.nRays, 8, 6, 3)

			b.ResetTimer()

			for b.Loop() {
				_, err = w.TraceRays(ctx, bvh, gpuRays)
				if err != nil {
					b.Fatalf("TraceRays: %v", err)
				}
			}

			b.ReportMetric(float64(cfg.nRays)/1e6, "Mrays/op")
		})
	}
}

// BenchmarkRayTraceEndToEndCPU benchmarks CPU-only BVH traversal for the
// same scene and ray counts, providing a direct comparison baseline.
func BenchmarkRayTraceEndToEndCPU(b *testing.B) {
	mesh, _, _ := shoeboxMesh(8, 6, 3)
	cpuBVH := geometry.BuildBVH(mesh)

	for _, cfg := range rayBenchConfigs {
		b.Run(cfg.name, func(b *testing.B) {
			_, cpuRays := randomRaysInBox(cfg.nRays, 8, 6, 3)

			b.ResetTimer()

			for b.Loop() {
				for i := range cfg.nRays {
					cpuBVH.Intersect(cpuRays[i])
				}
			}

			b.ReportMetric(float64(cfg.nRays)/1e6, "Mrays/op")
		})
	}
}
