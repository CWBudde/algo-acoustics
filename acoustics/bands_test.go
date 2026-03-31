package acoustics_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

func TestOctave6BandCount(t *testing.T) {
	if n := acoustics.Octave6.BandCount(); n != 6 {
		t.Errorf("Octave6.BandCount() = %d, want 6", n)
	}
}

func TestOctave8BandCount(t *testing.T) {
	if n := acoustics.Octave8.BandCount(); n != 8 {
		t.Errorf("Octave8.BandCount() = %d, want 8", n)
	}
}

func TestOctave6Centers(t *testing.T) {
	want := []float64{125, 250, 500, 1000, 2000, 4000}
	got := acoustics.Octave6.CenterFreqs

	if len(got) != len(want) {
		t.Fatalf("Octave6 has %d centers, want %d", len(got), len(want))
	}

	for i, w := range want {
		if got[i] != w {
			t.Errorf("Octave6.CenterFreqs[%d] = %v, want %v", i, got[i], w)
		}
	}
}

func TestBandEdgesSymmetric(t *testing.T) {
	// Each upper edge should equal the next lower edge (contiguous bands).
	spec := acoustics.Octave6
	for i := 0; i < len(spec.UpperEdges)-1; i++ {
		if math.Abs(spec.UpperEdges[i]-spec.LowerEdges[i+1]) > 1e-6 {
			t.Errorf("gap between band %d upper (%.2f) and band %d lower (%.2f)",
				i, spec.UpperEdges[i], i+1, spec.LowerEdges[i+1])
		}
	}
}

func TestBandEdgesContainCenter(t *testing.T) {
	for _, spec := range []acoustics.BandSpec{acoustics.Octave6, acoustics.Octave8} {
		for i, fc := range spec.CenterFreqs {
			if fc < spec.LowerEdges[i] || fc > spec.UpperEdges[i] {
				t.Errorf("center freq %.1f Hz outside its own band edges [%.1f, %.1f]",
					fc, spec.LowerEdges[i], spec.UpperEdges[i])
			}
		}
	}
}

func TestOctaveRatio(t *testing.T) {
	// Upper / lower edge should equal 2 for 1-octave bands.
	for _, spec := range []acoustics.BandSpec{acoustics.Octave6, acoustics.Octave8} {
		for i := range spec.CenterFreqs {
			ratio := spec.UpperEdges[i] / spec.LowerEdges[i]
			if math.Abs(ratio-2.0) > 1e-6 {
				t.Errorf("band %d: upper/lower ratio = %.6f, want 2.0", i, ratio)
			}
		}
	}
}
