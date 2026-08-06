package algoacoustics

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

const (
	defaultDirectionGroupAzimuth   = 12
	defaultDirectionGroupElevation = 6
)

// ISMEngine adapts the shipped image-source solver to EventEngine. It supports
// one or more sources and requires exactly one receiver, matching ISMSolver.
type ISMEngine struct {
	Config ism.ISMConfig
}

// NewISMEngine constructs an EventEngine backed by the shipped image-source
// solver. Zero SpeedOfSound and BandSpec values inherit the solver defaults.
func NewISMEngine(cfg ism.ISMConfig) *ISMEngine {
	return &ISMEngine{Config: cfg}
}

// Generate solves the scene for direct and specular events.
func (e *ISMEngine) Generate(sc *scene.Scene, _ ir.RenderConfig) ([]ir.Event, error) {
	if e == nil {
		return nil, errors.New("ISM engine is nil")
	}

	events, err := (ism.ISMSolver{}).Solve(sc, e.Config)
	if err != nil {
		return nil, fmt.Errorf("solve ISM events: %w", err)
	}

	return events, nil
}

// RaytraceEngineConfig configures the shipped dense late-field engine.
// Launch.MaxTimeSeconds defaults to the render duration and
// Launch.SpeedOfSound defaults to acoustics.SpeedOfSound.
type RaytraceEngineConfig struct {
	Launch                  raytrace.LaunchConfig
	ReceiverRadius          float64
	BinDurationSeconds      float64
	DirectionGroupAzimuth   int
	DirectionGroupElevation int
}

// RaytraceEngine adapts the shipped ray tracer to the canonical dense
// late-field interfaces. It requires exactly one source and one receiver.
//
// The ray tracer produces banded energy histograms, so this adapter deliberately
// does not implement EventEngine: collapsing a histogram to one sparse pressure
// event per bin would discard its band-energy distribution.
type RaytraceEngine struct {
	Config RaytraceEngineConfig
}

// NewRaytraceEngine constructs a dense late-field engine backed by the shipped
// Monte Carlo ray tracer.
func NewRaytraceEngine(cfg RaytraceEngineConfig) *RaytraceEngine {
	return &RaytraceEngine{Config: cfg}
}

// RenderMono traces and synthesizes the late-field energy as a mono buffer.
func (e *RaytraceEngine) RenderMono(sc *scene.Scene, cfg ir.RenderConfig) (*ir.Buffer, error) {
	tracer, err := e.newTracer(sc, cfg, false)
	if err != nil {
		return nil, err
	}

	histogram, err := tracer.Trace()
	if err != nil {
		return nil, fmt.Errorf("trace late field: %w", err)
	}

	return hybrid.HistogramToBuffer(histogram, cfg.SampleRate), nil
}

// RenderBinaural traces directional late-field energy and spatializes it with
// the selected receiver's HRTF using binaural Poisson synthesis.
func (e *RaytraceEngine) RenderBinaural(
	sc *scene.Scene,
	receiver scene.Receiver,
	cfg ir.RenderConfig,
) (left, right *ir.Buffer, err error) {
	if receiver.HRTF == nil {
		return nil, nil, errors.New("binaural receiver is missing an HRTF")
	}

	tracer, err := e.newTracer(sc, cfg, true)
	if err != nil {
		return nil, nil, err
	}

	histogram, err := tracer.Trace()
	if err != nil {
		return nil, nil, fmt.Errorf("trace directional late field: %w", err)
	}

	bounds, ok := sc.Room.Bounds()
	if !ok {
		return nil, nil, errors.New("derive room volume for binaural late field")
	}

	bins := make([]ir.EnergyBin, len(histogram.Bins))
	for index, bin := range histogram.Bins {
		bins[index] = ir.EnergyBin{
			TimeSeconds: bin.TimeSeconds,
			BandEnergy:  append([]float64(nil), bin.BandEnergy...),
		}
	}

	directions := make([]geometry.Vec3, len(tracer.DirectivityGroups))
	for index, group := range tracer.DirectivityGroups {
		directions[index] = receiver.WorldToHeadDir(group.Direction)
	}

	left, right, err = ir.RenderBinauralPoisson(ir.BinauralPoissonConfig{
		Bins:            bins,
		BinDuration:     histogram.BinDuration,
		Volume:          bounds.Volume(),
		BandSpec:        sc.BandSpec,
		SampleRate:      cfg.SampleRate,
		HRTF:            receiver.HRTF,
		DGDirections:    directions,
		DGProbabilities: raytrace.DGHitProbabilities(tracer.DirectivityGroups),
	}, rand.New(rand.NewSource(1))) //nolint:gosec // Reproducible noise is part of deterministic acoustic rendering.
	if err != nil {
		return nil, nil, fmt.Errorf("render binaural late field: %w", err)
	}

	return left, right, nil
}

func (e *RaytraceEngine) newTracer(
	sc *scene.Scene,
	cfg ir.RenderConfig,
	directional bool,
) (*raytrace.RayTracer, error) {
	if e == nil {
		return nil, errors.New("raytrace engine is nil")
	}

	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	if len(sc.Sources) != 1 {
		return nil, fmt.Errorf("raytrace engine requires exactly one source, got %d", len(sc.Sources))
	}

	if len(sc.Receivers) != 1 {
		return nil, fmt.Errorf("raytrace engine requires exactly one receiver, got %d", len(sc.Receivers))
	}

	launch := e.Config.Launch
	if launch.MaxTimeSeconds <= 0 {
		launch.MaxTimeSeconds = cfg.DurationSeconds
	}

	if launch.SpeedOfSound <= 0 {
		launch.SpeedOfSound = acoustics.SpeedOfSound
	}

	tracer := &raytrace.RayTracer{
		Config:             launch,
		Scene:              sc,
		ReceiverRadius:     e.Config.ReceiverRadius,
		BinDurationSeconds: e.Config.BinDurationSeconds,
	}

	if directional {
		azimuth, elevation := e.directionGroupCounts()
		tracer.DirectivityGroups = raytrace.NewDirectivityGroups(azimuth, elevation)
	}

	return tracer, nil
}

func (e *RaytraceEngine) directionGroupCounts() (azimuth, elevation int) {
	azimuth = e.Config.DirectionGroupAzimuth
	if azimuth <= 0 {
		azimuth = defaultDirectionGroupAzimuth
	}

	elevation = e.Config.DirectionGroupElevation
	if elevation <= 0 {
		elevation = defaultDirectionGroupElevation
	}

	return azimuth, elevation
}
