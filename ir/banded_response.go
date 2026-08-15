package ir

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/acoustics"
	algofft "github.com/cwbudde/algo-fft"
)

// BandedResponse holds one *unfiltered* impulse response per frequency band.
//
// It exists so that a multi-room propagation path can be composed as a product
// of separately simulated transfer functions, as docs/raven.md section 5.2
// describes. Composing hops by re-expanding events instead would cost one full
// solve per event per hop, which is exponential in the number of hops.
//
// The bandpass weighting is deliberately *not* applied to the stored bands. It
// is applied exactly once, when the response is finally rendered to a wideband
// buffer. Applying it per factor would band-limit the same signal once per hop
// and progressively hollow out the band edges.
type BandedResponse struct {
	SampleRate int
	Bands      [][]float64
}

// NewBandedResponse allocates a zeroed response.
func NewBandedResponse(sampleRate, bandCount, length int) *BandedResponse {
	bands := make([][]float64, bandCount)
	for index := range bands {
		bands[index] = make([]float64, length)
	}

	return &BandedResponse{SampleRate: sampleRate, Bands: bands}
}

// BandCount returns the number of frequency bands.
func (r *BandedResponse) BandCount() int {
	if r == nil {
		return 0
	}

	return len(r.Bands)
}

// Len returns the length of each band in samples.
func (r *BandedResponse) Len() int {
	if r == nil || len(r.Bands) == 0 {
		return 0
	}

	return len(r.Bands[0])
}

// Clone returns a deep copy.
func (r *BandedResponse) Clone() *BandedResponse {
	if r == nil {
		return nil
	}

	out := &BandedResponse{SampleRate: r.SampleRate, Bands: make([][]float64, len(r.Bands))}
	for index, band := range r.Bands {
		out.Bands[index] = append([]float64(nil), band...)
	}

	return out
}

// BandedFromEvents builds one impulse train per band from sparse events.
//
// Wideband events, which carry no BandGain, are placed in every band at full
// amplitude. Because the bandpass weights form a partition of unity, summing
// the weighted bands reproduces exactly the flat spectrum such an event should
// have.
func BandedFromEvents(events []Event, cfg RenderConfig) (*BandedResponse, error) {
	bandCount := cfg.BandSpec.BandCount()
	if bandCount == 0 {
		return nil, errors.New("banded response requires a band spec")
	}

	if cfg.SampleRate <= 0 {
		return nil, errors.New("banded response requires a positive sample rate")
	}

	err := validateBandedEvents(events, bandCount)
	if err != nil {
		return nil, err
	}

	length := NewBuffer(cfg.SampleRate, cfg.DurationSeconds).Len()
	out := NewBandedResponse(cfg.SampleRate, bandCount, length)

	for _, event := range events {
		sampleIndex := int(math.Round(event.TimeSeconds * float64(cfg.SampleRate)))
		if sampleIndex < 0 || sampleIndex >= length {
			continue
		}

		amplitude := event.Amplitude * math.Cos(event.PhaseRadians)

		for band := range bandCount {
			gain := amplitude
			if len(event.BandGain) > 0 {
				gain *= event.BandGain[band]
			}

			out.Bands[band][sampleIndex] += gain
		}
	}

	return out, nil
}

// ApplyBandGains multiplies each band by a scalar, which is how a portal filter
// enters the chain.
func (r *BandedResponse) ApplyBandGains(gains []float64) error {
	if r == nil {
		return errors.New("banded response is nil")
	}

	if len(gains) != len(r.Bands) {
		return fmt.Errorf("band gain count = %d, want %d", len(gains), len(r.Bands))
	}

	for band, gain := range gains {
		for index := range r.Bands[band] {
			r.Bands[band][index] *= gain
		}
	}

	return nil
}

// ActiveBands reports which bands carry energy within floorDB of the strongest
// band. Bands below that floor can be skipped entirely by the filter network.
func (r *BandedResponse) ActiveBands(floorDB float64) []bool {
	if r == nil {
		return nil
	}

	peaks := make([]float64, len(r.Bands))
	strongest := 0.0

	for band, samples := range r.Bands {
		for _, sample := range samples {
			peaks[band] = math.Max(peaks[band], math.Abs(sample))
		}

		strongest = math.Max(strongest, peaks[band])
	}

	active := make([]bool, len(r.Bands))
	if strongest <= 0 {
		return active
	}

	threshold := strongest * math.Pow(10, floorDB/20)
	for band, peak := range peaks {
		active[band] = peak > threshold
	}

	return active
}

// RenderBandedMono applies the bandpass weights once and sums the bands into a
// wideband impulse response.
//
// This shares buildBandpassWeights with renderMonoBanded rather than being
// routed through it: the event-based path is a well-covered hot path with
// pinned regression output, and reordering its summation would perturb that
// output for no gain. TestRenderBandedMonoMatchesRenderMono pins that the two
// agree.
func RenderBandedMono(response *BandedResponse, spec acoustics.BandSpec, length int) (*Buffer, error) {
	if response == nil || len(response.Bands) == 0 {
		return nil, errors.New("banded response is empty")
	}

	if spec.BandCount() != len(response.Bands) {
		return nil, fmt.Errorf("band spec has %d bands, response has %d", spec.BandCount(), len(response.Bands))
	}

	if length <= 0 {
		length = response.Len()
	}

	fftSize := nextPow2(2 * max(length, response.Len()))

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("create FFT plan: %w", err)
	}

	weights := buildBandpassWeights(spec, fftSize, response.SampleRate)
	combined := make([]complex128, fftSize)
	scratch := make([]complex128, fftSize)
	bandTime := make([]complex128, fftSize)

	for band, samples := range response.Bands {
		clear(bandTime)

		for index, sample := range samples {
			bandTime[index] = complex(sample, 0)
		}

		err = plan.Forward(scratch, bandTime)
		if err != nil {
			return nil, fmt.Errorf("FFT band %d: %w", band, err)
		}

		for bin := range combined {
			combined[bin] += scratch[bin] * complex(weights[band][bin], 0)
		}
	}

	timeDomain := make([]complex128, fftSize)

	err = plan.Inverse(timeDomain, combined)
	if err != nil {
		return nil, fmt.Errorf("IFFT: %w", err)
	}

	out := &Buffer{SampleRate: response.SampleRate, Samples: make([]float64, length)}
	for index := range out.Samples {
		out.Samples[index] = real(timeDomain[index])
	}

	return out, nil
}
