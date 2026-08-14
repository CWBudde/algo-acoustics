package algoacoustics

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

// Tier identifies a progressive rendering quality level.
type Tier int

const (
	// TierStatistical produces instant statistical estimates (< 50 ms).
	TierStatistical Tier = iota
	// TierPreview runs low-order ISM + few rays (50–500 ms).
	TierPreview
	// TierRefined runs full ISM + progressive ray batches (0.5–5 s).
	TierRefined
	// TierFinal runs the full-quality simulation in background.
	TierFinal
)

const (
	previewLabel = "preview"
	finalLabel   = "final"
)

// String returns a human-readable tier name.
func (t Tier) String() string {
	switch t {
	case TierStatistical:
		return "statistical"
	case TierPreview:
		return previewLabel
	case TierRefined:
		return "refined"
	case TierFinal:
		return finalLabel
	default:
		return fmt.Sprintf("tier(%d)", int(t))
	}
}

// StatisticalMetrics holds instant estimates derived from room geometry
// and material absorption, with no simulation required.
type StatisticalMetrics struct {
	SabineRT60ByBand []float64
	EyringRT60ByBand []float64
	C80ByBand        []float64
	D50ByBand        []float64
}

// TierResult carries the output of a single tier or batch computation.
type TierResult struct {
	Tier       Tier
	Metrics    *StatisticalMetrics
	Buffer     *ir.Buffer // nil for TierStatistical
	RayBatches int        // progressive batch count (TierRefined only)
}

// UpdateFunc is called after each tier or progressive batch completes.
type UpdateFunc func(TierResult)

// ProgressiveConfig controls the 4-tier progressive rendering pipeline.
type ProgressiveConfig struct {
	// Render controls dense buffer rendering. RenderProgressive always uses the
	// scene's SampleRate and BandSpec so every simulation tier shares one format.
	Render       ir.RenderConfig
	Hybrid       hybrid.HybridConfig
	SpeedOfSound float64

	// ISM order for Tier 3/4. Tier 2 uses PreviewISMOrder.
	MaxOrder        int
	PreviewISMOrder int // default 2

	// Ray tracer settings for the final tier.
	NumRays        int
	MaxBounces     int
	MaxTimeSeconds float64
	ReceiverRadius float64
	DiffuseRain    bool

	// Tier 2 preview ray count (default 2000).
	PreviewRayCount int

	// Tier 3 progressive batch size (default 1000).
	RaysPerBatch int
}

const (
	defaultPreviewISMOrder = 2
	defaultPreviewRayCount = 2000
	defaultRaysPerBatch    = 1000
	defaultReceiverRadius  = 0.25
)

// RenderProgressive runs the 4-tier progressive rendering pipeline,
// calling update after each tier or batch completes. Cancel ctx to abort
// Tier 3/4 and return early.
func RenderProgressive(ctx context.Context, sc *scene.Scene, cfg ProgressiveConfig, update UpdateFunc) error {
	if ctx == nil {
		return errors.New("context is nil")
	}

	if sc == nil {
		return errors.New("scene is nil")
	}

	if update == nil {
		return errors.New("update callback is nil")
	}

	err := scene.Validate(sc)
	if err != nil {
		return fmt.Errorf("validate scene: %w", err)
	}

	cfg = fillProgressiveDefaults(cfg)
	cfg.Render.SampleRate = sc.SampleRate
	cfg.Render.BandSpec = sc.BandSpec

	err = validateProgressiveConfig(sc, cfg)
	if err != nil {
		return fmt.Errorf("validate progressive config: %w", err)
	}

	// Tier 1: statistical estimates (< 50 ms).
	statsMetrics := computeStatisticalMetrics(sc)
	update(TierResult{Tier: TierStatistical, Metrics: statsMetrics})

	err = ctx.Err()
	if err != nil {
		return fmt.Errorf("cancelled after statistical tier: %w", err)
	}

	// Tier 2: fast preview — low ISM order + few rays.
	previewBuf, err := renderPreviewTier(sc, cfg)
	if err != nil {
		return fmt.Errorf("tier preview: %w", err)
	}

	update(TierResult{Tier: TierPreview, Metrics: statsMetrics, Buffer: previewBuf})

	err = ctx.Err()
	if err != nil {
		return fmt.Errorf("cancelled after preview tier: %w", err)
	}

	// Tier 3: refined — full ISM + progressive ray batches.
	err = renderRefinedTier(ctx, sc, cfg, statsMetrics, update)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("tier refined: %w", err)
	}

	err = ctx.Err()
	if err != nil {
		return fmt.Errorf("cancelled after refined tier: %w", err)
	}

	// Tier 4: final quality — full ray count with scattering/diffuse rain.
	finalBuf, err := renderFinalTier(sc, cfg)
	if err != nil {
		return fmt.Errorf("tier final: %w", err)
	}

	update(TierResult{Tier: TierFinal, Metrics: statsMetrics, Buffer: finalBuf})

	return nil
}

