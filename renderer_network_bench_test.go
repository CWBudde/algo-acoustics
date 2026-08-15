package algoacoustics

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

// benchmarkFourRoomScene loads the four-room office fixture with the receiver
// two partitions away, which is the scenario the Phase 25.4 target names.
func benchmarkFourRoomScene(b *testing.B) *scene.Scene {
	b.Helper()

	sc, err := scene.LoadSceneFile("examples/scenes/office_floor.json")
	if err != nil {
		b.Fatalf("load office floor fixture: %v", err)
	}

	sc.Receivers[0].Position = geometry.Vec3{X: 10, Y: 2, Z: 1.5}

	return sc
}

func benchmarkNetworkConfig() NetworkRendererConfig {
	return NetworkRendererConfig{
		ISM: ism.ISMConfig{MaxOrder: 3},
		Raytrace: RaytraceEngineConfig{
			Launch:             raytrace.LaunchConfig{NumRays: 16000, MaxBounces: 30},
			ReceiverRadius:     0.5,
			BinDurationSeconds: 0.005,
		},
		BandFloorDB: -90,
	}
}

// BenchmarkNetworkColdRender4Rooms is the reference: a full render with nothing
// prepared. The Phase 25.4 target is explicitly about the warm case, and this
// benchmark exists so the two numbers can be compared honestly.
func BenchmarkNetworkColdRender4Rooms(b *testing.B) {
	sc := benchmarkFourRoomScene(b)
	cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 1.0, BandSpec: sc.BandSpec}

	b.ResetTimer()

	for range b.N {
		// A fresh renderer each iteration, so this really is the cold cost.
		// Reusing one would let the group-response cache carry work across
		// iterations and quietly turn this into a second warm benchmark.
		renderer := NewNetworkRenderer(benchmarkNetworkConfig())

		_, err := renderer.RenderMono(sc, cfg)
		if err != nil {
			b.Fatalf("RenderMono: %v", err)
		}
	}
}

// BenchmarkNetworkPortalStateChange4Rooms measures the mono half of the Phase
// 25.4 target: toggling a portal on a prepared plan and re-rendering.
//
// The plan is prepared outside the timer, so this is the warm case. Each
// iteration toggles the same portal open and then closed again, which keeps the
// configuration cycling rather than drifting to one state. Because the two
// directions differ in group and path count, the reported metrics are averaged
// over the whole run rather than taken from whichever iteration happened to be
// last.
func BenchmarkNetworkPortalStateChange4Rooms(b *testing.B) {
	benchmarkPortalStateChange(b, func(plan *NetworkPlan) error {
		_, err := plan.RenderMono()

		return err
	})
}

// BenchmarkNetworkPortalStateChange4RoomsBinaural is the benchmark the Phase
// 25.4 target is actually about: a portal change through to an updated BRIR.
//
// Binaural rendering adds the directional late field and the HRTF convolution
// on top of the mono path, so the mono benchmark alone cannot support a claim
// about BRIR latency.
func BenchmarkNetworkPortalStateChange4RoomsBinaural(b *testing.B) {
	benchmarkPortalStateChange(b, func(plan *NetworkPlan) error {
		_, err := plan.RenderBinaural(plan.Scene().Receivers[0])

		return err
	})
}

func benchmarkPortalStateChange(b *testing.B, render func(*NetworkPlan) error) {
	b.Helper()

	sc := benchmarkFourRoomScene(b)
	sc.Receivers[0].HRTF = hrtf.NoopDataset{SampleRateHz: sc.SampleRate}
	cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 1.0, BandSpec: sc.BandSpec}
	renderer := NewNetworkRenderer(benchmarkNetworkConfig())

	plan, err := renderer.Prepare(sc, cfg)
	if err != nil {
		b.Fatalf("Prepare: %v", err)
	}

	err = render(plan)
	if err != nil {
		b.Fatalf("warm-up render: %v", err)
	}

	if len(plan.plan.paths) > renderer.maxPaths() {
		b.Fatalf("plan holds %d paths, above the %d cap", len(plan.plan.paths), renderer.maxPaths())
	}

	var added, reused, paths int

	b.ResetTimer()

	for index := range b.N {
		state := scene.PortalOpen
		if index%2 == 1 {
			state = scene.PortalClosed
		}

		set, err := plan.Apply(PortalStateChange{PortalIndex: 1, State: state, Aperture: 1})
		if err != nil {
			b.Fatalf("Apply: %v", err)
		}

		err = render(plan)
		if err != nil {
			b.Fatalf("render: %v", err)
		}

		added += set.AddedGroups
		reused += set.ReusedGroups
		paths += len(plan.plan.paths)
	}

	b.StopTimer()

	iterations := float64(max(b.N, 1))
	b.ReportMetric(float64(added)/iterations, "groups-added/op")
	b.ReportMetric(float64(reused)/iterations, "groups-reused/op")
	b.ReportMetric(float64(paths)/iterations, "paths/op")
}
