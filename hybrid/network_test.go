package hybrid

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-dsp/dsp/conv"
)

// impulseResponse builds a one-band-per-entry response with a single impulse.
func impulseResponse(bandCount, length, at int, amplitude float64) *ir.BandedResponse {
	response := ir.NewBandedResponse(48000, bandCount, length)
	for band := range response.Bands {
		response.Bands[band][at] = amplitude
	}

	return response
}

func TestConvolveBandedMatchesDirectConvolution(t *testing.T) {
	t.Parallel()

	first := ir.NewBandedResponse(48000, 2, 8)
	second := ir.NewBandedResponse(48000, 2, 8)

	for band := range first.Bands {
		for index := range first.Bands[band] {
			first.Bands[band][index] = math.Sin(float64(index+band) * 0.7)
			second.Bands[band][index] = math.Cos(float64(index*2+band) * 0.3)
		}
	}

	got, err := ConvolveBanded(first, second, 0)
	if err != nil {
		t.Fatalf("ConvolveBanded: %v", err)
	}

	for band := range first.Bands {
		want, err := conv.Direct(first.Bands[band], second.Bands[band])
		if err != nil {
			t.Fatalf("conv.Direct: %v", err)
		}

		for index := range want {
			if index >= got.Len() {
				break
			}

			if math.Abs(got.Bands[band][index]-want[index]) > 1e-10 {
				t.Fatalf("band %d sample %d = %v, want %v", band, index, got.Bands[band][index], want[index])
			}
		}
	}
}

// TestPathChainResolveComposesDelaysAndGains pins the composition rule the
// filter network rests on: convolving impulse trains adds their delays and
// multiplies their amplitudes, with the portal filter applied at the handoff.
func TestPathChainResolveComposesDelaysAndGains(t *testing.T) {
	t.Parallel()

	chain := PathChain{
		Factors: []*ir.BandedResponse{
			impulseResponse(2, 16, 3, 0.5),
			impulseResponse(2, 16, 5, 0.25),
		},
		PortalGains: []ScalarFilter{{0.5, 0.5}},
	}

	resolved, err := chain.Resolve(0)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Delays add: 3 + 5 = 8. Amplitudes multiply, times the portal filter:
	// 0.5 * 0.25 * 0.5 = 0.0625.
	for band := range resolved.Bands {
		if got := resolved.Bands[band][8]; math.Abs(got-0.0625) > 1e-12 {
			t.Fatalf("band %d sample 8 = %v, want 0.0625", band, got)
		}

		for index, value := range resolved.Bands[band] {
			if index != 8 && value != 0 {
				t.Fatalf("band %d sample %d = %v, want silence", band, index, value)
			}
		}
	}
}

func TestPathChainResolveSkipsInactiveBands(t *testing.T) {
	t.Parallel()

	chain := PathChain{
		Factors: []*ir.BandedResponse{
			impulseResponse(3, 8, 1, 1),
			impulseResponse(3, 8, 1, 1),
		},
		PortalGains: []ScalarFilter{{1, 1, 1}},
		ActiveBands: []bool{true, false, true},
	}

	resolved, err := chain.Resolve(0)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	for _, value := range resolved.Bands[1] {
		if value != 0 {
			t.Fatal("an eliminated band must stay exactly zero")
		}
	}

	for _, band := range []int{0, 2} {
		if resolved.Bands[band][2] == 0 {
			t.Fatalf("band %d lost its contribution", band)
		}
	}
}

func TestPathChainResolveRejectsMismatchedInput(t *testing.T) {
	t.Parallel()

	_, err := PathChain{}.Resolve(0)
	if err == nil {
		t.Fatal("Resolve accepted an empty chain")
	}

	_, err = PathChain{
		Factors:     []*ir.BandedResponse{impulseResponse(2, 4, 0, 1), impulseResponse(2, 4, 0, 1)},
		PortalGains: nil,
	}.Resolve(0)
	if err == nil {
		t.Fatal("Resolve accepted a chain missing its portal gain")
	}

	_, err = PathChain{
		Factors:     []*ir.BandedResponse{impulseResponse(2, 4, 0, 1), impulseResponse(2, 4, 0, 1)},
		PortalGains: []ScalarFilter{{1}},
	}.Resolve(0)
	if err == nil {
		t.Fatal("Resolve accepted a portal gain of the wrong width")
	}
}

func TestPathChainResolveHonoursMaxLength(t *testing.T) {
	t.Parallel()

	chain := PathChain{
		Factors:     []*ir.BandedResponse{impulseResponse(1, 16, 0, 1), impulseResponse(1, 16, 0, 1)},
		PortalGains: []ScalarFilter{{1}},
	}

	resolved, err := chain.Resolve(8)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if resolved.Len() != 8 {
		t.Fatalf("length = %d, want the requested 8", resolved.Len())
	}
}

func TestSumBandedResponsesZeroPadsToTheLongest(t *testing.T) {
	t.Parallel()

	short := impulseResponse(2, 4, 1, 1)
	long := impulseResponse(2, 9, 6, 2)

	summed, err := SumBandedResponses([]*ir.BandedResponse{short, nil, long})
	if err != nil {
		t.Fatalf("SumBandedResponses: %v", err)
	}

	if summed.Len() != 9 {
		t.Fatalf("length = %d, want 9", summed.Len())
	}

	if got := summed.Bands[0][1]; got != 1 {
		t.Fatalf("sample 1 = %v, want 1", got)
	}

	if got := summed.Bands[0][6]; got != 2 {
		t.Fatalf("sample 6 = %v, want 2", got)
	}

	_, err = SumBandedResponses(nil)
	if err == nil {
		t.Fatal("SumBandedResponses accepted an empty list")
	}
}