func validateProgressiveConfig(sc *scene.Scene, cfg ProgressiveConfig) error {
	err := validateProgressiveSceneCardinality(sc)
	if err != nil {
		return err
	}

	err = validateProgressiveRenderConfig(cfg)
	if err != nil {
		return err
	}

	return validateProgressiveSimulationConfig(cfg)
}

func validateProgressiveSceneCardinality(sc *scene.Scene) error {
	if len(sc.Sources) != 1 {
		return fmt.Errorf("progressive rendering requires exactly one source, got %d", len(sc.Sources))
	}

	if len(sc.Receivers) != 1 {
		return fmt.Errorf("progressive rendering requires exactly one receiver, got %d", len(sc.Receivers))
	}

	return nil
}

func validateProgressiveRenderConfig(cfg ProgressiveConfig) error {
	if cfg.Render.DurationSeconds <= 0 {
		return errors.New("render duration must be positive")
	}

	bandCount := cfg.Render.BandSpec.BandCount()
	if bandCount <= 0 {
		return errors.New("scene band spec must contain at least one band")
	}

	if len(cfg.Render.BandSpec.LowerEdges) != bandCount || len(cfg.Render.BandSpec.UpperEdges) != bandCount {
		return errors.New("scene band spec lengths must match")
	}

	return nil
}

func validateProgressiveSimulationConfig(cfg ProgressiveConfig) error {
	if cfg.MaxOrder < 0 {
		return errors.New("maximum ISM order must be non-negative")
	}

	if cfg.PreviewISMOrder < 0 {
		return errors.New("preview ISM order must be non-negative")
	}

	if cfg.NumRays <= 0 {
		return errors.New("ray count must be positive")
	}

	if cfg.PreviewRayCount <= 0 {
		return errors.New("preview ray count must be positive")
	}

	if cfg.RaysPerBatch <= 0 {
		return errors.New("rays per batch must be positive")
	}

	if cfg.MaxBounces < 0 {
		return errors.New("maximum bounce count must be non-negative")
	}

	if cfg.MaxTimeSeconds <= 0 {
		return errors.New("maximum ray time must be positive")
	}

	if cfg.SpeedOfSound <= 0 {
		return errors.New("speed of sound must be positive")
	}

	if cfg.ReceiverRadius <= 0 {
		return errors.New("receiver radius must be positive")
	}

	if cfg.Hybrid.CrossoverTimeSeconds < 0 {
		return errors.New("hybrid crossover time must be non-negative")
	}

	return nil
}

func fillProgressiveDefaults(cfg ProgressiveConfig) ProgressiveConfig {
	if cfg.SpeedOfSound == 0 {
		cfg.SpeedOfSound = acoustics.SpeedOfSound
	}

	if cfg.PreviewISMOrder == 0 {
		cfg.PreviewISMOrder = defaultPreviewISMOrder
	}

	if cfg.PreviewRayCount == 0 {
		cfg.PreviewRayCount = defaultPreviewRayCount
	}

	if cfg.RaysPerBatch == 0 {
		cfg.RaysPerBatch = defaultRaysPerBatch
	}

	if cfg.MaxTimeSeconds == 0 {
		cfg.MaxTimeSeconds = cfg.Render.DurationSeconds
	}

	if cfg.ReceiverRadius == 0 {
		cfg.ReceiverRadius = defaultReceiverRadius
	}

	return cfg
}

