package algoacoustics

import (
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestRendererRejectsNilCrossRoomBuffers(t *testing.T) {
	t.Parallel()

	sc := twoRoomScene(0.5)
	renderer := Renderer{Transmission: nilCrossRoomEngine{}}

	_, err := renderer.RenderMono(sc, twoRoomRenderConfig(sc))
	if err == nil || !strings.Contains(err.Error(), "nil mono buffer") {
		t.Fatalf("RenderMono() error = %v, want nil-buffer error", err)
	}

	_, _, err = renderer.RenderStereo(sc, twoRoomRenderConfig(sc))
	if err == nil || !strings.Contains(err.Error(), "nil binaural buffer") {
		t.Fatalf("RenderStereo() error = %v, want nil-buffer error", err)
	}
}

type nilCrossRoomEngine struct{}

func (nilCrossRoomEngine) RenderMono(*scene.Scene, ir.RenderConfig) (*ir.Buffer, error) {
	return nil, nil //nolint:nilnil // Deliberately violates the engine contract to verify Renderer validation.
}

func (nilCrossRoomEngine) RenderBinaural(
	*scene.Scene,
	scene.Receiver,
	ir.RenderConfig,
) (*ir.Buffer, *ir.Buffer, error) {
	return nil, nil, nil
}

// twoRoomScene is a minimal multi-room scene with a binaural receiver, which
// RenderStereo requires. Renderer only routes here — the engine is a fake — so
// the geometry needs to be well-formed, not acoustically interesting.
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
