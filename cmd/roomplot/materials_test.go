package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterialsCommandPrintsLibraryMaterial(t *testing.T) {
	t.Parallel()

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"materials", "glass"})

	if exitCode := run(cmd); exitCode != 0 {
		t.Fatalf("run() = %d, want 0; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	got := stdout.String()
	checks := []string{
		"material: glass",
		"band",
		"125",
		"4000",
	}

	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Fatalf("materials output = %q, want substring %q", got, want)
		}
	}
}

func TestMaterialsCommandLoadsCSVFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "stage_curtain.csv")

	err := os.WriteFile(path, []byte(strings.TrimSpace(`
band,center_hz,absorption,scattering
0,125,0.20,0.15
1,250,0.35,0.18
2,500,0.50,0.20
3,1000,0.65,0.22
4,2000,0.70,0.25
5,4000,0.65,0.28
`)), 0o600)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"materials", path, "--format", "csv"})

	if exitCode := run(cmd); exitCode != 0 {
		t.Fatalf("run() = %d, want 0; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	got := stdout.String()
	checks := []string{
		"name,band,center_hz,absorption,scattering",
		"stage_curtain,0,125,0.2,0.15",
		"stage_curtain,5,4000,0.65,0.28",
	}

	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Fatalf("materials CSV output = %q, want substring %q", got, want)
		}
	}
}
