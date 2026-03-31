package hybrid

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

func TestFadeHelpers(t *testing.T) {
	t.Parallel()

	linear := LinearFade(0, 4, 4)
	if got, want := len(linear), 4; got != want {
		t.Fatalf("len(LinearFade()) = %d, want %d", got, want)
	}
	if linear[0] != 0 || math.Abs(linear[3]-1) > 1e-12 {
		t.Fatalf("LinearFade() = %#v", linear)
	}

	hann := HannFade(5)
	if got, want := len(hann), 5; got != want {
		t.Fatalf("len(HannFade()) = %d, want %d", got, want)
	}
	if hann[0] != 0 || hann[4] != 0 {
		t.Fatalf("HannFade() = %#v", hann)
	}
}

func TestApplyFade(t *testing.T) {
	t.Parallel()

	buf := &ir.Buffer{SampleRate: 1000, Samples: []float64{1, 1, 1, 1, 1, 1}}
	fadeOut := ApplyFade(buf, 1, 5, false)
	if fadeOut.Samples[1] <= fadeOut.Samples[2] || fadeOut.Samples[2] <= fadeOut.Samples[3] || fadeOut.Samples[3] <= fadeOut.Samples[4] {
		t.Fatalf("expected fade out to decrease, got %#v", fadeOut.Samples)
	}
	if math.Abs(fadeOut.Samples[1]-1) > 1e-12 || math.Abs(fadeOut.Samples[4]) > 1e-12 {
		t.Fatalf("expected fade out endpoints to be 1 -> 0, got %#v", fadeOut.Samples)
	}

	fadeIn := ApplyFade(buf, 1, 5, true)
	if fadeIn.Samples[1] >= fadeIn.Samples[2] || fadeIn.Samples[2] >= fadeIn.Samples[3] || fadeIn.Samples[3] >= fadeIn.Samples[4] {
		t.Fatalf("expected fade in to increase, got %#v", fadeIn.Samples)
	}
	if math.Abs(fadeIn.Samples[1]) > 1e-12 || math.Abs(fadeIn.Samples[4]-1) > 1e-12 {
		t.Fatalf("expected fade in endpoints to be 0 -> 1, got %#v", fadeIn.Samples)
	}
}

func TestApplyFadeWithWindowUsesSelectedShape(t *testing.T) {
	t.Parallel()

	buf := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 12)}
	for i := range buf.Samples {
		buf.Samples[i] = 1
	}

	hann := ApplyFadeWithWindow(buf, 1, 10, true, FadeWindowConfig{Name: "hann"})
	blackman := ApplyFadeWithWindow(buf, 1, 10, true, FadeWindowConfig{Name: "blackman"})
	if math.Abs(hann.Samples[2]-blackman.Samples[2]) < 1e-6 {
		t.Fatalf("expected different fade shapes, got hann=%#v blackman=%#v", hann.Samples, blackman.Samples)
	}
	if blackman.Samples[1] != 0 || blackman.Samples[9] != 1 {
		t.Fatalf("expected normalized blackman fade endpoints 0 -> 1, got %#v", blackman.Samples)
	}

	defaultTukey := ApplyFadeWithWindow(buf, 1, 10, true, FadeWindowConfig{Name: "tukey"})
	narrowTukey := ApplyFadeWithWindow(buf, 1, 10, true, FadeWindowConfig{Name: "tukey", Alpha: 0.25})
	if math.Abs(defaultTukey.Samples[3]-narrowTukey.Samples[3]) < 1e-6 {
		t.Fatalf("expected tukey alpha to affect fade shape, got default=%#v narrow=%#v", defaultTukey.Samples, narrowTukey.Samples)
	}

	for i := 1; i < 10; i++ {
		if math.Abs(hann.Samples[i]+ApplyFadeWithWindow(buf, 1, 10, false, FadeWindowConfig{Name: "hann"}).Samples[i]-1) > 1e-12 {
			t.Fatalf("expected complementary fade weights at sample %d", i)
		}
	}
}
