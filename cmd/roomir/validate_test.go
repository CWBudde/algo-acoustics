package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/scene"
)

func TestValidateCommandAcceptsValidScene(t *testing.T) {
	t.Parallel()

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"validate", filepath.Join("..", "..", "testdata", "rooms", "shoebox_simple.json")})

	if exitCode := run(cmd); exitCode != 0 {
		t.Fatalf("run() = %d, want 0; stderr=%q", exitCode, stderr.String())
	}

	if got := stdout.String(); !strings.Contains(got, "✓ scene is valid") {
		t.Fatalf("stdout = %q, want success marker", got)
	}
}

func TestValidateCommandReportsInvalidScene(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("..", "..", "testdata", "rooms", "shoebox_simple.json")

	fixtureBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("ReadFile() failed: %v", err)
	}

	var sc scene.Scene
	err = json.Unmarshal(fixtureBytes, &sc)
	if err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}

	sc.SampleRate = 0

	invalidBytes, err := json.Marshal(sc)
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}

	invalidPath := filepath.Join(t.TempDir(), "invalid-scene.json")
	err = os.WriteFile(invalidPath, invalidBytes, 0o600)
	if err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"validate", invalidPath})

	if exitCode := run(cmd); exitCode != 1 {
		t.Fatalf("run() = %d, want 1; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	if got := stdout.String(); !strings.Contains(got, "sample rate") {
		t.Fatalf("stdout = %q, want validation error", got)
	}
}
