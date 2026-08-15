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
//
// Hops are memoised by their endpoint identity, not by path, so a hop that
// several paths share through the same group and the same two portals costs one
// simulation in total rather than one per path. Behind that per-render memo sits
// the signature-keyed GroupResponseCache, which carries a hop across renders so
// a portal toggle re-simulates only the groups it actually changed.
//
// needs selects which fields are simulated. SolveEarly wants only the
// image-source factors and RenderLateMono only the ray-traced ones; computing
// the other would be pure waste.
func (r *NetworkRenderer) renderPathFactors(
	plan *networkPlan,
	pathIndex int,
	cfg ir.RenderConfig,
	needs factorNeeds,
) ([]*GroupFactor, error) {
	path := plan.paths[pathIndex]
	factors := make([]*GroupFactor, 0, len(path.groups))
	configHash := r.configHash(cfg)

	for index, group := range path.groups {
		key := hopKeyAt(path, index)
		cached := plan.factors[key]

		from, to := r.hopPorts(plan, path, index, index == 0, index == len(path.groups)-1)
		responseKey, keyed := plan.groupResponseKey(group, from, to, configHash)

		// A group whose signature survived the last portal change keeps its
		// simulated response across renders, which is what makes an interactive
		// portal toggle cost only the group that actually changed. The
		// within-render memo wins where both hold the hop, since it is at least
		// as complete.
		if cached.factor == nil && keyed {
			if stored, ok := r.Cache().Get(responseKey); ok {
				// Copy before it is filled in: several renderers may share one
				// cache, so a stored factor must never be written through.
				cached = cachedFactor{factor: cloneFactor(stored), needs: factorNeedsOf(stored)}
			}
		}

		// A hop cached with only its early field keeps that work and adds the
		// missing ray trace to the same factor, rather than solving it again.
		missing := cached.needs.missing(needs)
		if cached.factor != nil && missing.empty() {
			plan.storeFactor(key, cached.factor, cached.needs)
			factors = append(factors, cached.factor)

			continue
		}

		factor, err := r.solveHop(plan, path, index, cfg, cached.factor, missing)
		if err != nil {
			return nil, fmt.Errorf("render group %d of path: %w", group, err)
		}

		plan.storeFactor(key, factor, cached.needs.union(needs))

		if keyed {
			r.Cache().Put(responseKey, factor)
		}

		factors = append(factors, factor)
	}

	return factors, nil
}

