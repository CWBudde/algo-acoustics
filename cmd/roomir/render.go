package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/internal/pipeline"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/pde"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

const (
	defaultRenderMaxOrder     = 3
	defaultRenderDurationSecs = 1.5
	defaultRenderNumRays      = 4096
	defaultRenderCrossoverSec = 0.25
	defaultRenderWindowName   = "hann"
	renderModeEarly           = "early"
	renderModeLate            = "late"
	renderModeHybrid          = "hybrid"
)

type renderCommandConfig struct {
	outputPath           string
	maxOrder             int
	durationSeconds      float64
	mode                 string
	crossoverTimeSeconds float64
	crossoverWindowName  string
	crossoverWindowAlpha float64
	numRays              int
	enableLowFreq        bool
	lowFreqMin           float64
	lowFreqMax           float64
	lowFreqPoints        int
	lowFreqCrossoverHz   float64
	lowFreqBoundary      string
}

func newRenderCommand() *cobra.Command {
	var cfg renderCommandConfig

	cmd := &cobra.Command{
		Use:   "render <scene.json>",
		Short: "Render a scene to a WAV file.",
		Args:  cobra.ExactArgs(1),
		RunE:  func(cmd *cobra.Command, args []string) error { return runRenderCommand(cmd, args[0], cfg) },
	}

	cmd.Flags().StringVarP(&cfg.outputPath, "output", "o", "", "output WAV file")
	cmd.Flags().IntVar(&cfg.maxOrder, "max-order", defaultRenderMaxOrder, "maximum reflection order")
	cmd.Flags().Float64Var(&cfg.durationSeconds, "duration", defaultRenderDurationSecs, "render duration in seconds")
	cmd.Flags().StringVar(&cfg.mode, "mode", renderModeHybrid, "render mode: early, late, or hybrid")
	cmd.Flags().Float64Var(&cfg.crossoverTimeSeconds, "crossover-time", defaultRenderCrossoverSec, "hybrid crossover time in seconds")
	cmd.Flags().StringVar(&cfg.crossoverWindowName, "crossover-window", defaultRenderWindowName, fmt.Sprintf("hybrid crossover window (%s)", strings.Join(hybrid.SupportedFadeWindows(), ", ")))
	cmd.Flags().Float64Var(&cfg.crossoverWindowAlpha, "crossover-window-alpha", 0, "shape parameter for parametric hybrid crossover windows")
	cmd.Flags().IntVar(&cfg.numRays, "num-rays", defaultRenderNumRays, "number of rays for late-field rendering")
	cmd.Flags().BoolVar(&cfg.enableLowFreq, "enable-lowfreq", false, "enable low-frequency PDE blending")
	cmd.Flags().Float64Var(&cfg.lowFreqMin, "lowfreq-min", 20, "minimum low-frequency sweep value in Hz")
	cmd.Flags().Float64Var(&cfg.lowFreqMax, "lowfreq-max", 300, "maximum low-frequency sweep value in Hz")
	cmd.Flags().IntVar(&cfg.lowFreqPoints, "lowfreq-points", 32, "number of low-frequency sweep points")
	cmd.Flags().Float64Var(&cfg.lowFreqCrossoverHz, "lowfreq-crossover", 200, "low-frequency blend crossover in Hz")
	cmd.Flags().StringVar(&cfg.lowFreqBoundary, "lowfreq-boundary", "neumann", "PDE boundary condition: neumann, dirichlet, or periodic")

	return cmd
}

func runRenderCommand(cmd *cobra.Command, scenePath string, cfg renderCommandConfig) error {
	window := hybrid.FadeWindowConfig{Name: cfg.crossoverWindowName, Alpha: cfg.crossoverWindowAlpha}

	err := validateRenderCommandConfig(cfg, window)
	if err != nil {
		return err
	}

	sc, err := scene.LoadSceneFile(scenePath)
	if err != nil {
		return fmt.Errorf("load scene %q: %w", scenePath, err)
	}

	err = scene.Validate(sc)
	if err != nil {
		return &validationError{message: err.Error()}
	}

	renderCfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: cfg.durationSeconds, BandSpec: sc.BandSpec}

	buffer, err := renderSelectedMode(cmd, sc, renderCfg, cfg, window)
	if err != nil {
		return err
	}

	if cfg.enableLowFreq {
		buffer, err = applyLowFrequencyBlend(cmd, sc, renderCfg, buffer, cfg)
		if err != nil {
			return err
		}
	}

	err = export.WriteMonoWAV(cfg.outputPath, buffer)
	if err != nil {
		return fmt.Errorf("write WAV: %w", err)
	}

	return nil
}

func validateRenderCommandConfig(cfg renderCommandConfig, window hybrid.FadeWindowConfig) error {
	if cfg.outputPath == "" {
		return errors.New("output path must not be empty")
	}

	if cfg.mode != renderModeEarly && cfg.mode != renderModeLate && cfg.mode != renderModeHybrid {
		return fmt.Errorf("unsupported mode %q", cfg.mode)
	}

	if cfg.mode == renderModeHybrid {
		err := hybrid.ValidateFadeWindowConfig(window)
		if err != nil {
			return fmt.Errorf("invalid crossover window: %w", err)
		}
	}

	return nil
}

