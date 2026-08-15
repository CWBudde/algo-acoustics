package scene_test

import (
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func triangleMaterialTestMaterials() map[string]scene.Material {
	return map[string]scene.Material{
		"soft": {Name: "soft", AbsorptionByBand: []float64{0.6}, ScatteringByBand: []float64{0.1}},
		"hard": {Name: "hard", AbsorptionByBand: []float64{0.02}, ScatteringByBand: []float64{0.05}},
	}
}

func TestTriangleMaterialNameFallsBackToMeshMaterial(t *testing.T) {
	t.Parallel()

	room := scene.Room{
		Kind:              scene.RoomKindMesh,
		MeshMaterial:      "hard",
		TriangleMaterials: []string{"soft", "", "soft"},
	}

	tests := []struct {
		name  string
		index int
		want  string
	}{
		{name: "explicit override", index: 0, want: "soft"},
		{name: "empty entry falls back", index: 1, want: "hard"},
		{name: "past the end falls back", index: 3, want: "hard"},
		{name: "negative index falls back", index: -1, want: "hard"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := room.TriangleMaterialName(test.index); got != test.want {
				t.Fatalf("TriangleMaterialName(%d) = %q, want %q", test.index, got, test.want)
			}
		})
	}
}

func TestMaterialForTriangleResolvesAndFallsBackToFullyReflective(t *testing.T) {
	t.Parallel()

	materials := triangleMaterialTestMaterials()
	room := scene.Room{
		Kind:              scene.RoomKindMesh,
		MeshMaterial:      "hard",
		TriangleMaterials: []string{"soft", "", "missing"},
	}

	if got := room.MaterialForTriangle(0, materials).Name; got != "soft" {
		t.Fatalf("triangle 0 material = %q, want %q", got, "soft")
	}

	if got := room.MaterialForTriangle(1, materials).Name; got != "hard" {
		t.Fatalf("triangle 1 material = %q, want %q", got, "hard")
	}

	// An undefined name resolves to fully reflective, matching the contract the
	// solvers already relied on for whole-mesh materials.
	if got := room.MaterialForTriangle(2, materials).Name; got != "fully_reflective" {
		t.Fatalf("triangle 2 material = %q, want fully_reflective", got)
	}

	bare := scene.Room{Kind: scene.RoomKindMesh}
	if got := bare.MaterialForTriangle(0, materials).Name; got != "fully_reflective" {
		t.Fatalf("unnamed mesh material = %q, want fully_reflective", got)
	}
}

func triangleMaterialScene(t *testing.T, room scene.Room) *scene.Scene {
	t.Helper()

	return &scene.Scene{
		Room:      room,
		Materials: triangleMaterialTestMaterials(),
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 1, Y: 1, Z: 1},
			Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 3, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}

func TestValidateTriangleMaterials(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(geometry.Vec3Zero, geometry.Vec3{X: 4, Y: 3, Z: 2.5})
	triangleCount := len(mesh.Triangles)

	full := make([]string, triangleCount)
	for index := range full {
		full[index] = "hard"
	}

	tests := []struct {
		name       string
		room       scene.Room
		wantErr    bool
		wantSubstr string
	}{
		{
			name: "matching count is accepted",
			room: scene.Room{Kind: scene.RoomKindMesh, Mesh: mesh, MeshMaterial: "hard", TriangleMaterials: full},
		},
		{
			name: "absent table is accepted",
			room: scene.Room{Kind: scene.RoomKindMesh, Mesh: mesh, MeshMaterial: "hard"},
		},
		{
			name:       "count mismatch is rejected",
			room:       scene.Room{Kind: scene.RoomKindMesh, Mesh: mesh, MeshMaterial: "hard", TriangleMaterials: []string{"hard"}},
			wantErr:    true,
			wantSubstr: "triangle material count",
		},
		{
			name: "undefined name is rejected",
			room: scene.Room{
				Kind: scene.RoomKindMesh, Mesh: mesh, MeshMaterial: "hard",
				TriangleMaterials: append(append([]string(nil), full[:triangleCount-1]...), "nope"),
			},
			wantErr:    true,
			wantSubstr: `undefined material "nope"`,
		},
		{
			name: "shoebox room must not set triangle materials",
			room: scene.Room{
				Kind: scene.RoomKindShoebox,
				Shoebox: &scene.Shoebox{
					Width: 4, Depth: 3, Height: 2.5,
					WallMaterials: [6]string{"hard", "hard", "hard", "hard", "hard", "hard"},
				},
				TriangleMaterials: []string{"hard"},
			},
			wantErr:    true,
			wantSubstr: "must not set triangle materials",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := scene.Validate(triangleMaterialScene(t, test.room))
			if test.wantErr {
				if err == nil {
					t.Fatal("Validate succeeded, want an error")
				}

				if !strings.Contains(err.Error(), test.wantSubstr) {
					t.Fatalf("Validate error = %v, want it to mention %q", err, test.wantSubstr)
				}

				return
			}

			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

func TestTriangleMaterialsRoundTripThroughJSON(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(geometry.Vec3Zero, geometry.Vec3{X: 4, Y: 3, Z: 2.5})

	materials := make([]string, len(mesh.Triangles))
	for index := range materials {
		materials[index] = "hard"
	}

	materials[0] = "soft"

	original := triangleMaterialScene(t, scene.Room{
		Kind: scene.RoomKindMesh, Mesh: mesh, MeshMaterial: "hard", TriangleMaterials: materials,
	})

	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	if !strings.Contains(string(data), `"triangleMaterials"`) {
		t.Fatal("marshalled scene does not carry triangleMaterials")
	}

	var restored scene.Scene

	err = restored.UnmarshalJSON(data)
	if err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}

	if got := restored.Room.TriangleMaterialName(0); got != "soft" {
		t.Fatalf("restored triangle 0 material = %q, want soft", got)
	}

	if got := len(restored.Room.TriangleMaterials); got != len(mesh.Triangles) {
		t.Fatalf("restored triangle material count = %d, want %d", got, len(mesh.Triangles))
	}
}
