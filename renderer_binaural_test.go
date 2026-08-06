package algoacoustics

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

type fixedEventEngine struct {
	events []ir.Event
}

func (e fixedEventEngine) Generate(_ *scene.Scene, _ ir.RenderConfig) ([]ir.Event, error) {
	return e.events, nil
}

type recordingHRTF struct {
	direction geometry.Vec3
}

func (*recordingHRTF) SampleRate() int { return 100 }

func (d *recordingHRTF) Lookup(direction geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	d.direction = direction

	return []float64{1}, []float64{1}, 0, nil
}

func TestRendererRenderStereoLooksUpHeadRelativeDirection(t *testing.T) {
	t.Parallel()

	dataset := &recordingHRTF{}
	worldDirection := geometry.Vec3{Y: 1}
	events := []ir.Event{{TimeSeconds: 0.01, Amplitude: 1, Direction: worldDirection}}
	renderer := Renderer{Early: fixedEventEngine{events: events}}
	sc := &scene.Scene{Receivers: []scene.Receiver{{
		Orientation: geometry.QuatFromAxisAngle(geometry.Vec3{Z: 1}, math.Pi/2),
		Type:        scene.ReceiverBinaural,
		HRTF:        dataset,
	}}}

	_, _, err := renderer.RenderStereo(sc, ir.RenderConfig{SampleRate: 100, DurationSeconds: 0.1})
	if err != nil {
		t.Fatalf("RenderStereo() error = %v", err)
	}

	if math.Abs(dataset.direction.X-1) > 1e-12 || math.Abs(dataset.direction.Y) > 1e-12 || math.Abs(dataset.direction.Z) > 1e-12 {
		t.Fatalf("HRTF lookup direction = %#v, want head-relative +X", dataset.direction)
	}

	if events[0].Direction != worldDirection {
		t.Fatalf("RenderStereo() mutated engine event direction to %#v", events[0].Direction)
	}
}