func TestPressureAndEnergyFiltersDifferBySquareRoot(t *testing.T) {
	t.Parallel()

	transmission := []float64{1, 0.25, 0.01}

	pressure := PressureFilterFromTransmission(transmission)
	energy := EnergyFilterFromTransmission(transmission)

	for band, tau := range transmission {
		if math.Abs(pressure[band]-math.Sqrt(tau)) > 1e-12 {
			t.Fatalf("band %d pressure filter = %v, want sqrt(%v)", band, pressure[band], tau)
		}

		if math.Abs(energy[band]-tau) > 1e-12 {
			t.Fatalf("band %d energy filter = %v, want %v", band, energy[band], tau)
		}
	}
}

func TestScaleHistogramAppliesPerBandGains(t *testing.T) {
	t.Parallel()

	histogram := raytrace.NewEnergyHistogram(0.1, 0.01, 2)
	histogram.Bins[3].BandEnergy[0] = 4
	histogram.Bins[3].BandEnergy[1] = 8

	scaled, err := ScaleHistogram(histogram, ScalarFilter{0.5, 0.25})
	if err != nil {
		t.Fatalf("ScaleHistogram: %v", err)
	}

	if got := scaled.Bins[3].BandEnergy[0]; math.Abs(got-2) > 1e-12 {
		t.Fatalf("band 0 = %v, want 2", got)
	}

	if got := scaled.Bins[3].BandEnergy[1]; math.Abs(got-2) > 1e-12 {
		t.Fatalf("band 1 = %v, want 2", got)
	}

	_, err = ScaleHistogram(histogram, ScalarFilter{1})
	if err == nil {
		t.Fatal("ScaleHistogram accepted the wrong number of gains")
	}
}

func TestConvolveHistogramsAddsDelays(t *testing.T) {
	t.Parallel()

	first := raytrace.NewEnergyHistogram(0.1, 0.01, 1)
	first.Bins[2].BandEnergy[0] = 3

	second := raytrace.NewEnergyHistogram(0.1, 0.01, 1)
	second.Bins[4].BandEnergy[0] = 5

	composed, err := ConvolveHistograms(first, second)
	if err != nil {
		t.Fatalf("ConvolveHistograms: %v", err)
	}

	if got := composed.Bins[6].BandEnergy[0]; math.Abs(got-15) > 1e-12 {
		t.Fatalf("bin 6 = %v, want 15 at the summed delay", got)
	}

	mismatched := raytrace.NewEnergyHistogram(0.1, 0.02, 1)

	_, err = ConvolveHistograms(first, mismatched)
	if err == nil {
		t.Fatal("ConvolveHistograms accepted differing bin durations")
	}
}

func TestAttenuationFloorMaskAccumulatesAcrossPortals(t *testing.T) {
	t.Parallel()

	// Three portals at 0.01 each accumulate to 1e-6, which is -120 dB in
	// pressure and therefore below a -60 dB floor.
	gains := []ScalarFilter{{1, 0.01}, {1, 0.01}, {1, 0.01}}

	active := AttenuationFloorMask(gains, DefaultBandFloorDB)
	if !active[0] {
		t.Fatal("an unattenuated band must survive the floor")
	}

	if active[1] {
		t.Fatal("a band 120 dB down must be eliminated")
	}

	if got := AttenuationFloorMask(nil, DefaultBandFloorDB); got != nil {
		t.Fatalf("AttenuationFloorMask(nil) = %v, want nil", got)
	}
}

// TestAlignLateTailBufferMatchesEarlyEnergy pins the property the buffer
// variant guarantees: after alignment, the late tail's RMS just after the
// crossover equals the early field's RMS just before it.
//
// It is deliberately not compared against the event-based AlignLateTail. That
// one averages over events while this one averages over samples, so the two
// normalisations differ by the sparsity of the event list; each is
// self-consistent, but they are not interchangeable measures.
func TestAlignLateTailBufferMatchesEarlyEnergy(t *testing.T) {
	t.Parallel()

	const (
		sampleRate = 48000
		length     = 4800
	)

	cfg := HybridConfig{CrossoverTimeSeconds: 0.05, CrossoverMode: TimeBased}

	early := &ir.Buffer{SampleRate: sampleRate, Samples: make([]float64, length)}
	for index := range early.Samples {
		early.Samples[index] = 0.4
	}

	late := &ir.Buffer{SampleRate: sampleRate, Samples: make([]float64, length)}
	for index := range late.Samples {
		late.Samples[index] = 0.05
	}

	aligned := AlignLateTailBuffer(late, early, cfg)
	if aligned == nil {
		t.Fatal("AlignLateTailBuffer returned nil")
	}

	// Both fields are constant, so alignment must scale the late one onto the
	// early level exactly.
	if got := aligned.Samples[length/2]; math.Abs(got-0.4) > 1e-9 {
		t.Fatalf("aligned late level = %v, want the early level 0.4", got)
	}

	// The original buffer must be left untouched.
	if late.Samples[0] != 0.05 {
		t.Fatal("AlignLateTailBuffer mutated its input")
	}
}

func TestAlignLateTailBufferHandlesMissingInput(t *testing.T) {
	t.Parallel()

	if got := AlignLateTailBuffer(nil, nil, HybridConfig{}); got != nil {
		t.Fatal("AlignLateTailBuffer(nil late) must return nil")
	}

	late := &ir.Buffer{SampleRate: 48000, Samples: []float64{1, 2, 3}}

	got := AlignLateTailBuffer(late, nil, HybridConfig{})
	if got == nil || got.Samples[0] != 1 {
		t.Fatal("a nil early buffer must leave the late buffer untouched")
	}
}
