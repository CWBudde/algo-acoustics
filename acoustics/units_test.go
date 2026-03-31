package acoustics_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

func TestDecibelToLinear(t *testing.T) {
	cases := []struct {
		dB   float64
		want float64
	}{
		{0, 1.0},
		{20, 10.0},
		{-20, 0.1},
		{6, math.Pow(10, 6.0/20)},
	}

	for _, c := range cases {
		got := acoustics.DecibelToLinear(c.dB)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("DecibelToLinear(%v) = %v, want %v", c.dB, got, c.want)
		}
	}
}

func TestLinearToDecibel(t *testing.T) {
	cases := []struct {
		linear float64
		want   float64
	}{
		{1.0, 0},
		{10.0, 20},
		{0.1, -20},
	}

	for _, c := range cases {
		got := acoustics.LinearToDecibel(c.linear)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("LinearToDecibel(%v) = %v, want %v", c.linear, got, c.want)
		}
	}
}

func TestLinearToDecibelZero(t *testing.T) {
	got := acoustics.LinearToDecibel(0)
	if !math.IsInf(got, -1) {
		t.Errorf("LinearToDecibel(0) = %v, want -Inf", got)
	}
}

func TestDecibelRoundTrip(t *testing.T) {
	for _, dB := range []float64{-40, -20, 0, 6, 20, 40} {
		got := acoustics.LinearToDecibel(acoustics.DecibelToLinear(dB))
		if math.Abs(got-dB) > 1e-9 {
			t.Errorf("round-trip dB=%v: got %v", dB, got)
		}
	}
}

func TestMetersToSamples(t *testing.T) {
	// 343 m at 343 m/s with 48000 Hz → exactly 48000 samples
	got := acoustics.MetersToSamples(343.0, 343.0, 48000)
	if got != 48000 {
		t.Errorf("MetersToSamples(343, 343, 48000) = %d, want 48000", got)
	}
}

func TestSamplesToSeconds(t *testing.T) {
	got := acoustics.SamplesToSeconds(48000, 48000)
	if math.Abs(got-1.0) > 1e-12 {
		t.Errorf("SamplesToSeconds(48000, 48000) = %v, want 1.0", got)
	}
}

func TestMetersToSamplesSamplesToSecondsRoundTrip(t *testing.T) {
	const c = 343.0
	const sr = 48000

	dist := 5.0
	n := acoustics.MetersToSamples(dist, c, sr)
	secs := acoustics.SamplesToSeconds(n, sr)
	expected := dist / c

	if math.Abs(secs-expected) > 1.0/sr {
		t.Errorf("round-trip dist=%vm: got %vs, want %vs", dist, secs, expected)
	}
}
