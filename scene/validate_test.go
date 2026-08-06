package scene_test

import (
	"encoding/json"
	"errors"
	"math"
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

func cubeMesh(minCorner, maxCorner geometry.Vec3) *geometry.Mesh {
	v000 := geometry.Vec3{X: minCorner.X, Y: minCorner.Y, Z: minCorner.Z}
	v001 := geometry.Vec3{X: minCorner.X, Y: minCorner.Y, Z: maxCorner.Z}
	v010 := geometry.Vec3{X: minCorner.X, Y: maxCorner.Y, Z: minCorner.Z}
	v011 := geometry.Vec3{X: minCorner.X, Y: maxCorner.Y, Z: maxCorner.Z}
	v100 := geometry.Vec3{X: maxCorner.X, Y: minCorner.Y, Z: minCorner.Z}
	v101 := geometry.Vec3{X: maxCorner.X, Y: minCorner.Y, Z: maxCorner.Z}
	v110 := geometry.Vec3{X: maxCorner.X, Y: maxCorner.Y, Z: minCorner.Z}
	v111 := geometry.Vec3{X: maxCorner.X, Y: maxCorner.Y, Z: maxCorner.Z}

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

	err := scene.Validate(&sc)
	if err != nil {
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

	err := scene.Validate(&sc)
	if err != nil {
		t.Fatalf("Validate() returned error for valid mesh scene: %v", err)
	}
}

func TestValidateNilScene(t *testing.T) {
	err := scene.Validate(nil)
	if err == nil || err.Error() != "scene is nil" {
		t.Fatalf("Validate(nil) = %v, want scene is nil", err)
	}
}

func TestValidateReportsRoomErrors(t *testing.T) {
	sc := validScene()

	sc.Room.Shoebox.Width = 0

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "shoebox width") {
		t.Fatalf("expected shoebox width validation error, got %v", err)
	}
}

func TestValidateReportsMissingMaterial(t *testing.T) {
	sc := validScene()

	sc.Room.Shoebox.WallMaterials[0] = "unknown"

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected missing material validation error, got %v", err)
	}
}

func TestValidateReportsMaterialBandMismatch(t *testing.T) {
	sc := validScene()
	material := sc.Materials["plaster"]
	material.AbsorptionByBand = []float64{0.1, 0.2}

	sc.Materials["plaster"] = material

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "absorption band count") {
		t.Fatalf("expected band mismatch validation error, got %v", err)
	}
}

func TestValidateRejectsInvalidAbsorption(t *testing.T) {
	sc := validScene()
	material := sc.Materials["plaster"]
	material.AbsorptionByBand[2] = 1.2
	sc.Materials["plaster"] = material

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "absorption[2]") {
		t.Fatalf("expected absorption range validation error, got %v", err)
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

	err := scene.Validate(&sc)
	if err != nil {
		t.Fatalf("non-monotonic scattering should emit warning only, got %v", err)
	}
}

func TestValidateReportsSourceOutsideRoom(t *testing.T) {
	sc := validScene()

	sc.Sources[0].Position = geometry.Vec3{X: 7, Y: 2, Z: 1.2}

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "source[0]") {
		t.Fatalf("expected source position validation error, got %v", err)
	}
}

func TestValidateReportsReceiverOutsideRoom(t *testing.T) {
	sc := validScene()

	sc.Receivers[0].Position = geometry.Vec3{X: 7, Y: 2, Z: 1.2}

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "receiver[0] position") {
		t.Fatalf("expected receiver position validation error, got %v", err)
	}
}

func TestValidateReportsMissingHRTF(t *testing.T) {
	sc := validScene()

	sc.Receivers[0].HRTF = nil

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "binaural receivers require an HRTF") {
		t.Fatalf("expected HRTF validation error, got %v", err)
	}
}

func TestValidateReportsSampleRate(t *testing.T) {
	sc := validScene()

	sc.SampleRate = 0

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "sample rate") {
		t.Fatalf("expected sample rate validation error, got %v", err)
	}
}

func TestValidateAggregatesNonFiniteValues(t *testing.T) {
	sc := validScene()
	sc.Room.Shoebox.Width = math.NaN()
	sc.Room.Shoebox.Depth = math.Inf(1)
	sc.Sources[0].Position.X = math.Inf(-1)
	sc.Receivers[0].Position.Y = math.NaN()
	material := sc.Materials["plaster"]
	material.AbsorptionByBand[0] = math.NaN()
	material.ScatteringByBand[1] = math.Inf(1)
	sc.Materials["plaster"] = material

	err := scene.Validate(&sc)
	if err == nil {
		t.Fatal("Validate() returned nil for non-finite scene values")
	}

	var validationErrors scene.ValidationErrors
	if !errors.As(err, &validationErrors) || len(validationErrors) < 6 {
		t.Fatalf("Validate() error = %T %v, want at least six aggregated ValidationErrors", err, err)
	}

	for _, want := range []string{"shoebox width", "shoebox depth", "source[0] position", "receiver[0] position", "absorption[0]", "scattering[1]"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() error %q does not contain %q", err, want)
		}
	}
}

