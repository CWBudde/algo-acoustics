package hybrid

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

func TestAlignLateTailMatchesEarlyEnergy(t *testing.T) {
	t.Parallel()

	late := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 100)}
	for i := 20; i < 30; i++ {
		late.Samples[i] = 1
	}

	aligned := AlignLateTail(late, []ir.Event{{TimeSeconds: 0.02, Amplitude: 2}}, HybridConfig{CrossoverTimeSeconds: 0.02})
	if aligned == nil {
		t.Fatal("AlignLateTail() = nil")
	}

	if aligned.Samples[20] < 1.5 {
		t.Fatalf("aligned sample = %v, want scaled up", aligned.Samples[20])
	}
}

func TestAlignLateTailIgnoresDistantDirectEvent(t *testing.T) {
	t.Parallel()

	late := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 120)}
	for i := 80; i < 90; i++ {
		late.Samples[i] = 1
	}

	aligned := AlignLateTail(late, []ir.Event{
		{TimeSeconds: 0.01, Amplitude: 100, Kind: ir.EventDirect},
		{TimeSeconds: 0.075, Amplitude: 2, Kind: ir.EventSpecular},
	}, HybridConfig{CrossoverTimeSeconds: 0.08})

	if got := aligned.Samples[80]; math.Abs(got-2) > 1e-12 {
		t.Fatalf("aligned sample = %v, want 2 from the local pre-crossover event", got)
	}
}

func TestAlignLateTailUsesLocalEventBandEnergy(t *testing.T) {
	t.Parallel()

	late := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 120)}
	for i := 80; i < 90; i++ {
		late.Samples[i] = 1
	}

	aligned := AlignLateTail(late, []ir.Event{
		{TimeSeconds: 0.075, Amplitude: 2, BandGain: []float64{-1, 1}},
	}, HybridConfig{CrossoverTimeSeconds: 0.08})

	if got := aligned.Samples[80]; math.Abs(got-2) > 1e-12 {
		t.Fatalf("aligned sample = %v, want 2 from non-cancelling per-band energy", got)
	}
}

func TestEnergyRMSHandlesExtremeMagnitudes(t *testing.T) {
	t.Parallel()

	for _, amplitude := range []float64{1e-100, 1e100} {
		got := eventEnergyRMS([]ir.Event{{Amplitude: amplitude}}, 0)
		if math.Abs(got-amplitude)/amplitude > 1e-15 {
			t.Errorf("eventEnergyRMS(amplitude=%g) = %g, want %g", amplitude, got, amplitude)
		}

		buf := &ir.Buffer{SampleRate: 1, Samples: []float64{amplitude}}

		got = bufferEnergyRMS(buf, 0, 1)
		if math.Abs(got-amplitude)/amplitude > 1e-15 {
			t.Errorf("bufferEnergyRMS(sample=%g) = %g, want %g", amplitude, got, amplitude)
		}
	}
}
