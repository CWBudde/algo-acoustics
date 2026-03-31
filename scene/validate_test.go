package scene_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	"github.com/cwbudde/algo-acoustics/scene"
)

func cubeMesh(min, max geometry.Vec3) *geometry.Mesh {
	v000 := geometry.Vec3{X: min.X, Y: min.Y, Z: min.Z}
	v001 := geometry.Vec3{X: min.X, Y: min.Y, Z: max.Z}
	v010 := geometry.Vec3{X: min.X, Y: max.Y, Z: min.Z}
	v011 := geometry.Vec3{X: min.X, Y: max.Y, Z: max.Z}
	v100 := geometry.Vec3{X: max.X, Y: min.Y, Z: min.Z}
	v101 := geometry.Vec3{X: max.X, Y: min.Y, Z: max.Z}
	v110 := geometry.Vec3{X: max.X, Y: max.Y, Z: min.Z}
	v111 := geometry.Vec3{X: max.X, Y: max.Y, Z: max.Z}

	return &geometry.Mesh{Triangles: []geometry.Triangle{
		{V0: v000, V1: v110, V2: v100},
		{V0: v000, V1: v010, V2: v110},
		{V0: v001, V1: v101, V2: v111},
		{V0: v001, V1: v111, V2: v011},
		{V0: v000, V1: v101, V2: v001},
		{V0: v000, V1: v100, V2: v101},
		{V0: v010, V1: v011, V2: v111},
		{V0: v010, V1: v111, V2: v110},
		{V0: v000, V1: v001, V2: v011},
		{V0: v000, V1: v011, V2: v010},
		{V0: v100, V1: v110, V2: v111},
		{V0: v100, V1: v111, V2: v101},
	}}
}

func validMeshScene() scene.Scene {
	sc := validScene()
	sc.Room = scene.Room{
		Kind:     scene.RoomKindMesh,
		MeshPath: "cube.obj",
		Mesh:     cubeMesh(geometry.Vec3Zero, geometry.Vec3{X: 4, Y: 4, Z: 4}),
	}
	sc.Sources[0].Position = geometry.Vec3{X: 1, Y: 1, Z: 1}
	sc.Receivers[0].Position = geometry.Vec3{X: 3, Y: 3, Z: 1.2}
	return sc
}

func validScene() scene.Scene {
	return scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6,
				Depth:  4.5,
				Height: 2.8,
				WallMaterials: [6]string{
					"plaster", "plaster", "plaster", "plaster", "plaster", "plaster",
				},
			},
		},
		Materials: map[string]scene.Material{
			"plaster": {
				Name:             "plaster",
				AbsorptionByBand: []float64{0.1, 0.1, 0.15, 0.2, 0.2, 0.25},
				Scattering:       [scene.NumBands]float64{0.02, 0.02, 0.02, 0.03, 0.03, 0.04},
				ScatteringByBand: []float64{0.02, 0.02, 0.02, 0.03, 0.03, 0.04},
			},
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 1.5, Y: 2.0, Z: 1.2},
			Orientation: geometry.QuatIdentity(),
			GainDB:      0,
			Directivity: directivity.OmniModel{},
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 4.0, Y: 2.0, Z: 1.2},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverBinaural,
			HRTF:        hrtf.NearestNeighborDataset{SampleRateHz: 48000},
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}

func TestValidateValidScene(t *testing.T) {
	sc := validScene()
	if err := scene.Validate(&sc); err != nil {
		t.Fatalf("Validate() returned error for valid scene: %v", err)
	}
}

func TestRoomHelpersForMesh(t *testing.T) {
	room := scene.Room{Kind: scene.RoomKindMesh}
	if !room.IsMesh() {
		t.Fatal("IsMesh() = false, want true")
	}
	if room.IsValid() {
		t.Fatal("IsValid() = true, want false when mesh is nil")
	}

	room.Mesh = cubeMesh(geometry.Vec3Zero, geometry.Vec3{X: 1, Y: 1, Z: 1})
	if !room.IsValid() {
		t.Fatal("IsValid() = false, want true when mesh is present")
	}
}

func TestValidateAcceptsMeshRoom(t *testing.T) {
	sc := validMeshScene()
	if err := scene.Validate(&sc); err != nil {
		t.Fatalf("Validate() returned error for valid mesh scene: %v", err)
	}
}

func TestValidateNilScene(t *testing.T) {
	if err := scene.Validate(nil); err == nil || err.Error() != "scene is nil" {
		t.Fatalf("Validate(nil) = %v, want scene is nil", err)
	}
}

func TestValidateReportsRoomErrors(t *testing.T) {
	sc := validScene()
	sc.Room.Shoebox.Width = 0
	if err := scene.Validate(&sc); err == nil || !strings.Contains(err.Error(), "shoebox width") {
		t.Fatalf("expected shoebox width validation error, got %v", err)
	}
}

