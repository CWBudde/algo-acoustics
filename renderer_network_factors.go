package algoacoustics

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

// networkPortalOffsetMeters nudges a portal port just off the portal plane so
// it sits inside the group it belongs to.
const networkPortalOffsetMeters = 1e-5

// portKind classifies the endpoints of a single hop.
type portKind uint8

const (
	portKindSource portKind = iota
	portKindPortal
	portKindReceiver
)

// groupPort is one end of a hop: the primary source, a portal, or the receiver.
type groupPort struct {
	Kind     portKind
	Index    int
	Position geometry.Vec3
	Polygon  []geometry.Vec3
}

// GroupFactor is one factor of the H_PP product of docs/raven.md section 5.2:
// the transfer function of a single room group between two ports.
//
// Early carries the pressure-domain image-source response and LateEnergy the
// energy-domain ray-traced response. Events, DGDirections, and DGHistograms are
// populated only for a hop that ends at the real receiver, since only there does
// the response stay directional.
//
// DGHistograms holds one energy histogram per directivity group rather than
// ready-made probabilities, because the arrival probabilities are only
// meaningful once the upstream hops and every other path have been folded in.
type GroupFactor struct {
	Early        *ir.BandedResponse
	LateEnergy   *raytrace.EnergyHistogram
	Events       []ir.Event
	DGDirections []geometry.Vec3
	DGHistograms []*raytrace.EnergyHistogram
}

// Path-type taxonomy.
//
// PLAN.md names four path types — PS2R, PS2P, SS2R, SS2P — that docs/raven.md
// never expands. The expansion below is ours, not the reference's. It follows
// the structure of H_PP directly: raven.md marks only the source and receiver
// factors as complex and binaural, so a hop ending at a portal is scalar per
// band while a hop ending at the receiver carries direction through the HRTF.
//
//	PS2P  primary source   -> portal    first hop out of the source's own group
//	SS2P  secondary source -> portal    an intermediate hop, portal to portal
//	SS2R  secondary source -> receiver  the terminal hop, binaural
//	PS2R  primary source   -> receiver  the zero-hop case, source and receiver
//	                                    share a group; identical to an ordinary
//	                                    single-room render

// solvePS2P renders the first hop: the real source to the exit portal of its
// own group.
func (r *NetworkRenderer) solvePS2P(
	gsc *scene.Scene,
	source scene.Source,
	exit groupPort,
	cfg ir.RenderConfig,
	into *GroupFactor,
	needs factorNeeds,
) (*GroupFactor, error) {
	entry := groupPort{Kind: portKindSource, Position: source.Position}

	return r.solveFactor(gsc, entry, exit, cfg, &source, nil, into, needs)
}

// solveSS2P renders an intermediate hop: an entry portal to an exit portal.
func (r *NetworkRenderer) solveSS2P(
	gsc *scene.Scene,
	entry, exit groupPort,
	cfg ir.RenderConfig,
	into *GroupFactor,
	needs factorNeeds,
) (*GroupFactor, error) {
	return r.solveFactor(gsc, entry, exit, cfg, nil, nil, into, needs)
}

// solveSS2R renders the terminal hop: an entry portal to the real receiver.
func (r *NetworkRenderer) solveSS2R(
	gsc *scene.Scene,
	entry groupPort,
	receiver scene.Receiver,
	cfg ir.RenderConfig,
	into *GroupFactor,
	needs factorNeeds,
) (*GroupFactor, error) {
	exit := groupPort{Kind: portKindReceiver, Position: receiver.Position}

	return r.solveFactor(gsc, entry, exit, cfg, nil, &receiver, into, needs)
}

// solvePS2R renders the degenerate zero-hop case where source and receiver
// share a group.
func (r *NetworkRenderer) solvePS2R(
	gsc *scene.Scene,
	source scene.Source,
	receiver scene.Receiver,
	cfg ir.RenderConfig,
	into *GroupFactor,
	needs factorNeeds,
) (*GroupFactor, error) {
	entry := groupPort{Kind: portKindSource, Position: source.Position}
	exit := groupPort{Kind: portKindReceiver, Position: receiver.Position}

	return r.solveFactor(gsc, entry, exit, cfg, &source, &receiver, into, needs)
}

