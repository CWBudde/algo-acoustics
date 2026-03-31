package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/wav"
)

func TestRenderStereoCommandWritesStereoWAV(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rendered-stereo.wav")

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"render-stereo",
		filepath.Join("..", "..", "testdata", "rooms", "shoebox_simple.json"),
		"-o", outputPath,
		"--max-order", "3",
		"--duration", "1.5",
	})

	if exitCode := run(cmd); exitCode != 0 {
		t.Fatalf("run() = %d, want 0; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	if got := stderr.String(); !strings.Contains(got, "rendered stereo") {
		t.Fatalf("stderr = %q, want stereo render summary", got)
	}

	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("open wav: %v", err)
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	decoded, err := decoder.FullPCMBuffer()
	if err != nil {
		t.Fatalf("FullPCMBuffer() error = %v", err)
	}

	if got, want := int(decoder.NumChans), 2; got != want {
		t.Fatalf("NumChans = %d, want %d", got, want)
	}
	if len(decoded.Data) == 0 {
		t.Fatal("decoded stereo WAV is empty")
	}
}
