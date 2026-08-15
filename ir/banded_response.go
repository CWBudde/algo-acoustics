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
//
// Bands holds the in-phase component and Quadrature the optional out-of-phase
// one, so a factor is a complex signal A*e^(i*phi) rather than its real
// projection A*cos(phi). Convolution has to compose phases additively —
// cos(phi1+phi2), not cos(phi1)*cos(phi2) — and only the complex form does. The
// real projection is taken once, when the composed response is rendered.
//
// Quadrature is nil whenever every contributing event has zero phase, which is
// everything except ISM diffraction. That case keeps the cheap real-only
// convolution.
type BandedResponse struct {
	SampleRate int
	Bands      [][]float64
	Quadrature [][]float64
}

// NewBandedResponse allocates a zeroed response with no quadrature component.
func NewBandedResponse(sampleRate, bandCount, length int) *BandedResponse {
	return &BandedResponse{SampleRate: sampleRate, Bands: allocBands(bandCount, length)}
}

// NewComplexBandedResponse allocates a zeroed response that also carries a
// quadrature component.
func NewComplexBandedResponse(sampleRate, bandCount, length int) *BandedResponse {
	return &BandedResponse{
		SampleRate: sampleRate,
		Bands:      allocBands(bandCount, length),
		Quadrature: allocBands(bandCount, length),
	}
}

func allocBands(bandCount, length int) [][]float64 {
	bands := make([][]float64, bandCount)
	for index := range bands {
		bands[index] = make([]float64, length)
	}

	return bands
}

// HasQuadrature reports whether the response carries an out-of-phase component
// that convolution must preserve.
func (r *BandedResponse) HasQuadrature() bool {
	return r != nil && len(r.Quadrature) > 0
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

	out := &BandedResponse{SampleRate: r.SampleRate, Bands: cloneBands(r.Bands)}
	if r.HasQuadrature() {
		out.Quadrature = cloneBands(r.Quadrature)
	}

	return out
}

func cloneBands(bands [][]float64) [][]float64 {
	out := make([][]float64, len(bands))
	for index, band := range bands {
		out[index] = append([]float64(nil), band...)
	}

	return out
}

// BandedFromEvents builds one impulse train per band from sparse events.
//
// Wideband events, which carry no BandGain, are placed in every band at full
// amplitude. Because the bandpass weights form a partition of unity, summing
// the weighted bands reproduces exactly the flat spectrum such an event should
// have.
//
// An event with a nonzero phase contributes A*cos(phi) in phase and A*sin(phi)
// in quadrature, so that convolving factors composes phases additively. The
// quadrature bands are allocated only when some event actually carries phase,
// which keeps the common phase-free case on the real-only path and its output
// bit-identical.
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
	if eventsCarryPhase(events) {
		out = NewComplexBandedResponse(cfg.SampleRate, bandCount, length)
	}

	for _, event := range events {
		sampleIndex := int(math.Round(event.TimeSeconds * float64(cfg.SampleRate)))
		if sampleIndex < 0 || sampleIndex >= length {
			continue
		}

		inPhase := event.Amplitude * math.Cos(event.PhaseRadians)
		quadrature := event.Amplitude * math.Sin(event.PhaseRadians)

		for band := range bandCount {
			weight := 1.0
			if len(event.BandGain) > 0 {
				weight = event.BandGain[band]
			}

			out.Bands[band][sampleIndex] += inPhase * weight
			if out.HasQuadrature() {
				out.Quadrature[band][sampleIndex] += quadrature * weight
			}
		}
	}

	return out, nil
}

// eventsCarryPhase reports whether any event needs the quadrature component.
// Only ISM diffraction produces phase, so this is false for most renders.
//
// The comparison is against a tolerance rather than zero because an inverting
// phase of pi has a sine of 1e-16, not 0, and a whole render should not switch
// to the complex representation over that.
func eventsCarryPhase(events []Event) bool {
	const quadratureEpsilon = 1e-12

	for _, event := range events {
		if math.Abs(math.Sin(event.PhaseRadians))*math.Abs(event.Amplitude) > quadratureEpsilon {
			return true
		}
	}

	return false
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

		if r.HasQuadrature() {
			for index := range r.Quadrature[band] {
				r.Quadrature[band][index] *= gain
			}
		}
	}

	return nil
}

// ActiveBands reports which bands carry energy within floorDB of the strongest
// band. Bands below that floor can be skipped entirely by the filter network.
//
// The comparison is on complex magnitude, so a band whose energy sits entirely
// in quadrature is not mistaken for silence.
func (r *BandedResponse) ActiveBands(floorDB float64) []bool {
	if r == nil {
		return nil
	}

	peaks := make([]float64, len(r.Bands))
	strongest := 0.0

	for band, samples := range r.Bands {
		for index, sample := range samples {
			magnitude := math.Abs(sample)
			if r.HasQuadrature() {
				magnitude = math.Hypot(sample, r.Quadrature[band][index])
			}

			peaks[band] = math.Max(peaks[band], magnitude)
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