// ComputeStatisticalMetrics returns the Tier 1 estimates for a scene without
// running any simulation. It is the same computation RenderProgressive performs
// before its first simulated tier, exported so callers that drive their own tier
// sequence — the browser demo does, because it must also cover render modes and
// connected rooms that RenderProgressive does not — still share one definition
// of what the statistical tier reports.
//
// Non-shoebox rooms yield an empty result: the Sabine and Eyring estimators are
// defined from a shoebox's volume and surface areas.
func ComputeStatisticalMetrics(sc *scene.Scene) *StatisticalMetrics {
	if sc == nil {
		return &StatisticalMetrics{}
	}

	return computeStatisticalMetrics(sc)
}

func computeStatisticalMetrics(sc *scene.Scene) *StatisticalMetrics {
	if sc.Room.Kind != scene.RoomKindShoebox {
		return &StatisticalMetrics{}
	}

	stats, err := metrics.ShoeboxStatsFromScene(sc)
	if err != nil {
		return &StatisticalMetrics{}
	}

	bandCount := len(stats.AlphaByBand)
	m := &StatisticalMetrics{
		SabineRT60ByBand: make([]float64, bandCount),
		EyringRT60ByBand: make([]float64, bandCount),
		C80ByBand:        make([]float64, bandCount),
		D50ByBand:        make([]float64, bandCount),
	}

	for band := range bandCount {
		v, err := metrics.SabineRT60(stats, band)
		if err == nil {
			m.SabineRT60ByBand[band] = v
		}

		v, err = metrics.EyringRT60(stats, band)
		if err == nil {
			m.EyringRT60ByBand[band] = v
		}

		v, err = metrics.EstimateC80(stats, band)
		if err == nil {
			m.C80ByBand[band] = v
		}

		v, err = metrics.EstimateD50(stats, band)
		if err == nil {
			m.D50ByBand[band] = v
		}
	}

	return m
}

func renderPreviewTier(sc *scene.Scene, cfg ProgressiveConfig) (*ir.Buffer, error) {
	earlyEvents, err := solveISM(sc, cfg.PreviewISMOrder, cfg.SpeedOfSound)
	if err != nil {
		return nil, fmt.Errorf("ISM preview: %w", err)
	}

	histogram, err := traceRays(sc, cfg.PreviewRayCount, cfg)
	if err != nil {
		return nil, fmt.Errorf("ray tracer preview: %w", err)
	}

	return combineEarlyLate(earlyEvents, histogram, sc.SampleRate, cfg)
}

func renderRefinedTier(ctx context.Context, sc *scene.Scene, cfg ProgressiveConfig, statsMetrics *StatisticalMetrics, update UpdateFunc) error {
	earlyEvents, err := solveISM(sc, cfg.MaxOrder, cfg.SpeedOfSound)
	if err != nil {
		return fmt.Errorf("ISM refined: %w", err)
	}

	earlyBuffer, err := ir.RenderMono(earlyEvents, cfg.Render)
	if err != nil {
		return fmt.Errorf("render early refined: %w", err)
	}

	// Progressive ray batches use increasing cumulative ray counts. Ray launch
	// directions depend on the total ray count, and the raytrace API cannot
	// select a deterministic direction range, so retracing is currently needed
	// to preserve the coverage of each advertised cumulative result.
	raysCompleted := 0
	batchCount := 0

	for raysCompleted < cfg.NumRays {
		err = ctx.Err()
		if err != nil {
			return fmt.Errorf("cancelled during ray batch: %w", err)
		}

		raysCompleted += cfg.RaysPerBatch
		if raysCompleted > cfg.NumRays {
			raysCompleted = cfg.NumRays
		}

		histogram, err := traceRays(sc, raysCompleted, cfg)
		if err != nil {
			return fmt.Errorf("ray batch %d: %w", batchCount+1, err)
		}

		lateBuffer := hybrid.HistogramToBuffer(histogram, sc.SampleRate)
		lateBuffer = hybrid.AlignLateTail(lateBuffer, earlyEvents, cfg.Hybrid)
		combinedBuf := hybrid.CombineBuffers(earlyBuffer, lateBuffer, cfg.Hybrid)
		batchCount++

		update(TierResult{
			Tier:       TierRefined,
			Metrics:    statsMetrics,
			Buffer:     combinedBuf,
			RayBatches: batchCount,
		})
	}

	return nil
}

