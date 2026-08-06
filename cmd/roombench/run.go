package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

const defaultFixtureDir = "testdata/rooms"

type regressionFixture struct {
	name         string
	fixturePath  string
	baselinePath string
}

func newRunCommand() *cobra.Command {
	var maxOrder int
	var fixtureDir string
	var baselineDir string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run regression fixtures and compare them against baselines.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fixtures, err := discoverRegressionFixtures(fixtureDir, baselineDir)
			if err != nil {
				return err
			}

			passed := 0
			failed := make([]string, 0)

			for _, fixture := range fixtures {
				sc, err := scene.LoadSceneFile(fixture.fixturePath)
				if err != nil {
					return fmt.Errorf("load fixture %s: %w", fixture.name, err)
				}

				err = scene.Validate(sc)
				if err != nil {
					return fmt.Errorf("validate fixture %s: %w", fixture.name, err)
				}

				events, err := solveFixture(sc, maxOrder)
				if err != nil {
					return fmt.Errorf("solve fixture %s: %w", fixture.name, err)
				}

				baselineEvents, err := loadEventsJSON(fixture.baselinePath)
				if err != nil {
					return fmt.Errorf("load baseline %s: %w", fixture.baselinePath, err)
				}

				if compareEvents(events, baselineEvents) {
					passed++

					fmt.Fprintf(cmd.OutOrStdout(), "PASS %s (%d events)\n", fixture.name, len(events))

					continue
				}

				fmt.Fprintf(cmd.OutOrStdout(), "FAIL %s\n", fixture.name)
				failed = append(failed, fixture.name)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%d/%d regression fixtures passed\n", passed, len(fixtures))

			if len(failed) > 0 {
				return fmt.Errorf("%d regression fixture(s) failed: %v", len(failed), failed)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&maxOrder, "max-order", 2, "maximum reflection order")
	cmd.Flags().StringVar(&fixtureDir, "fixtures", defaultFixtureDir, "fixture directory")
	cmd.Flags().StringVar(&baselineDir, "baselines", filepath.Join("testdata", "regression"), "baseline directory")

	return cmd
}

func discoverRegressionFixtures(fixtureDir, baselineDir string) ([]regressionFixture, error) {
	fixturePaths, err := filepath.Glob(filepath.Join(fixtureDir, "shoebox_*.json"))
	if err != nil {
		return nil, fmt.Errorf("list fixtures: %w", err)
	}

	sort.Strings(fixturePaths)

	if len(fixturePaths) == 0 {
		return nil, fmt.Errorf("no fixtures found in %s", fixtureDir)
	}

	baselinePaths, err := filepath.Glob(filepath.Join(baselineDir, "shoebox_*.json"))
	if err != nil {
		return nil, fmt.Errorf("list baselines: %w", err)
	}

	sort.Strings(baselinePaths)

	fixturesByName := pathsByBase(fixturePaths)
	baselinesByName := pathsByBase(baselinePaths)
	missingBaselines := missingNames(fixturesByName, baselinesByName)
	orphanBaselines := missingNames(baselinesByName, fixturesByName)

	if len(missingBaselines) > 0 || len(orphanBaselines) > 0 {
		problems := make([]string, 0, 2)

		if len(missingBaselines) > 0 {
			problems = append(problems, "missing baselines for fixtures: "+strings.Join(missingBaselines, ", "))
		}

		if len(orphanBaselines) > 0 {
			problems = append(problems, "orphan baselines without fixtures: "+strings.Join(orphanBaselines, ", "))
		}

		return nil, fmt.Errorf("invalid regression corpus: %s", strings.Join(problems, "; "))
	}

	fixtures := make([]regressionFixture, 0, len(fixturePaths))

	for _, fixturePath := range fixturePaths {
		name := filepath.Base(fixturePath)
		fixtures = append(fixtures, regressionFixture{
			name:         name,
			fixturePath:  fixturePath,
			baselinePath: baselinesByName[name],
		})
	}

	return fixtures, nil
}

func pathsByBase(paths []string) map[string]string {
	byName := make(map[string]string, len(paths))
	for _, path := range paths {
		byName[filepath.Base(path)] = path
	}

	return byName
}

func missingNames(want, got map[string]string) []string {
	missing := make([]string, 0)

	for name := range want {
		if _, ok := got[name]; !ok {
			missing = append(missing, name)
		}
	}

	sort.Strings(missing)

	return missing
}

func solveFixture(sc *scene.Scene, maxOrder int) ([]ir.Event, error) {
	solver := ism.ISMSolver{}
	return solver.Solve(sc, ism.ISMConfig{MaxOrder: maxOrder, SpeedOfSound: acoustics.SpeedOfSound, BandSpec: sc.BandSpec})
}

func loadEventsJSON(path string) ([]ir.Event, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var events []ir.Event

	err = json.Unmarshal(data, &events)
	if err != nil {
		return nil, err
	}

	return events, nil
}

func compareEvents(got, want []ir.Event) bool {
	if len(got) != len(want) {
		return false
	}

	for index := range got {
		if !eventClose(got[index], want[index]) {
			return false
		}
	}

	return true
}

func eventClose(got, want ir.Event) bool {
	if got.Kind != want.Kind {
		return false
	}

	if math.Abs(got.TimeSeconds-want.TimeSeconds) > 0.00005 {
		return false
	}

	if !amplitudeClose(got.Amplitude, want.Amplitude) {
		return false
	}

	if math.Abs(got.DistanceMeters-want.DistanceMeters) > 1e-9 {
		return false
	}

	if !vecClose(got.Direction, want.Direction) {
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

func amplitudeClose(got, want float64) bool {
	if got == 0 || want == 0 {
		return math.Abs(got-want) <= 1e-12
	}

	return math.Abs(20*math.Log10(math.Abs(got)/math.Abs(want))) <= 0.5
}

func vecClose(got, want geometry.Vec3) bool {
	return math.Abs(got.X-want.X) <= 1e-9 && math.Abs(got.Y-want.Y) <= 1e-9 && math.Abs(got.Z-want.Z) <= 1e-9
}
