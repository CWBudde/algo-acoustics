package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/cwbudde/wav"
)

func TestRenderStereoCommandWritesStereoWAV(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scenePath := filepath.Join("..", "..", "testdata", "rooms", "shoebox_simple.json")

	sc, err := scene.LoadSceneFile(scenePath)
	if err != nil {
		t.Fatalf("LoadSceneFile() error = %v", err)
	}

	sc.Receivers[0].HRTF = hrtf.NoopDataset{SampleRateHz: sc.SampleRate}

	sceneJSON, err := json.Marshal(sc)
	if err != nil {
		t.Fatalf("Marshal(scene) error = %v", err)
	}

	noopScenePath := filepath.Join(tmpDir, "noop-scene.json")
	outputPath := filepath.Join(tmpDir, "rendered-stereo.wav")

	err = os.WriteFile(noopScenePath, sceneJSON, 0o600)
	if err != nil {
		t.Fatalf("WriteFile(scene) error = %v", err)
	}

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"render-stereo",
		noopScenePath,
		"-o", outputPath,
		"--max-order", "3",
		"--duration", "0.15",
		"--num-rays", "128",
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

	for frame := 0; frame+1 < len(decoded.Data); frame += 2 {
		if decoded.Data[frame] != decoded.Data[frame+1] {
			t.Fatalf("Noop HRTF channels differ at frame %d: left=%g right=%g", frame/2, decoded.Data[frame], decoded.Data[frame+1])
		}
	}
}

func TestRenderStereoCommandSpatializesLateField(t *testing.T) {
	t.Parallel()

	scenePath := filepath.Join("..", "..", "testdata", "rooms", "shoebox_simple.json")

	sc, err := scene.LoadSceneFile(scenePath)
	if err != nil {
		t.Fatalf("LoadSceneFile() error = %v", err)
	}

	sc.Receivers[0].HRTF = hrtf.NearestNeighborDataset{
		SampleRateHz: sc.SampleRate,
		Grid: &hrtf.MeasurementGrid{
			Directions: []geometry.Vec3{{X: 1}},
			LeftHRIRs:  [][]float64{{1}},
			RightHRIRs: [][]float64{{0.25}},
		},
	}

	sceneJSON, err := json.Marshal(sc)
	if err != nil {
		t.Fatalf("Marshal(scene) error = %v", err)
	}

	tmpDir := t.TempDir()
	directionalScenePath := filepath.Join(tmpDir, "directional-scene.json")
	outputPath := filepath.Join(tmpDir, "directional-stereo.wav")

	err = os.WriteFile(directionalScenePath, sceneJSON, 0o600)
	if err != nil {
		t.Fatalf("WriteFile(scene) error = %v", err)
	}

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"render-stereo", directionalScenePath,
		"--output", outputPath,
		"--max-order", "0",
		"--duration", "0.12",
		"--crossover-time", "0.02",
		"--num-rays", "8192",
	})

	if exitCode := run(cmd); exitCode != 0 {
		t.Fatalf("run() = %d, want 0; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("Open(WAV) error = %v", err)
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)

	decoded, err := decoder.FullPCMBuffer()
	if err != nil {
		t.Fatalf("FullPCMBuffer() error = %v", err)
	}

	startFrame := int(0.04 * float64(decoder.SampleRate))
	var lateNonzero, lateChannelsDiffer bool

	for frame := startFrame; 2*frame+1 < len(decoded.Data); frame++ {
		left := decoded.Data[2*frame]

		right := decoded.Data[2*frame+1]
		if left != 0 || right != 0 {
			lateNonzero = true
		}

		if left != right {
			lateChannelsDiffer = true
			break
		}
	}

	if !lateNonzero {
		t.Fatal("late-field portion of rendered WAV is silent")
	}

	if !lateChannelsDiffer {
		t.Fatal("late-field left and right channels are identical with directional HRTF")
	}
}
