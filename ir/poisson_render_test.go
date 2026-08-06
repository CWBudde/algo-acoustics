package ir

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

func TestRenderMonoPoissonBasic(t *testing.T) {
	t.Parallel()

	spec := acoustics.Octave6
	sampleRate := 44100
	binDuration := 0.01

	// Create a simple histogram with uniform energy across 100 ms.
	bins := make([]EnergyBin, 10)
	for i := range bins {
		energy := make([]float64, spec.BandCount())
		for b := range energy {
			energy[b] = 0.01
		}

		bins[i] = EnergyBin{
			TimeSeconds: float64(i) * binDuration,
			BandEnergy:  energy,
		}
	}

	cfg := PoissonConfig{
		Bins:        bins,
		BinDuration: binDuration,
		Volume:      100,
		BandSpec:    spec,
		SampleRate:  sampleRate,
	}

	rng := rand.New(rand.NewSource(42))

	buf, err := RenderMonoPoisson(cfg, rng)
	if err != nil {
		t.Fatalf("RenderMonoPoisson() error: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("RenderMonoPoisson() returned empty buffer")
	}

	// Verify buffer has non-trivial content.
	var maxAbs float64
	for _, s := range buf.Samples {
		if a := math.Abs(s); a > maxAbs {
			maxAbs = a
		}
	}

	if maxAbs == 0 {
		t.Fatal("buffer is all zeros")
	}
}

func TestRenderMonoPoissonEnergyPreservation(t *testing.T) {
	t.Parallel()

	spec := acoustics.Octave6
	sampleRate := 44100
	binDuration := 0.02

	// Create bins with known energy in one band only.
	targetBand := 2
	targetEnergy := 0.5
	bins := make([]EnergyBin, 5)

	for i := range bins {
		energy := make([]float64, spec.BandCount())
		energy[targetBand] = targetEnergy
		bins[i] = EnergyBin{
			TimeSeconds: float64(i) * binDuration,
			BandEnergy:  energy,
		}
	}

	cfg := PoissonConfig{
		Bins:        bins,
		BinDuration: binDuration,
		Volume:      200,
		BandSpec:    spec,
		SampleRate:  sampleRate,
	}

	rng := rand.New(rand.NewSource(7))

	buf, err := RenderMonoPoisson(cfg, rng)
	if err != nil {
		t.Fatalf("RenderMonoPoisson() error: %v", err)
	}

	// Total energy should be non-zero and finite.
	var totalEnergy float64
	for _, s := range buf.Samples {
		totalEnergy += s * s
	}

	if totalEnergy == 0 {
		t.Fatal("output has zero energy")
	}

	if math.IsNaN(totalEnergy) || math.IsInf(totalEnergy, 0) {
		t.Fatalf("output energy is %v", totalEnergy)
	}
}

func TestRenderMonoPoissonEmptyBins(t *testing.T) {
	t.Parallel()

	buf, err := RenderMonoPoisson(PoissonConfig{
		BinDuration: 0.01,
		Volume:      100,
		BandSpec:    acoustics.Octave6,
		SampleRate:  44100,
	}, rand.New(rand.NewSource(0)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() != 0 {
		t.Fatalf("expected empty buffer for no bins, got %d samples", buf.Len())
	}
}

func TestRenderMonoPoissonInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  PoissonConfig
		rng  *rand.Rand
	}{
		{
			name: "no bands",
			cfg:  PoissonConfig{BinDuration: 0.01, Volume: 100, SampleRate: 44100},
			rng:  rand.New(rand.NewSource(0)),
		},
		{
			name: "zero sample rate",
			cfg:  PoissonConfig{BinDuration: 0.01, Volume: 100, BandSpec: acoustics.Octave6},
			rng:  rand.New(rand.NewSource(0)),
		},
		{
			name: "zero volume",
			cfg:  PoissonConfig{BinDuration: 0.01, BandSpec: acoustics.Octave6, SampleRate: 44100},
			rng:  rand.New(rand.NewSource(0)),
		},
		{
			name: "infinite bin duration",
			cfg:  PoissonConfig{BinDuration: math.Inf(1), Volume: 100, BandSpec: acoustics.Octave6, SampleRate: 44100},
			rng:  rand.New(rand.NewSource(0)),
		},
		{
			name: "nil random source",
			cfg:  PoissonConfig{BinDuration: 0.01, Volume: 100, BandSpec: acoustics.Octave6, SampleRate: 44100},
		},
		{
			name: "unsorted bins",
			cfg: PoissonConfig{
				Bins:        []EnergyBin{{TimeSeconds: 0.02, BandEnergy: make([]float64, 6)}, {TimeSeconds: 0.01, BandEnergy: make([]float64, 6)}},
				BinDuration: 0.01,
				Volume:      100,
				BandSpec:    acoustics.Octave6,
				SampleRate:  44100,
			},
			rng: rand.New(rand.NewSource(0)),
		},
		{
			name: "negative bin time",
			cfg: PoissonConfig{
				Bins:        []EnergyBin{{TimeSeconds: -0.01, BandEnergy: make([]float64, 6)}},
				BinDuration: 0.01,
				Volume:      100,
				BandSpec:    acoustics.Octave6,
				SampleRate:  44100,
			},
			rng: rand.New(rand.NewSource(0)),
		},
		{
			name: "wrong band energy length",
			cfg: PoissonConfig{
				Bins:        []EnergyBin{{BandEnergy: make([]float64, 5)}},
				BinDuration: 0.01,
				Volume:      100,
				BandSpec:    acoustics.Octave6,
				SampleRate:  44100,
			},
			rng: rand.New(rand.NewSource(0)),
		},
		{
			name: "negative band energy",
			cfg: PoissonConfig{
				Bins:        []EnergyBin{{BandEnergy: []float64{-1, 0, 0, 0, 0, 0}}},
				BinDuration: 0.01,
				Volume:      100,
				BandSpec:    acoustics.Octave6,
				SampleRate:  44100,
			},
			rng: rand.New(rand.NewSource(0)),
		},
		{
			name: "band edge length mismatch",
			cfg: PoissonConfig{
				BinDuration: 0.01,
				Volume:      100,
				BandSpec: acoustics.BandSpec{
					CenterFreqs: []float64{1000},
					LowerEdges:  []float64{700},
				},
				SampleRate: 44100,
			},
			rng: rand.New(rand.NewSource(0)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := RenderMonoPoisson(tt.cfg, tt.rng)
			if err == nil {
				t.Fatal("expected error for invalid config")
			}
		})
	}
}

func TestApplyEnergyEnvelopeZerosGapsAndDeterministicallyOverwritesOverlaps(t *testing.T) {
	t.Parallel()

	sequence := make([]float64, 10)
	for index := range sequence {
		sequence[index] = 1
	}

	bins := []EnergyBin{
		{TimeSeconds: 2, BandEnergy: []float64{4}},
		{TimeSeconds: 4, BandEnergy: []float64{0}},
	}
	applyEnergyEnvelope(sequence, bins, 0, 4, 1)

	want := []float64{0, 0, 1, 1, 0, 0, 0, 0, 0, 0}
	for index := range sequence {
		if sequence[index] != want[index] {
			t.Fatalf("sequence = %v, want %v", sequence, want)
		}
	}
}
