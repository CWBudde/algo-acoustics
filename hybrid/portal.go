package hybrid

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/ir"
)

const defaultPortalCrossfadeRootOrder = 2.0

// BRIR stores the left and right channels of a binaural room impulse response.
type BRIR struct {
	Left  *ir.Buffer
	Right *ir.Buffer
}

// PortalBRIRCache retains immutable copies of the responses for a fully closed
// portal and the fully open, merged room group.
type PortalBRIRCache struct {
	closed     BRIR
	mergedOpen BRIR
}

// NewPortalBRIRCache caches closed and fully open BRIRs. All channels are
// zero-padded to a common length so changing aperture never changes the output
// shape. Inputs are copied and may be safely reused or modified by the caller.
func NewPortalBRIRCache(closed, mergedOpen BRIR) (*PortalBRIRCache, error) {
	err := validatePortalBRIR("closed", closed)
	if err != nil {
		return nil, err
	}

	err = validatePortalBRIR("merged open", mergedOpen)
	if err != nil {
		return nil, err
	}

	sampleRate := closed.Left.SampleRate
	if mergedOpen.Left.SampleRate != sampleRate {
		return nil, fmt.Errorf("portal BRIR sample rates must match: closed=%d, merged open=%d", sampleRate, mergedOpen.Left.SampleRate)
	}

	length := max(closed.Left.Len(), closed.Right.Len(), mergedOpen.Left.Len(), mergedOpen.Right.Len())

	return &PortalBRIRCache{
		closed:     clonePortalBRIR(closed, sampleRate, length),
		mergedOpen: clonePortalBRIR(mergedOpen, sampleRate, length),
	}, nil
}

// PortalCrossfadeWeight maps relative portal aperture to interpolation weight
// using y(x) = x^(1/n). Aperture is clamped to [0, 1]. A zero root order uses
// the default square-root curve (n=2).
func PortalCrossfadeWeight(aperture, rootOrder float64) (float64, error) {
	if math.IsNaN(aperture) || math.IsInf(aperture, 0) {
		return 0, errors.New("portal aperture must be finite")
	}

	if rootOrder == 0 {
		rootOrder = defaultPortalCrossfadeRootOrder
	}

	if math.IsNaN(rootOrder) || math.IsInf(rootOrder, 0) || rootOrder < 0 {
		return 0, errors.New("portal crossfade root order must be finite and positive")
	}

	aperture = min(max(aperture, 0), 1)

	return math.Pow(aperture, 1/rootOrder), nil
}

// AtAperture returns a newly allocated BRIR for the requested relative portal
// aperture. Intermediate apertures interpolate cached samples. A fully open
// portal returns an exact copy of the merged-room response.
func (c *PortalBRIRCache) AtAperture(aperture, rootOrder float64) (BRIR, error) {
	if c == nil {
		return BRIR{}, errors.New("portal BRIR cache is nil")
	}

	weight, err := PortalCrossfadeWeight(aperture, rootOrder)
	if err != nil {
		return BRIR{}, err
	}

	if aperture <= 0 {
		return clonePortalBRIR(c.closed, c.closed.Left.SampleRate, c.closed.Left.Len()), nil
	}

	if aperture >= 1 {
		return clonePortalBRIR(c.mergedOpen, c.mergedOpen.Left.SampleRate, c.mergedOpen.Left.Len()), nil
	}

	return BRIR{
		Left:  interpolatePortalBuffer(c.closed.Left, c.mergedOpen.Left, weight),
		Right: interpolatePortalBuffer(c.closed.Right, c.mergedOpen.Right, weight),
	}, nil
}

func validatePortalBRIR(name string, response BRIR) error {
	if response.Left == nil || response.Right == nil {
		return fmt.Errorf("%s portal BRIR channels must not be nil", name)
	}

	if response.Left.SampleRate <= 0 || response.Right.SampleRate <= 0 {
		return fmt.Errorf("%s portal BRIR sample rates must be positive", name)
	}

	if response.Left.SampleRate != response.Right.SampleRate {
		return fmt.Errorf("%s portal BRIR channel sample rates must match", name)
	}

	return nil
}

func clonePortalBRIR(response BRIR, sampleRate, length int) BRIR {
	return BRIR{
		Left:  clonePortalBuffer(response.Left, sampleRate, length),
		Right: clonePortalBuffer(response.Right, sampleRate, length),
	}
}

func clonePortalBuffer(buffer *ir.Buffer, sampleRate, length int) *ir.Buffer {
	out := &ir.Buffer{SampleRate: sampleRate, Samples: make([]float64, length)}
	copy(out.Samples, buffer.Samples)

	return out
}

func interpolatePortalBuffer(closed, mergedOpen *ir.Buffer, weight float64) *ir.Buffer {
	out := &ir.Buffer{SampleRate: closed.SampleRate, Samples: make([]float64, closed.Len())}
	for index := range out.Samples {
		out.Samples[index] = closed.Samples[index] + weight*(mergedOpen.Samples[index]-closed.Samples[index])
	}

	return out
}
