package hybrid

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-dsp/dsp/conv"
)

// DefaultBandFloorDB is the level below which a band is considered fully
// attenuated and skipped. It matches the culling convention used elsewhere in
// the library.
const DefaultBandFloorDB = -60.0

// ScalarFilter is a per-band magnitude, the form a portal transfer function
// takes. docs/raven.md section 5.2 marks only the source and receiver factors
// of H_PP as complex; the portal and room-group factors are scalar per band.
type ScalarFilter []float64

// PathChain is one propagation path expressed as an ordered list of factors,
// the H_PP product of docs/raven.md section 5.2. Factors are room-group
// transfer functions ordered from the source side; PortalGains are the handoffs
// between them, so there is exactly one fewer gain than factor.
type PathChain struct {
	Factors     []*ir.BandedResponse
	PortalGains []ScalarFilter
	// ActiveBands comes from the path search. Bands it clears are never
	// transformed, which is where source elimination actually saves work.
	ActiveBands []bool
}

// Resolve folds the chain into a single banded response, convolving each factor
// with the running product and applying the portal gain at every handoff.
//
// Bands that ActiveBands clears are left at zero and never transformed. That is
// the point of source elimination: a band the portals already killed costs
// nothing to carry.
//
// Factors carrying a quadrature component are composed as complex signals, so
// that phases add rather than their cosines multiplying. The real projection
// belongs at render time, not here.
func (c PathChain) Resolve(maxLength int) (*ir.BandedResponse, error) {
	if len(c.Factors) == 0 {
		return nil, errors.New("path chain has no factors")
	}

	if len(c.PortalGains) != len(c.Factors)-1 {
		return nil, fmt.Errorf("path chain has %d factors but %d portal gains", len(c.Factors), len(c.PortalGains))
	}

	running := c.Factors[0].Clone()

	for index, next := range c.Factors[1:] {
		gains := c.PortalGains[index]
		if len(gains) != running.BandCount() {
			return nil, fmt.Errorf("portal gain %d has %d bands, want %d", index, len(gains), running.BandCount())
		}

		folded, err := c.convolveActive(running, next, gains, maxLength)
		if err != nil {
			return nil, err
		}

		running = folded
	}

	return running, nil
}

