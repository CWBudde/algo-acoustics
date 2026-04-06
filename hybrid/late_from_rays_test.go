package hybrid

import (
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/raytrace"
)

func TestHistogramToEventsAndBuffer(t *testing.T) {
	t.Parallel()

	hist := raytrace.NewEnergyHistogram(0.1, 0.05, 2)
	hist.Add(0.01, []float64{1, 0})
	hist.Add(0.06, []float64{0, 2})

	events := HistogramToEvents(hist, rand.New(rand.NewSource(1)))
	if got, want := len(events), 2; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}

	buf := HistogramToBuffer(hist, 48000)
	if buf.Len() == 0 {
		t.Fatal("HistogramToBuffer() returned empty buffer")
	}
}

func TestHistogramToPoissonBuffer(t *testing.T) {
	t.Parallel()

	spec := acoustics.Octave6
	hist := raytrace.NewEnergyHistogram(0.1, 0.01, spec.BandCount())

	for i := range 10 {
		energy := make([]float64, spec.BandCount())
		for b := range energy {
			energy[b] = 0.01
		}

		hist.Add(float64(i)*0.01+0.005, energy)
	}

	rng := rand.New(rand.NewSource(42))

	buf, err := HistogramToPoissonBuffer(hist, 100, spec, 44100, rng)
	if err != nil {
		t.Fatalf("HistogramToPoissonBuffer() error: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("HistogramToPoissonBuffer() returned empty buffer")
	}
}

func TestHistogramToPoissonBufferNil(t *testing.T) {
	t.Parallel()

	buf, err := HistogramToPoissonBuffer(nil, 100, acoustics.Octave6, 44100, rand.New(rand.NewSource(0)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() != 0 {
		t.Fatalf("expected empty buffer for nil histogram, got %d samples", buf.Len())
	}
}
