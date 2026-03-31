package ir

import (
	"math"
	"testing"
)

func TestNormalizePeak(t *testing.T) {
	t.Parallel()

	buf := &Buffer{SampleRate: 10, Samples: []float64{0.25, -0.5, 0.125}}
	scale := NormalizePeak(buf)

	if got, want := scale, 2.0; math.Abs(got-want) > 1e-12 {
		t.Fatalf("scale = %v, want %v", got, want)
	}

	want := []float64{0.5, -1.0, 0.25}
	for index, sample := range buf.Samples {
		if math.Abs(sample-want[index]) > 1e-12 {
			t.Fatalf("Samples[%d] = %v, want %v", index, sample, want[index])
		}
	}
}

func TestNormalizePeakSilentOrNil(t *testing.T) {
	t.Parallel()

	zero := &Buffer{SampleRate: 10, Samples: []float64{0, 0, 0}}
	if got := NormalizePeak(zero); got != 0 {
		t.Fatalf("NormalizePeak(silent) = %v, want 0", got)
	}

	var nilBuf *Buffer
	if got := NormalizePeak(nilBuf); got != 0 {
		t.Fatalf("NormalizePeak(nil) = %v, want 0", got)
	}
}

func TestNormalizeRMS(t *testing.T) {
	t.Parallel()

	buf := &Buffer{SampleRate: 10, Samples: []float64{1, -1, 1, -1}}
	scale := NormalizeRMS(buf, 0.5)

	if got, want := scale, 0.5; math.Abs(got-want) > 1e-12 {
		t.Fatalf("scale = %v, want %v", got, want)
	}

	for index, sample := range buf.Samples {
		want := []float64{0.5, -0.5, 0.5, -0.5}[index]
		if math.Abs(sample-want) > 1e-12 {
			t.Fatalf("Samples[%d] = %v, want %v", index, sample, want)
		}
	}
}

func TestNormalizeRMSRejectsNoOpInputs(t *testing.T) {
	t.Parallel()

	if got := NormalizeRMS(nil, 0.5); got != 0 {
		t.Fatalf("NormalizeRMS(nil) = %v, want 0", got)
	}

	silent := &Buffer{SampleRate: 10, Samples: []float64{0, 0}}
	if got := NormalizeRMS(silent, 0.5); got != 0 {
		t.Fatalf("NormalizeRMS(silent) = %v, want 0", got)
	}

	buf := &Buffer{SampleRate: 10, Samples: []float64{1, 2}}
	if got := NormalizeRMS(buf, 0); got != 0 {
		t.Fatalf("NormalizeRMS(target=0) = %v, want 0", got)
	}

	if got, want := buf.Samples, []float64{1, 2}; !slicesEqual(got, want) {
		t.Fatalf("samples changed = %v, want %v", got, want)
	}
}

func slicesEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}

	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}

	return true
}
