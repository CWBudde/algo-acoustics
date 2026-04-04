package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectCommandPrintsNormalizedSceneSummary(t *testing.T) {
	t.Parallel()

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", filepath.Join("..", "..", "testdata", "rooms", "shoebox_simple.json")})

	if exitCode := run(cmd); exitCode != 0 {
		t.Fatalf("run() = %d, want 0; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	got := stdout.String()
	checks := []string{
		"scene summary",
		"room: shoebox",
		"materials: 1",
		"sources: 1",
		"receivers: 1",
		"band count: 6",
		"sample rate: 48000",
	}

	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Fatalf("inspect output = %q, want substring %q", got, want)
		}
	}
}
