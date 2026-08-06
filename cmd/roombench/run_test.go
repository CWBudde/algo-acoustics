package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/scene"
)

func TestRunCommandReturnsErrorWhenBaselineDiffers(t *testing.T) {
	t.Parallel()

	fixtureDir := t.TempDir()
	baselineDir := t.TempDir()
	fixturePath := filepath.Join("..", "..", "testdata", "rooms", "shoebox_absorptive.json")

	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(fixtureDir, "shoebox_absorptive.json"), fixture, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(baselineDir, "shoebox_absorptive.json"), []byte("[]\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cmd := newRunCommand()
	cmd.SetArgs([]string{"--fixtures", fixtureDir, "--baselines", baselineDir})

	var output bytes.Buffer
	cmd.SetOut(&output)

	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected a regression mismatch error")
	}

	if !strings.Contains(output.String(), "FAIL shoebox_absorptive.json") {
		t.Fatalf("output = %q, want FAIL line", output.String())
	}
}

func TestCheckedInRegressionBaselinesMatch(t *testing.T) {
	t.Parallel()

	fixtureDir := filepath.Join("..", "..", "testdata", "rooms")
	baselineDir := filepath.Join("..", "..", "testdata", "regression")

	baselines, err := filepath.Glob(filepath.Join(baselineDir, "shoebox_*.json"))
	if err != nil {
		t.Fatal(err)
	}

	for _, baselinePath := range baselines {
		name := filepath.Base(baselinePath)
		fixturePath := filepath.Join(fixtureDir, name)

		sc, loadErr := scene.LoadSceneFile(fixturePath)
		if loadErr != nil {
			t.Fatalf("load %s: %v", name, loadErr)
		}

		got, solveErr := solveFixture(sc, 2)
		if solveErr != nil {
			t.Fatalf("solve %s: %v", name, solveErr)
		}

		want, baselineErr := loadEventsJSON(baselinePath)
		if baselineErr != nil {
			t.Fatalf("baseline %s: %v", name, baselineErr)
		}

		if len(got) != len(want) {
			t.Errorf("%s: event count got %d, want %d", name, len(got), len(want))
			continue
		}

		for i := range got {
			if !eventClose(got[i], want[i]) {
				t.Errorf("%s: event %d differs: got %+v, want %+v", name, i, got[i], want[i])
				break
			}
		}
	}
}