func TestValidateRejectsMalformedBandSpecs(t *testing.T) {
	tests := []struct {
		name string
		spec acoustics.BandSpec
		want string
	}{
		{name: "empty", spec: acoustics.BandSpec{}, want: "at least one band"},
		{
			name: "mismatched lengths",
			spec: acoustics.BandSpec{CenterFreqs: []float64{125, 250}, LowerEdges: []float64{80}, UpperEdges: []float64{180, 355}},
			want: "lengths must match",
		},
		{
			name: "non-positive",
			spec: acoustics.BandSpec{CenterFreqs: []float64{0}, LowerEdges: []float64{-1}, UpperEdges: []float64{100}},
			want: "greater than zero",
		},
		{
			name: "unordered band",
			spec: acoustics.BandSpec{CenterFreqs: []float64{125}, LowerEdges: []float64{150}, UpperEdges: []float64{200}},
			want: "lower edge < center frequency < upper edge",
		},
		{
			name: "unordered sequence",
			spec: acoustics.BandSpec{CenterFreqs: []float64{250, 125}, LowerEdges: []float64{180, 80}, UpperEdges: []float64{355, 180}},
			want: "strictly increasing",
		},
		{
			name: "non-finite",
			spec: acoustics.BandSpec{CenterFreqs: []float64{math.NaN()}, LowerEdges: []float64{80}, UpperEdges: []float64{180}},
			want: "finite",
		},
		{
			name: "above Nyquist",
			spec: acoustics.BandSpec{CenterFreqs: []float64{20000}, LowerEdges: []float64{18000}, UpperEdges: []float64{25000}},
			want: "exceeds Nyquist",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sc := validScene()
			sc.BandSpec = test.spec

			err := scene.Validate(&sc)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want message containing %q", err, test.want)
			}
		})
	}
}

func TestValidateRejectsUnsupportedReceiverType(t *testing.T) {
	sc := validScene()
	sc.Receivers[0].Type = scene.ReceiverType("ambisonic")

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "unsupported receiver type") {
		t.Fatalf("Validate() error = %v, want unsupported receiver type", err)
	}
}

func TestValidateAcceptsEmptyReceiverTypeAsOmni(t *testing.T) {
	sc := validScene()
	sc.Receivers[0].Type = ""
	sc.Receivers[0].HRTF = nil

	err := scene.Validate(&sc)
	if err != nil {
		t.Fatalf("Validate() error = %v, want empty receiver type accepted as implicit omni", err)
	}
}

func TestValidateRejectsHRTFSampleRateMismatch(t *testing.T) {
	sc := validScene()
	sc.Receivers[0].HRTF = hrtf.NearestNeighborDataset{SampleRateHz: 44100}

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "HRTF sample rate") {
		t.Fatalf("Validate() error = %v, want HRTF sample-rate mismatch", err)
	}
}

func TestValidateRejectsUndefinedMeshMaterial(t *testing.T) {
	sc := validMeshScene()
	sc.Room.MeshMaterial = "missing"

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), `undefined material "missing"`) {
		t.Fatalf("Validate() error = %v, want undefined mesh material", err)
	}
}

func TestValidateAcceptsBandIndependentConvenienceMaterials(t *testing.T) {
	tests := []struct {
		name     string
		spec     acoustics.BandSpec
		material scene.Material
	}{
		{name: "absorptive octave6", spec: acoustics.Octave6, material: scene.MaterialFullyAbsorptive()},
		{name: "absorptive octave8", spec: acoustics.Octave8, material: scene.MaterialFullyAbsorptive()},
		{name: "reflective octave6", spec: acoustics.Octave6, material: scene.MaterialFullyReflective()},
		{name: "reflective octave8", spec: acoustics.Octave8, material: scene.MaterialFullyReflective()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sc := validScene()
			sc.BandSpec = test.spec

			sc.Materials = map[string]scene.Material{test.material.Name: test.material}
			for index := range sc.Room.Shoebox.WallMaterials {
				sc.Room.Shoebox.WallMaterials[index] = test.material.Name
			}

			err := scene.Validate(&sc)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestSceneJSONRoundTrip(t *testing.T) {
	original := validScene()

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}

	var decoded scene.Scene

	err = json.Unmarshal(encoded, &decoded)
	if err != nil {
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

	err = scene.Validate(sc)
	if err != nil {
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

	err = scene.Validate(sc)
	if err != nil {
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
