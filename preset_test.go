package algoacoustics

import (
	"context"
	"testing"
)

func TestQualityPreset_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		preset QualityPreset
		want   string
	}{
		{QualityDraft, "draft"},
		{QualityPreview, "preview"},
		{QualityFinal, "final"},
		{QualityPreset(42), "quality(42)"},
	}

	for _, tt := range tests {
		if got := tt.preset.String(); got != tt.want {
			t.Errorf("QualityPreset(%d).String() = %q, want %q", int(tt.preset), got, tt.want)
		}
	}
}

func TestPresetConfig_DraftValues(t *testing.T) {
	t.Parallel()

	cfg := PresetConfig(QualityDraft)

	if cfg.MaxOrder != 2 {
		t.Errorf("Draft MaxOrder = %d, want 2", cfg.MaxOrder)
	}

	if cfg.NumRays != 1000 {
		t.Errorf("Draft NumRays = %d, want 1000", cfg.NumRays)
	}

	if cfg.Render.DurationSeconds != 1.0 {
		t.Errorf("Draft DurationSeconds = %g, want 1.0", cfg.Render.DurationSeconds)
	}

	if cfg.DiffuseRain {
		t.Error("Draft should disable DiffuseRain")
	}

	if cfg.Render.BandSpec.BandCount() != 6 {
		t.Errorf("Draft BandSpec has %d bands, want 6 (Octave6)", cfg.Render.BandSpec.BandCount())
	}
}

func TestPresetConfig_PreviewValues(t *testing.T) {
	t.Parallel()

	cfg := PresetConfig(QualityPreview)

	if cfg.MaxOrder != 3 {
		t.Errorf("Preview MaxOrder = %d, want 3", cfg.MaxOrder)
	}

	if cfg.NumRays != 5000 {
		t.Errorf("Preview NumRays = %d, want 5000", cfg.NumRays)
	}

	if cfg.DiffuseRain {
		t.Error("Preview should disable DiffuseRain")
	}
}

func TestPresetConfig_FinalValues(t *testing.T) {
	t.Parallel()

	cfg := PresetConfig(QualityFinal)

	if cfg.MaxOrder != 5 {
		t.Errorf("Final MaxOrder = %d, want 5", cfg.MaxOrder)
	}

	if cfg.NumRays != 50000 {
		t.Errorf("Final NumRays = %d, want 50000", cfg.NumRays)
	}

	if !cfg.DiffuseRain {
		t.Error("Final should enable DiffuseRain")
	}

	if cfg.Render.BandSpec.BandCount() != 8 {
		t.Errorf("Final BandSpec has %d bands, want 8 (Octave8)", cfg.Render.BandSpec.BandCount())
	}
}

func TestPresetConfig_Monotonic(t *testing.T) {
	t.Parallel()

	draft := PresetConfig(QualityDraft)
	preview := PresetConfig(QualityPreview)
	final := PresetConfig(QualityFinal)

	// Quality must strictly increase: ISM order, ray count, IR duration.
	if draft.MaxOrder >= preview.MaxOrder || preview.MaxOrder >= final.MaxOrder {
		t.Errorf("MaxOrder must strictly increase: draft=%d preview=%d final=%d",
			draft.MaxOrder, preview.MaxOrder, final.MaxOrder)
	}

	if draft.NumRays >= preview.NumRays || preview.NumRays >= final.NumRays {
		t.Errorf("NumRays must strictly increase: draft=%d preview=%d final=%d",
			draft.NumRays, preview.NumRays, final.NumRays)
	}

	if draft.Render.DurationSeconds > preview.Render.DurationSeconds ||
		preview.Render.DurationSeconds > final.Render.DurationSeconds {
		t.Errorf("DurationSeconds must not decrease across presets")
	}
}

func TestPresetConfig_UnknownPresetReturnsPreview(t *testing.T) {
	t.Parallel()

	got := PresetConfig(QualityPreset(99))
	want := PresetConfig(QualityPreview)

	if got.MaxOrder != want.MaxOrder || got.NumRays != want.NumRays {
		t.Error("unknown preset should fall back to preview")
	}
}

func TestPresetConfig_FieldOverride(t *testing.T) {
	t.Parallel()

	// Override a single field and verify the rest is preserved.
	cfg := PresetConfig(QualityPreview)
	original := cfg

	cfg.NumRays = 10000

	if cfg.NumRays != 10000 {
		t.Errorf("override NumRays = %d, want 10000", cfg.NumRays)
	}

	if cfg.MaxOrder != original.MaxOrder {
		t.Error("overriding NumRays must not change MaxOrder")
	}

	if cfg.Render.DurationSeconds != original.Render.DurationSeconds {
		t.Error("overriding NumRays must not change DurationSeconds")
	}
}

// TestPresetConfig_UsableWithRenderProgressive verifies that the scene remains
// authoritative for sample rate and band layout, even when a named preset has
// a different preferred band resolution.
func TestPresetConfig_UsableWithRenderProgressive(t *testing.T) {
	t.Parallel()

	sc := progressiveTestScene()

	cfg := PresetConfig(QualityFinal)
	if cfg.Render.BandSpec.BandCount() != 8 || sc.BandSpec.BandCount() != 6 {
		t.Fatal("test requires an Octave8 preset and Octave6 scene")
	}

	// Keep the test focused on format reconciliation rather than preset cost.
	cfg.Render.SampleRate = 44100
	cfg.Render.DurationSeconds = 0.05
	cfg.MaxOrder = 1
	cfg.PreviewISMOrder = 1
	cfg.NumRays = 1
	cfg.PreviewRayCount = 1
	cfg.RaysPerBatch = 1
	cfg.MaxTimeSeconds = 0.05
	cfg.DiffuseRain = false

	var sawFinal bool

	err := RenderProgressive(context.Background(), sc, cfg, func(r TierResult) {
		if r.Buffer != nil && r.Buffer.SampleRate != sc.SampleRate {
			t.Errorf("%s buffer sample rate = %d, want scene rate %d", r.Tier, r.Buffer.SampleRate, sc.SampleRate)
		}

		if r.Tier == TierFinal {
			sawFinal = true
		}
	})
	if err != nil {
		t.Fatalf("RenderProgressive with Final preset and Octave6 scene: %v", err)
	}

	if !sawFinal {
		t.Error("expected a TierFinal result with Draft preset")
	}
}
