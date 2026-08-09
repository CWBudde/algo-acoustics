package scene

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestGeometryHash_ShoeboxDeterministic(t *testing.T) {
	s := Scene{
		Room: Room{
			Kind: RoomKindShoebox,
			Shoebox: &Shoebox{
				Width: 5.0, Depth: 4.0, Height: 3.0,
			},
		},
		Sources:   []Source{{Position: geometry.Vec3{X: 1, Y: 2, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 3, Y: 2, Z: 1.5}}},
	}

	h1 := s.GeometryHash()
	h2 := s.GeometryHash()

	if h1 == 0 {
		t.Fatal("hash must be non-zero")
	}

	if h1 != h2 {
		t.Fatalf("expected deterministic hash, got %d and %d", h1, h2)
	}
}

func TestGeometryHash_ShoeboxChangeDimensions(t *testing.T) {
	base := Scene{
		Room: Room{
			Kind: RoomKindShoebox,
			Shoebox: &Shoebox{
				Width: 5.0, Depth: 4.0, Height: 3.0,
			},
		},
		Sources:   []Source{{Position: geometry.Vec3{X: 1, Y: 2, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 3, Y: 2, Z: 1.5}}},
	}

	changed := base
	changed.Room.Shoebox = &Shoebox{
		Width: 6.0, Depth: 4.0, Height: 3.0,
	}

	if base.GeometryHash() == changed.GeometryHash() {
		t.Fatal("different dimensions must produce different hashes")
	}
}

func TestGeometryHash_MaterialChangeNoEffect(t *testing.T) {
	s1 := Scene{
		Room: Room{
			Kind: RoomKindShoebox,
			Shoebox: &Shoebox{
				Width: 5.0, Depth: 4.0, Height: 3.0,
			},
		},
		Materials: map[string]Material{"wall": {Name: "concrete"}},
		Sources:   []Source{{Position: geometry.Vec3{X: 1, Y: 2, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 3, Y: 2, Z: 1.5}}},
	}

	s2 := Scene{
		Room: Room{
			Kind: RoomKindShoebox,
			Shoebox: &Shoebox{
				Width: 5.0, Depth: 4.0, Height: 3.0,
			},
		},
		Materials: map[string]Material{"wall": {Name: "glass"}},
		Sources:   []Source{{Position: geometry.Vec3{X: 1, Y: 2, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 3, Y: 2, Z: 1.5}}},
	}

	if s1.GeometryHash() != s2.GeometryHash() {
		t.Fatal("material changes must not affect geometry hash")
	}
}

func TestGeometryHash_SourcePositionChange(t *testing.T) {
	base := Scene{
		Room: Room{
			Kind: RoomKindShoebox,
			Shoebox: &Shoebox{
				Width: 5.0, Depth: 4.0, Height: 3.0,
			},
		},
		Sources:   []Source{{Position: geometry.Vec3{X: 1, Y: 2, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 3, Y: 2, Z: 1.5}}},
	}

	changed := Scene{
		Room: Room{
			Kind: RoomKindShoebox,
			Shoebox: &Shoebox{
				Width: 5.0, Depth: 4.0, Height: 3.0,
			},
		},
		Sources:   []Source{{Position: geometry.Vec3{X: 2, Y: 2, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 3, Y: 2, Z: 1.5}}},
	}

	if base.GeometryHash() == changed.GeometryHash() {
		t.Fatal("different source position must produce different hash")
	}
}

func TestRoomHash_IgnoresSourceAndReceiverMoves(t *testing.T) {
	base := Scene{
		Room: Room{
			Kind: RoomKindShoebox,
			Shoebox: &Shoebox{
				Width: 5.0, Depth: 4.0, Height: 3.0,
			},
		},
		Sources:   []Source{{Position: geometry.Vec3{X: 1, Y: 2, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 3, Y: 2, Z: 1.5}}},
	}

	moved := Scene{
		Room: Room{
			Kind: RoomKindShoebox,
			Shoebox: &Shoebox{
				Width: 5.0, Depth: 4.0, Height: 3.0,
			},
		},
		Sources:   []Source{{Position: geometry.Vec3{X: 2.5, Y: 1, Z: 0.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 4, Y: 3, Z: 2}}},
	}

	if base.RoomHash() != moved.RoomHash() {
		t.Fatal("RoomHash must be invariant under source/receiver moves")
	}

	if base.GeometryHash() == moved.GeometryHash() {
		t.Fatal("baseline GeometryHash should differ when sources/receivers move")
	}
}

