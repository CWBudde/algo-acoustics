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
