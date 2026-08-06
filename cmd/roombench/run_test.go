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

func TestDiscoverRegressionFixturesRejectsMissingBaseline(t *testing.T) {
	t.Parallel()

	fixtureDir := t.TempDir()
	baselineDir := t.TempDir()
	fixturePath := filepath.Join(fixtureDir, "shoebox_missing.json")

	err := os.WriteFile(fixturePath, []byte("{}\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = discoverRegressionFixtures(fixtureDir, baselineDir)
	if err == nil {
		t.Fatal("discoverRegressionFixtures() error = nil, want missing baseline error")
	}

	if !strings.Contains(err.Error(), "missing baselines for fixtures: shoebox_missing.json") {
		t.Fatalf("discoverRegressionFixtures() error = %q, want missing baseline detail", err)
	}
}

func TestDiscoverRegressionFixturesRejectsOrphanBaseline(t *testing.T) {
	t.Parallel()

	fixtureDir := t.TempDir()
	baselineDir := t.TempDir()

	for _, path := range []string{
		filepath.Join(fixtureDir, "shoebox_matched.json"),
		filepath.Join(baselineDir, "shoebox_matched.json"),
		filepath.Join(baselineDir, "shoebox_orphan.json"),
	} {
		err := os.WriteFile(path, []byte("{}\n"), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err := discoverRegressionFixtures(fixtureDir, baselineDir)
	if err == nil {
		t.Fatal("discoverRegressionFixtures() error = nil, want orphan baseline error")
	}

	if !strings.Contains(err.Error(), "orphan baselines without fixtures: shoebox_orphan.json") {
		t.Fatalf("discoverRegressionFixtures() error = %q, want orphan baseline detail", err)
	}
}

func TestCheckedInRegressionBaselinesMatch(t *testing.T) {
	t.Parallel()

	fixtureDir := filepath.Join("..", "..", "testdata", "rooms")
	baselineDir := filepath.Join("..", "..", "testdata", "regression")

	fixtures, err := discoverRegressionFixtures(fixtureDir, baselineDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, fixture := range fixtures {
		sc, loadErr := scene.LoadSceneFile(fixture.fixturePath)
		if loadErr != nil {
			t.Fatalf("load %s: %v", fixture.name, loadErr)
		}

		got, solveErr := solveFixture(sc, 2)
		if solveErr != nil {
			t.Fatalf("solve %s: %v", fixture.name, solveErr)
		}

		want, baselineErr := loadEventsJSON(fixture.baselinePath)
		if baselineErr != nil {
			t.Fatalf("baseline %s: %v", fixture.name, baselineErr)
		}

		if len(got) != len(want) {
			t.Errorf("%s: event count got %d, want %d", fixture.name, len(got), len(want))
			continue
		}

		for i := range got {
			if !eventClose(got[i], want[i]) {
				t.Errorf("%s: event %d differs: got %+v, want %+v", fixture.name, i, got[i], want[i])
				break
			}
		}
	}
}
