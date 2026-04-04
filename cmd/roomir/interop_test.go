package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalToolMeshSceneLateRenderMatchesReferenceIR(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("..", "..", "testdata", "interop", "external_gui_mesh.json")
	referenceB64Path := filepath.Join("..", "..", "testdata", "interop", "external_gui_mesh_reference.wav.b64")

	referenceData, err := os.ReadFile(referenceB64Path)
	if err != nil {
		t.Fatalf("ReadFile(reference) error = %v", err)
	}

	referenceWAV, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(referenceData)))
	if err != nil {
		t.Fatalf("DecodeString(reference) error = %v", err)
	}

	tmpDir := t.TempDir()
	referencePath := filepath.Join(tmpDir, "reference.wav")
	renderedPath := filepath.Join(tmpDir, "rendered.wav")

	err = os.WriteFile(referencePath, referenceWAV, 0o600)
	if err != nil {
		t.Fatalf("WriteFile(reference) error = %v", err)
	}

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"render",
		fixturePath,
		"-o", renderedPath,
		"--mode", "late",
		"--duration", "0.15",
		"--num-rays", "128",
		"--max-order", "1",
	})

	if exitCode := run(cmd); exitCode != 0 {
		t.Fatalf("run(render) = %d, want 0; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	compareCmd := newRootCommand()
	compareStdout := &bytes.Buffer{}
	compareStderr := &bytes.Buffer{}

	compareCmd.SetOut(compareStdout)
	compareCmd.SetErr(compareStderr)
	compareCmd.SetArgs([]string{
		"compare",
		referencePath,
		renderedPath,
	})

	if exitCode := run(compareCmd); exitCode != 0 {
		t.Fatalf("run(compare) = %d, want 0; stdout=%q stderr=%q", exitCode, compareStdout.String(), compareStderr.String())
	}

	if got := compareStdout.String(); !strings.Contains(got, "correlation") || !strings.Contains(got, "125 Hz band") {
		t.Fatalf("comparison output = %q, want report rows", got)
	}
}
