package algoacoustics

import (
	"fmt"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
)

// QualityPreset selects a named level-of-detail preset with concrete
// parameter mappings (ISM order, ray count, band resolution, IR length,
// scattering). The presets are intended as a starting point for the
// progressive renderer; callers may override any individual field after
// constructing a preset config.
type QualityPreset int

const (
	// QualityDraft is the fastest preset — minimal ISM order, few rays,
	// short IR length, scattering and diffuse rain disabled. Target use:
	// real-time slider feedback and first-frame previews.
	QualityDraft QualityPreset = iota
	// QualityPreview balances fidelity and responsiveness — moderate ISM
	// order, thousands of rays, scattering disabled. Target use: interactive
	// editing while keeping per-update latency under a second.
	QualityPreview
	// QualityFinal is the highest-fidelity preset — full ISM order, tens of
	// thousands of rays, 8-band spectrum, scattering and diffuse rain on.
	// Target use: final renders for delivery.
	QualityFinal
)

// String returns a human-readable preset name.
func (q QualityPreset) String() string {
	switch q {
	case QualityDraft:
		return "draft"
	case QualityPreview:
		return "preview"
	case QualityFinal:
		return "final"
	default:
		return fmt.Sprintf("quality(%d)", int(q))
	}
}

// Concrete parameter mappings for each preset. Kept as constants so they can
// be inspected (and adjusted) without digging through a function body.
const (
	draftISMOrder    = 2
	draftNumRays     = 1000
	draftDuration    = 1.0
	draftPreviewRays = 500
	draftRaysPerBtch = 500

	previewISMOrder    = 3
	previewNumRays     = 5000
	previewDuration    = 2.0
	previewPreviewRays = 1500
	previewRaysPerBtch = 1000

	finalISMOrder    = 5
	finalNumRays     = 50000
	finalDuration    = 3.0
	finalPreviewRays = 2500
	finalRaysPerBtch = 2500
)

// PresetConfig returns a ProgressiveConfig populated with sensible defaults
// for the given quality preset. The returned config uses a fresh
// ir.RenderConfig and hybrid.HybridConfig — callers only need to set the
// scene sample rate (typically 48000) and any field they wish to override.
//
// Example:
//
//	cfg := PresetConfig(QualityPreview)
//	cfg.Render.SampleRate = sc.SampleRate
//	cfg.NumRays = 8000 // override
//	RenderProgressive(ctx, sc, cfg, update)
func PresetConfig(preset QualityPreset) ProgressiveConfig {
	switch preset {
	case QualityDraft:
		return draftPreset()
	case QualityPreview:
		return previewPreset()
	case QualityFinal:
		return finalPreset()
	default:
		return previewPreset()
	}
}

func draftPreset() ProgressiveConfig {
	return ProgressiveConfig{
		Render: ir.RenderConfig{
			DurationSeconds: draftDuration,
			BandSpec:        acoustics.Octave6,
		},
		Hybrid: hybrid.HybridConfig{
			CrossoverTimeSeconds: 0.05,
		},
		MaxOrder:        draftISMOrder,
		PreviewISMOrder: 1,
		NumRays:         draftNumRays,
		MaxTimeSeconds:  draftDuration,
		PreviewRayCount: draftPreviewRays,
		RaysPerBatch:    draftRaysPerBtch,
		DiffuseRain:     false,
	}
}

func previewPreset() ProgressiveConfig {
	return ProgressiveConfig{
		Render: ir.RenderConfig{
			DurationSeconds: previewDuration,
			BandSpec:        acoustics.Octave6,
		},
		Hybrid: hybrid.HybridConfig{
			CrossoverTimeSeconds: 0.08,
		},
		MaxOrder:        previewISMOrder,
		PreviewISMOrder: 2,
		NumRays:         previewNumRays,
		MaxTimeSeconds:  previewDuration,
		PreviewRayCount: previewPreviewRays,
		RaysPerBatch:    previewRaysPerBtch,
		DiffuseRain:     false,
	}
}

func finalPreset() ProgressiveConfig {
	return ProgressiveConfig{
		Render: ir.RenderConfig{
			DurationSeconds: finalDuration,
			BandSpec:        acoustics.Octave8,
		},
		Hybrid: hybrid.HybridConfig{
			CrossoverTimeSeconds: 0.1,
		},
		MaxOrder:        finalISMOrder,
		PreviewISMOrder: 3,
		NumRays:         finalNumRays,
		MaxTimeSeconds:  finalDuration,
		PreviewRayCount: finalPreviewRays,
		RaysPerBatch:    finalRaysPerBtch,
		DiffuseRain:     true,
	}
}
