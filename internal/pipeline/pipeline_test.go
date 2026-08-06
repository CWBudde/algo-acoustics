package pipeline

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestStockPipelineRendersMonoAndBinaural(t *testing.T) {
	t.Parallel()

	sc := pipelineScene()
	lateCfg := LateConfig{
		NumRays:            16,
		MaxOrder:           1,
		DurationSeconds:    0.04,
		ReceiverRadius:     0.25,
		BinDurationSeconds: 0.01,
	}

	events, err := SolveEarly(sc, EarlyConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("SolveEarly() error = %v", err)
	}

	if len(events) == 0 {
		t.Fatal("SolveEarly() returned no stock ISM events")
	}

	mono, err := RenderLateBuffer(sc, lateCfg)
	if err != nil {
		t.Fatalf("RenderLateBuffer() error = %v", err)
	}

	if mono == nil || mono.Len() == 0 {
		t.Fatal("RenderLateBuffer() returned an empty buffer")
	}

	left, right, err := RenderLateBinaural(sc, sc.Receivers[0], lateCfg)
	if err != nil {
		t.Fatalf("RenderLateBinaural() error = %v", err)
	}

	if left == nil || right == nil || left.Len() == 0 || right.Len() == 0 {
		t.Fatal("RenderLateBinaural() returned empty buffers")
	}
}

func pipelineScene() *scene.Scene {
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
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 4, Y: 2, Z: 1.2},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverBinaural,
			HRTF:        hrtf.NoopDataset{SampleRateHz: 48000},
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}
