package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

const defaultFixtureDir = "testdata/rooms"

func newRunCommand() *cobra.Command {
	var maxOrder int
	var fixtureDir string
	var baselineDir string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run regression fixtures and compare them against baselines.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fixtures, err := filepath.Glob(filepath.Join(fixtureDir, "shoebox_*.json"))
			if err != nil {
				return fmt.Errorf("list fixtures: %w", err)
			}

			sort.Strings(fixtures)

			if len(fixtures) == 0 {
				return fmt.Errorf("no fixtures found in %s", fixtureDir)
			}

			passed := 0

			for _, fixturePath := range fixtures {
				fixtureName := filepath.Base(fixturePath)
				baselinePath := filepath.Join(baselineDir, fixtureName)

				sc, err := scene.LoadSceneFile(fixturePath)
				if err != nil {
					return fmt.Errorf("load fixture %s: %w", fixtureName, err)
				}

				err = scene.Validate(sc)
				if err != nil {
					return fmt.Errorf("validate fixture %s: %w", fixtureName, err)
				}

				events, err := solveFixture(sc, maxOrder)
				if err != nil {
					return fmt.Errorf("solve fixture %s: %w", fixtureName, err)
				}

				baselineEvents, err := loadEventsJSON(baselinePath)
				if err != nil {
					return fmt.Errorf("load baseline %s: %w", baselinePath, err)
				}

				if compareEvents(events, baselineEvents) {
					passed++

					fmt.Fprintf(cmd.OutOrStdout(), "PASS %s (%d events)\n", fixtureName, len(events))

					continue
				}

				fmt.Fprintf(cmd.OutOrStdout(), "FAIL %s\n", fixtureName)

				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%d/%d regression fixtures passed\n", passed, len(fixtures))

			return nil
		},
	}

	cmd.Flags().IntVar(&maxOrder, "max-order", 3, "maximum reflection order")
	cmd.Flags().StringVar(&fixtureDir, "fixtures", defaultFixtureDir, "fixture directory")
	cmd.Flags().StringVar(&baselineDir, "baselines", filepath.Join("testdata", "regression"), "baseline directory")

	return cmd
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
