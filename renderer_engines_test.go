package algoacoustics_test

import (
	"strings"
	"testing"

	algoacoustics "github.com/cwbudde/algo-acoustics"
	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestRendererStockEnginesRenderMono(t *testing.T) {
	t.Parallel()

	sc := rendererEngineScene(false)
	cfg := rendererEngineRenderConfig(sc)
	renderer := stockRenderer()

	samples, err := renderer.RenderMono(sc, cfg)
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	if len(samples) != int(cfg.DurationSeconds*float64(cfg.SampleRate)) {
		t.Fatalf("RenderMono() sample count = %d, want %d", len(samples), int(cfg.DurationSeconds*float64(cfg.SampleRate)))
	}

	if signalEnergy(samples) == 0 {
		t.Fatal("RenderMono() returned a silent impulse response")
	}
}

func TestRendererStockEnginesRenderBinaural(t *testing.T) {
	t.Parallel()

	sc := rendererEngineScene(true)
	cfg := rendererEngineRenderConfig(sc)
	renderer := stockRenderer()

	left, right, err := renderer.RenderStereo(sc, cfg)
	if err != nil {
		t.Fatalf("RenderStereo() error = %v", err)
	}

	wantSamples := int(cfg.DurationSeconds * float64(cfg.SampleRate))
	if len(left) != wantSamples || len(right) != wantSamples {
		t.Fatalf("RenderStereo() sample counts = (%d, %d), want (%d, %d)", len(left), len(right), wantSamples, wantSamples)
	}

	if signalEnergy(left) == 0 || signalEnergy(right) == 0 {
		t.Fatal("RenderStereo() returned a silent binaural impulse response")
	}
}

func TestRaytraceEngineRejectsUnsupportedCardinality(t *testing.T) {
	t.Parallel()

	sc := rendererEngineScene(false)
	sc.Sources = append(sc.Sources, sc.Sources[0])
	engine := algoacoustics.NewRaytraceEngine(algoacoustics.RaytraceEngineConfig{
		Launch: raytrace.LaunchConfig{NumRays: 8, MaxBounces: 1},
	})

	_, err := engine.RenderMono(sc, rendererEngineRenderConfig(sc))
	if err == nil {
		t.Fatal("RenderMono() error = nil, want source-cardinality error")
	}
}

func TestRaytraceEngineRejectsMeshWithoutEnclosedVolume(t *testing.T) {
	t.Parallel()

	sc := rendererEngineScene(true)
	sc.Room = scene.Room{
		Kind: scene.RoomKindMesh,
		Mesh: &geometry.Mesh{Triangles: []geometry.Triangle{
			{V0: geometry.Vec3{}, V1: geometry.Vec3{Y: 1}, V2: geometry.Vec3{X: 1}},
			{V0: geometry.Vec3{}, V1: geometry.Vec3{X: 1}, V2: geometry.Vec3{Z: 1}},
			{V0: geometry.Vec3{}, V1: geometry.Vec3{Z: 1}, V2: geometry.Vec3{Y: 1}},
		}},
	}
	engine := algoacoustics.NewRaytraceEngine(algoacoustics.RaytraceEngineConfig{})

	_, _, err := engine.RenderBinaural(sc, sc.Receivers[0], rendererEngineRenderConfig(sc))
	if err == nil || !strings.Contains(err.Error(), "enclosed room volume") {
		t.Fatalf("RenderBinaural() error = %v, want enclosed-volume error", err)
	}
}

func stockRenderer() algoacoustics.Renderer {
	return algoacoustics.Renderer{
		Early: algoacoustics.NewISMEngine(ism.ISMConfig{MaxOrder: 1}),
		LateBuffer: algoacoustics.NewRaytraceEngine(algoacoustics.RaytraceEngineConfig{
			Launch: raytrace.LaunchConfig{
				NumRays:    32,
				MaxBounces: 2,
			},
			ReceiverRadius:     0.25,
			BinDurationSeconds: 0.01,
		}),
		Hybrid: hybrid.HybridConfig{
			CrossoverTimeSeconds: 0.02,
			CrossoverMode:        hybrid.TimeBased,
			SmoothenCrossover:    true,
		},
	}
}

func rendererEngineRenderConfig(sc *scene.Scene) ir.RenderConfig {
	return ir.RenderConfig{
		SampleRate:      sc.SampleRate,
		DurationSeconds: 0.08,
		BandSpec:        sc.BandSpec,
	}
}

func rendererEngineScene(binaural bool) *scene.Scene {
	receiver := scene.Receiver{
		Position:    geometry.Vec3{X: 4, Y: 2, Z: 1.2},
		Orientation: geometry.QuatIdentity(),
		Type:        scene.ReceiverOmni,
	}
	if binaural {
		receiver.Type = scene.ReceiverBinaural
		receiver.HRTF = hrtf.NoopDataset{SampleRateHz: 48000}
	}

	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:         6,
				Depth:         4.5,
				Height:        2.8,
				WallMaterials: [6]string{"plaster", "plaster", "plaster", "plaster", "plaster", "plaster"},
			},
		},
		Materials: map[string]scene.Material{
			"plaster": {
				Name:             "plaster",
				AbsorptionByBand: []float64{0.1, 0.1, 0.15, 0.2, 0.2, 0.25},
				ScatteringByBand: []float64{0.02, 0.02, 0.02, 0.03, 0.03, 0.04},
			},
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 1.5, Y: 2, Z: 1.2},
			Orientation: geometry.QuatIdentity(),
			Directivity: directivity.OmniModel{},
		}},
		Receivers:  []scene.Receiver{receiver},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}

func signalEnergy(samples []float64) float64 {
	var energy float64
	for _, sample := range samples {
		energy += sample * sample
	}

	return energy
}
