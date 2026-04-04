package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestSceneSummaryCommandPrintsSceneMetadata(t *testing.T) {
	t.Parallel()

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"scene-summary", filepath.Join("..", "..", "testdata", "rooms", "mesh_cube.json")})

	if exitCode := run(cmd); exitCode != 0 {
		t.Fatalf("run() = %d, want 0; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	got := stdout.String()
	checks := []string{
		"scene summary",
		"room: mesh",
		"cube.obj",
		"materials: 1",
		"sources: 1",
		"receivers: 1",
		"band count: 6",
	}

	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Fatalf("scene-summary output = %q, want substring %q", got, want)
		}
	}
}
