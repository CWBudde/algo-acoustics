package algoacoustics

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

// PortalStateChange describes an interactive portal toggle.
type PortalStateChange struct {
	PortalIndex int
	State       scene.PortalState
	// Aperture in [0,1] drives the crossfade. Only a fully open portal at
	// aperture 1 triggers the hard switch to the merged room-group response.
	Aperture float64
}

// ChangeSet reports what a portal toggle actually cost.
type ChangeSet struct {
	// InvalidatedSignatures lists the room groups that did not survive the
	// change and therefore had to be re-simulated.
	InvalidatedSignatures []uint64
	RecomputedFactors     int
	ReusedFactors         int
}

// NetworkPlan is a prepared multi-room render that survives portal toggles.
type NetworkPlan struct {
	renderer   *NetworkRenderer
	scene      *scene.Scene
	plan       *networkPlan
	signatures map[scene.GroupID]uint64
	cfg        ir.RenderConfig
}

// Prepare resolves the scene graph and propagation paths once, so that later
// portal toggles only re-simulate what actually changed.
func (r *NetworkRenderer) Prepare(sc *scene.Scene, cfg ir.RenderConfig) (*NetworkPlan, error) {
	if r == nil {
		return nil, errors.New("network renderer is nil")
	}

	copied := clonePortalScene(sc)

	inner, err := r.prepare(copied)
	if err != nil {
		return nil, err
	}

	return &NetworkPlan{
		renderer:   r,
		scene:      copied,
		plan:       inner,
		signatures: inner.graph.GroupSignatures(),
		cfg:        cfg,
	}, nil
}

// Scene returns the plan's own copy of the scene, whose portal states the plan
// mutates.
func (p *NetworkPlan) Scene() *scene.Scene {
	if p == nil {
		return nil
	}

	return p.scene
}

// Apply toggles a portal and rebuilds the plan, reporting which room groups had
// to be re-simulated.
//
// Every group whose signature survives the change keeps its cached factors,
// including groups on the far side of the toggled portal. For a four-room chain
// opening one door merges exactly two groups into one, so exactly one signature
// is new and only that group is simulated again.
func (p *NetworkPlan) Apply(change PortalStateChange) (ChangeSet, error) {
	if p == nil || p.renderer == nil {
		return ChangeSet{}, errors.New("network plan is nil")
	}

	if change.PortalIndex < 0 || change.PortalIndex >= len(p.scene.Portals) {
		return ChangeSet{}, fmt.Errorf("portal index %d is out of range", change.PortalIndex)
	}

	previous := make(map[uint64]bool, len(p.signatures))
	for _, signature := range p.signatures {
		previous[signature] = true
	}

	p.scene.Portals[change.PortalIndex].State = change.State

	inner, err := p.renderer.prepare(p.scene)
	if err != nil {
		return ChangeSet{}, err
	}

	signatures := inner.graph.GroupSignatures()

	set := ChangeSet{}

	for _, signature := range signatures {
		if previous[signature] {
			set.ReusedFactors++

			continue
		}

		set.InvalidatedSignatures = append(set.InvalidatedSignatures, signature)
		set.RecomputedFactors++
	}

	p.plan = inner
	p.signatures = signatures

	return set, nil
}

// RenderMono renders the prepared plan's summed mono response.
func (p *NetworkPlan) RenderMono() (*ir.Buffer, error) {
	if p == nil {
		return nil, errors.New("network plan is nil")
	}

	early, late, err := p.renderer.renderPaths(p.plan, p.cfg)
	if err != nil {
		return nil, err
	}

	late = hybrid.AlignLateTailBuffer(late, early, p.renderer.Config.Hybrid)

	combined := hybrid.CombineBuffers(early, late, p.renderer.Config.Hybrid)
	if combined == nil {
		return nil, errors.New("combine multi-room mono field")
	}

	p.renderer.reportTruncation(p.plan)

	return combined, nil
}

// RenderBinaural renders the prepared plan's summed BRIR.
func (p *NetworkPlan) RenderBinaural(receiver scene.Receiver) (hybrid.BRIR, error) {
	if p == nil {
		return hybrid.BRIR{}, errors.New("network plan is nil")
	}

	left, right, err := p.renderer.RenderBinaural(p.scene, receiver, p.cfg)
	if err != nil {
		return hybrid.BRIR{}, err
	}

	return hybrid.BRIR{Left: left, Right: right}, nil
}