// solveHop renders the factor of one hop, choosing the path type from where the
// hop sits along the path. A non-nil into is filled in place, so only the
// missing halves are simulated.
func (r *NetworkRenderer) solveHop(
	plan *networkPlan,
	path networkPath,
	index int,
	cfg ir.RenderConfig,
	into *GroupFactor,
	needs factorNeeds,
) (*GroupFactor, error) {
	group := path.groups[index]

	gsc, err := plan.graph.GroupScene(group)
	if err != nil {
		return nil, fmt.Errorf("extract group %d: %w", group, err)
	}

	first := index == 0
	last := index == len(path.groups)-1

	switch {
	case first && last:
		// PS2R: source and receiver share a group, so the path is an ordinary
		// single-room render.
		return r.solvePS2R(gsc, plan.source, plan.receiver, cfg, into, needs)
	case first:
		// PS2P: the real source to the exit portal of its own group.
		exit := portalPort(plan.graph.Scene(), path.portals[index], false)

		return r.solvePS2P(gsc, plan.source, exit, cfg, into, needs)
	case last:
		// SS2R: the entry portal to the real receiver.
		entry := portalPort(plan.graph.Scene(), path.portals[index-1], true)

		return r.solveSS2R(gsc, entry, plan.receiver, cfg, into, needs)
	default:
		// SS2P: portal to portal through an intermediate group.
		entry := portalPort(plan.graph.Scene(), path.portals[index-1], true)
		exit := portalPort(plan.graph.Scene(), path.portals[index], false)

		return r.solveSS2P(gsc, entry, exit, cfg, into, needs)
	}
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
		factors, err := r.renderPathFactors(plan, pathIndex, cfg, factorNeeds{early: true, late: true})
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

// renderLatePaths sums the late field of every ranked path without solving the
// early field at all.
func (r *NetworkRenderer) renderLatePaths(plan *networkPlan, cfg ir.RenderConfig) (*raytrace.EnergyHistogram, error) {
	var histograms []*raytrace.EnergyHistogram

	for pathIndex, path := range plan.paths {
		factors, err := r.renderPathFactors(plan, pathIndex, cfg, factorNeeds{late: true})
		if err != nil {
			return nil, err
		}

		histogram, err := r.resolvePathLate(plan.graph.Scene(), path, factors)
		if err != nil {
			return nil, err
		}

		if histogram != nil {
			histograms = append(histograms, histogram)
		}
	}

	summed := sumHistograms(histograms)
	if summed == nil {
		return nil, errors.New("no late field was rendered")
	}

	return summed, nil
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
	total, _, err := r.composePathLate(sc, path, factors)

	return total, err
}

// composePathLate folds a path's late factors into one energy histogram and, in
// the same pass, folds the terminal hop's per-direction histograms along the
// identical chain.
//
// Carrying the directional histograms through the same portal scaling and
// convolution is what keeps the arrival directions honest. Reading the terminal
// hop's own probabilities instead would ignore how the preceding hops shift and
// spread the arrival times, and — once several paths reach the receiver through
// different portals — would spatialize every path's energy with one path's
// directions.
func (r *NetworkRenderer) composePathLate(
	sc *scene.Scene,
	path networkPath,
	factors []*GroupFactor,
) (total *raytrace.EnergyHistogram, directional []*raytrace.EnergyHistogram, err error) {
	running := factors[0].LateEnergy
	if running == nil {
		return nil, nil, nil
	}

	// upstream is the chain up to but excluding the terminal hop, already
	// scaled by the portal that enters it. It is what the terminal hop's
	// directional histograms must be convolved with.
	var upstream *raytrace.EnergyHistogram

	for index, factor := range factors[1:] {
		if factor.LateEnergy == nil {
			return nil, nil, nil
		}

		transmission := portalTransmission(sc, path.portals[index])

		scaled, err := hybrid.ScaleHistogram(running, hybrid.EnergyFilterFromTransmission(transmission))
		if err != nil {
			return nil, nil, fmt.Errorf("apply portal energy filter: %w", err)
		}

		if index == len(factors)-2 {
			upstream = scaled
		}

		running, err = hybrid.ConvolveHistograms(scaled, factor.LateEnergy)
		if err != nil {
			return nil, nil, fmt.Errorf("compose late field across a portal: %w", err)
		}
	}

	directional, err = composeDirectionalLate(factors[len(factors)-1].DGHistograms, upstream)
	if err != nil {
		return nil, nil, err
	}

	return running, directional, nil
}

// composeDirectionalLate convolves each of the terminal hop's directional
// histograms with the upstream chain. A single-hop path has no upstream, so its
// directional histograms pass through untouched.
func composeDirectionalLate(
	terminal []*raytrace.EnergyHistogram,
	upstream *raytrace.EnergyHistogram,
) ([]*raytrace.EnergyHistogram, error) {
	if len(terminal) == 0 {
		return nil, nil
	}

	composed := make([]*raytrace.EnergyHistogram, len(terminal))

	for index, histogram := range terminal {
		if histogram == nil {
			continue
		}

		if upstream == nil {
			composed[index] = histogram

			continue
		}

		folded, err := hybrid.ConvolveHistograms(upstream, histogram)
		if err != nil {
			return nil, fmt.Errorf("compose directional late field across a portal: %w", err)
		}

		composed[index] = folded
	}

	return composed, nil
}

// hopPorts returns the entry and exit ports of one hop along a path.
func (r *NetworkRenderer) hopPorts(
	plan *networkPlan,
	path networkPath,
	index int,
	first, last bool,
) (from, to groupPort) {
	sc := plan.graph.Scene()

	if first {
		from = groupPort{Kind: portKindSource, Position: plan.source.Position}
	} else {
		from = portalPort(sc, path.portals[index-1], true)
	}

	if last {
		to = groupPort{Kind: portKindReceiver, Position: plan.receiver.Position}
	} else {
		to = portalPort(sc, path.portals[index], false)
	}

	return from, to
}

// sumLateHistograms renders and sums the directional late field of every path,
// keeping the directivity groups of the terminal hop.
//
// Both the total and the per-direction histograms are summed across paths, and
// the arrival probabilities are derived only afterwards, so every path
// contributes its own directions in proportion to the energy it delivers.
func (r *NetworkRenderer) sumLateHistograms(
	plan *networkPlan,
	cfg ir.RenderConfig,
) (*raytrace.EnergyHistogram, []geometry.Vec3, [][]float64, error) {
	var (
		histograms  []*raytrace.EnergyHistogram
		directions  []geometry.Vec3
		directional []*raytrace.EnergyHistogram
	)

	for pathIndex, path := range plan.paths {
		factors, err := r.renderPathFactors(plan, pathIndex, cfg, factorNeeds{late: true})
		if err != nil {
			return nil, nil, nil, err
		}

		histogram, pathDirectional, err := r.composePathLate(plan.graph.Scene(), path, factors)
		if err != nil {
			return nil, nil, nil, err
		}

		if histogram != nil {
			histograms = append(histograms, histogram)
		}

		terminal := factors[len(factors)-1]
		if directions == nil && terminal.DGDirections != nil {
			directions = terminal.DGDirections
		}

		directional, err = accumulateDirectional(directional, pathDirectional)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	summed := sumHistograms(histograms)
	if summed == nil {
		return nil, nil, nil, errors.New("no late field was rendered")
	}

	return summed, directions, raytrace.DGProbabilitiesFromHistograms(directional), nil
}

// accumulateDirectional adds one path's per-direction histograms into the
// running sum. Every path shares the same directivity-group grid, so the slices
// line up by index.
func accumulateDirectional(running, next []*raytrace.EnergyHistogram) ([]*raytrace.EnergyHistogram, error) {
	if len(next) == 0 {
		return running, nil
	}

	if running == nil {
		return next, nil
	}

	if len(running) != len(next) {
		return nil, fmt.Errorf("directivity group counts differ: %d and %d", len(running), len(next))
	}

	for index, histogram := range next {
		running[index] = sumHistograms([]*raytrace.EnergyHistogram{running[index], histogram})
	}

	return running, nil
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
