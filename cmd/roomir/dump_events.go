package main

import (
	"fmt"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

const defaultDumpEventsMaxOrder = 3

func newDumpEventsCommand() *cobra.Command {
	var outputPath string
	var outputFormat string
	var maxOrder int

	cmd := &cobra.Command{
		Use:   "dump-events <scene.json>",
		Short: "Dump sparse IR events to JSON or CSV.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputPath == "" {
				return fmt.Errorf("output path must not be empty")
			}

			scenePath := args[0]
			sc, err := scene.LoadSceneFile(scenePath)
			if err != nil {
				return fmt.Errorf("load scene %q: %w", scenePath, err)
			}

			if err := scene.Validate(sc); err != nil {
				return &validationError{message: err.Error()}
			}

			solver := ism.ISMSolver{}
			events, err := solver.Solve(sc, ism.ISMConfig{
				MaxOrder:     maxOrder,
				SpeedOfSound: acoustics.SpeedOfSound,
				BandSpec:     sc.BandSpec,
			})
			if err != nil {
				return fmt.Errorf("solve scene: %w", err)
			}

			switch outputFormat {
			case "json":
				if err := export.WriteEventsJSON(outputPath, events); err != nil {
					return fmt.Errorf("write events json: %w", err)
				}
			case "csv":
				if err := export.WriteEventsCSV(outputPath, events); err != nil {
					return fmt.Errorf("write events csv: %w", err)
				}
			default:
				return fmt.Errorf("unsupported format %q", outputFormat)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "dumped %d events to %s (%s)\n", len(events), outputPath, outputFormat)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output file")
	cmd.Flags().StringVar(&outputFormat, "format", "json", "output format (json|csv)")
	cmd.Flags().IntVar(&maxOrder, "max-order", defaultDumpEventsMaxOrder, "maximum reflection order")

	return cmd
}
