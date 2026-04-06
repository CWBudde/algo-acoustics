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