func renderSelectedMode(cmd *cobra.Command, sc *scene.Scene, renderCfg ir.RenderConfig, cfg renderCommandConfig, window hybrid.FadeWindowConfig) (*ir.Buffer, error) {
	earlyCfg := pipeline.EarlyConfig{MaxOrder: cfg.maxOrder}
	lateCfg := pipeline.LateConfig{
		NumRays:            cfg.numRays,
		MaxOrder:           cfg.maxOrder,
		DurationSeconds:    cfg.durationSeconds,
		ReceiverRadius:     0.25,
		BinDurationSeconds: 0.01,
	}

	switch cfg.mode {
	case renderModeEarly:
		return renderEarlyMode(cmd, sc, renderCfg, earlyCfg, cfg)
	case renderModeLate:
		return renderLateMode(cmd, sc, lateCfg, cfg)
	case renderModeHybrid:
		return renderHybridMode(cmd, sc, renderCfg, earlyCfg, lateCfg, cfg, window)
	default:
		return nil, fmt.Errorf("unsupported mode %q", cfg.mode)
	}
}

func renderEarlyMode(cmd *cobra.Command, sc *scene.Scene, renderCfg ir.RenderConfig, earlyCfg pipeline.EarlyConfig, cfg renderCommandConfig) (*ir.Buffer, error) {
	events, err := pipeline.SolveEarly(sc, earlyCfg)
	if err != nil {
		return nil, err
	}

	buffer, err := ir.RenderMono(events, renderCfg)
	if err != nil {
		return nil, fmt.Errorf("render mono IR: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "rendered early mode with %d events in %.3fs to %s\n", len(events), cfg.durationSeconds, cfg.outputPath)

	return buffer, nil
}

func renderLateMode(cmd *cobra.Command, sc *scene.Scene, lateCfg pipeline.LateConfig, cfg renderCommandConfig) (*ir.Buffer, error) {
	buffer, err := pipeline.RenderLateBuffer(sc, lateCfg)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "rendered late mode with %d rays in %.3fs to %s\n", cfg.numRays, cfg.durationSeconds, cfg.outputPath)

	return buffer, nil
}

func renderHybridMode(cmd *cobra.Command, sc *scene.Scene, renderCfg ir.RenderConfig, earlyCfg pipeline.EarlyConfig, lateCfg pipeline.LateConfig, cfg renderCommandConfig, window hybrid.FadeWindowConfig) (*ir.Buffer, error) {
	events, err := pipeline.SolveEarly(sc, earlyCfg)
	if err != nil {
		return nil, err
	}

	early, err := ir.RenderMono(events, renderCfg)
	if err != nil {
		return nil, fmt.Errorf("render early IR: %w", err)
	}

	late, err := pipeline.RenderLateBuffer(sc, lateCfg)
	if err != nil {
		return nil, err
	}

	buffer, err := pipeline.RenderHybrid(early, late, events, hybrid.HybridConfig{
		CrossoverTimeSeconds: cfg.crossoverTimeSeconds,
		CrossoverMode:        hybrid.TimeBased,
		SmoothenCrossover:    true,
		CrossoverWindow:      window,
	})
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "rendered hybrid mode with %d early events and %d rays in %.3fs to %s\n", len(events), cfg.numRays, cfg.durationSeconds, cfg.outputPath)

	return buffer, nil
}

func applyLowFrequencyBlend(cmd *cobra.Command, sc *scene.Scene, renderCfg ir.RenderConfig, buffer *ir.Buffer, cfg renderCommandConfig) (*ir.Buffer, error) {
	if sc != nil && sc.RoomCount() > 1 {
		return nil, errors.New("low-frequency PDE blending is not supported for multi-room transmission")
	}

	engine := pde.PDELowFreqEngine{
		Sweep: pde.SweepConfig{
			FreqMin:           cfg.lowFreqMin,
			FreqMax:           cfg.lowFreqMax,
			NumPoints:         cfg.lowFreqPoints,
			BoundaryCondition: cfg.lowFreqBoundary,
		},
		CrossoverFreqHz: cfg.lowFreqCrossoverHz,
	}

	transfer, err := engine.Transfer(sc, renderCfg)
	if err != nil {
		return nil, fmt.Errorf("render low-frequency transfer: %w", err)
	}

	if transfer == nil {
		return nil, errors.New("render low-frequency transfer: nil transfer")
	}

	lowIR := transfer.ToTimeDomain(sc.SampleRate, len(buffer.Samples))

	blended := hybrid.BlendLowFreq(lowIR, buffer, engine.CrossoverHz(), sc.SampleRate)
	if blended == nil {
		return nil, errors.New("blend low-frequency output")
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "applied low-frequency blend at %.1f Hz\n", engine.CrossoverHz())

	return blended, nil
}