// solveFactor renders one room-group transfer function between two ports.
//
// Crucially it runs ONE simulation per hop, independent of how many events
// arrived at the entry port. Composing hops by re-emitting every event, as the
// Phase 21 one-hop renderer does, costs a full solve per event and is
// exponential in the number of hops.
//
// needs names only the halves still wanted, and into is the partially solved
// factor to fill, so a hop already carrying its early field never solves it
// twice.
func (r *NetworkRenderer) solveFactor(
	gsc *scene.Scene,
	entry, exit groupPort,
	cfg ir.RenderConfig,
	source *scene.Source,
	receiver *scene.Receiver,
	into *GroupFactor,
	needs factorNeeds,
) (*GroupFactor, error) {
	if r == nil {
		return nil, errors.New("network renderer is nil")
	}

	factor := into
	if factor == nil {
		factor = &GroupFactor{}
	}

	if needs.early {
		early, events, err := r.solveFactorEarly(gsc, entry, exit, cfg, source, receiver)
		if err != nil {
			return nil, err
		}

		factor.Early = early
		factor.Events = events
	}

	if !needs.late {
		return factor, nil
	}

	err := r.traceFactorLate(factor, gsc, entry, exit, cfg, source, receiver)
	if err != nil {
		return nil, err
	}

	return factor, nil
}

