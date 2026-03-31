package hybrid

import (
	"math/rand"
	"testing"

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
