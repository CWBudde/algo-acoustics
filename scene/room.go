package scene

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// RoomKind describes the supported room representations.
type RoomKind string

const (
	// RoomKindShoebox is an axis-aligned rectangular room.
	RoomKindShoebox RoomKind = "shoebox"
	// RoomKindMesh is a triangulated room enclosure.
	RoomKindMesh RoomKind = "mesh"
)

// Shoebox stores axis-aligned room dimensions and wall-material references.
//
//nolint:recvcheck,tagliatelle // Mutable JSON receiver; wallMaterials is part of the established schema.
type Shoebox struct {
	Width         float64       `json:"width"`
	Depth         float64       `json:"depth"`
	Height        float64       `json:"height"`
	WallMaterials [6]string     `json:"wallMaterials"`
	Origin        geometry.Vec3 `json:"-"`
}

//nolint:tagliatelle // This compatibility payload mirrors the public scene schema.
type shoeboxJSON struct {
	Width         float64        `json:"width"`
	Depth         float64        `json:"depth"`
	Height        float64        `json:"height"`
	WallMaterials [6]string      `json:"wallMaterials"`
	Origin        *geometry.Vec3 `json:"origin,omitempty"`
}

// MarshalJSON preserves the legacy shoebox representation when the origin is
// the zero vector.
func (s Shoebox) MarshalJSON() ([]byte, error) {
	payload := shoeboxJSON{
		Width:         s.Width,
		Depth:         s.Depth,
		Height:        s.Height,
		WallMaterials: s.WallMaterials,
	}

	if s.Origin != geometry.Vec3Zero {
		origin := s.Origin
		payload.Origin = &origin
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal shoebox: %w", err)
	}

	return data, nil
}

// UnmarshalJSON accepts both legacy origin-anchored shoeboxes and translated
// shoeboxes.
func (s *Shoebox) UnmarshalJSON(data []byte) error {
	var payload shoeboxJSON

	err := json.Unmarshal(data, &payload)
	if err != nil {
		return fmt.Errorf("unmarshal shoebox: %w", err)
	}

	s.Width = payload.Width
	s.Depth = payload.Depth
	s.Height = payload.Height
	s.WallMaterials = payload.WallMaterials

	s.Origin = geometry.Vec3Zero
	if payload.Origin != nil {
		s.Origin = *payload.Origin
	}

	return nil
}

// Bounds returns the shoebox as an axis-aligned world-space box.
func (s Shoebox) Bounds() geometry.Box {
	return geometry.NewBox(s.Origin, s.Origin.Add(geometry.Vec3{X: s.Width, Y: s.Depth, Z: s.Height}))
}

// Room is the top-level geometry container for a scene.
//
//nolint:tagliatelle // Camel-case tags are part of the established public scene schema.
type Room struct {
	Kind     RoomKind       `json:"kind"`
	Shoebox  *Shoebox       `json:"shoebox,omitempty"`
	MeshPath string         `json:"meshPath,omitempty"`
	Mesh     *geometry.Mesh `json:"mesh,omitempty"`
	// MeshMaterial names the material applied to every triangle that
	// TriangleMaterials does not override.
	MeshMaterial string `json:"meshMaterial,omitempty"`
	// TriangleMaterials optionally names one material per mesh triangle,
	// parallel to Mesh.Triangles. It exists so that geometry assembled from
	// several surfaces — most importantly a room group merged across open
	// portals — keeps each surface's own material instead of collapsing them
	// onto a single MeshMaterial. Empty entries fall back to MeshMaterial.
	TriangleMaterials []string `json:"triangleMaterials,omitempty"`
}

// TriangleMaterialName returns the material name governing a mesh triangle,
// falling back to MeshMaterial when TriangleMaterials is absent, too short, or
// carries an empty entry.
func (r Room) TriangleMaterialName(index int) string {
	if index < 0 || index >= len(r.TriangleMaterials) {
		return r.MeshMaterial
	}

	if name := r.TriangleMaterials[index]; name != "" {
		return name
	}

	return r.MeshMaterial
}

// MaterialForTriangle resolves the material governing a mesh triangle. An unset
// or unknown name yields MaterialFullyReflective, matching the behaviour the
// solvers already relied on for whole-mesh materials.
func (r Room) MaterialForTriangle(index int, materials map[string]Material) Material {
	name := r.TriangleMaterialName(index)
	if name == "" {
		return MaterialFullyReflective()
	}

	material, ok := materials[name]
	if !ok {
		return MaterialFullyReflective()
	}

	return material
}

// IsMesh reports whether the room uses triangulated geometry.
func (r Room) IsMesh() bool {
	return r.Kind == RoomKindMesh
}

// IsValid reports whether the room carries the geometry payload needed by its kind.
func (r Room) IsValid() bool {
	switch r.Kind {
	case RoomKindShoebox:
		return r.Shoebox != nil
	case RoomKindMesh:
		return r.Mesh != nil
	default:
		return false
	}
}

// Bounds returns the room bounding box if it can be derived from the room data.
func (r Room) Bounds() (geometry.Box, bool) {
	switch r.Kind {
	case RoomKindShoebox:
		if r.Shoebox == nil {
			return geometry.Box{}, false
		}

		return r.Shoebox.Bounds(), true
	case RoomKindMesh:
		if r.Mesh == nil || len(r.Mesh.Triangles) == 0 {
			return geometry.Box{}, false
		}

		bounds := r.Mesh.BoundingBox()
		if bounds.Volume() <= 0 {
			return geometry.Box{}, false
		}

		return bounds, true
	default:
		return geometry.Box{}, false
	}
}

// Volume returns the room's physical enclosed volume when it can be derived
// without approximating arbitrary geometry by its axis-aligned bounds.
func (r Room) Volume() (float64, bool) {
	switch r.Kind {
	case RoomKindShoebox:
		if r.Shoebox == nil {
			return 0, false
		}

		if !positiveFiniteDimension(r.Shoebox.Width) ||
			!positiveFiniteDimension(r.Shoebox.Depth) ||
			!positiveFiniteDimension(r.Shoebox.Height) {
			return 0, false
		}

		volume := r.Shoebox.Width * r.Shoebox.Depth * r.Shoebox.Height
		if !positiveFiniteDimension(volume) {
			return 0, false
		}

		return volume, true
	case RoomKindMesh:
		if r.Mesh == nil {
			return 0, false
		}

		return r.Mesh.EnclosedVolume()
	default:
		return 0, false
	}
}

func positiveFiniteDimension(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
