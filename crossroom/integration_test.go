package crossroom_test

// TestRendererRoutesMultiRoomMonoAndBinaural is the one external test in this
// package. It needs both algoacoustics.Renderer and a real cross-room engine,
// and an external test package may import a package that imports the package
// under test, so crossroom_test -> algoacoustics -> crossroom is legal and
// cycle-free. Everything else here stays white-box.

import (
	"testing"

	algoacoustics "github.com/cwbudde/algo-acoustics"
	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/crossroom"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestRendererRoutesMultiRoomMonoAndBinaural(t *testing.T) {
	t.Parallel()

	sc := twoRoomScene(0.5)
	renderCfg := twoRoomRenderConfig(sc)
	transmission := crossroom.NewOneHop(crossroom.OneHopConfig{
		ISM: ism.ISMConfig{MaxOrder: 0},
		Raytrace: raytrace.EngineConfig{
			Launch: raytrace.LaunchConfig{
				NumRays:        256,
				MaxBounces:     2,
				MaxTimeSeconds: renderCfg.DurationSeconds,
				SpeedOfSound:   acoustics.SpeedOfSound,
			},
			ReceiverRadius:     0.3,
			BinDurationSeconds: 0.005,
		},
		Hybrid: hybrid.HybridConfig{
			CrossoverMode:        hybrid.TimeBased,
			CrossoverTimeSeconds: 0.03,
			SmoothenCrossover:    true,
		},
	})
	renderer := algoacoustics.Renderer{Transmission: transmission}

	mono, err := renderer.RenderMono(sc, renderCfg)
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	if !hasSignal(mono) {
		t.Fatal("RenderMono() returned silence")
	}

	left, right, err := renderer.RenderStereo(sc, renderCfg)
	if err != nil {
		t.Fatalf("RenderStereo() error = %v", err)
	}

	if !hasSignal(left) || !hasSignal(right) {
		t.Fatal("RenderStereo() returned a silent channel")
	}
}

// twoRoomScene mirrors the white-box transmissionTestScene fixture. The
// external test package cannot reach that one, and a shoebox pair joined by a
// closed portal is the whole setup, so it is restated rather than exported.
func twoRoomScene(tau float64) *scene.Scene {
	wallMaterials := [6]string{"wall", "wall", "wall", "wall", "wall", "wall"}

	return &scene.Scene{
		Rooms: []scene.Room{
			{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{Width: 4, Depth: 3, Height: 2.5, WallMaterials: wallMaterials}},
			{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{Origin: geometry.Vec3{X: 4}, Width: 4, Depth: 3, Height: 2.5, WallMaterials: wallMaterials}},
		},
		Portals: []scene.Portal{{
			RoomIndices: [2]int{0, 1},
			Polygon: []geometry.Vec3{
				{X: 4, Y: 0, Z: 0},
				{X: 4, Y: 3, Z: 0},
				{X: 4, Y: 3, Z: 2.5},
				{X: 4, Y: 0, Z: 2.5},
			},
			Material: "portal",
			State:    scene.PortalClosed,
		}},
		Materials: map[string]scene.Material{
			"wall":   {Name: "wall", AbsorptionByBand: []float64{0.1}},
			"portal": {Name: "portal", AbsorptionByBand: []float64{0}, TransmissionByBand: []float64{tau}},
		},
		Sources: []scene.Source{{Position: geometry.Vec3{X: 1, Y: 1.5, Z: 1.25}}},
		Receivers: []scene.Receiver{{
			Position: geometry.Vec3{X: 7, Y: 1.5, Z: 1.25},
			Type:     scene.ReceiverBinaural,
			HRTF:     hrtf.NoopDataset{SampleRateHz: 8000},
		}},
		BandSpec: acoustics.BandSpec{
			CenterFreqs: []float64{500},
			LowerEdges:  []float64{350},
			UpperEdges:  []float64{700},
		},
		SampleRate: 8000,
	}
}

func twoRoomRenderConfig(sc *scene.Scene) ir.RenderConfig {
	return ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 0.08, BandSpec: sc.BandSpec}
}

func hasSignal(samples []float64) bool {
	for _, sample := range samples {
		if sample != 0 {
			return true
		}
	}

	return false
}
