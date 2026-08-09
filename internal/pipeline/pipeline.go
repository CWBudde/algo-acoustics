// Package pipeline provides shared rendering helpers used by the CLI and WASM demo.
package pipeline

import (
	"errors"
	"fmt"
	"math"

	algoacoustics "github.com/cwbudde/algo-acoustics"
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
	var bandSpec acoustics.BandSpec
	if sc != nil {
		bandSpec = sc.BandSpec
	}

	ismConfig := ism.ISMConfig{
		MaxOrder:     cfg.MaxOrder,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     bandSpec,
	}
	if sc != nil && sc.RoomCount() > 1 {
		engine := algoacoustics.NewTransmissionRenderer(algoacoustics.TransmissionRendererConfig{ISM: ismConfig})

		events, err := engine.SolveEarly(sc, renderConfig(sc, 0))
		if err != nil {
			return nil, fmt.Errorf("solve transmitted early reflections: %w", err)
		}

		return events, nil
	}

	engine := algoacoustics.NewISMEngine(ismConfig)

	events, err := engine.Generate(sc, renderConfig(sc, 0))
	if err != nil {
		return nil, fmt.Errorf("solve early reflections: %w", err)
	}

	return events, nil
}

// RenderLateBuffer traces late-field energy via ray tracing and returns a dense buffer.
func RenderLateBuffer(sc *scene.Scene, cfg LateConfig) (*ir.Buffer, error) {
	if sc != nil && sc.RoomCount() > 1 {
		engine := algoacoustics.NewTransmissionRenderer(algoacoustics.TransmissionRendererConfig{
			Raytrace: newLateEngine(cfg).Config,
		})

		buffer, err := engine.RenderLateMono(sc, renderConfig(sc, cfg.DurationSeconds))
		if err != nil {
			return nil, fmt.Errorf("render transmitted late field: %w", err)
		}

		return buffer, nil
	}

	buffer, err := newLateEngine(cfg).RenderMono(sc, renderConfig(sc, cfg.DurationSeconds))
	if err != nil {
		return nil, err
	}

	return buffer, nil
}

// RenderLateBinaural traces directional late-field energy and spatializes it
// through the receiver's HRTF using binaural Poisson synthesis.
func RenderLateBinaural(sc *scene.Scene, receiver scene.Receiver, cfg LateConfig) (left, right *ir.Buffer, err error) {
	if sc != nil && sc.RoomCount() > 1 {
		engine := algoacoustics.NewTransmissionRenderer(algoacoustics.TransmissionRendererConfig{
			Raytrace: newLateEngine(cfg).Config,
		})

		left, right, err = engine.RenderLateBinaural(sc, receiver, renderConfig(sc, cfg.DurationSeconds))
		if err != nil {
			return nil, nil, fmt.Errorf("render transmitted binaural late field: %w", err)
		}

		return left, right, nil
	}

	left, right, err = newLateEngine(cfg).RenderBinaural(sc, receiver, renderConfig(sc, cfg.DurationSeconds))
	if err != nil {
		return nil, nil, err
	}

	return left, right, nil
}

func newLateEngine(cfg LateConfig) *algoacoustics.RaytraceEngine {
	bounceEstimate := int(math.Ceil(cfg.DurationSeconds*acoustics.SpeedOfSound/8.0)) + 4
	maxBounces := max(bounceEstimate, cfg.MaxOrder*2)

	return algoacoustics.NewRaytraceEngine(algoacoustics.RaytraceEngineConfig{
		Launch: raytrace.LaunchConfig{
			NumRays:        cfg.NumRays,
			MaxBounces:     maxBounces,
			MaxTimeSeconds: cfg.DurationSeconds,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		ReceiverRadius:     cfg.ReceiverRadius,
		BinDurationSeconds: cfg.BinDurationSeconds,
	})
}

func renderConfig(sc *scene.Scene, durationSeconds float64) ir.RenderConfig {
	if sc == nil {
		return ir.RenderConfig{DurationSeconds: durationSeconds}
	}

	return ir.RenderConfig{
		SampleRate:      sc.SampleRate,
		DurationSeconds: durationSeconds,
		BandSpec:        sc.BandSpec,
	}
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
