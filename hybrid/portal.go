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

// portalMergedLevelToleranceDB bounds how far the all-pass and merged responses
// may differ in broadband level before the transition between them would be
// audible as a level jump.
const portalMergedLevelToleranceDB = 1.5

// portalMergeCrossfadeSpan is the aperture interval over which the all-pass
// response gives way to the merged room-group response.
//
// docs/raven.md section 5.3 describes this last step as a hard switch, but
// matching broadband levels does not make two independent simulations sample-
// or phase-continuous: the merged response has different reflection times, so
// replacing one with the other in a single step is a discontinuity whatever
// their levels. Crossfading over a short interval instead keeps the whole
// aperture sweep continuous, and the level guard then only has to keep that
// interval short enough to stay imperceptible.
const portalMergeCrossfadeSpan = 0.05

// PortalBRIRCache retains immutable copies of the responses a portal
// crossfades between.
//
// Three states matter, following docs/raven.md section 5.3. The portal starts
// closed. As it opens, its sound reduction indices fall until the portal filter
// is an all-pass. Only near that endpoint does the response of the physically
// merged room group take over, which is a different simulation: an aperture in
// a shared wall, not a transparent partition.
type PortalBRIRCache struct {
	closed BRIR
	// allPass is the response with the portal filter fully transmissive but
	// the rooms still geometrically separate.
	allPass BRIR
	// merged is the response of the physically merged room group. Until Phase
	// 25.4 this was simply allPass, which docs/sound-transmission.md flagged as
	// unfinished.
	merged BRIR
}

// NewPortalBRIRCache caches closed and fully open BRIRs. All channels are
// zero-padded to a common length so changing aperture never changes the output
// shape. Inputs are copied and may be safely reused or modified by the caller.
//
// The open endpoint doubles as the merged-group response, which is what callers
// that have only two renders want. Use NewPortalBRIRCacheWithFilter to
// distinguish the all-pass portal from the physically merged room group.
func NewPortalBRIRCache(closed, mergedOpen BRIR) (*PortalBRIRCache, error) {
	return newPortalBRIRCache(closed, mergedOpen, mergedOpen, false)
}

// NewPortalBRIRCacheWithFilter caches all three portal states: closed, the
// all-pass portal filter, and the physically merged room group.
//
// It rejects an allPass and merged pair whose broadband levels differ by more
// than 1.5 dB. AtApertureMerged crossfades between them over a short aperture
// interval, which handles the sample-level discontinuity; the level guard
// handles the other half of the problem, since a crossfade that short cannot
// disguise a large level step.
func NewPortalBRIRCacheWithFilter(closed, allPass, merged BRIR) (*PortalBRIRCache, error) {
	return newPortalBRIRCache(closed, allPass, merged, true)
}

func newPortalBRIRCache(closed, allPass, merged BRIR, checkLevels bool) (*PortalBRIRCache, error) {
	for name, response := range map[string]BRIR{"closed": closed, "all-pass": allPass, "merged": merged} {
		err := validatePortalBRIR(name, response)
		if err != nil {
			return nil, err
		}
	}

	sampleRate := closed.Left.SampleRate
	if allPass.Left.SampleRate != sampleRate || merged.Left.SampleRate != sampleRate {
		return nil, fmt.Errorf(
			"portal BRIR sample rates must match: closed=%d, all-pass=%d, merged=%d",
			sampleRate, allPass.Left.SampleRate, merged.Left.SampleRate,
		)
	}

	if checkLevels {
		err := checkPortalSwitchLevels(allPass, merged)
		if err != nil {
			return nil, err
		}
	}

	length := max(
		closed.Left.Len(), closed.Right.Len(),
		allPass.Left.Len(), allPass.Right.Len(),
		merged.Left.Len(), merged.Right.Len(),
	)

	return &PortalBRIRCache{
		closed:  clonePortalBRIR(closed, sampleRate, length),
		allPass: clonePortalBRIR(allPass, sampleRate, length),
		merged:  clonePortalBRIR(merged, sampleRate, length),
	}, nil
}

// checkPortalSwitchLevels guards the hard switch at full aperture.
func checkPortalSwitchLevels(allPass, merged BRIR) error {
	allPassLevel := portalBRIRLevelDB(allPass)
	mergedLevel := portalBRIRLevelDB(merged)

	if math.IsInf(allPassLevel, -1) || math.IsInf(mergedLevel, -1) {
		return errors.New("portal BRIR states must carry energy to be crossfaded")
	}

	if diff := math.Abs(allPassLevel - mergedLevel); diff > portalMergedLevelToleranceDB {
		return fmt.Errorf(
			"all-pass and merged portal responses differ by %.2f dB, more than the %.1f dB that keeps the "+
				"hard switch at full aperture inaudible",
			diff, portalMergedLevelToleranceDB,
		)
	}

	return nil
}

// portalBRIRLevelDB returns the broadband RMS level of both channels.
func portalBRIRLevelDB(response BRIR) float64 {
	sum := 0.0
	count := 0

	for _, channel := range []*ir.Buffer{response.Left, response.Right} {
		for _, sample := range channel.Samples {
			sum += sample * sample
			count++
		}
	}

	if count == 0 || sum <= 0 {
		return math.Inf(-1)
	}

	return 10 * math.Log10(sum/float64(count))
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
		return clonePortalBRIR(c.allPass, c.allPass.Left.SampleRate, c.allPass.Left.Len()), nil
	}

	return BRIR{
		Left:  interpolatePortalBuffer(c.closed.Left, c.allPass.Left, weight),
		Right: interpolatePortalBuffer(c.closed.Right, c.allPass.Right, weight),
	}, nil
}

// AtApertureMerged crossfades from closed toward the all-pass portal and then,
// over the last portalMergeCrossfadeSpan of aperture, on into the merged
// room-group response.
//
// The sequencing follows docs/raven.md section 5.3 — the portal's reduction
// indices fall until its filter is an all-pass, and only at that endpoint does
// the merged room group take over — but the final step is a crossfade rather
// than the hard switch RAVEN describes. Two independently simulated responses
// are never sample-continuous, so swapping one for the other in a single step
// is audible however well their broadband levels match.
func (c *PortalBRIRCache) AtApertureMerged(aperture, rootOrder float64) (BRIR, error) {
	if c == nil {
		return BRIR{}, errors.New("portal BRIR cache is nil")
	}

	if aperture >= 1 {
		return clonePortalBRIR(c.merged, c.merged.Left.SampleRate, c.merged.Left.Len()), nil
	}

	base, err := c.AtAperture(aperture, rootOrder)
	if err != nil {
		return BRIR{}, err
	}

	const mergeStart = 1 - portalMergeCrossfadeSpan
	if aperture <= mergeStart {
		return base, nil
	}

	// Linear in aperture rather than in the crossfade curve: this leg blends
	// two responses of the same nominal level, so the equal-power correction
	// that PortalCrossfadeWeight applies to the closed-to-open leg would only
	// introduce a bump of its own.
	weight := (aperture - mergeStart) / portalMergeCrossfadeSpan

	return BRIR{
		Left:  interpolatePortalBuffer(base.Left, c.merged.Left, weight),
		Right: interpolatePortalBuffer(base.Right, c.merged.Right, weight),
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

func interpolatePortalBuffer(closed, open *ir.Buffer, weight float64) *ir.Buffer {
	out := &ir.Buffer{SampleRate: closed.SampleRate, Samples: make([]float64, closed.Len())}
	for index := range out.Samples {
		out.Samples[index] = closed.Samples[index] + weight*(open.Samples[index]-closed.Samples[index])
	}

	return out
}
