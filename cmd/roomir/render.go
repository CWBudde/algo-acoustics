package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/pde"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

const (
	defaultRenderMaxOrder     = 3
	defaultRenderDurationSecs = 1.5
	defaultRenderNumRays      = 4096
	defaultRenderCrossoverSec = 0.25
	defaultRenderWindowName   = "hann"
	renderModeHybrid          = "hybrid"
)

func newRenderCommand() *cobra.Command {
	var outputPath string
	var maxOrder int
	var durationSeconds float64
	var mode string
	var crossoverTimeSeconds float64
	var crossoverWindowName string
	var crossoverWindowAlpha float64
	var numRays int
	var enableLowFreq bool
	var lowFreqMin float64
	var lowFreqMax float64
	var lowFreqPoints int
	var lowFreqCrossoverHz float64
	var lowFreqBoundary string

	cmd := &cobra.Command{
		Use:   "render <scene.json>",
		Short: "Render a scene to a WAV file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputPath == "" {
				return errors.New("output path must not be empty")
			}

			if mode != "early" && mode != "late" && mode != renderModeHybrid {
				return fmt.Errorf("unsupported mode %q", mode)
			}

			crossoverWindow := hybrid.FadeWindowConfig{
				Name:  crossoverWindowName,
				Alpha: crossoverWindowAlpha,
			}
			if mode == renderModeHybrid {
				err := hybrid.ValidateFadeWindowConfig(crossoverWindow)
				if err != nil {
					return fmt.Errorf("invalid crossover window: %w", err)
				}
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

			renderCfg := ir.RenderConfig{
				SampleRate:      sc.SampleRate,
				DurationSeconds: durationSeconds,
				BandSpec:        sc.BandSpec,
			}

			var buffer *ir.Buffer

			switch mode {
			case "early":
				events, err := solveEarly(sc, maxOrder)
				if err != nil {
					return err
				}

				buffer, err = ir.RenderMono(events, renderCfg)
				if err != nil {
					return fmt.Errorf("render mono IR: %w", err)
				}

				fmt.Fprintf(cmd.ErrOrStderr(), "rendered early mode with %d events in %.3fs to %s\n", len(events), durationSeconds, outputPath)
			case "late":
				buffer, err = renderLateBuffer(sc, durationSeconds, numRays, maxOrder)
				if err != nil {
					return err
				}

				fmt.Fprintf(cmd.ErrOrStderr(), "rendered late mode with %d rays in %.3fs to %s\n", numRays, durationSeconds, outputPath)
			case renderModeHybrid:
				earlyEvents, err := solveEarly(sc, maxOrder)
				if err != nil {
					return err
				}

				earlyBuffer, err := ir.RenderMono(earlyEvents, renderCfg)
				if err != nil {
					return fmt.Errorf("render early IR: %w", err)
				}

				lateBuffer, err := renderLateBuffer(sc, durationSeconds, numRays, maxOrder)
				if err != nil {
					return err
				}

				hybridCfg := hybrid.HybridConfig{
					CrossoverTimeSeconds: crossoverTimeSeconds,
					CrossoverMode:        hybrid.TimeBased,
					SmoothenCrossover:    true,
					CrossoverWindow:      crossoverWindow,
				}

				lateBuffer = hybrid.AlignLateTail(lateBuffer, earlyEvents, hybridCfg)
				buffer = hybrid.CombineBuffers(earlyBuffer, lateBuffer, hybridCfg)
				if buffer == nil {
					return errors.New("combine hybrid buffers")
				}

				fmt.Fprintf(cmd.ErrOrStderr(), "rendered hybrid mode with %d early events and %d rays in %.3fs to %s\n", len(earlyEvents), numRays, durationSeconds, outputPath)
			}

			if enableLowFreq {
				engine := pde.PDELowFreqEngine{
					Sweep: pde.SweepConfig{
						FreqMin:           lowFreqMin,
						FreqMax:           lowFreqMax,
						NumPoints:         lowFreqPoints,
						BoundaryCondition: lowFreqBoundary,
					},
					CrossoverFreqHz: lowFreqCrossoverHz,
				}

				transfer, err := engine.Transfer(sc, renderCfg)
				if err != nil {
					return fmt.Errorf("render low-frequency transfer: %w", err)
				}

				if transfer == nil {
					return errors.New("render low-frequency transfer: nil transfer")
				}

				lowIR := transfer.ToTimeDomain(sc.SampleRate, len(buffer.Samples))

				buffer = hybrid.BlendLowFreq(lowIR, buffer, engine.CrossoverHz(), sc.SampleRate)
				if buffer == nil {
					return errors.New("blend low-frequency output")
				}

				fmt.Fprintf(cmd.ErrOrStderr(), "applied low-frequency blend at %.1f Hz\n", engine.CrossoverHz())
			}

			err = export.WriteMonoWAV(outputPath, buffer)
			if err != nil {
				return fmt.Errorf("write WAV: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output WAV file")
	cmd.Flags().IntVar(&maxOrder, "max-order", defaultRenderMaxOrder, "maximum reflection order")
	cmd.Flags().Float64Var(&durationSeconds, "duration", defaultRenderDurationSecs, "render duration in seconds")
	cmd.Flags().StringVar(&mode, "mode", "hybrid", "render mode: early, late, or hybrid")
	cmd.Flags().Float64Var(&crossoverTimeSeconds, "crossover-time", defaultRenderCrossoverSec, "hybrid crossover time in seconds")
	cmd.Flags().StringVar(&crossoverWindowName, "crossover-window", defaultRenderWindowName, fmt.Sprintf("hybrid crossover window (%s)", strings.Join(hybrid.SupportedFadeWindows(), ", ")))
	cmd.Flags().Float64Var(&crossoverWindowAlpha, "crossover-window-alpha", 0, "shape parameter for parametric hybrid crossover windows")
	cmd.Flags().IntVar(&numRays, "num-rays", defaultRenderNumRays, "number of rays for late-field rendering")
	cmd.Flags().BoolVar(&enableLowFreq, "enable-lowfreq", false, "enable low-frequency PDE blending")
	cmd.Flags().Float64Var(&lowFreqMin, "lowfreq-min", 20, "minimum low-frequency sweep value in Hz")
	cmd.Flags().Float64Var(&lowFreqMax, "lowfreq-max", 300, "maximum low-frequency sweep value in Hz")
	cmd.Flags().IntVar(&lowFreqPoints, "lowfreq-points", 32, "number of low-frequency sweep points")
	cmd.Flags().Float64Var(&lowFreqCrossoverHz, "lowfreq-crossover", 200, "low-frequency blend crossover in Hz")
	cmd.Flags().StringVar(&lowFreqBoundary, "lowfreq-boundary", "neumann", "PDE boundary condition: neumann, dirichlet, or periodic")

	return cmd
}

func solveEarly(sc *scene.Scene, maxOrder int) ([]ir.Event, error) {
	solver := ism.ISMSolver{}

	events, err := solver.Solve(sc, ism.ISMConfig{
		MaxOrder:     maxOrder,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     sc.BandSpec,
	})
	if err != nil {
		return nil, fmt.Errorf("solve scene: %w", err)
	}

	return events, nil
}

func renderLateBuffer(sc *scene.Scene, durationSeconds float64, numRays, maxOrder int) (*ir.Buffer, error) {
	maxBounces := max(maxOrder*2, 1)

	tracer := raytrace.RayTracer{
		Config: raytrace.LaunchConfig{
			NumRays:        numRays,
			MaxBounces:     maxBounces,
			MaxTimeSeconds: durationSeconds,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:              sc,
		ReceiverRadius:     0.25,
		BinDurationSeconds: 0.01,
	}

	hist, err := tracer.Trace()
	if err != nil {
		return nil, fmt.Errorf("trace scene: %w", err)
	}

	return hybrid.HistogramToBuffer(hist, sc.SampleRate), nil
}