func TestValidateReportsMissingMaterial(t *testing.T) {
	sc := validScene()
	sc.Room.Shoebox.WallMaterials[0] = "unknown"
	if err := scene.Validate(&sc); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected missing material validation error, got %v", err)
	}
}

func TestValidateReportsMaterialBandMismatch(t *testing.T) {
	sc := validScene()
	material := sc.Materials["plaster"]
	material.AbsorptionByBand = []float64{0.1}
	sc.Materials["plaster"] = material
	if err := scene.Validate(&sc); err == nil || !strings.Contains(err.Error(), "absorption band count") {
		t.Fatalf("expected band mismatch validation error, got %v", err)
	}
}

func TestValidateReportsScatteringOutOfRange(t *testing.T) {
	sc := validScene()
	material := sc.Materials["plaster"]
	material.Scattering[2] = 1.2
	material.ScatteringByBand = nil
	sc.Materials["plaster"] = material

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "scattering[2]") {
		t.Fatalf("expected scattering range validation error, got %v", err)
	}
}

func TestValidateAllowsNonMonotonicScattering(t *testing.T) {
	sc := validScene()
	material := sc.Materials["plaster"]
	material.Scattering = [scene.NumBands]float64{0.4, 0.3, 0.5, 0.4, 0.6, 0.5}
	material.ScatteringByBand = nil
	sc.Materials["plaster"] = material

	if err := scene.Validate(&sc); err != nil {
		t.Fatalf("non-monotonic scattering should emit warning only, got %v", err)
	}
}

func TestValidateReportsSourceOutsideRoom(t *testing.T) {
	sc := validScene()
	sc.Sources[0].Position = geometry.Vec3{X: 7, Y: 2, Z: 1.2}
	if err := scene.Validate(&sc); err == nil || !strings.Contains(err.Error(), "source[0]") {
		t.Fatalf("expected source position validation error, got %v", err)
	}
}

func TestValidateReportsReceiverOutsideRoom(t *testing.T) {
	sc := validScene()
	sc.Receivers[0].Position = geometry.Vec3{X: 7, Y: 2, Z: 1.2}
	if err := scene.Validate(&sc); err == nil || !strings.Contains(err.Error(), "receiver[0] position") {
		t.Fatalf("expected receiver position validation error, got %v", err)
	}
}

func TestValidateReportsMissingHRTF(t *testing.T) {
	sc := validScene()
	sc.Receivers[0].HRTF = nil
	if err := scene.Validate(&sc); err == nil || !strings.Contains(err.Error(), "binaural receivers require an HRTF") {
		t.Fatalf("expected HRTF validation error, got %v", err)
	}
}

func TestValidateReportsSampleRate(t *testing.T) {
	sc := validScene()
	sc.SampleRate = 0
	if err := scene.Validate(&sc); err == nil || !strings.Contains(err.Error(), "sample rate") {
		t.Fatalf("expected sample rate validation error, got %v", err)
	}
}

func TestSceneJSONRoundTrip(t *testing.T) {
	original := validScene()
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}

	var decoded scene.Scene
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}

	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("round-trip mismatch:\noriginal: %#v\ndecoded:  %#v", original, decoded)
	}
}

func TestLoadSceneFile(t *testing.T) {
	path := filepath.Join("..", "testdata", "rooms", "shoebox_simple.json")
	sc, err := scene.LoadSceneFile(path)
	if err != nil {
		t.Fatalf("LoadSceneFile() failed: %v", err)
	}

	if err := scene.Validate(sc); err != nil {
		t.Fatalf("loaded scene should validate: %v", err)
	}
}

func TestLoadSceneFileLoadsMeshOBJ(t *testing.T) {
	path := filepath.Join("..", "testdata", "rooms", "mesh_cube.json")
	sc, err := scene.LoadSceneFile(path)
	if err != nil {
		t.Fatalf("LoadSceneFile() failed: %v", err)
	}
	if !sc.Room.IsMesh() {
		t.Fatal("loaded room is not marked as mesh")
	}
	if sc.Room.Mesh == nil {
		t.Fatal("loaded mesh room has nil Mesh")
	}
	if len(sc.Room.Mesh.Triangles) != 12 {
		t.Fatalf("loaded mesh triangle count = %d, want 12", len(sc.Room.Mesh.Triangles))
	}
	if err := scene.Validate(sc); err != nil {
		t.Fatalf("loaded mesh scene should validate: %v", err)
	}
}

func TestLoadSceneReportsMeshLoadError(t *testing.T) {
	jsonScene := `{
		"room": {"kind": "mesh", "meshPath": "missing.obj"},
		"bandSpec": {"CenterFreqs": [125], "LowerEdges": [88.38834764831843], "UpperEdges": [176.7766952966369]},
		"sampleRate": 48000
	}`

	_, err := scene.LoadScene(strings.NewReader(jsonScene))
	if err == nil || !strings.Contains(err.Error(), "load room mesh") {
		t.Fatalf("LoadScene() error = %v, want mesh load failure", err)
	}
}
