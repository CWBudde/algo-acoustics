package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/wav"
)

func TestRunProducesNonSilentOutput(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	outputPath := filepath.Join(tmpDir, outputFilename)

	err := run(outputPath)
	if err != nil {
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

	var nonZeroFound bool

	for _, sample := range decoded.Data {
		if sample != 0 {
			nonZeroFound = true
			break
		}
	}

	if !nonZeroFound {
		t.Fatal("decoded WAV is silent")
	}
}

func TestRunWithOptionsSupportsCrossoverWindowSelection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	outputPath := filepath.Join(tmpDir, outputFilename)

	err := runWithOptions(outputPath, exampleOptions{
		CrossoverWindow: hybrid.FadeWindowConfig{Name: "blackman", Alpha: 0.25},
	})
	if err != nil {
		t.Fatalf("runWithOptions() error = %v", err)
	}
}

func TestRunWithOptionsRejectsUnknownCrossoverWindow(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, outputFilename)

	err := runWithOptions(outputPath, exampleOptions{
		CrossoverWindow: hybrid.FadeWindowConfig{Name: "nope"},
	})
	if err == nil {
		t.Fatal("runWithOptions() error = nil, want invalid crossover window error")
	}

	if !strings.Contains(err.Error(), "unsupported fade window") {
		t.Fatalf("runWithOptions() error = %v, want unsupported fade window", err)
	}
}

func TestRunWASMProducesComparisonAndAudioBytes(t *testing.T) {
	t.Parallel()

	fixture, err := os.ReadFile(filepath.Clean(gllFixturePath))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	result, err := runWASM(fixture)
	if err != nil {
		t.Fatalf("runWASM() error = %v", err)
	}

	if got, want := result.FrontEnergyRatio > 1, true; got != want {
		t.Fatalf("FrontEnergyRatio > 1 = %v, want %v", got, want)
	}

	if got, want := result.RearEnergyRatio < 1, true; got != want {
		t.Fatalf("RearEnergyRatio < 1 = %v, want %v", got, want)
	}

	if len(result.WAVBytes) == 0 {
		t.Fatal("runWASM() returned empty WAV bytes")
	}
}