func (r *NetworkRenderer) solveFactorEarly(
	gsc *scene.Scene,
	entry, exit groupPort,
	cfg ir.RenderConfig,
	source *scene.Source,
	receiver *scene.Receiver,
) (*ir.BandedResponse, []ir.Event, error) {
	ismConfig := r.Config.ISM
	if ismConfig.SpeedOfSound <= 0 {
		ismConfig.SpeedOfSound = acoustics.SpeedOfSound
	}

	if ismConfig.BandSpec.BandCount() == 0 {
		ismConfig.BandSpec = gsc.BandSpec
	}

	sub := *gsc
	sub.Sources = []scene.Source{factorSource(entry, source)}
	sub.Receivers = []scene.Receiver{factorReceiver(exit, receiver)}

	// Translation leaves arrival directions, distances, and times unchanged, so
	// the events need no correction on the way back out.
	localized := networkLocalizedScene(&sub)

	events, err := (ism.ISMSolver{}).Solve(localized, ismConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("solve group early field: %w", err)
	}

	early, err := ir.BandedFromEvents(events, ir.RenderConfig{
		SampleRate:      cfg.SampleRate,
		DurationSeconds: cfg.DurationSeconds,
		BandSpec:        ismConfig.BandSpec,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build group early response: %w", err)
	}

	return early, events, nil
}

func (r *NetworkRenderer) traceFactorLate(
	factor *GroupFactor,
	gsc *scene.Scene,
	entry, exit groupPort,
	cfg ir.RenderConfig,
	source *scene.Source,
	receiver *scene.Receiver,
) error {
	launch := r.Config.Raytrace.Launch
	if launch.MaxTimeSeconds <= 0 {
		launch.MaxTimeSeconds = cfg.DurationSeconds
	}

	if launch.SpeedOfSound <= 0 {
		launch.SpeedOfSound = acoustics.SpeedOfSound
	}

	if r.Config.DynamicRays > 0 {
		launch.NumRays = r.Config.DynamicRays
	}

	sub := *gsc
	sub.Sources = []scene.Source{factorSource(entry, source)}
	sub.Receivers = nil

	if exit.Kind == portKindReceiver {
		return r.traceFactorLateToReceiver(factor, &sub, entry, launch, factorReceiver(exit, receiver))
	}

	tracer := &raytrace.RayTracer{
		Config:             launch,
		Scene:              &sub,
		BinDurationSeconds: r.Config.Raytrace.BinDurationSeconds,
	}

	detector, err := raytrace.NewSurfaceReceiver(append([]geometry.Vec3(nil), exit.Polygon...))
	if err != nil {
		return fmt.Errorf("construct portal %d surface receiver: %w", exit.Index, err)
	}

	incident, err := tracer.TraceSurfaces([]raytrace.SurfaceReceiver{detector})
	if err != nil {
		return fmt.Errorf("trace group late field to portal %d: %w", exit.Index, err)
	}

	factor.LateEnergy = incident[0]

	return nil
}

func (r *NetworkRenderer) traceFactorLateToReceiver(
	factor *GroupFactor,
	sub *scene.Scene,
	entry groupPort,
	launch raytrace.LaunchConfig,
	receiver scene.Receiver,
) error {
	sub.Receivers = []scene.Receiver{receiver}

	tracer := &raytrace.RayTracer{
		Config:             launch,
		Scene:              sub,
		ReceiverRadius:     r.Config.Raytrace.ReceiverRadius,
		BinDurationSeconds: r.Config.Raytrace.BinDurationSeconds,
	}

	if receiver.HRTF != nil {
		azimuth := r.Config.Raytrace.DirectionGroupAzimuth
		if azimuth <= 0 {
			azimuth = defaultDirectionGroupAzimuth
		}

		elevation := r.Config.Raytrace.DirectionGroupElevation
		if elevation <= 0 {
			elevation = defaultDirectionGroupElevation
		}

		tracer.DirectivityGroups = raytrace.NewDirectivityGroups(azimuth, elevation)
	}

	var (
		histogram *raytrace.EnergyHistogram
		err       error
	)

	if entry.Kind == portKindSource {
		histogram, err = tracer.Trace()
	} else {
		// An intermediate group hands over a unit impulse at the entry portal;
		// the actual level is carried by the portal filter and the preceding
		// factors, so the group's own response is what is wanted here.
		histogram, err = tracer.TraceSecondary([]ir.EnergyEmission{{
			Position:   entry.Position,
			BandEnergy: unitBandEnergy(sub.BandSpec.BandCount()),
		}})
	}

	if err != nil {
		return fmt.Errorf("trace group late field to receiver: %w", err)
	}

	factor.LateEnergy = histogram

	if tracer.DirectivityGroups != nil {
		factor.DGDirections = make([]geometry.Vec3, len(tracer.DirectivityGroups))
		factor.DGHistograms = make([]*raytrace.EnergyHistogram, len(tracer.DirectivityGroups))

		for index, group := range tracer.DirectivityGroups {
			factor.DGDirections[index] = receiver.WorldToHeadDir(group.Direction)
			factor.DGHistograms[index] = group.Histogram
		}
	}

	return nil
}

// factorSource returns the source used for one hop. An intermediate hop is
// driven by an omnidirectional secondary source at the entry portal.
func factorSource(entry groupPort, source *scene.Source) scene.Source {
	if entry.Kind == portKindSource && source != nil {
		return *source
	}

	return scene.Source{Position: entry.Position, Orientation: geometry.QuatIdentity()}
}

// factorReceiver returns the receiver used for one hop. A hop ending at a
// portal is measured by an omnidirectional probe at the portal centre.
func factorReceiver(exit groupPort, receiver *scene.Receiver) scene.Receiver {
	if exit.Kind == portKindReceiver && receiver != nil {
		return *receiver
	}

	return scene.Receiver{Position: exit.Position, Orientation: geometry.QuatIdentity(), Type: scene.ReceiverOmni}
}

func unitBandEnergy(bandCount int) []float64 {
	energy := make([]float64, bandCount)
	for index := range energy {
		energy[index] = 1
	}

	return energy
}

// networkLocalizedScene translates a shoebox group to the origin, which the
// shoebox image-source solver assumes. A merged group is a mesh room and the
// mesh solver works in world coordinates, so it is returned untouched — which
// is why the network renderer does not inherit the Phase 21 asymmetry where the
// image-source path localised coordinates and the ray-tracing path did not.
func networkLocalizedScene(sc *scene.Scene) *scene.Scene {
	if sc.Room.Kind != scene.RoomKindShoebox || sc.Room.Shoebox == nil {
		return sc
	}

	origin := sc.Room.Shoebox.Origin
	if origin == geometry.Vec3Zero {
		return sc
	}

	localized := *sc
	shoebox := *sc.Room.Shoebox
	shoebox.Origin = geometry.Vec3Zero
	room := sc.Room
	room.Shoebox = &shoebox
	localized.Room = room
	localized.Sources = translatedSources(sc.Sources, origin.Scale(-1))
	localized.Receivers = translatedReceivers(sc.Receivers, origin.Scale(-1))

	return &localized
}

// networkRandom returns the renderer's deterministic random source.
func (r *NetworkRenderer) networkRandom() *rand.Rand {
	seed := r.Config.Seed
	if seed == 0 {
		seed = 1
	}

	return rand.New(rand.NewSource(seed)) //nolint:gosec // Reproducible acoustic synthesis.
}
