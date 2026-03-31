package hybrid

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

func TestBlendLowFreqReturnsCombinedBuffer(t *testing.T) {
	t.Parallel()

	low := make([]float64, 256)
	for i := 0; i < 32; i++ {
		low[i] = 1
	}
	geo := &ir.Buffer{SampleRate: 1024, Samples: make([]float64, 256)}
	for i := 128; i < 256; i++ {
		geo.Samples[i] = 1
	}

	out := BlendLowFreq(low, geo, 128, 1024)
	if out == nil {
		t.Fatal("BlendLowFreq returned nil")
	}
	if len(out.Samples) == 0 {
		t.Fatal("BlendLowFreq returned empty buffer")
	}
	if math.Abs(out.Samples[0]) < 1e-9 {
		t.Fatal("expected low-frequency contribution near the start")
	}
}
