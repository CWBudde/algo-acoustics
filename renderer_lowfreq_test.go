package algoacoustics

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

type recordingLowFreqEngine struct {
	calls int
}

func (e *recordingLowFreqEngine) Transfer(_ *scene.Scene, _ ir.RenderConfig) (*TransferFunction, error) {
	e.calls++

	return &TransferFunction{
		Freqs: []float64{0, 100, 200},
		H:     []complex128{1, 1, 1},
	}, nil
}

type unitRendererHRTF struct{}

func (unitRendererHRTF) SampleRate() int { return 100 }

func (unitRendererHRTF) Lookup(geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	return []float64{1}, []float64{1}, 0, nil
}

func TestRendererRenderMonoUsesLowFreqEngineWithoutCrossoverProvider(t *testing.T) {
	t.Parallel()

	engine := &recordingLowFreqEngine{}
	renderer := Renderer{LowFreq: engine}
	cfg := ir.RenderConfig{SampleRate: 1000, DurationSeconds: 0.064}

	samples, err := renderer.RenderMono(&scene.Scene{}, cfg)
	if err != nil {
		t.Fatal(err)
	}

	if engine.calls != 1 {
		t.Fatalf("LowFreqEngine.Transfer called %d times, want 1", engine.calls)
	}

	var energy float64
	for _, sample := range samples {
		energy += sample * sample
	}

	if energy == 0 {
		t.Fatal("mono output is silent; default-crossover low-frequency transfer was not blended")
	}
}

func TestRendererRenderStereoDoesNotApplyMonauralLowFreqEngine(t *testing.T) {
	t.Parallel()

	engine := &recordingLowFreqEngine{}
	renderer := Renderer{LowFreq: engine}
	sc := &scene.Scene{Receivers: []scene.Receiver{{
		Type: scene.ReceiverBinaural,
		HRTF: unitRendererHRTF{},
	}}}

	_, _, err := renderer.RenderStereo(sc, ir.RenderConfig{SampleRate: 100, DurationSeconds: 0.1})
	if err != nil {
		t.Fatal(err)
	}

	if engine.calls != 0 {
		t.Fatalf("stereo render called monaural LowFreqEngine.Transfer %d times, want 0", engine.calls)
	}
}
