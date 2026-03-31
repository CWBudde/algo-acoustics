package ism

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestRegressionFixturesMatchGoldenEvents(t *testing.T) {
	t.Parallel()

	fixtures := []string{"shoebox_absorptive.json", "shoebox_livelier.json", "shoebox_symmetric.json"}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			sc := loadRegressionScene(t, filepath.Join("..", "testdata", "rooms", fixture))
			events := solveRegressionScene(t, sc)
			baseline := loadRegressionEvents(t, filepath.Join("..", "testdata", "regression", fixture))

			if !compareRegressionEvents(events, baseline) {
				t.Fatalf("regression events for %s do not match baseline", fixture)
			}
		})
	}
}

func TestAbsorptiveRoomHasShorterT60ThanLivelierRoom(t *testing.T) {
	t.Parallel()

	absorptive := renderRegressionIR(t, filepath.Join("..", "testdata", "rooms", "shoebox_absorptive.json"))
	livelier := renderRegressionIR(t, filepath.Join("..", "testdata", "rooms", "shoebox_livelier.json"))

	absorptiveT60, err := metrics.T60FromDecaySlope(absorptive)
	if err != nil {
		t.Fatalf("T60FromDecaySlope(absorptive) error = %v", err)
	}

	livelierT60, err := metrics.T60FromDecaySlope(livelier)
	if err != nil {
		t.Fatalf("T60FromDecaySlope(livelier) error = %v", err)
	}

	if !(absorptiveT60 < livelierT60) {
		t.Fatalf("absorptive T60 = %v, livelier T60 = %v; want absorptive < livelier", absorptiveT60, livelierT60)
	}
}

func loadRegressionScene(t *testing.T, path string) *scene.Scene {
	t.Helper()

	loaded, err := scene.LoadSceneFile(path)
	if err != nil {
		t.Fatalf("LoadSceneFile(%s) error = %v", path, err)
	}

	if err := scene.Validate(loaded); err != nil {
		t.Fatalf("Validate(%s) error = %v", path, err)
	}

	return loaded
}

func solveRegressionScene(t *testing.T, sc *scene.Scene) []ir.Event {
	t.Helper()

	solver := ISMSolver{}

	events, err := solver.Solve(sc, ISMConfig{MaxOrder: 2, SpeedOfSound: acoustics.SpeedOfSound, BandSpec: sc.BandSpec})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	return events
}

func renderRegressionIR(t *testing.T, path string) *ir.Buffer {
	t.Helper()

	sc := loadRegressionScene(t, path)
	events := solveRegressionScene(t, sc)

	buf, err := ir.RenderMono(events, ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 1.25, BandSpec: sc.BandSpec})
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	return buf
}

func loadRegressionEvents(t *testing.T, path string) []ir.Event {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}

	var events []ir.Event
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", path, err)
	}

	return events
}

func compareRegressionEvents(got, want []ir.Event) bool {
	if len(got) != len(want) {
		return false
	}

	for index := range got {
		if !regressionEventClose(got[index], want[index]) {
			return false
		}
	}

	return true
}

func regressionEventClose(got, want ir.Event) bool {
	if got.Kind != want.Kind {
		return false
	}

	if math.Abs(got.TimeSeconds-want.TimeSeconds) > 0.00005 {
		return false
	}

	if !regressionAmplitudeClose(got.Amplitude, want.Amplitude) {
		return false
	}

	if math.Abs(got.DistanceMeters-want.DistanceMeters) > 1e-9 {
		return false
	}

	if math.Abs(got.Direction.X-want.Direction.X) > 1e-9 || math.Abs(got.Direction.Y-want.Direction.Y) > 1e-9 || math.Abs(got.Direction.Z-want.Direction.Z) > 1e-9 {
		return false
	}

	if len(got.BandGain) != len(want.BandGain) {
		return false
	}

	for index := range got.BandGain {
		if math.Abs(got.BandGain[index]-want.BandGain[index]) > 1e-9 {
			return false
		}
	}

	return true
}

func regressionAmplitudeClose(got, want float64) bool {
	if got == 0 || want == 0 {
		return math.Abs(got-want) <= 1e-12
	}

	return math.Abs(20*math.Log10(math.Abs(got)/math.Abs(want))) <= 0.5
}
