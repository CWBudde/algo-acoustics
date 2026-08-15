package hybrid

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

func TestPortalCrossfadeWeight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		aperture  float64
		rootOrder float64
		want      float64
		wantError bool
	}{
		{name: "closed", aperture: 0, rootOrder: 2, want: 0},
		{name: "square root", aperture: 0.25, rootOrder: 2, want: 0.5},
		{name: "default square root", aperture: 0.25, want: 0.5},
		{name: "open", aperture: 1, rootOrder: 3, want: 1},
		{name: "clamp below", aperture: -1, rootOrder: 2, want: 0},
		{name: "clamp above", aperture: 2, rootOrder: 2, want: 1},
		{name: "negative root", aperture: 0.5, rootOrder: -1, wantError: true},
		{name: "non-finite aperture", aperture: math.Inf(1), rootOrder: 2, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := PortalCrossfadeWeight(test.aperture, test.rootOrder)
			if test.wantError {
				if err == nil {
					t.Fatal("PortalCrossfadeWeight() error = nil, want error")
				}

				return
			}

			if err != nil {
				t.Fatalf("PortalCrossfadeWeight() error = %v", err)
			}

			if math.Abs(got-test.want) > 1e-12 {
				t.Fatalf("PortalCrossfadeWeight() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPortalCrossfadeWeightIsMonotonic(t *testing.T) {
	t.Parallel()

	previous := -1.0

	for step := range 101 {
		weight, err := PortalCrossfadeWeight(float64(step)/100, 3)
		if err != nil {
			t.Fatalf("PortalCrossfadeWeight() error = %v", err)
		}

		if weight < previous {
			t.Fatalf("weight at step %d = %v, previous = %v", step, weight, previous)
		}

		previous = weight
	}
}

func TestPortalBRIRCacheCrossfadeAndHardOpen(t *testing.T) {
	t.Parallel()

	closed := testBRIR(1000, []float64{0.5, 0.5}, []float64{0.25, 0.25})
	mergedOpen := testBRIR(1000, []float64{1.5, 1.5}, []float64{1.25, 1.25})

	cache, err := NewPortalBRIRCache(closed, mergedOpen)
	if err != nil {
		t.Fatalf("NewPortalBRIRCache() error = %v", err)
	}

	closedResult, err := cache.AtAperture(0, 2)
	if err != nil {
		t.Fatalf("AtAperture(closed) error = %v", err)
	}

	if closedResult.Left.Samples[0] != 0.5 || closedResult.Right.Samples[0] != 0.25 {
		t.Fatalf("closed result = %#v/%#v", closedResult.Left.Samples, closedResult.Right.Samples)
	}

	middle, err := cache.AtAperture(0.25, 2)
	if err != nil {
		t.Fatalf("AtAperture(middle) error = %v", err)
	}

	if math.Abs(middle.Left.Samples[0]-1) > 1e-12 || math.Abs(middle.Right.Samples[0]-0.75) > 1e-12 {
		t.Fatalf("middle result = %#v/%#v", middle.Left.Samples, middle.Right.Samples)
	}

	openResult, err := cache.AtAperture(1, 2)
	if err != nil {
		t.Fatalf("AtAperture(open) error = %v", err)
	}

	if openResult.Left.Samples[0] != mergedOpen.Left.Samples[0] ||
		openResult.Right.Samples[0] != mergedOpen.Right.Samples[0] {
		t.Fatalf("open result = %#v/%#v, want merged response", openResult.Left.Samples, openResult.Right.Samples)
	}
}

func TestPortalBRIRCacheEnergyIncreasesMonotonically(t *testing.T) {
	t.Parallel()

	cache, err := NewPortalBRIRCache(
		testBRIR(1000, []float64{0.25, 0.25}, []float64{0.25, 0.25}),
		testBRIR(1000, []float64{1, 1}, []float64{1, 1}),
	)
	if err != nil {
		t.Fatalf("NewPortalBRIRCache() error = %v", err)
	}

	previousEnergy := -1.0

	for step := range 101 {
		response, apertureErr := cache.AtAperture(float64(step)/100, 2)
		if apertureErr != nil {
			t.Fatalf("AtAperture() error = %v", apertureErr)
		}

		energy := portalBRIREnergy(response)
		if energy+1e-12 < previousEnergy {
			t.Fatalf("energy at step %d = %v, previous = %v", step, energy, previousEnergy)
		}

		previousEnergy = energy
	}
}

func TestPortalBRIRCacheIsContinuousAtOpenEndpoint(t *testing.T) {
	t.Parallel()

	cache, err := NewPortalBRIRCache(
		testBRIR(1000, []float64{0.5}, []float64{0.25}),
		testBRIR(1000, []float64{1.5}, []float64{1.25}),
	)
	if err != nil {
		t.Fatalf("NewPortalBRIRCache() error = %v", err)
	}

	almostOpen, err := cache.AtAperture(1-1e-12, 2)
	if err != nil {
		t.Fatalf("AtAperture(almost open) error = %v", err)
	}

	open, err := cache.AtAperture(1, 2)
	if err != nil {
		t.Fatalf("AtAperture(open) error = %v", err)
	}

	if math.Abs(almostOpen.Left.Samples[0]-open.Left.Samples[0]) > 1e-9 ||
		math.Abs(almostOpen.Right.Samples[0]-open.Right.Samples[0]) > 1e-9 {
		t.Fatalf("open endpoint discontinuity: almost=%#v/%#v open=%#v/%#v",
			almostOpen.Left.Samples, almostOpen.Right.Samples, open.Left.Samples, open.Right.Samples)
	}
}

func TestPortalBRIRCachePadsAndDefensivelyCopies(t *testing.T) {
	t.Parallel()

	closed := testBRIR(1000, []float64{1}, []float64{2, 3})
	mergedOpen := testBRIR(1000, []float64{4, 5, 6}, []float64{7})

	cache, err := NewPortalBRIRCache(closed, mergedOpen)
	if err != nil {
		t.Fatalf("NewPortalBRIRCache() error = %v", err)
	}

	closed.Left.Samples[0] = 99
	mergedOpen.Right.Samples[0] = 99

	first, err := cache.AtAperture(0.25, 2)
	if err != nil {
		t.Fatalf("AtAperture() error = %v", err)
	}

	if first.Left.Len() != 3 || first.Right.Len() != 3 {
		t.Fatalf("padded channel lengths = %d/%d, want 3/3", first.Left.Len(), first.Right.Len())
	}

	if math.Abs(first.Left.Samples[0]-2.5) > 1e-12 || math.Abs(first.Right.Samples[0]-4.5) > 1e-12 {
		t.Fatalf("cache was affected by input mutation: %#v/%#v", first.Left.Samples, first.Right.Samples)
	}

	first.Left.Samples[0] = 88

	second, err := cache.AtAperture(0.25, 2)
	if err != nil {
		t.Fatalf("AtAperture() second error = %v", err)
	}

	if math.Abs(second.Left.Samples[0]-2.5) > 1e-12 {
		t.Fatalf("cache was affected by output mutation: %#v", second.Left.Samples)
	}
}

func TestNewPortalBRIRCacheValidatesChannels(t *testing.T) {
	t.Parallel()

	valid := testBRIR(1000, []float64{1}, []float64{1})
	tests := []struct {
		name       string
		closed     BRIR
		mergedOpen BRIR
	}{
		{name: "nil channel", closed: BRIR{Left: valid.Left}, mergedOpen: valid},
		{
			name: "mismatched stereo rates",
			closed: BRIR{
				Left:  &ir.Buffer{SampleRate: 1000, Samples: []float64{1}},
				Right: &ir.Buffer{SampleRate: 2000, Samples: []float64{1}},
			},
			mergedOpen: valid,
		},
		{name: "mismatched state rates", closed: valid, mergedOpen: testBRIR(2000, []float64{1}, []float64{1})},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewPortalBRIRCache(test.closed, test.mergedOpen)
			if err == nil {
				t.Fatal("NewPortalBRIRCache() error = nil, want error")
			}
		})
	}
}

func testBRIR(sampleRate int, left, right []float64) BRIR {
	return BRIR{
		Left:  &ir.Buffer{SampleRate: sampleRate, Samples: append([]float64(nil), left...)},
		Right: &ir.Buffer{SampleRate: sampleRate, Samples: append([]float64(nil), right...)},
	}
}

func portalBRIREnergy(response BRIR) float64 {
	energy := 0.0
	for _, sample := range response.Left.Samples {
		energy += sample * sample
	}

	for _, sample := range response.Right.Samples {
		energy += sample * sample
	}

	return energy
}

// TestAtApertureMergedApproachesTheMergedResponseContinuously is the guard
// against the click the hard switch would have produced.
//
// The all-pass and merged responses are level-matched but not sample-matched,
// so a switch at aperture 1 would step from one to the other in a single
// buffer. The crossfade must instead leave the output arbitrarily close to the
// merged response just below full aperture.
func TestAtApertureMergedApproachesTheMergedResponseContinuously(t *testing.T) {
	t.Parallel()

	const rate = 48000

	closed := testBRIR(rate, []float64{0.1, 0, 0, 0}, []float64{0.1, 0, 0, 0})
	allPass := testBRIR(rate, []float64{1, 0, 0, 0}, []float64{1, 0, 0, 0})
	// Same broadband level, entirely different arrival pattern: exactly the
	// case where matching levels does not make a switch inaudible.
	merged := testBRIR(rate, []float64{0, 0, 1, 0}, []float64{0, 0, 1, 0})

	cache, err := NewPortalBRIRCacheWithFilter(closed, allPass, merged)
	if err != nil {
		t.Fatalf("NewPortalBRIRCacheWithFilter: %v", err)
	}

	full, err := cache.AtApertureMerged(1, 2)
	if err != nil {
		t.Fatalf("AtApertureMerged(1): %v", err)
	}

	near, err := cache.AtApertureMerged(1-1e-6, 2)
	if err != nil {
		t.Fatalf("AtApertureMerged(1-eps): %v", err)
	}

	for index := range full.Left.Samples {
		if math.Abs(full.Left.Samples[index]-near.Left.Samples[index]) > 1e-4 {
			t.Fatalf("sample %d jumps from %v to %v at full aperture",
				index, near.Left.Samples[index], full.Left.Samples[index])
		}
	}

	// Well below the merge interval the output must still be the plain
	// closed-to-all-pass crossfade, untouched by the merged response.
	plain, err := cache.AtAperture(0.5, 2)
	if err != nil {
		t.Fatalf("AtAperture(0.5): %v", err)
	}

	blended, err := cache.AtApertureMerged(0.5, 2)
	if err != nil {
		t.Fatalf("AtApertureMerged(0.5): %v", err)
	}

	for index := range plain.Left.Samples {
		if math.Abs(plain.Left.Samples[index]-blended.Left.Samples[index]) > 1e-12 {
			t.Fatalf("sample %d differs at half aperture: %v vs %v",
				index, plain.Left.Samples[index], blended.Left.Samples[index])
		}
	}
}

// TestAtApertureMergedIsSampleContinuousAcrossTheSweep walks the whole aperture
// range and bounds the step between adjacent samples, which is what an audible
// click would show up as.
func TestAtApertureMergedIsSampleContinuousAcrossTheSweep(t *testing.T) {
	t.Parallel()

	const rate = 48000

	cache, err := NewPortalBRIRCacheWithFilter(
		testBRIR(rate, []float64{0.1, 0, 0, 0}, []float64{0.1, 0, 0, 0}),
		testBRIR(rate, []float64{1, 0, 0, 0}, []float64{1, 0, 0, 0}),
		testBRIR(rate, []float64{0, 0, 1, 0}, []float64{0, 0, 1, 0}),
	)
	if err != nil {
		t.Fatalf("NewPortalBRIRCacheWithFilter: %v", err)
	}

	const steps = 200

	previous, err := cache.AtApertureMerged(0, 2)
	if err != nil {
		t.Fatalf("AtApertureMerged(0): %v", err)
	}

	for step := 1; step <= steps; step++ {
		aperture := float64(step) / steps

		current, err := cache.AtApertureMerged(aperture, 2)
		if err != nil {
			t.Fatalf("AtApertureMerged(%v): %v", aperture, err)
		}

		for index := range current.Left.Samples {
			// One step of the sweep may move a sample by at most the full
			// span of the responses divided by the number of steps, with
			// generous headroom for the square-root crossfade curve.
			if delta := math.Abs(current.Left.Samples[index] - previous.Left.Samples[index]); delta > 0.2 {
				t.Fatalf("sample %d steps by %v at aperture %v", index, delta, aperture)
			}
		}

		previous = current
	}
}