func (c PathChain) convolveActive(
	running, next *ir.BandedResponse,
	gains ScalarFilter,
	maxLength int,
) (*ir.BandedResponse, error) {
	if next.BandCount() != running.BandCount() {
		return nil, fmt.Errorf("factor has %d bands, want %d", next.BandCount(), running.BandCount())
	}

	length := running.Len() + next.Len() - 1
	if maxLength > 0 && length > maxLength {
		length = maxLength
	}

	complexChain := running.HasQuadrature() || next.HasQuadrature()

	out := ir.NewBandedResponse(running.SampleRate, running.BandCount(), length)
	if complexChain {
		out = ir.NewComplexBandedResponse(running.SampleRate, running.BandCount(), length)
	}

	for band := range running.Bands {
		if !c.bandActive(band) || gains[band] == 0 {
			continue
		}

		err := convolveBand(out, running, next, band, length, gains[band], complexChain)
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

// convolveBand folds one band of two factors. A complex chain needs the four
// real convolutions of (a+ib)*(c+id); the phase-free case keeps the single
// convolution it had before.
func convolveBand(out, running, next *ir.BandedResponse, band, length int, gain float64, complexChain bool) error {
	realPart, err := conv.Convolve(running.Bands[band], next.Bands[band])
	if err != nil {
		return fmt.Errorf("convolve band %d: %w", band, err)
	}

	if !complexChain {
		copyBandProduct(out.Bands[band], realPart, length, gain)

		return nil
	}

	runningImag := quadratureBand(running, band)
	nextImag := quadratureBand(next, band)

	crossImag, err := conv.Convolve(runningImag, nextImag)
	if err != nil {
		return fmt.Errorf("convolve band %d quadrature: %w", band, err)
	}

	firstCross, err := conv.Convolve(running.Bands[band], nextImag)
	if err != nil {
		return fmt.Errorf("convolve band %d in-phase with quadrature: %w", band, err)
	}

	secondCross, err := conv.Convolve(runningImag, next.Bands[band])
	if err != nil {
		return fmt.Errorf("convolve band %d quadrature with in-phase: %w", band, err)
	}

	for index := range min(len(realPart), len(crossImag)) {
		realPart[index] -= crossImag[index]
	}

	for index := range min(len(firstCross), len(secondCross)) {
		firstCross[index] += secondCross[index]
	}

	copyBandProduct(out.Bands[band], realPart, length, gain)
	copyBandProduct(out.Quadrature[band], firstCross, length, gain)

	return nil
}

func copyBandProduct(target, product []float64, length int, gain float64) {
	for index := range min(len(product), length) {
		target[index] = product[index] * gain
	}
}

// quadratureBand returns a factor's out-of-phase band, or a zero band of the
// same length when the factor is purely real.
func quadratureBand(response *ir.BandedResponse, band int) []float64 {
	if response.HasQuadrature() {
		return response.Quadrature[band]
	}

	return make([]float64, len(response.Bands[band]))
}

func (c PathChain) bandActive(band int) bool {
	if band >= len(c.ActiveBands) {
		return true
	}

	return c.ActiveBands[band]
}

// ConvolveBanded convolves two banded responses band by band.
func ConvolveBanded(a, b *ir.BandedResponse, maxLength int) (*ir.BandedResponse, error) {
	if a == nil || b == nil {
		return nil, errors.New("banded response is nil")
	}

	if a.BandCount() != b.BandCount() {
		return nil, fmt.Errorf("band counts differ: %d and %d", a.BandCount(), b.BandCount())
	}

	unity := make(ScalarFilter, a.BandCount())
	for index := range unity {
		unity[index] = 1
	}

	return PathChain{Factors: []*ir.BandedResponse{a, b}, PortalGains: []ScalarFilter{unity}}.Resolve(maxLength)
}

// SumBandedResponses adds responses band by band, zero-padding to the longest.
// This is how the contributions of several propagation paths combine.
func SumBandedResponses(responses []*ir.BandedResponse) (*ir.BandedResponse, error) {
	present := make([]*ir.BandedResponse, 0, len(responses))

	for _, response := range responses {
		if response != nil && response.BandCount() > 0 {
			present = append(present, response)
		}
	}

	if len(present) == 0 {
		return nil, errors.New("no responses to sum")
	}

	bandCount := present[0].BandCount()
	length := 0

	for _, response := range present {
		if response.BandCount() != bandCount {
			return nil, fmt.Errorf("band counts differ: %d and %d", response.BandCount(), bandCount)
		}

		length = max(length, response.Len())
	}

	out := ir.NewBandedResponse(present[0].SampleRate, bandCount, length)

	for _, response := range present {
		if response.HasQuadrature() && !out.HasQuadrature() {
			out.Quadrature = make([][]float64, bandCount)
			for band := range out.Quadrature {
				out.Quadrature[band] = make([]float64, length)
			}
		}

		for band := range response.Bands {
			for index, value := range response.Bands[band] {
				out.Bands[band][index] += value
			}

			if response.HasQuadrature() {
				for index, value := range response.Quadrature[band] {
					out.Quadrature[band][index] += value
				}
			}
		}
	}

	return out, nil
}

// ScaleHistogram multiplies each band of an energy histogram by a scalar. This
// is the energy-domain portal handoff, using tau where the pressure domain uses
// sqrt(tau).
func ScaleHistogram(h *raytrace.EnergyHistogram, gains ScalarFilter) (*raytrace.EnergyHistogram, error) {
	if h == nil {
		return nil, errors.New("energy histogram is nil")
	}

	if len(gains) != h.BandCount {
		return nil, fmt.Errorf("gain count = %d, want %d", len(gains), h.BandCount)
	}

	out := raytrace.NewEnergyHistogram(h.Duration, h.BinDuration, h.BandCount)
	for index := range h.Bins {
		if index >= len(out.Bins) {
			break
		}

		source := h.Bins[index].BandEnergy
		target := out.Bins[index].BandEnergy
		bandCount := min(len(source), len(target), len(gains))

		for band := range bandCount {
			target[band] = source[band] * gains[band] // #nosec G602 -- bandCount is the minimum length of all three slices.
		}
	}

	return out, nil
}

// ConvolveHistograms convolves two energy histograms band by band, which is how
// the late field of consecutive room groups composes. Both must share a bin
// duration and band count.
//
// Note the modelling consequence: an energy histogram carries no direction, so
// composing hops this way makes an intermediate room group a scalar-per-band
// filter. Only the terminal group retains true directionality.
func ConvolveHistograms(a, b *raytrace.EnergyHistogram) (*raytrace.EnergyHistogram, error) {
	if a == nil || b == nil {
		return nil, errors.New("energy histogram is nil")
	}

	if a.BandCount != b.BandCount {
		return nil, fmt.Errorf("band counts differ: %d and %d", a.BandCount, b.BandCount)
	}

	if math.Abs(a.BinDuration-b.BinDuration) > 1e-12 {
		return nil, fmt.Errorf("bin durations differ: %v and %v", a.BinDuration, b.BinDuration)
	}

	out := raytrace.NewEnergyHistogram(a.Duration, a.BinDuration, a.BandCount)

	for firstIndex := range a.Bins {
		for secondIndex := range b.Bins {
			target := firstIndex + secondIndex
			if target >= len(out.Bins) {
				break
			}

			for band := range a.BandCount {
				out.Bins[target].BandEnergy[band] += a.Bins[firstIndex].BandEnergy[band] * b.Bins[secondIndex].BandEnergy[band]
			}
		}
	}

	return out, nil
}

// AttenuationFloorMask reports which bands survive the accumulated product of a
// path's portal gains. A band the portals have already driven below floorDB can
// be skipped by the whole chain.
func AttenuationFloorMask(gains []ScalarFilter, floorDB float64) []bool {
	if len(gains) == 0 {
		return nil
	}

	accumulated := make([]float64, len(gains[0]))
	for index := range accumulated {
		accumulated[index] = 1
	}

	for _, filter := range gains {
		for band := range accumulated {
			if band < len(filter) {
				accumulated[band] *= filter[band]
			}
		}
	}

	threshold := math.Pow(10, floorDB/20)

	active := make([]bool, len(accumulated))
	for band, value := range accumulated {
		active[band] = math.Abs(value) > threshold
	}

	return active
}

// PressureFilterFromTransmission converts per-band energy transmission
// coefficients into the pressure-domain portal filter sqrt(tau).
func PressureFilterFromTransmission(transmission []float64) ScalarFilter {
	filter := make(ScalarFilter, len(transmission))
	for index, tau := range transmission {
		filter[index] = math.Sqrt(math.Max(tau, 0))
	}

	return filter
}

// EnergyFilterFromTransmission returns the energy-domain portal filter, which
// is the transmission coefficient itself.
func EnergyFilterFromTransmission(transmission []float64) ScalarFilter {
	filter := make(ScalarFilter, len(transmission))
	for index, tau := range transmission {
		filter[index] = math.Max(tau, 0)
	}

	return filter
}
