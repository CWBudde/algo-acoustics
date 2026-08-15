package algoacoustics

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"maps"
	"math"
	"reflect"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

// PortalStateChange describes an interactive portal toggle.
type PortalStateChange struct {
	PortalIndex int
	State       scene.PortalState
	// Aperture in [0,1] selects the topology a PortalOpen change resolves to.
	// Only aperture 1 merges the two room groups into one cavity; every
	// intermediate value keeps the rooms separate, which is what the
	// closed-to-all-pass crossfade in hybrid.PortalBRIRCache interpolates
	// between. Aperture is ignored for PortalClosed.
	Aperture float64
}

// EffectiveState returns the portal state the change actually resolves to.
//
// A partly open portal is still two rooms joined by a transmissive partition,
// not one merged cavity, so it keeps the closed topology and is rendered
// through the portal filter.
func (c PortalStateChange) EffectiveState() scene.PortalState {
	if c.State == scene.PortalOpen && c.Aperture < 1 {
		return scene.PortalClosed
	}

	return c.State
}

// ChangeSet reports what a portal toggle actually cost, counted in room-group
// signatures rather than in rendered factors.
//
// A group that is new to the plan still hits the response cache when the same
// configuration was rendered before — reopening a door that was open a moment
// ago costs nothing — so AddedSignatures is an upper bound on the simulation
// work, not a measurement of it. GroupResponseCache.Stats reports the actual
// hit and miss counts.
type ChangeSet struct {
	// RemovedSignatures lists the room groups the change dissolved. Their
	// cached responses are deliberately kept: toggling the same portal back
	// rebuilds exactly these signatures, and evicting them would make every
	// second toggle a cold render. The cache's byte budget reclaims them.
	RemovedSignatures []uint64
	// AddedSignatures lists the room groups the change created.
	AddedSignatures []uint64
	// AddedGroups counts AddedSignatures; ReusedGroups counts the groups whose
	// signature survived and whose cached responses therefore stay valid.
	AddedGroups  int
	ReusedGroups int
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

	if sc == nil {
		return nil, errors.New("scene is nil")
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

	restore := p.scene.Portals[change.PortalIndex].State
	p.scene.Portals[change.PortalIndex].State = change.EffectiveState()

	inner, err := p.renderer.prepare(p.scene)
	if err != nil {
		// The graph in p.plan points at this same scene, so leaving the new
		// state in place would pair the old topology with the new portal state
		// on every later render.
		p.scene.Portals[change.PortalIndex].State = restore

		return ChangeSet{}, err
	}

	signatures := inner.graph.GroupSignatures()

	set := ChangeSet{}

	current := make(map[uint64]bool, len(signatures))

	for _, signature := range signatures {
		current[signature] = true

		if previous[signature] {
			set.ReusedGroups++

			continue
		}

		set.AddedSignatures = append(set.AddedSignatures, signature)
		set.AddedGroups++
	}

	for _, signature := range p.signatures {
		if !current[signature] {
			set.RemovedSignatures = append(set.RemovedSignatures, signature)
		}
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

	left, right, err := r.crossRoomEngineFor(copied).RenderBinaural(copied, receiver, cfg)
	if err != nil {
		return hybrid.BRIR{}, err //nolint:wrapcheck // The callers name the state.
	}

	return hybrid.BRIR{Left: left, Right: right}, nil
}

// crossRoomEngineFor picks the engine for a portal endpoint while keeping this
// renderer's own settings.
//
// Routing through CrossRoomEngineConfig would drop DynamicRays, Seed, the path
// and floor limits, and the shared response cache — and DynamicRays in
// particular is the whole reason the WASM demo can afford the merged endpoint.
func (r *NetworkRenderer) crossRoomEngineFor(sc *scene.Scene) CrossRoomEngine {
	if sceneMatchesOneHopTransmission(sc) {
		return NewTransmissionRenderer(TransmissionRendererConfig{
			ISM:      r.Config.ISM,
			Raytrace: r.Config.Raytrace,
			Hybrid:   r.Config.Hybrid,
		})
	}

	network := NewNetworkRenderer(r.Config)
	network.SetCache(r.Cache())

	return network
}

// clonePortalScene copies a scene deeply enough that portal state and materials
// can be changed without touching the caller's scene.
func clonePortalScene(sc *scene.Scene) *scene.Scene {
	copied := *sc
	copied.Portals = append([]scene.Portal(nil), sc.Portals...)
	copied.Rooms = append([]scene.Room(nil), sc.Rooms...)

	return &copied
}

// endpointHash covers everything about the source and receiver that changes a
// simulated factor, plus the band layout, none of which a room-group signature
// carries.
//
// Orientation and directivity belong here as much as position does: the ISM
// solver applies the source's pattern along Source.DirectionTo, and the
// terminal late factor spatializes through the receiver's orientation and HRTF.
// Omitting them would let a shared cache hand back a response simulated for a
// differently aimed loudspeaker.
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

	writeQuat := func(q geometry.Quaternion) {
		write(q.W)
		write(q.X)
		write(q.Y)
		write(q.Z)
	}

	for _, source := range sc.Sources {
		writeVec(source.Position)
		writeQuat(source.Orientation)
		write(source.GainDB)
		writeModelIdentity(hash, source.Directivity)
	}

	for _, receiver := range sc.Receivers {
		writeVec(receiver.Position)
		writeQuat(receiver.Orientation)
		_, _ = fmt.Fprintf(hash, "|type:%v|", receiver.Type)
		writeModelIdentity(hash, receiver.HRTF)
	}

	for _, freq := range sc.BandSpec.CenterFreqs {
		write(freq)
	}

	return hash.Sum64()
}

// writeModelIdentity folds an interface-valued endpoint attribute into a hash.
//
// Small value types are hashed by their contents. Reference types are hashed by
// identity instead, which is conservative — an equal-content copy simply misses
// — and avoids formatting a whole HRTF grid or GLL balloon on every render.
func writeModelIdentity(hash io.Writer, value any) {
	if value == nil {
		_, _ = io.WriteString(hash, "|nil|")

		return
	}

	reflected := reflect.ValueOf(value)

	switch reflected.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		_, _ = fmt.Fprintf(hash, "|%T:%d|", value, reflected.Pointer())
	default:
		_, _ = fmt.Fprintf(hash, "|%T:%v|", value, value)
	}
}

// configHash folds every solver setting that changes a simulated response into
// a single value, so a configuration change cannot be served from the cache.
//
// The solver configurations are hashed wholesale rather than field by field.
// SetCache exists so several renderers can share one budget, which makes a
// forgotten field a correctness bug rather than a missed optimisation — and a
// field-by-field list silently stops covering the struct the moment someone
// adds an option to it.
func (r *NetworkRenderer) configHash(cfg ir.RenderConfig) uint64 {
	hash := fnv.New64a()

	_, _ = fmt.Fprintf(hash, "render:%d|%v|", cfg.SampleRate, cfg.DurationSeconds)
	_, _ = fmt.Fprintf(hash, "bands:%v|", cfg.BandSpec)
	_, _ = fmt.Fprintf(hash, "ism:%#v|", r.Config.ISM)
	_, _ = fmt.Fprintf(hash, "raytrace:%#v|", r.Config.Raytrace)
	_, _ = fmt.Fprintf(hash, "hops:%d|paths:%d|", r.maxPathHops(), r.maxPaths())
	_, _ = fmt.Fprintf(hash, "dynamic:%d|seed:%d|floor:%v|", r.Config.DynamicRays, r.Config.Seed, r.bandFloorDB())

	return hash.Sum64()
}
