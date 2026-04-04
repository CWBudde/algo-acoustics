package algoacoustics

// Phase 14.1 — CPU Profiling Baseline
//
// Benchmarks covering the three hot paths in the simulation pipeline:
//   - ISM (image-source method, early specular reflections)
//   - RayTrace (Monte Carlo, late diffuse field)
//   - PDE (Helmholtz shoebox sweep, low-frequency modal content)
//   - Full hybrid pipeline (ISM + RayTrace)
//
// IBM FDTD benchmarks are in pde/ibm_bench_test.go and are included
// via the justfile's bench and profile-* recipes.
//
// Run with profiling:
//
//	just profile-cpu
//	just profile-mem
//	just profile-block
//
// Run GOMAXPROCS scaling sweep:
//
//	just gomaxprocs-sweep
import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/pde"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

// benchScene returns a canonical shoebox scene suitable for all pipeline benchmarks.
// Defined inline to avoid I/O overhead inside benchmark loops.
func benchScene() *scene.Scene {
	mat := scene.Material{
		Name:             "plaster",
		AbsorptionByBand: []float64{0.10, 0.10, 0.15, 0.20, 0.20, 0.25},
		ScatteringByBand: []float64{0.02, 0.02, 0.02, 0.03, 0.03, 0.04},
	}

	bandSpec := acoustics.Octave6

	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width: 6.0, Depth: 4.5, Height: 2.8,
				WallMaterials: [6]string{"plaster", "plaster", "plaster", "plaster", "plaster", "plaster"},
			},
		},
		Materials: map[string]scene.Material{"plaster": mat},
		Sources: []scene.Source{
			{
				Position:    geometry.Vec3{X: 1.5, Y: 2.0, Z: 1.2},
				Orientation: geometry.Quaternion{W: 1},
			},
		},
		Receivers: []scene.Receiver{
			{
				Position:    geometry.Vec3{X: 4.0, Y: 2.0, Z: 1.2},
				Orientation: geometry.Quaternion{W: 1},
				Type:        scene.ReceiverOmni,
			},
		},
		BandSpec:   bandSpec,
		SampleRate: 48000,
	}
}

const (
	benchDurationSec    = 1.5
	benchMaxOrder       = 3
	benchReceiverRadius = 0.25
)

// BenchmarkISM measures the image-source solver (early specular reflections).
// This is O(6^order) in reflection count and fully sequential.
func BenchmarkISM(b *testing.B) {
	sc := benchScene()

	cfg := ism.ISMConfig{
		MaxOrder:     benchMaxOrder,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     sc.BandSpec,
	}

	solver := ism.ISMSolver{}

	// Warm-up: count events so we can log problem size.
	events, err := solver.Solve(sc, cfg)
	if err != nil {
		b.Fatalf("ISM solve: %v", err)
	}

	b.Logf("ISM order=%d, events=%d, room=%.1fx%.1fx%.1f m",
		benchMaxOrder, len(events),
		sc.Room.Shoebox.Width, sc.Room.Shoebox.Depth, sc.Room.Shoebox.Height)

	b.ResetTimer()

	for b.Loop() {
		_, err = solver.Solve(sc, cfg)
		if err != nil {
			b.Fatalf("ISM solve: %v", err)
		}
	}
}

// BenchmarkRayTrace_4K benchmarks the Monte Carlo ray tracer at 4 096 rays.
func BenchmarkRayTrace_4K(b *testing.B) {
	benchRayTrace(b, 4096)
}

// BenchmarkRayTrace_16K benchmarks the Monte Carlo ray tracer at 16 384 rays.
func BenchmarkRayTrace_16K(b *testing.B) {
	benchRayTrace(b, 16384)
}

// BenchmarkRayTrace_64K benchmarks the Monte Carlo ray tracer at 65 536 rays.
func BenchmarkRayTrace_64K(b *testing.B) {
	benchRayTrace(b, 65536)
}

