package export_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestExternalToolSceneMetadataRoundTrip(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("..", "testdata", "interop", "external_gui_mesh.json")

	sc, err := scene.LoadSceneFile(fixturePath)
	if err != nil {
		t.Fatalf("LoadSceneFile() error = %v", err)
	}

	if err := scene.Validate(sc); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	tmpRoot := t.TempDir()
	projectDir := filepath.Join(tmpRoot, "project")
	roomsDir := filepath.Join(tmpRoot, "rooms")

	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}

	if err := os.MkdirAll(roomsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(rooms) error = %v", err)
	}

	cubeSource, err := os.ReadFile(filepath.Join("..", "testdata", "rooms", "cube.obj"))
	if err != nil {
		t.Fatalf("ReadFile(cube.obj) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(roomsDir, "cube.obj"), cubeSource, 0o600); err != nil {
		t.Fatalf("WriteFile(cube.obj) error = %v", err)
	}

	outputPath := filepath.Join(projectDir, "scene.json")

	if err := export.WriteSceneJSON(outputPath, sc); err != nil {
		t.Fatalf("WriteSceneJSON() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	text := string(data)
	for _, want := range []string{
		`"kind": "mesh"`,
		`"meshPath": "../rooms/cube.obj"`,
		`"concrete"`,
		`"glass"`,
		`"sources"`,
		`"receivers"`,
		`"cardioid"`,
		`"omni"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("canonical scene JSON missing %q:\n%s", want, text)
		}
	}

	roundTripped, err := scene.LoadSceneFile(outputPath)
	if err != nil {
		t.Fatalf("LoadSceneFile(round-tripped) error = %v", err)
	}

	if err := scene.Validate(roundTripped); err != nil {
		t.Fatalf("Validate(round-tripped) error = %v", err)
	}

	if len(roundTripped.Sources) != 1 || len(roundTripped.Receivers) != 1 {
		t.Fatalf("round-tripped scene counts = %d sources, %d receivers", len(roundTripped.Sources), len(roundTripped.Receivers))
	}
}
