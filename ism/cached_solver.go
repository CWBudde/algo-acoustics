package ism

import (
	"errors"
	"fmt"
	"sort"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

// CachedISMSolver is an incremental ISM solver that retains geometry-dependent
// image-source data between calls. It short-circuits work based on what
// changed between solves:
//
//   - Room geometry unchanged, source positions unchanged → reuse cached image
//     sources and only re-evaluate energy (fastest path; bypasses the image
//     source generation entirely).
//   - Room geometry unchanged, a source moved → regenerate image sources for
//     the moved source(s) using the existing room topology (cheap arithmetic
//     for shoebox rooms), then evaluate.
//   - Room geometry changed → invalidate all cached state and rebuild from
//     scratch.
//
// The first call to Solve performs a full rebuild. Subsequent calls take the
// fastest path that is valid for the change that occurred.
//
// CachedISMSolver supports both shoebox and mesh rooms. Its Solve results are
// intended to be bit-identical to a fresh ISMSolver{}.Solve() call for the
// same scene and config.
type CachedISMSolver struct {
	// Shoebox state.
	shoeboxCache     [][]ImageSource // one slice of image sources per scene source
	lastShoeboxDims  *scene.Shoebox
	lastSourcePosSbx []geometry.Vec3

	// Mesh state.
	meshCache        [][]MeshImageSource
	lastMesh         *geometry.Mesh
	lastSourcePosMsh []geometry.Vec3
	lastMeshMaxDist  float64

	// Shared state.
	lastRoomHash uint64
	lastMaxOrder int
	lastRoomKind scene.RoomKind
}

// Solve computes direct and specular events for the given scene, reusing
// cached image-source trees where possible.
func (c *CachedISMSolver) Solve(sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	if cfg.MaxOrder < 0 {
		return nil, errors.New("max order must be non-negative")
	}

	if cfg.MaxDiffractionOrder < 0 || cfg.MaxDiffractionOrder > 2 {
		return nil, errors.New("max diffraction order must be 0, 1, or 2")
	}

	err := scene.Validate(sc)
	if err != nil {
		return nil, fmt.Errorf("validate scene: %w", err)
	}

	if len(sc.Sources) == 0 {
		return nil, errors.New("ISM solver requires at least one source")
	}

	if len(sc.Receivers) == 0 {
		return nil, errors.New("ISM solver requires exactly one receiver")
	}

	if len(sc.Receivers) > 1 {
		return nil, fmt.Errorf("ISM solver currently supports exactly one receiver, got %d", len(sc.Receivers))
	}

	switch sc.Room.Kind {
	case scene.RoomKindShoebox:
		if sc.Room.Shoebox == nil {
			return nil, errors.New("ISM solver requires shoebox dimensions")
		}

		return c.solveShoebox(sc, cfg)
	case scene.RoomKindMesh:
		if sc.Room.Mesh == nil {
			return nil, errors.New("ISM solver requires a mesh for mesh rooms")
		}

		return c.solveMesh(sc, cfg)
	default:
		return nil, fmt.Errorf("ISM solver does not support room kind %q", sc.Room.Kind)
	}
}

// Invalidate clears all cached state, forcing the next Solve to perform a
// full rebuild. Useful when the caller knows that cached data is stale for
// reasons outside the solver's view (for example, after swapping an entire
// scene).
func (c *CachedISMSolver) Invalidate() {
	c.shoeboxCache = nil
	c.lastShoeboxDims = nil
	c.lastSourcePosSbx = nil
	c.meshCache = nil
	c.lastMesh = nil
	c.lastSourcePosMsh = nil
	c.lastMeshMaxDist = 0
	c.lastRoomHash = 0
	c.lastMaxOrder = 0
	c.lastRoomKind = ""
}

func (c *CachedISMSolver) solveShoebox(sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	roomHash := sc.RoomHash()
	sourcePositions := collectSourcePositions(sc)

	roomUnchanged := c.lastRoomKind == scene.RoomKindShoebox &&
		c.lastRoomHash == roomHash &&
		c.lastMaxOrder == cfg.MaxOrder &&
		c.shoeboxCache != nil &&
		len(c.shoeboxCache) == len(sc.Sources)

	if !roomUnchanged {
		// Full rebuild: regenerate image sources for every scene source.
		c.shoeboxCache = make([][]ImageSource, len(sc.Sources))
		for i, src := range sc.Sources {
			c.shoeboxCache[i] = GenerateImageSources(src.Position, sc.Room.Shoebox, cfg.MaxOrder)
		}
	} else {
		// Room unchanged: regenerate only sources whose position changed.
		for i, src := range sc.Sources {
			if i < len(c.lastSourcePosSbx) && c.lastSourcePosSbx[i] == src.Position {
				continue
			}

			c.shoeboxCache[i] = GenerateImageSources(src.Position, sc.Room.Shoebox, cfg.MaxOrder)
		}
	}

	// Update state bookkeeping.
	c.lastRoomHash = roomHash
	c.lastMaxOrder = cfg.MaxOrder
	c.lastRoomKind = scene.RoomKindShoebox
	c.lastShoeboxDims = sc.Room.Shoebox
	c.lastSourcePosSbx = sourcePositions
	c.meshCache = nil
	c.lastMesh = nil
	c.lastSourcePosMsh = nil

	return evaluateShoeboxMultiSource(c.shoeboxCache, sc, cfg)
}

func (c *CachedISMSolver) solveMesh(sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	roomHash := sc.RoomHash()
	sourcePositions := collectSourcePositions(sc)

	speedOfSound := cfg.SpeedOfSound
	if speedOfSound <= 0 {
		speedOfSound = acoustics.SpeedOfSound
	}

	imgCfg := MeshISMConfig{
		MaxOrder:    cfg.MaxOrder,
		MaxDistance: meshMaxDistance(sc, speedOfSound),
	}

	roomUnchanged := c.lastRoomKind == scene.RoomKindMesh &&
		c.lastRoomHash == roomHash &&
		c.lastMaxOrder == cfg.MaxOrder &&
		c.lastMeshMaxDist == imgCfg.MaxDistance &&
		c.meshCache != nil &&
		len(c.meshCache) == len(sc.Sources)

	if !roomUnchanged {
		c.meshCache = make([][]MeshImageSource, len(sc.Sources))
		for i, src := range sc.Sources {
			c.meshCache[i] = GenerateMeshImageSources(src.Position, sc.Room.Mesh, imgCfg)
		}
	} else {
		for i, src := range sc.Sources {
			if i < len(c.lastSourcePosMsh) && c.lastSourcePosMsh[i] == src.Position {
				continue
			}

			c.meshCache[i] = GenerateMeshImageSources(src.Position, sc.Room.Mesh, imgCfg)
		}
	}

	c.lastRoomHash = roomHash
	c.lastMaxOrder = cfg.MaxOrder
	c.lastRoomKind = scene.RoomKindMesh
	c.lastMesh = sc.Room.Mesh
	c.lastSourcePosMsh = sourcePositions
	c.lastMeshMaxDist = imgCfg.MaxDistance
	c.shoeboxCache = nil
	c.lastShoeboxDims = nil
	c.lastSourcePosSbx = nil

	return evaluateMeshMultiSource(c.meshCache, sc, cfg)
}

func collectSourcePositions(sc *scene.Scene) []geometry.Vec3 {
	positions := make([]geometry.Vec3, len(sc.Sources))
	for i, src := range sc.Sources {
		positions[i] = src.Position
	}

	return positions
}

// evaluateShoeboxMultiSource produces ir.Events from per-source cached image
// sources. Unlike EvaluateShoebox, each scene source uses its own image-source
// list (indexed by source position in sc.Sources).
func evaluateShoeboxMultiSource(perSource [][]ImageSource, sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	if len(perSource) != len(sc.Sources) {
		return nil, fmt.Errorf("image source cache length %d does not match source count %d", len(perSource), len(sc.Sources))
	}

	bandSpec := cfg.BandSpec
	if bandSpec.BandCount() == 0 {
		bandSpec = sc.BandSpec
	}

	speedOfSound := cfg.SpeedOfSound
	if speedOfSound <= 0 {
		speedOfSound = acoustics.SpeedOfSound
	}

	receiver := sc.Receivers[0]
	events := make([]ir.Event, 0)

	for srcIndex, source := range sc.Sources {
		direct, ok := directEvent(source, receiver, bandSpec, speedOfSound)
		if ok {
			events = append(events, direct)
		}

		for _, imgSrc := range perSource[srcIndex] {
			if imgSrc.Order == 0 {
				continue
			}

			path, pathOK := reflectionPath(imgSrc, receiver.Position)
			if !pathOK {
				continue
			}

			event, evOK := specularEvent(source, receiver, imgSrc, path, sc.Room.Shoebox, sc.Materials, bandSpec, speedOfSound)
			if evOK {
				events = append(events, event)
			}
		}
	}

	sortEvents(events)

	return events, nil
}

// evaluateMeshMultiSource produces ir.Events from per-source cached mesh
// image sources.
func evaluateMeshMultiSource(perSource [][]MeshImageSource, sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	if len(perSource) != len(sc.Sources) {
		return nil, fmt.Errorf("mesh image source cache length %d does not match source count %d", len(perSource), len(sc.Sources))
	}

	bandSpec := cfg.BandSpec
	if bandSpec.BandCount() == 0 {
		bandSpec = sc.BandSpec
	}

	speedOfSound := cfg.SpeedOfSound
	if speedOfSound <= 0 {
		speedOfSound = acoustics.SpeedOfSound
	}

	bvh := geometry.BuildBVH(sc.Room.Mesh)
	material := meshMaterial(sc)
	receiver := sc.Receivers[0]

	events := make([]ir.Event, 0)

	for srcIndex, source := range sc.Sources {
		direct, ok := directEvent(source, receiver, bandSpec, speedOfSound)
		if ok {
			events = append(events, direct)
		}

		for _, imgSrc := range perSource[srcIndex] {
			if imgSrc.Order == 0 {
				continue
			}

			event, evOK := meshSpecularEvent(source, receiver, imgSrc, sc.Room.Mesh, bvh, material, bandSpec, speedOfSound)
			if evOK {
				events = append(events, event)
			}
		}
	}

	if cfg.EnableDiffraction {
		events = append(events, meshDiffractionEvents(sc, cfg, bvh, material, bandSpec, speedOfSound)...)
	}

	sortEvents(events)

	return events, nil
}

func sortEvents(events []ir.Event) {
	sort.Slice(events, func(i, j int) bool {
		if events[i].TimeSeconds != events[j].TimeSeconds {
			return events[i].TimeSeconds < events[j].TimeSeconds
		}

		if events[i].Kind != events[j].Kind {
			return events[i].Kind < events[j].Kind
		}

		return events[i].DistanceMeters < events[j].DistanceMeters
	})
}