func benchRayTrace(b *testing.B, numRays int) {
	b.Helper()

	sc := benchScene()

	maxBounces := max(benchMaxOrder*2, 1)

	tracer := raytrace.RayTracer{
		Config: raytrace.LaunchConfig{
			NumRays:        numRays,
			MaxBounces:     maxBounces,
			MaxTimeSeconds: benchDurationSec,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:              sc,
		ReceiverRadius:     benchReceiverRadius,
		BinDurationSeconds: 0.01,
	}

	b.Logf("RayTrace rays=%d, maxBounces=%d, duration=%.1fs, room=%.1fx%.1fx%.1f m",
		numRays, maxBounces, benchDurationSec,
		sc.Room.Shoebox.Width, sc.Room.Shoebox.Depth, sc.Room.Shoebox.Height)

	b.ResetTimer()

	for b.Loop() {
		_, err := tracer.Trace()
		if err != nil {
			b.Fatalf("raytrace: %v", err)
		}
	}
}

// BenchmarkPDEShoebox measures the Helmholtz shoebox sweep (frequency-domain
// Poisson solves for low-frequency modal content).
func BenchmarkPDEShoebox(b *testing.B) {
	sc := benchScene()

	sweepCfg := pde.SweepConfig{
		FreqMin:           20,
		FreqMax:           300,
		NumPoints:         32,
		BoundaryCondition: "neumann",
	}

	src := sc.Sources[0].Position
	rcv := sc.Receivers[0].Position
	room := sc.Room.Shoebox

	b.Logf("PDE shoebox sweep: %.0f–%.0f Hz, %d points, room=%.1fx%.1fx%.1f m",
		sweepCfg.FreqMin, sweepCfg.FreqMax, sweepCfg.NumPoints,
		room.Width, room.Depth, room.Height)

	b.ResetTimer()

	for b.Loop() {
		_, err := pde.SweepShoebox(room, src, rcv, sweepCfg)
		if err != nil {
			b.Fatalf("PDE sweep: %v", err)
		}
	}
}

// BenchmarkHybridPipeline_4K benchmarks the complete hybrid pipeline
// (ISM order 3 + 4096 rays) at the standard shoebox scene.
// This is the primary end-to-end benchmark for Amdahl fraction analysis.
func BenchmarkHybridPipeline_4K(b *testing.B) {
	benchHybridPipeline(b, 4096)
}

// BenchmarkHybridPipeline_64K benchmarks the hybrid pipeline with 64K rays,
// representing a production-quality render.
func BenchmarkHybridPipeline_64K(b *testing.B) {
	benchHybridPipeline(b, 65536)
}

func benchHybridPipeline(b *testing.B, numRays int) {
	b.Helper()

	sc := benchScene()

	renderCfg := ir.RenderConfig{
		SampleRate:      sc.SampleRate,
		DurationSeconds: benchDurationSec,
		BandSpec:        sc.BandSpec,
	}

	ismCfg := ism.ISMConfig{
		MaxOrder:     benchMaxOrder,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     sc.BandSpec,
	}

	maxBounces := max(benchMaxOrder*2, 1)

	tracer := raytrace.RayTracer{
		Config: raytrace.LaunchConfig{
			NumRays:        numRays,
			MaxBounces:     maxBounces,
			MaxTimeSeconds: benchDurationSec,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:              sc,
		ReceiverRadius:     benchReceiverRadius,
		BinDurationSeconds: 0.01,
	}

	hybridCfg := hybrid.HybridConfig{
		CrossoverTimeSeconds: 0.25,
		CrossoverMode:        hybrid.TimeBased,
		SmoothenCrossover:    true,
	}

	solver := ism.ISMSolver{}

	// Warm-up: count events to log problem size.
	earlyEvents, err := solver.Solve(sc, ismCfg)
	if err != nil {
		b.Fatalf("ISM warm-up: %v", err)
	}

	b.Logf("HybridPipeline: ISM order=%d (%d events), raytrace rays=%d, duration=%.1fs, sampleRate=%d Hz",
		benchMaxOrder, len(earlyEvents), numRays, benchDurationSec, sc.SampleRate)

	b.ResetTimer()

	for b.Loop() {
		earlyEvents, err = solver.Solve(sc, ismCfg)
		if err != nil {
			b.Fatalf("ISM solve: %v", err)
		}

		earlyBuf, err := ir.RenderMono(earlyEvents, renderCfg)
		if err != nil {
			b.Fatalf("render early: %v", err)
		}

		lateHist, err := tracer.Trace()
		if err != nil {
			b.Fatalf("raytrace: %v", err)
		}

		lateBuf := hybrid.HistogramToBuffer(lateHist, sc.SampleRate)
		lateBuf = hybrid.AlignLateTail(lateBuf, earlyEvents, hybridCfg)

		_ = hybrid.CombineBuffers(earlyBuf, lateBuf, hybridCfg)
	}
}
