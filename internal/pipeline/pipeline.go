// Package pipeline provides shared rendering helpers used by the CLI and WASM demo.
package pipeline

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

// EarlyConfig configures the image-source method solver.
type EarlyConfig struct {
	MaxOrder int
}

// LateConfig configures the Monte Carlo ray tracer.
type LateConfig struct {
	NumRays            int
	MaxOrder           int
	DurationSeconds    float64
	ReceiverRadius     float64
	BinDurationSeconds float64
}

// SolveEarly runs the image-source method solver and returns sparse early events.
func SolveEarly(sc *scene.Scene, cfg EarlyConfig) ([]ir.Event, error) {
	solver := ism.ISMSolver{}

	events, err := solver.Solve(sc, ism.ISMConfig{
		MaxOrder:     cfg.MaxOrder,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     sc.BandSpec,
	})
	if err != nil {
		return nil, fmt.Errorf("solve early reflections: %w", err)
	}

	return events, nil
}

// RenderLateBuffer traces late-field energy via ray tracing and returns a dense buffer.
func RenderLateBuffer(sc *scene.Scene, cfg LateConfig) (*ir.Buffer, error) {
	bounceEstimate := int(math.Ceil(cfg.DurationSeconds*acoustics.SpeedOfSound/8.0)) + 4
	maxBounces := max(bounceEstimate, cfg.MaxOrder*2)

	tracer := raytrace.RayTracer{
		Config: raytrace.LaunchConfig{
			NumRays:        cfg.NumRays,
			MaxBounces:     maxBounces,
			MaxTimeSeconds: cfg.DurationSeconds,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:              sc,
		ReceiverRadius:     cfg.ReceiverRadius,
		BinDurationSeconds: cfg.BinDurationSeconds,
	}

	histogram, err := tracer.Trace()
	if err != nil {
		return nil, fmt.Errorf("trace late field: %w", err)
	}

	return hybrid.HistogramToBuffer(histogram, sc.SampleRate), nil
}

// RenderHybrid combines early and late buffers using a crossover blend.
func RenderHybrid(earlyBuffer, lateBuffer *ir.Buffer, earlyEvents []ir.Event, cfg hybrid.HybridConfig) (*ir.Buffer, error) {
	lateBuffer = hybrid.AlignLateTail(lateBuffer, earlyEvents, cfg)

	buffer := hybrid.CombineBuffers(earlyBuffer, lateBuffer, cfg)
	if buffer == nil {
		return nil, errors.New("combine hybrid buffers returned nil")
	}

	return buffer, nil
}
