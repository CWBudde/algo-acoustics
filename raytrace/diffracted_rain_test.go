package raytrace

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestDAPDFRainPerBandCylinderAndDelay(t *testing.T) {
	edge := testDAPDFEdge(geometry.Vec3{Z: -1}, geometry.Vec3{Z: 1})
	index := testDiffractionIndex([]geometry.DiffractionEdge{edge})
	ray := geometry.NewRay(geometry.Vec3{X: -2, Y: 1}, geometry.Vec3{X: 1})
	receiver := SphereReceiver{Center: geometry.Vec3{X: 1, Y: 2}, Radius: 0.25}
	config := LaunchConfig{
		SpeedOfSound:        343,
		MaxTimeSeconds:      2,
		DiffractionMode:     DiffractionDAPDFRain,
		MaxDiffractionDepth: 1,
	}
	state := RayState{Ray: ray, Energy: []float64{1, 1}, DelaySeconds: 0.25}

	deposits := dapdfRainForSegment(state, ray, 4, index, nil, &receiver, nil, config, []float64{100, 10000}, 1)
	if len(deposits) != 1 {
		t.Fatalf("dapdfRainForSegment() returned %d deposits, want one low-frequency deposit: %#v", len(deposits), deposits)
	}

	if deposits[0].BandIndex != 0 {
		t.Fatalf("deposit band = %d, want 0", deposits[0].BandIndex)
	}

	if deposits[0].ArrivalTime <= state.DelaySeconds {
		t.Fatalf("arrival = %g, want later than delay %g", deposits[0].ArrivalTime, state.DelaySeconds)
	}
}

func TestDAPDFRainUsesNearestCylinderPerBand(t *testing.T) {
	edges := []geometry.DiffractionEdge{
		testDAPDFEdge(geometry.Vec3{Z: -1}, geometry.Vec3{Z: 1}),
		testDAPDFEdge(geometry.Vec3{X: 1, Z: -1}, geometry.Vec3{X: 1, Z: 1}),
	}
	index := testDiffractionIndex(edges)
	ray := geometry.NewRay(geometry.Vec3{X: -2, Y: 0.1}, geometry.Vec3{X: 1})
	receiver := SphereReceiver{Center: geometry.Vec3{X: 3, Y: 1}, Radius: 0.25}
	config := LaunchConfig{
		SpeedOfSound:        343,
		MaxTimeSeconds:      2,
		DiffractionMode:     DiffractionDAPDFRain,
		MaxDiffractionDepth: 1,
	}
	state := RayState{Ray: ray, Energy: []float64{1}}

	deposits := dapdfRainForSegment(state, ray, 5, index, nil, &receiver, nil, config, []float64{1000}, 1)
	if len(deposits) != 1 {
		t.Fatalf("dapdfRainForSegment() returned %d deposits, want one from the nearest cylinder: %#v", len(deposits), deposits)
	}
}

func TestDAPDFRainRecursiveCycleSuppression(t *testing.T) {
	edges := []geometry.DiffractionEdge{
		testDAPDFEdge(geometry.Vec3{Z: -1}, geometry.Vec3{Z: 1}),
		testDAPDFEdge(geometry.Vec3{X: 1, Z: -1}, geometry.Vec3{X: 1, Z: 1}),
	}
	index := testDiffractionIndex(edges)
	receiver := SphereReceiver{Center: geometry.Vec3{X: 2, Y: 0.5}, Radius: 0.25}
	config := LaunchConfig{
		SpeedOfSound:        343,
		MaxTimeSeconds:      2,
		DiffractionMode:     DiffractionDAPDFRain,
		MaxDiffractionDepth: 2,
	}

	deposits := dapdfForward(0, geometry.Vec3Zero, edges[0].Direction, geometry.Vec3{X: 1}, 0.1, 0, 1000, 1, 1, 0, 1, map[int]bool{0: true}, index, nil, &receiver, nil, config, 1e-6)
	if len(deposits) == 0 {
		t.Fatal("dapdfForward() returned no direct or recursively forwarded deposits")
	}

	if len(deposits) > 2 {
		t.Fatalf("dapdfForward() returned %d deposits, want at most one per visited edge", len(deposits))
	}
}

func TestTracePathsRejectsDAPDF(t *testing.T) {
	tracer := newTracePathsRayTracer()
	tracer.Config.DiffractionMode = DiffractionDAPDFRain

	_, err := tracer.TracePaths()
	if err == nil || !strings.Contains(err.Error(), "does not support DAPDF") {
		t.Fatalf("TracePaths() error = %v, want explicit DAPDF unsupported error", err)
	}
}

func TestLegacyDiffractionConfigurationSelectsKeller(t *testing.T) {
	config := LaunchConfig{DiffractionAngularThreshold: 0.3, DiffractionConeSamples: 8}
	if got := config.effectiveDiffractionMode(); got != DiffractionKellerCone {
		t.Fatalf("effectiveDiffractionMode() = %v, want Keller", got)
	}
}

func BenchmarkDiffractionKellerVsDAPDF(b *testing.B) {
	edge := testDAPDFEdge(geometry.Vec3{Z: -1}, geometry.Vec3{Z: 1})
	index := testDiffractionIndex([]geometry.DiffractionEdge{edge})
	ray := geometry.NewRay(geometry.Vec3{X: -2, Y: 0.1}, geometry.Vec3{X: 1})
	state := RayState{Ray: ray, Energy: []float64{1}}
	receiver := SphereReceiver{Center: geometry.Vec3{X: 1, Y: 1}, Radius: 0.25}
	frequencies := []float64{1000}

	b.Run("Keller", func(b *testing.B) {
		b.ReportAllocs()

		rng := rand.New(rand.NewSource(1)) // #nosec G404 -- deterministic benchmark.

		config := LaunchConfig{DiffractionMode: DiffractionKellerCone, DiffractionAngularThreshold: 0.3, DiffractionConeSamples: 8}
		for range b.N {
			_ = spawnDiffractionBranches(state, ray, ray.At(4), 4, nil, index, config, rng, 1, frequencies)
		}
	})

	b.Run("DAPDF", func(b *testing.B) {
		b.ReportAllocs()

		config := LaunchConfig{DiffractionMode: DiffractionDAPDFRain, MaxDiffractionDepth: 1, SpeedOfSound: 343, MaxTimeSeconds: 1}
		for range b.N {
			_ = dapdfRainForSegment(state, ray, 4, index, nil, &receiver, nil, config, frequencies, 1)
		}
	})
}

func testDAPDFEdge(start, end geometry.Vec3) geometry.DiffractionEdge {
	return geometry.DiffractionEdge{
		Start:     start,
		End:       end,
		Direction: end.Sub(start).Normalize(),
		Length:    start.Distance(end),
	}
}

func testDiffractionIndex(edges []geometry.DiffractionEdge) *DiffractionEdgeIndex {
	index := &DiffractionEdgeIndex{
		CellSize: 10,
		Bounds: geometry.Box{
			Min: geometry.Vec3{X: -10, Y: -10, Z: -10},
			Max: geometry.Vec3{X: 10, Y: 10, Z: 10},
		},
		cells: make(map[diffractionCellKey][]diffractionEdgeRef),
		edges: append([]geometry.DiffractionEdge(nil), edges...),
	}
	for id, edge := range edges {
		index.insert(id, edge)
	}

	return index
}
