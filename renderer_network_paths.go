package algoacoustics

import (
	"errors"
	"fmt"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

// renderPathFactors renders one room-group transfer function per hop of a path.
//
// Exactly one simulation runs per hop, whatever the event count at the entry
// port. That is the whole reason the network composes by convolution rather
// than by re-emitting events.
// needLate selects whether the ray-traced late field is computed. SolveEarly
// needs only the image-source factors, and tracing rays for it would be pure
// waste.
func (r *NetworkRenderer) renderPathFactors(
	plan *networkPlan,
	pathIndex int,
	cfg ir.RenderConfig,
	needLate bool,
) ([]*GroupFactor, error) {
	if cached := plan.cachedFactors(pathIndex, needLate); cached != nil {
		return cached, nil
	}

	path := plan.paths[pathIndex]
	factors := make([]*GroupFactor, 0, len(path.groups))

	for index, group := range path.groups {
		gsc, err := plan.graph.GroupScene(group)
		if err != nil {
			return nil, fmt.Errorf("extract group %d: %w", group, err)
		}

		first := index == 0
		last := index == len(path.groups)-1

		var factor *GroupFactor

		switch {
		case first && last:
			// PS2R: source and receiver share a group, so the path is an
			// ordinary single-room render.
			factor, err = r.solvePS2R(gsc, plan.source, plan.receiver, cfg, needLate)
		case first:
			// PS2P: the real source to the exit portal of its own group.
			exit := portalPort(plan.graph.Scene(), path.portals[index], false)
			factor, err = r.solvePS2P(gsc, plan.source, exit, cfg, needLate)
		case last:
			// SS2R: the entry portal to the real receiver.
			entry := portalPort(plan.graph.Scene(), path.portals[index-1], true)
			factor, err = r.solveSS2R(gsc, entry, plan.receiver, cfg, needLate)
		default:
			// SS2P: portal to portal through an intermediate group.
			entry := portalPort(plan.graph.Scene(), path.portals[index-1], true)
			exit := portalPort(plan.graph.Scene(), path.portals[index], false)
			factor, err = r.solveSS2P(gsc, entry, exit, cfg, needLate)
		}

		if err != nil {
			return nil, fmt.Errorf("render group %d of path: %w", group, err)
		}

		factors = append(factors, factor)
	}

	plan.storeFactors(pathIndex, factors, needLate)

	return factors, nil
}

// renderPaths renders every ranked path and sums their early and late fields.
func (r *NetworkRenderer) renderPaths(plan *networkPlan, cfg ir.RenderConfig) (early, late *ir.Buffer, err error) {
	bandSpec := r.Config.ISM.BandSpec
	if bandSpec.BandCount() == 0 {
		bandSpec = plan.graph.Scene().BandSpec
	}

	length := ir.NewBuffer(cfg.SampleRate, cfg.DurationSeconds).Len()

	var (
		earlyResponses []*ir.BandedResponse
		lateHistograms []*raytrace.EnergyHistogram
	)

	for pathIndex, path := range plan.paths {
		factors, err := r.renderPathFactors(plan, pathIndex, cfg, true)
		if err != nil {
			return nil, nil, err
		}

		resolved, err := r.resolvePathEarly(plan.graph.Scene(), path, factors, length)
		if err != nil {
			return nil, nil, err
		}

		earlyResponses = append(earlyResponses, resolved)

		lateHistogram, err := r.resolvePathLate(plan.graph.Scene(), path, factors)
		if err != nil {
			return nil, nil, err
		}

		if lateHistogram != nil {
			lateHistograms = append(lateHistograms, lateHistogram)
		}
	}

	summedEarly, err := hybrid.SumBandedResponses(earlyResponses)
	if err != nil {
		return nil, nil, fmt.Errorf("sum path early fields: %w", err)
	}

	early, err = ir.RenderBandedMono(summedEarly, bandSpec, length)
	if err != nil {
		return nil, nil, fmt.Errorf("render summed early field: %w", err)
	}

	summedLate := sumHistograms(lateHistograms)
	if summedLate == nil {
		return nil, nil, errors.New("no late field was rendered")
	}

	return early, hybrid.HistogramToBuffer(summedLate, cfg.SampleRate), nil
}

// resolvePathEarly folds a path's early factors into one banded response,
// applying the pressure-domain portal filter at each handoff.
func (r *NetworkRenderer) resolvePathEarly(
	sc *scene.Scene,
	path networkPath,
	factors []*GroupFactor,
	length int,
) (*ir.BandedResponse, error) {
	chain := hybrid.PathChain{ActiveBands: path.activeBands}
	for _, factor := range factors {
		chain.Factors = append(chain.Factors, factor.Early)
	}

	for _, view := range path.portals {
		transmission := portalTransmission(sc, view)
		chain.PortalGains = append(chain.PortalGains, hybrid.PressureFilterFromTransmission(transmission))
	}

	resolved, err := chain.Resolve(length)
	if err != nil {
		return nil, fmt.Errorf("resolve path early chain: %w", err)
	}

	return resolved, nil
}

// resolvePathLate folds a path's late factors into one energy histogram,
// applying the energy-domain portal filter at each handoff. The pressure domain
// uses sqrt(tau) where the energy domain uses tau.
func (r *NetworkRenderer) resolvePathLate(sc *scene.Scene, path networkPath, factors []*GroupFactor) (*raytrace.EnergyHistogram, error) {
	running := factors[0].LateEnergy
	if running == nil {
		return nil, nil //nolint:nilnil // A path with no late field simply contributes none.
	}

	for index, factor := range factors[1:] {
		if factor.LateEnergy == nil {
			return nil, nil //nolint:nilnil // As above.
		}

		transmission := portalTransmission(sc, path.portals[index])

		scaled, err := hybrid.ScaleHistogram(running, hybrid.EnergyFilterFromTransmission(transmission))
		if err != nil {
			return nil, fmt.Errorf("apply portal energy filter: %w", err)
		}

		running, err = hybrid.ConvolveHistograms(scaled, factor.LateEnergy)
		if err != nil {
			return nil, fmt.Errorf("compose late field across a portal: %w", err)
		}
	}

	return running, nil
}

// sumLateHistograms renders and sums the directional late field of every path,
// keeping the directivity groups of the terminal hop.
func (r *NetworkRenderer) sumLateHistograms(
	plan *networkPlan,
	cfg ir.RenderConfig,
) (*raytrace.EnergyHistogram, []geometry.Vec3, [][]float64, error) {
	var (
		histograms    []*raytrace.EnergyHistogram
		directions    []geometry.Vec3
		probabilities [][]float64
	)

	for pathIndex, path := range plan.paths {
		factors, err := r.renderPathFactors(plan, pathIndex, cfg, true)
		if err != nil {
			return nil, nil, nil, err
		}

		histogram, err := r.resolvePathLate(plan.graph.Scene(), path, factors)
		if err != nil {
			return nil, nil, nil, err
		}

		if histogram != nil {
			histograms = append(histograms, histogram)
		}

		terminal := factors[len(factors)-1]
		if directions == nil && terminal.DGDirections != nil {
			directions = terminal.DGDirections
			probabilities = terminal.DGProbabilities
		}
	}

	summed := sumHistograms(histograms)
	if summed == nil {
		return nil, nil, nil, errors.New("no late field was rendered")
	}

	return summed, directions, probabilities, nil
}

// sumHistograms adds energy histograms bin by bin.
func sumHistograms(histograms []*raytrace.EnergyHistogram) *raytrace.EnergyHistogram {
	var out *raytrace.EnergyHistogram

	for _, histogram := range histograms {
		if histogram == nil {
			continue
		}

		if out == nil {
			out = raytrace.NewEnergyHistogram(histogram.Duration, histogram.BinDuration, histogram.BandCount)
		}

		for index := range histogram.Bins {
			if index >= len(out.Bins) {
				break
			}

			for band, energy := range histogram.Bins[index].BandEnergy {
				if band < len(out.Bins[index].BandEnergy) {
					out.Bins[index].BandEnergy[band] += energy
				}
			}
		}
	}

	return out
}

// portalTransmission returns a portal's per-band energy transmission.
func portalTransmission(sc *scene.Scene, view scene.GroupPortalView) []float64 {
	bandCount := sc.BandSpec.BandCount()
	portal := sc.Portals[view.PortalIndex]

	transmission := make([]float64, bandCount)
	for band := range transmission {
		transmission[band] = portal.TransmissionAt(sc.Materials, band)
	}

	return transmission
}
