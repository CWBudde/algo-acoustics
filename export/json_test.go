package export_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestWriteSceneJSONWritesCanonicalScene(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "scene.json")

	sc := scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6,
				Depth:  4.5,
				Height: 2.8,
				WallMaterials: [6]string{
					"alpha", "alpha", "alpha", "alpha", "alpha", "alpha",
				},
			},
		},
		Materials: map[string]scene.Material{
			"zeta": {
				Name:             "zeta",
				AbsorptionByBand: []float64{0.1, 0.1, 0.15, 0.2, 0.2, 0.25},
				ScatteringByBand: []float64{0.02, 0.02, 0.02, 0.03, 0.03, 0.04},
			},
			"alpha": {
				Name:             "alpha",
				AbsorptionByBand: []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
				ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	err := export.WriteSceneJSON(outputPath, &sc)
	if err != nil {
		t.Fatalf("WriteSceneJSON() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := string(data)
	want := `{
  "room": {
    "kind": "shoebox",
    "shoebox": {
      "width": 6,
      "depth": 4.5,
      "height": 2.8,
      "wallMaterials": [
        "alpha",
        "alpha",
        "alpha",
        "alpha",
        "alpha",
        "alpha"
      ]
    }
  },
  "materials": {
    "alpha": {
      "name": "alpha",
      "absorptionByBand": [
        0.2,
        0.2,
        0.2,
        0.2,
        0.2,
        0.2
      ]
    },
    "zeta": {
      "name": "zeta",
      "absorptionByBand": [
        0.1,
        0.1,
        0.15,
        0.2,
        0.2,
        0.25
      ],
      "scatteringByBand": [
        0.02,
        0.02,
        0.02,
        0.03,
        0.03,
        0.04
      ]
    }
  },
  "bandSpec": {
    "CenterFreqs": [
      125,
      250,
      500,
      1000,
      2000,
      4000
    ],
    "LowerEdges": [
      88.38834764831843,
      176.77669529663686,
      353.5533905932737,
      707.1067811865474,
      1414.2135623730949,
      2828.4271247461897
    ],
    "UpperEdges": [
      176.7766952966369,
      353.5533905932738,
      707.1067811865476,
      1414.213562373095,
      2828.42712474619,
      5656.85424949238
    ]
  },
  "sampleRate": 48000
}
`

	if got != want {
		t.Fatalf("canonical JSON mismatch:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestWriteSceneJSONOmitsResolvedMeshPayload(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mesh-scene.json")

	sc, err := scene.LoadSceneFile(filepath.Join("..", "testdata", "rooms", "mesh_cube.json"))
	if err != nil {
		t.Fatalf("LoadSceneFile() error = %v", err)
	}

	err = export.WriteSceneJSON(outputPath, sc)
	if err != nil {
		t.Fatalf("WriteSceneJSON() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if strings.Contains(string(data), "\"mesh\":") {
		t.Fatalf("canonical mesh scene JSON should not inline resolved mesh data:\n%s", data)
	}

	if !strings.Contains(string(data), "\"meshPath\": \"cube.obj\"") {
		t.Fatalf("canonical mesh scene JSON should keep meshPath:\n%s", data)
	}
}