// PortalCache renders the three states a portal crossfades between and returns
// a cache over them.
//
// The open endpoint is the physically merged room group, not the tau = 1
// all-pass stand-in the demo used before Phase 25.4. Both are rendered because
// docs/raven.md section 5.3 crossfades toward the all-pass filter and only
// hard-switches to the merged response at full aperture.
func (r *NetworkRenderer) PortalCache(
	sc *scene.Scene,
	receiver scene.Receiver,
	cfg ir.RenderConfig,
	portalIndex int,
) (*hybrid.PortalBRIRCache, error) {
	closed, err := r.renderPortalState(sc, receiver, cfg, portalIndex, scene.PortalClosed, false)
	if err != nil {
		return nil, fmt.Errorf("render the closed portal: %w", err)
	}

	allPass, err := r.renderPortalState(sc, receiver, cfg, portalIndex, scene.PortalClosed, true)
	if err != nil {
		return nil, fmt.Errorf("render the all-pass portal: %w", err)
	}

	merged, err := r.renderPortalState(sc, receiver, cfg, portalIndex, scene.PortalOpen, false)
	if err != nil {
		return nil, fmt.Errorf("render the merged room group: %w", err)
	}

	cache, err := hybrid.NewPortalBRIRCacheWithFilter(closed, allPass, merged)
	if err != nil {
		return nil, fmt.Errorf("build portal BRIR cache: %w", err)
	}

	return cache, nil
}

// renderPortalState renders one portal configuration. transparent replaces the
// portal material with a fully transmissive one, which is the all-pass filter
// state without merging the geometry.
func (r *NetworkRenderer) renderPortalState(
	sc *scene.Scene,
	receiver scene.Receiver,
	cfg ir.RenderConfig,
	portalIndex int,
	state scene.PortalState,
	transparent bool,
) (hybrid.BRIR, error) {
	copied := clonePortalScene(sc)
	if portalIndex < 0 || portalIndex >= len(copied.Portals) {
		return hybrid.BRIR{}, fmt.Errorf("portal index %d is out of range", portalIndex)
	}

	copied.Portals[portalIndex].State = state

	if transparent {
		const name = "__portal_all_pass"

		materials := make(map[string]scene.Material, len(copied.Materials)+1)
		maps.Copy(materials, copied.Materials)

		materials[name] = scene.Material{
			Name:               name,
			AbsorptionByBand:   []float64{0},
			ScatteringByBand:   []float64{0},
			TransmissionByBand: []float64{1},
		}
		copied.Materials = materials
		copied.Portals[portalIndex].Material = name
	}

	engine := NewCrossRoomEngine(copied, CrossRoomEngineConfig{
		ISM:          r.Config.ISM,
		Raytrace:     r.Config.Raytrace,
		Hybrid:       r.Config.Hybrid,
		OnTruncation: r.Config.OnTruncation,
	})

	left, right, err := engine.RenderBinaural(copied, receiver, cfg)
	if err != nil {
		return hybrid.BRIR{}, err //nolint:wrapcheck // The callers name the state.
	}

	return hybrid.BRIR{Left: left, Right: right}, nil
}

// clonePortalScene copies a scene deeply enough that portal state and materials
// can be changed without touching the caller's scene.
func clonePortalScene(sc *scene.Scene) *scene.Scene {
	copied := *sc
	copied.Portals = append([]scene.Portal(nil), sc.Portals...)
	copied.Rooms = append([]scene.Room(nil), sc.Rooms...)

	return &copied
}

// endpointHash covers the placement of the source and receiver and the band
// layout, none of which a room-group signature carries.
func endpointHash(sc *scene.Scene) uint64 {
	hash := fnv.New64a()

	var buffer [8]byte

	write := func(value float64) {
		binary.LittleEndian.PutUint64(buffer[:], math.Float64bits(value))
		_, _ = hash.Write(buffer[:])
	}

	writeVec := func(v geometry.Vec3) {
		write(v.X)
		write(v.Y)
		write(v.Z)
	}

	for _, source := range sc.Sources {
		writeVec(source.Position)
		write(source.GainDB)
	}

	for _, receiver := range sc.Receivers {
		writeVec(receiver.Position)
	}

	for _, freq := range sc.BandSpec.CenterFreqs {
		write(freq)
	}

	return hash.Sum64()
}

// configHash folds the settings that change a simulated response into a single
// value, so a configuration change invalidates the cache.
func (r *NetworkRenderer) configHash(cfg ir.RenderConfig) uint64 {
	hash := fnv.New64a()

	var buffer [8]byte

	write := func(value float64) {
		binary.LittleEndian.PutUint64(buffer[:], math.Float64bits(value))
		_, _ = hash.Write(buffer[:])
	}

	write(float64(cfg.SampleRate))
	write(cfg.DurationSeconds)
	write(float64(r.Config.ISM.MaxOrder))
	write(r.Config.ISM.SpeedOfSound)
	write(float64(r.Config.Raytrace.Launch.NumRays))
	write(float64(r.Config.Raytrace.Launch.MaxBounces))
	write(r.Config.Raytrace.ReceiverRadius)
	write(r.Config.Raytrace.BinDurationSeconds)
	write(float64(r.Config.DynamicRays))
	write(float64(r.Config.Seed))
	write(r.bandFloorDB())

	return hash.Sum64()
}
