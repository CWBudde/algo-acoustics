package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/wav"
)

func TestRunProducesNonSilentOutput(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, outputFilename)
	if err := run(outputPath); err != nil {
		t.Fatalf("run() error = %v", err)
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

	dx := 3.5 - 1.2
	dy := 2.2 - 1.0
	dz := 0.0
	distance := math.Sqrt(dx*dx + dy*dy + dz*dz)
	expectedSample := int(math.Round(distance / acoustics.SpeedOfSound * float64(decoder.SampleRate)))
	if expectedSample < 0 || expectedSample >= len(decoded.Data) {
		t.Fatalf("expected sample %d out of range for decoded length %d", expectedSample, len(decoded.Data))
	}

	var nonZeroFound bool
	for index, sample := range decoded.Data {
		if sample != 0 {
			nonZeroFound = true
			if index != expectedSample {
				t.Fatalf("first non-zero sample index = %d, want %d", index, expectedSample)
			}
			break
		}
	}
	if !nonZeroFound {
		t.Fatal("decoded WAV is silent")
	}
	if decoded.Data[expectedSample] == 0 {
		t.Fatalf("decoded[%d] = 0, want direct-path spike", expectedSample)
	}
}
