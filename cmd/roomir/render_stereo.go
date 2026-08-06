package main

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/internal/pipeline"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

func newRenderStereoCommand() *cobra.Command {
	var outputPath string
	var maxOrder int
	var durationSeconds float64
	var crossoverTimeSeconds float64
	var numRays int

	cmd := &cobra.Command{
		Use:   "render-stereo <scene.json>",
		Short: "Render a scene to a stereo WAV file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := validateRenderStereoOptions(outputPath, maxOrder, durationSeconds, crossoverTimeSeconds, numRays)
			if err != nil {
				return err
			}

			scenePath := args[0]

			sc, err := scene.LoadSceneFile(scenePath)
			if err != nil {
				return fmt.Errorf("load scene %q: %w", scenePath, err)
			}

			err = scene.Validate(sc)
			if err != nil {
				return &validationError{message: err.Error()}
			}

			receiver, err := firstBinauralReceiver(sc)
			if err != nil {
				return err
			}

			renderCfg := ir.RenderConfig{
				SampleRate:      sc.SampleRate,
				DurationSeconds: durationSeconds,
				BandSpec:        sc.BandSpec,
			}

			earlyCfg := pipeline.EarlyConfig{MaxOrder: maxOrder}
			lateCfg := pipeline.LateConfig{
				NumRays:            numRays,
				MaxOrder:           maxOrder,
				DurationSeconds:    durationSeconds,
				ReceiverRadius:     0.25,
				BinDurationSeconds: 0.01,
			}

			earlyEvents, err := pipeline.SolveEarly(sc, earlyCfg)
			if err != nil {
				return err
			}

			earlyLeft, earlyRight, err := ir.RenderBinaural(earlyEvents, receiver.HRTF, renderCfg)
			if err != nil {
				return fmt.Errorf("render binaural early IR: %w", err)
			}

			lateLeft, lateRight, err := pipeline.RenderLateBinaural(sc, receiver, lateCfg)
			if err != nil {
				return err
			}

			hybridCfg := hybrid.HybridConfig{
				CrossoverTimeSeconds: crossoverTimeSeconds,
				CrossoverMode:        hybrid.TimeBased,
				SmoothenCrossover:    true,
			}

			lateLeft = hybrid.AlignLateTail(lateLeft, earlyEvents, hybridCfg)
			lateRight = hybrid.AlignLateTail(lateRight, earlyEvents, hybridCfg)

			left := hybrid.CombineBuffers(earlyLeft, lateLeft, hybridCfg)

			right := hybrid.CombineBuffers(earlyRight, lateRight, hybridCfg)
			if left == nil || right == nil {
				return errors.New("combine stereo hybrid buffers")
			}

			err = export.WriteStereoWAV(outputPath, left, right)
			if err != nil {
				return fmt.Errorf("write stereo WAV: %w", err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "rendered stereo mode with %d early events, %d rays, and receiver at %v in %.3fs to %s\n", len(earlyEvents), numRays, receiver.Position, durationSeconds, outputPath)

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output WAV file")
	cmd.Flags().IntVar(&maxOrder, "max-order", defaultRenderMaxOrder, "maximum reflection order")
	cmd.Flags().Float64Var(&durationSeconds, "duration", defaultRenderDurationSecs, "render duration in seconds")
	cmd.Flags().Float64Var(&crossoverTimeSeconds, "crossover-time", defaultRenderCrossoverSec, "hybrid crossover time in seconds")
	cmd.Flags().IntVar(&numRays, "num-rays", defaultRenderNumRays, "number of rays for late-field rendering")

	return cmd
}

func validateRenderStereoOptions(outputPath string, maxOrder int, durationSeconds, crossoverTimeSeconds float64, numRays int) error {
	if outputPath == "" {
		return errors.New("output path must not be empty")
	}

	if maxOrder < 0 {
		return errors.New("max order must not be negative")
	}

	if durationSeconds <= 0 || math.IsNaN(durationSeconds) || math.IsInf(durationSeconds, 0) {
		return errors.New("duration must be a finite positive number of seconds")
	}

	if crossoverTimeSeconds < 0 || math.IsNaN(crossoverTimeSeconds) || math.IsInf(crossoverTimeSeconds, 0) {
		return errors.New("crossover time must be a finite non-negative number of seconds")
	}

	if numRays <= 0 {
		return errors.New("number of rays must be positive")
	}

	return nil
}

func firstBinauralReceiver(sc *scene.Scene) (scene.Receiver, error) {
	for _, receiver := range sc.Receivers {
		if receiver.Type == scene.ReceiverBinaural {
			if receiver.HRTF == nil {
				return scene.Receiver{}, errors.New("binaural receiver is missing an HRTF")
			}

			return receiver, nil
		}
	}

	return scene.Receiver{}, errors.New("scene does not contain a binaural receiver")
}
