package hybrid

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

func TestHybridCombinedLengthMatchesDuration(t *testing.T) {
	t.Parallel()

	early := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 1000)}
	late := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 1000)}

	combined := CombineBuffers(early, late, HybridConfig{CrossoverTimeSeconds: 0.25, SmoothenCrossover: true})
	if combined == nil {
		t.Fatal("CombineBuffers() = nil")
	}

	if got, want := len(combined.Samples), 1000; got != want {
		t.Fatalf("len(combined.Samples) = %d, want %d", got, want)
	}

	if got, want := combined.SampleRate, 1000; got != want {
		t.Fatalf("SampleRate = %d, want %d", got, want)
	}
}

func TestHybridEarlyWindowDominatedByEarlyOutput(t *testing.T) {
	t.Parallel()

	early := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 1000)}

	late := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 1000)}
	for i := range 100 {
		early.Samples[i] = 1
	}

	for i := 500; i < 1000; i++ {
		late.Samples[i] = 0.25
	}

	combined := CombineBuffers(early, late, HybridConfig{CrossoverTimeSeconds: 0.25, SmoothenCrossover: true})
	if combined == nil {
		t.Fatal("CombineBuffers() = nil")
	}

	if got := meanAbs(combined.Samples[:100]); got < 0.8 {
		t.Fatalf("early window mean = %v, want early-dominant signal", got)
	}
}

func TestHybridLateWindowDominatedByLateOutput(t *testing.T) {
	t.Parallel()

	early := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 1000)}

	late := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 1000)}
	for i := range 100 {
		early.Samples[i] = 0.1
	}

	for i := 500; i < 1000; i++ {
		late.Samples[i] = 1
	}

	combined := CombineBuffers(early, late, HybridConfig{CrossoverTimeSeconds: 0.25, SmoothenCrossover: true})
	if combined == nil {
		t.Fatal("CombineBuffers() = nil")
	}

	if got := meanAbs(combined.Samples[500:600]); got < 0.8 {
		t.Fatalf("late window mean = %v, want late-dominant signal", got)
	}
}

func TestHybridCrossoverIsContinuous(t *testing.T) {
	t.Parallel()

	early := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 1000)}

	late := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 1000)}
	for i := range early.Samples {
		early.Samples[i] = 1
		late.Samples[i] = 1
	}

	combined := CombineBuffers(early, late, HybridConfig{CrossoverTimeSeconds: 0.5, SmoothenCrossover: true})
	if combined == nil {
		t.Fatal("CombineBuffers() = nil")
	}

	left := windowRMS(combined.Samples[480:500])

	right := windowRMS(combined.Samples[500:520])
	if left == 0 || right == 0 {
		t.Fatalf("unexpected zero-energy crossover windows: left=%v right=%v", left, right)
	}

	jumpDB := math.Abs(20 * math.Log10(right/left))
	if jumpDB > 3 {
		t.Fatalf("crossover jump = %.2fdB, want <= 3dB", jumpDB)
	}
}

func meanAbs(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64

	for _, value := range values {
		if value < 0 {
			sum -= value
			continue
		}

		sum += value
	}

	return sum / float64(len(values))
}

func windowRMS(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, value := range values {
		sum += value * value
	}

	return math.Sqrt(sum / float64(len(values)))
}
