package main

import (
	"fmt"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

const (
	defaultRenderMaxOrder     = 3
	defaultRenderDurationSecs = 1.5
)

func newRenderCommand() *cobra.Command {
	var outputPath string
	var maxOrder int
	var durationSeconds float64

	cmd := &cobra.Command{
		Use:   "render <scene.json>",
		Short: "Render a scene to a WAV file.",
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

			buffer, err := ir.RenderMono(events, ir.RenderConfig{
				SampleRate:      sc.SampleRate,
				DurationSeconds: durationSeconds,
				BandSpec:        sc.BandSpec,
			})
			if err != nil {
				return fmt.Errorf("render mono IR: %w", err)
			}

			if err := export.WriteMonoWAV(outputPath, buffer); err != nil {
				return fmt.Errorf("write WAV: %w", err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "rendered %d events in %.3fs to %s\n", len(events), durationSeconds, outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output WAV file")
	cmd.Flags().IntVar(&maxOrder, "max-order", defaultRenderMaxOrder, "maximum reflection order")
	cmd.Flags().Float64Var(&durationSeconds, "duration", defaultRenderDurationSecs, "render duration in seconds")

	return cmd
}
