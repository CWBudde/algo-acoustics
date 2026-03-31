package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/wav"
)

func TestRenderCommandWritesWAVAndReportsSummary(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rendered.wav")

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"render",
		filepath.Join("..", "..", "testdata", "rooms", "shoebox_simple.json"),
		"-o", outputPath,
		"--max-order", "3",
		"--duration", "1.5",
		"--crossover-window", "blackman",
		"--crossover-window-alpha", "0.25",
	})

	if exitCode := run(cmd); exitCode != 0 {
		t.Fatalf("run() = %d, want 0; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	if got := stderr.String(); !strings.Contains(got, "rendered") || !strings.Contains(got, "1.500s") {
		t.Fatalf("stderr = %q, want render summary with event count and duration", got)
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

	if got, want := int(decoder.SampleRate), 48000; got != want {
		t.Fatalf("SampleRate = %d, want %d", got, want)
	}

	if got, want := int(decoder.NumChans), 1; got != want {
		t.Fatalf("NumChans = %d, want %d", got, want)
	}

	dx := 4.0 - 1.5
	dy := 0.0
	dz := 0.0
	distance := math.Sqrt(dx*dx + dy*dy + dz*dz)

	expectedSample := int(math.Round(distance / acoustics.SpeedOfSound * float64(decoder.SampleRate)))
	if expectedSample < 0 || expectedSample >= len(decoded.Data) {
		t.Fatalf("expected sample %d out of range for decoded length %d", expectedSample, len(decoded.Data))
	}

	start := max(expectedSample-1, 0)

	end := expectedSample + 1
	if end >= len(decoded.Data) {
		end = len(decoded.Data) - 1
	}

	for index := start; index <= end; index++ {
		if decoded.Data[index] != 0 {
			return
		}
	}

	t.Fatalf("decoded samples %d..%d are all zero, want direct-path spike near %d", start, end, expectedSample)
}

func TestRenderCommandRejectsUnknownCrossoverWindow(t *testing.T) {
	t.Parallel()

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"render",
		filepath.Join("..", "..", "testdata", "rooms", "shoebox_simple.json"),
		"-o", filepath.Join(t.TempDir(), "rendered.wav"),
		"--mode", "hybrid",
		"--crossover-window", "not-a-window",
	})

	if exitCode := run(cmd); exitCode == 0 {
		t.Fatalf("run() = %d, want non-zero; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	if got := stderr.String(); !strings.Contains(got, "invalid crossover window") {
		t.Fatalf("stderr = %q, want invalid crossover window error", got)
	}
}
