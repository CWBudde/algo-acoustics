package scene

import "github.com/cwbudde/algo-acoustics/geometry"

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
//nolint:tagliatelle // wallMaterials is part of the established public scene schema.
type Shoebox struct {
	Width         float64   `json:"width"`
	Depth         float64   `json:"depth"`
	Height        float64   `json:"height"`
	WallMaterials [6]string `json:"wallMaterials"`
}

// Bounds returns the shoebox as an axis-aligned box anchored at the origin.
func (s Shoebox) Bounds() geometry.Box {
	return geometry.NewBox(geometry.Vec3Zero, geometry.Vec3{X: s.Width, Y: s.Depth, Z: s.Height})
}

// Room is the top-level geometry container for a scene.
//
//nolint:tagliatelle // Camel-case tags are part of the established public scene schema.
type Room struct {
	Kind         RoomKind       `json:"kind"`
	Shoebox      *Shoebox       `json:"shoebox,omitempty"`
	MeshPath     string         `json:"meshPath,omitempty"`
	Mesh         *geometry.Mesh `json:"mesh,omitempty"`
	MeshMaterial string         `json:"meshMaterial,omitempty"`
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
