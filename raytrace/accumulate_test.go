package raytrace

import "testing"

func TestEnergyHistogramAddAndRender(t *testing.T) {
	t.Parallel()

	hist := NewEnergyHistogram(0.2, 0.05, 3)
	hist.Add(0.075, []float64{1, 2, 3})

	if got, want := len(hist.Bins), 4; got != want {
		t.Fatalf("len(Bins) = %d, want %d", got, want)
	}

	if hist.Bins[1].BandEnergy[0] != 1 || hist.Bins[1].BandEnergy[1] != 2 || hist.Bins[1].BandEnergy[2] != 3 {
		t.Fatalf("unexpected histogram accumulation: %#v", hist.Bins[1].BandEnergy)
	}

	buf := hist.ToLateMono(48000)
	if buf.Len() == 0 {
		t.Fatal("ToLateMono() returned empty buffer")
	}
	var nonZero bool

	for _, sample := range buf.Samples {
		if sample != 0 {
			nonZero = true
			break
		}
	}

	if !nonZero {
		t.Fatal("ToLateMono() produced silent buffer")
	}
}

func TestEnergyHistogramAddIgnoresOutOfDurationTimes(t *testing.T) {
	t.Parallel()

	hist := NewEnergyHistogram(0.12, 0.05, 1)
	hist.Add(-0.001, []float64{1})
	hist.Add(0.12, []float64{2})
	hist.Add(10, []float64{4})

	for i, bin := range hist.Bins {
		if bin.BandEnergy[0] != 0 {
			t.Fatalf("out-of-duration energy leaked into bin %d: %v", i, bin.BandEnergy[0])
		}
	}

	hist.Add(0.119, []float64{8})

	if got := hist.Bins[2].BandEnergy[0]; got != 8 {
		t.Fatalf("last valid partial-bin energy = %v, want 8", got)
	}
}
