package main

import (
	"bytes"
	"encoding/base64"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/metrics"
)

func TestExternalToolMeshSceneLateRenderMatchesReferenceIR(t *testing.T) {
	t.Parallel()

	// This is a repo-generated render regression golden for an externally
	// authored scene, not a WAV exported by a third-party acoustic renderer.
	// See docs/external-tool-compatibility.md for the regeneration command.
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

	referenceBuffer, err := loadComparisonBuffer(referencePath)
	if err != nil {
		t.Fatalf("loadComparisonBuffer(reference) error = %v", err)
	}

	renderedBuffer, err := loadComparisonBuffer(renderedPath)
	if err != nil {
		t.Fatalf("loadComparisonBuffer(rendered) error = %v", err)
	}

	rows, err := metrics.CompareBuffers(referenceBuffer, renderedBuffer, acoustics.Octave6)
	if err != nil {
		t.Fatalf("CompareBuffers() error = %v", err)
	}

	if got, want := len(rows), 3+acoustics.Octave6.BandCount(); got != want {
		t.Fatalf("len(comparison rows) = %d, want %d", got, want)
	}

	peak := rows[0]
	if peak.Expected <= 0 || peak.Actual <= 0 {
		t.Errorf("interop peak amplitudes must be non-zero: reference=%g rendered=%g", peak.Expected, peak.Actual)
	} else if relativeDelta(peak.Expected, peak.Actual) > 0.05 {
		t.Errorf("interop peak amplitude differs by more than 5%%: reference=%g rendered=%g", peak.Expected, peak.Actual)
	}

	rms := rows[1]
	if rms.Expected <= 0 || rms.Actual <= 0 {
		t.Errorf("interop RMS amplitudes must be non-zero: reference=%g rendered=%g", rms.Expected, rms.Actual)
	} else if relativeDelta(rms.Expected, rms.Actual) > 0.10 {
		t.Errorf("interop RMS amplitude differs by more than 10%%: reference=%g rendered=%g", rms.Expected, rms.Actual)
	}

	correlation := rows[2].Actual
	if math.IsNaN(correlation) || correlation < 0.9 {
		t.Errorf("interop waveform correlation = %.6f, want at least 0.9; reference requires regeneration or renderer drift must be resolved", correlation)
	}

	for _, row := range rows[3:] {
		if math.IsNaN(row.Delta) || math.Abs(row.Delta) > 3 {
			t.Errorf("interop %s delta = %.3f dB, want within +/-3 dB; reference requires regeneration or renderer drift must be resolved", row.Name, row.Delta)
		}
	}
}

func relativeDelta(expected, actual float64) float64 {
	return math.Abs(actual-expected) / math.Abs(expected)
}