func TestRoomHash_ChangesWithDimensions(t *testing.T) {
	base := Scene{
		Room: Room{
			Kind: RoomKindShoebox,
			Shoebox: &Shoebox{
				Width: 5.0, Depth: 4.0, Height: 3.0,
			},
		},
	}

	changed := Scene{
		Room: Room{
			Kind: RoomKindShoebox,
			Shoebox: &Shoebox{
				Width: 5.5, Depth: 4.0, Height: 3.0,
			},
		},
	}

	if base.RoomHash() == changed.RoomHash() {
		t.Fatal("RoomHash must differ when room dimensions change")
	}
}

func TestRoomHash_MeshChange(t *testing.T) {
	tri1 := geometry.Triangle{
		V0: geometry.Vec3{X: 0, Y: 0, Z: 0},
		V1: geometry.Vec3{X: 1, Y: 0, Z: 0},
		V2: geometry.Vec3{X: 0, Y: 1, Z: 0},
	}

	tri2 := geometry.Triangle{
		V0: geometry.Vec3{X: 0, Y: 0, Z: 0},
		V1: geometry.Vec3{X: 2, Y: 0, Z: 0},
		V2: geometry.Vec3{X: 0, Y: 1, Z: 0},
	}

	base := Scene{
		Room: Room{
			Kind: RoomKindMesh,
			Mesh: &geometry.Mesh{Triangles: []geometry.Triangle{tri1}},
		},
	}

	changed := Scene{
		Room: Room{
			Kind: RoomKindMesh,
			Mesh: &geometry.Mesh{Triangles: []geometry.Triangle{tri2}},
		},
	}

	if base.RoomHash() == changed.RoomHash() {
		t.Fatal("RoomHash must differ when mesh triangles change")
	}
}

func TestGeometryHash_MeshDeterministic(t *testing.T) {
	tri := geometry.Triangle{
		V0: geometry.Vec3{X: 0, Y: 0, Z: 0},
		V1: geometry.Vec3{X: 1, Y: 0, Z: 0},
		V2: geometry.Vec3{X: 0, Y: 1, Z: 0},
	}

	s := Scene{
		Room: Room{
			Kind: RoomKindMesh,
			Mesh: &geometry.Mesh{Triangles: []geometry.Triangle{tri}},
		},
		Sources:   []Source{{Position: geometry.Vec3{X: 0.2, Y: 0.2, Z: 0.1}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 0.5, Y: 0.5, Z: 0.1}}},
	}

	h1 := s.GeometryHash()
	h2 := s.GeometryHash()

	if h1 == 0 {
		t.Fatal("mesh hash must be non-zero")
	}

	if h1 != h2 {
		t.Fatalf("expected deterministic mesh hash, got %d and %d", h1, h2)
	}
}

func TestRoomHash_ChangesWithShoeboxOriginAndPortal(t *testing.T) {
	base := Scene{Rooms: []Room{
		{Kind: RoomKindShoebox, Shoebox: &Shoebox{Width: 5, Depth: 4, Height: 3}},
		{Kind: RoomKindShoebox, Shoebox: &Shoebox{Width: 5, Depth: 4, Height: 3, Origin: geometry.Vec3{X: 5}}},
	}}

	moved := base
	moved.Rooms = append([]Room(nil), base.Rooms...)

	moved.Rooms[1].Shoebox = &Shoebox{Width: 5, Depth: 4, Height: 3, Origin: geometry.Vec3{X: 6}}
	if base.RoomHash() == moved.RoomHash() {
		t.Fatal("RoomHash must differ when a room origin changes")
	}

	withPortal := base

	withPortal.Portals = []Portal{{
		RoomIndices: [2]int{0, 1},
		Polygon: []geometry.Vec3{
			{X: 5}, {X: 5, Y: 1}, {X: 5, Y: 1, Z: 1},
		},
		State: PortalClosed,
	}}
	if base.RoomHash() == withPortal.RoomHash() {
		t.Fatal("RoomHash must differ when portal geometry is added")
	}
}
