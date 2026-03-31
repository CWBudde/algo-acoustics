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

	buf := &ir.Buffer{SampleRate: 1000, Samples: []float64{1, 1, 1, 1, 1}}
	fadeOut := ApplyFade(buf, 1, 4, false)
	if fadeOut.Samples[1] <= fadeOut.Samples[3] {
		t.Fatalf("expected fade out to decrease, got %#v", fadeOut.Samples)
	}

	fadeIn := ApplyFade(buf, 1, 4, true)
	if fadeIn.Samples[1] >= fadeIn.Samples[3] {
		t.Fatalf("expected fade in to increase, got %#v", fadeIn.Samples)
	}
}