func renderFinalTier(sc *scene.Scene, cfg ProgressiveConfig) (*ir.Buffer, error) {
	earlyEvents, err := solveISM(sc, cfg.MaxOrder, cfg.SpeedOfSound)
	if err != nil {
		return nil, fmt.Errorf("ISM final: %w", err)
	}

	bounces := estimateBounces(cfg.MaxTimeSeconds, cfg.SpeedOfSound, cfg.MaxOrder)

	tracer := &raytrace.RayTracer{
		Config: raytrace.LaunchConfig{
			NumRays:        cfg.NumRays,
			MaxBounces:     bounces,
			MaxTimeSeconds: cfg.MaxTimeSeconds,
			SpeedOfSound:   cfg.SpeedOfSound,
			DiffuseRain:    cfg.DiffuseRain,
		},
		Scene:          sc,
		ReceiverRadius: cfg.ReceiverRadius,
	}

	histogram, err := tracer.Trace()
	if err != nil {
		return nil, fmt.Errorf("ray tracer final: %w", err)
	}

	return combineEarlyLate(earlyEvents, histogram, sc.SampleRate, cfg)
}

// solveISM runs the image-source method with the given order.
func solveISM(sc *scene.Scene, maxOrder int, speedOfSound float64) ([]ir.Event, error) {
	events, err := ism.ISMSolver{}.Solve(sc, ism.ISMConfig{
		MaxOrder:     maxOrder,
		SpeedOfSound: speedOfSound,
		BandSpec:     sc.BandSpec,
	})
	if err != nil {
		return nil, fmt.Errorf("ISM solve: %w", err)
	}

	return events, nil
}

// traceRays runs the ray tracer with the given ray count and returns the histogram.
func traceRays(sc *scene.Scene, numRays int, cfg ProgressiveConfig) (*raytrace.EnergyHistogram, error) {
	bounces := estimateBounces(cfg.MaxTimeSeconds, cfg.SpeedOfSound, cfg.MaxOrder)

	tracer := &raytrace.RayTracer{
		Config: raytrace.LaunchConfig{
			NumRays:        numRays,
			MaxBounces:     bounces,
			MaxTimeSeconds: cfg.MaxTimeSeconds,
			SpeedOfSound:   cfg.SpeedOfSound,
		},
		Scene:          sc,
		ReceiverRadius: cfg.ReceiverRadius,
	}

	hist, err := tracer.Trace()
	if err != nil {
		return nil, fmt.Errorf("ray trace: %w", err)
	}

	return hist, nil
}

// combineEarlyLate renders ISM events and combines with the late-field histogram.
func combineEarlyLate(earlyEvents []ir.Event, histogram *raytrace.EnergyHistogram, sampleRate int, cfg ProgressiveConfig) (*ir.Buffer, error) {
	earlyBuffer, err := ir.RenderMono(earlyEvents, cfg.Render)
	if err != nil {
		return nil, fmt.Errorf("render early: %w", err)
	}

	lateBuffer := hybrid.HistogramToBuffer(histogram, sampleRate)
	lateBuffer = hybrid.AlignLateTail(lateBuffer, earlyEvents, cfg.Hybrid)

	return hybrid.CombineBuffers(earlyBuffer, lateBuffer, cfg.Hybrid), nil
}

// estimateBounces computes a reasonable MaxBounces from the simulation duration.
func estimateBounces(maxTimeSeconds, speedOfSound float64, ismOrder int) int {
	bounceEstimate := int(math.Ceil(maxTimeSeconds*speedOfSound/8.0)) + 4

	return max(bounceEstimate, ismOrder*2)
}
