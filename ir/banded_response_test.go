package ir

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
)

func bandedTestConfig() RenderConfig {
	return RenderConfig{SampleRate: 48000, DurationSeconds: 0.05, BandSpec: acoustics.Octave6}
}

func bandedTestEvents() []Event {
	return []Event{
		{
			TimeSeconds: 0.001, Amplitude: 1.0, DistanceMeters: 0.343,
			Direction: geometry.Vec3{X: 1},
			BandGain:  []float64{1, 0.9, 0.8, 0.7, 0.6, 0.5},
			Kind:      EventDirect,
		},
		{
			TimeSeconds: 0.004, Amplitude: 0.5, DistanceMeters: 1.372,
			Direction: geometry.Vec3{X: -1},
			BandGain:  []float64{0.3, 0.4, 0.5, 0.4, 0.3, 0.2},
			Kind:      EventSpecular,
		},
		{
			TimeSeconds: 0.009, Amplitude: 0.25, DistanceMeters: 3.087,
			Direction:    geometry.Vec3{Y: 1},
			BandGain:     []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
			PhaseRadians: math.Pi,
			Kind:         EventSpecular,
		},
	}
}

// TestRenderBandedMonoMatchesRenderMono pins the invariant the whole filter
// network rests on: the bandpass weighting is applied exactly once, so routing
// events through a BandedResponse yields the same wideband impulse response as
// rendering them directly.
func TestRenderBandedMonoMatchesRenderMono(t *testing.T) {
	t.Parallel()

	cfg := bandedTestConfig()
	events := bandedTestEvents()

	direct, err := RenderMono(events, cfg)
	if err != nil {
		t.Fatalf("RenderMono: %v", err)
	}

	response, err := BandedFromEvents(events, cfg)
	if err != nil {
		t.Fatalf("BandedFromEvents: %v", err)
	}

	banded, err := RenderBandedMono(response, cfg.BandSpec, direct.Len())
	if err != nil {
		t.Fatalf("RenderBandedMono: %v", err)
	}

	if banded.Len() != direct.Len() {
		t.Fatalf("length = %d, want %d", banded.Len(), direct.Len())
	}

	for index := range direct.Samples {
		if math.Abs(direct.Samples[index]-banded.Samples[index]) > 1e-9 {
			t.Fatalf("sample %d: banded %v, direct %v", index, banded.Samples[index], direct.Samples[index])
		}
	}
}

// TestRenderBandedMonoMatchesRenderMonoForWidebandEvents covers the events that
// carry no BandGain. They are placed in every band, and because the bandpass
// weights form a partition of unity the sum reproduces the flat spectrum.
func TestRenderBandedMonoMatchesRenderMonoForWidebandEvents(t *testing.T) {
	t.Parallel()

	cfg := bandedTestConfig()

	events := append(bandedTestEvents(), Event{
		TimeSeconds: 0.006, Amplitude: 0.4, DistanceMeters: 2.058,
		Direction: geometry.Vec3{Z: 1}, Kind: EventDiffuse,
	})

	direct, err := RenderMono(events, cfg)
	if err != nil {
		t.Fatalf("RenderMono: %v", err)
	}

	response, err := BandedFromEvents(events, cfg)
	if err != nil {
		t.Fatalf("BandedFromEvents: %v", err)
	}

	banded, err := RenderBandedMono(response, cfg.BandSpec, direct.Len())
	if err != nil {
		t.Fatalf("RenderBandedMono: %v", err)
	}

	for index := range direct.Samples {
		if math.Abs(direct.Samples[index]-banded.Samples[index]) > 1e-9 {
			t.Fatalf("sample %d: banded %v, direct %v", index, banded.Samples[index], direct.Samples[index])
		}
	}
}

func TestBandedFromEventsPlacesImpulsesAtTheRightSamples(t *testing.T) {
	t.Parallel()

	cfg := bandedTestConfig()

	response, err := BandedFromEvents([]Event{{
		TimeSeconds: 0.002, Amplitude: 2, BandGain: []float64{1, 0, 0, 0, 0, 0},
	}}, cfg)
	if err != nil {
		t.Fatalf("BandedFromEvents: %v", err)
	}

	index := int(math.Round(0.002 * 48000))
	if got := response.Bands[0][index]; math.Abs(got-2) > 1e-12 {
		t.Fatalf("band 0 sample %d = %v, want 2", index, got)
	}

	if got := response.Bands[1][index]; got != 0 {
		t.Fatalf("band 1 sample %d = %v, want 0", index, got)
	}
}

func TestBandedFromEventsRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cfg := bandedTestConfig()

	_, err := BandedFromEvents(nil, RenderConfig{SampleRate: 48000, DurationSeconds: 0.05})
	if err == nil {
		t.Fatal("BandedFromEvents accepted a config without a band spec")
	}

	_, err = BandedFromEvents(nil, RenderConfig{DurationSeconds: 0.05, BandSpec: acoustics.Octave6})
	if err == nil {
		t.Fatal("BandedFromEvents accepted a zero sample rate")
	}

	_, err = BandedFromEvents([]Event{{BandGain: []float64{1, 2}}}, cfg)
	if err == nil {
		t.Fatal("BandedFromEvents accepted a band gain of the wrong length")
	}
}

func TestApplyBandGainsScalesEachBand(t *testing.T) {
	t.Parallel()

	response := NewBandedResponse(48000, 3, 4)
	for band := range response.Bands {
		response.Bands[band][1] = 1
	}

	err := response.ApplyBandGains([]float64{0.5, 0.25, 0})
	if err != nil {
		t.Fatalf("ApplyBandGains: %v", err)
	}

	for band, want := range []float64{0.5, 0.25, 0} {
		if got := response.Bands[band][1]; math.Abs(got-want) > 1e-12 {
			t.Fatalf("band %d = %v, want %v", band, got, want)
		}
	}

	err = response.ApplyBandGains([]float64{1})
	if err == nil {
		t.Fatal("ApplyBandGains accepted the wrong number of gains")
	}
}

func TestActiveBandsRespectsTheFloor(t *testing.T) {
	t.Parallel()

	response := NewBandedResponse(48000, 3, 4)
	response.Bands[0][0] = 1
	response.Bands[1][0] = 0.5  // -6 dB
	response.Bands[2][0] = 1e-4 // -80 dB

	active := response.ActiveBands(-60)
	if !active[0] || !active[1] {
		t.Fatalf("bands above the floor were eliminated: %v", active)
	}

	if active[2] {
		t.Fatal("a band 80 dB down must be eliminated at a -60 dB floor")
	}

	silent := NewBandedResponse(48000, 2, 4).ActiveBands(-60)
	for band, value := range silent {
		if value {
			t.Fatalf("band %d of a silent response is active", band)
		}
	}
}

func TestBandedResponseCloneIsDeep(t *testing.T) {
	t.Parallel()

	response := NewBandedResponse(48000, 2, 3)
	response.Bands[0][0] = 1

	clone := response.Clone()
	clone.Bands[0][0] = 2

	if response.Bands[0][0] != 1 {
		t.Fatal("Clone shares its band storage with the original")
	}

	if clone.BandCount() != 2 || clone.Len() != 3 {
		t.Fatalf("clone shape = %dx%d, want 2x3", clone.BandCount(), clone.Len())
	}
}
