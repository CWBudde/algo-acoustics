package algoacoustics

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

func progressiveTestScene() *scene.Scene {
	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width: 6.0, Depth: 4.5, Height: 2.8,
				WallMaterials: [6]string{
					"plaster", "plaster", "plaster",
					"plaster", "plaster", "plaster",
				},
			},
		},
		Materials: map[string]scene.Material{
			"plaster": {
				Name:             "plaster",
				AbsorptionByBand: []float64{0.10, 0.10, 0.15, 0.20, 0.20, 0.25},
			},
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 1.5, Y: 2.0, Z: 1.2},
			Orientation: geometry.Quaternion{W: 1},
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 4.0, Y: 2.0, Z: 1.2},
			Orientation: geometry.Quaternion{W: 1},
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}

func progressiveTestConfig() ProgressiveConfig {
	return ProgressiveConfig{
		Render: ir.RenderConfig{
			SampleRate:      48000,
			DurationSeconds: 0.5,
			BandSpec:        acoustics.Octave6,
		},
		Hybrid:          hybrid.HybridConfig{CrossoverTimeSeconds: 0.05},
		MaxOrder:        3,
		NumRays:         2000,
		MaxTimeSeconds:  0.5,
		PreviewISMOrder: 2,
		PreviewRayCount: 1000,
		RaysPerBatch:    1000,
	}
}

func TestRenderProgressive_AllTiers(t *testing.T) {
	t.Parallel()

	sc := progressiveTestScene()
	cfg := progressiveTestConfig()

	var results []TierResult

	err := RenderProgressive(context.Background(), sc, cfg, func(r TierResult) {
		results = append(results, r)
	})
	if err != nil {
		t.Fatalf("RenderProgressive() error = %v", err)
	}

	// Expect: Statistical, Preview, Refined batch 1, Refined batch 2, Final = 5 results
	if len(results) < 4 {
		t.Fatalf("got %d results, want at least 4", len(results))
	}

	if results[0].Tier != TierStatistical {
		t.Errorf("results[0].Tier = %v, want Statistical", results[0].Tier)
	}

	if results[0].Metrics == nil {
		t.Error("statistical tier has nil metrics")
	}

	if results[0].Buffer != nil {
		t.Error("statistical tier should have nil buffer")
	}

	if results[1].Tier != TierPreview {
		t.Errorf("results[1].Tier = %v, want Preview", results[1].Tier)
	}

	if results[1].Buffer == nil {
		t.Error("preview tier has nil buffer")
	}

	last := results[len(results)-1]
	if last.Tier != TierFinal {
		t.Errorf("last result tier = %v, want Final", last.Tier)
	}

	if last.Buffer == nil {
		t.Error("final tier has nil buffer")
	}
}

func TestRenderProgressive_StatisticalMetrics(t *testing.T) {
	t.Parallel()

	sc := progressiveTestScene()
	cfg := progressiveTestConfig()

	var statResult TierResult

	_ = RenderProgressive(context.Background(), sc, cfg, func(r TierResult) {
		if r.Tier == TierStatistical {
			statResult = r
		}
	})

	m := statResult.Metrics
	if m == nil {
		t.Fatal("statistical metrics is nil")
	}

	if len(m.SabineRT60ByBand) != 6 {
		t.Fatalf("SabineRT60ByBand has %d bands, want 6", len(m.SabineRT60ByBand))
	}

	// Sabine RT60 for band 0 (alpha=0.1): 0.161 * 75.6 / (112.8 * 0.1) ≈ 1.079 s
	if m.SabineRT60ByBand[0] < 0.5 || m.SabineRT60ByBand[0] > 2.0 {
		t.Errorf("SabineRT60ByBand[0] = %g, want in [0.5, 2.0]", m.SabineRT60ByBand[0])
	}

	// Eyring < Sabine for same band
	if m.EyringRT60ByBand[0] >= m.SabineRT60ByBand[0] {
		t.Errorf("Eyring (%g) should be < Sabine (%g)", m.EyringRT60ByBand[0], m.SabineRT60ByBand[0])
	}

	// C80 should be finite
	if m.C80ByBand[0] < -30 || m.C80ByBand[0] > 30 {
		t.Errorf("C80ByBand[0] = %g, want in [-30, 30]", m.C80ByBand[0])
	}

	// D50 should be in (0, 1)
	if m.D50ByBand[0] <= 0 || m.D50ByBand[0] >= 1 {
		t.Errorf("D50ByBand[0] = %g, want in (0, 1)", m.D50ByBand[0])
	}
}

func TestRenderProgressive_Cancellation(t *testing.T) {
	t.Parallel()

	sc := progressiveTestScene()
	cfg := progressiveTestConfig()

	ctx, cancel := context.WithCancel(context.Background())

	var mu sync.Mutex
	var results []TierResult

	err := RenderProgressive(ctx, sc, cfg, func(r TierResult) {
		mu.Lock()

		results = append(results, r)
		mu.Unlock()

		if r.Tier == TierPreview {
			cancel()
		}
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	for _, r := range results {
		if r.Tier > TierPreview {
			t.Errorf("unexpected tier after cancellation: %v", r.Tier)
		}
	}
}

func TestRenderProgressive_TierOrder(t *testing.T) {
	t.Parallel()

	sc := progressiveTestScene()
	cfg := progressiveTestConfig()

	var tiers []Tier

	err := RenderProgressive(context.Background(), sc, cfg, func(r TierResult) {
		tiers = append(tiers, r.Tier)
	})
	if err != nil {
		t.Fatalf("RenderProgressive() error = %v", err)
	}

	for i := 1; i < len(tiers); i++ {
		if tiers[i] < tiers[i-1] {
			t.Errorf("tier order violation: tiers[%d]=%v < tiers[%d]=%v",
				i, tiers[i], i-1, tiers[i-1])
		}
	}
}

func TestRenderProgressive_ProgressiveRayBatches(t *testing.T) {
	t.Parallel()

	sc := progressiveTestScene()
	cfg := progressiveTestConfig()
	cfg.NumRays = 3000
	cfg.RaysPerBatch = 1000

	var refinedBatches []int

	err := RenderProgressive(context.Background(), sc, cfg, func(r TierResult) {
		if r.Tier == TierRefined {
			refinedBatches = append(refinedBatches, r.RayBatches)
		}
	})
	if err != nil {
		t.Fatalf("RenderProgressive() error = %v", err)
	}

	// 3000 rays / 1000 per batch = 3 refined updates
	if len(refinedBatches) != 3 {
		t.Fatalf("got %d refined batches, want 3", len(refinedBatches))
	}

	for i, batch := range refinedBatches {
		if batch != i+1 {
			t.Errorf("refinedBatches[%d] = %d, want %d", i, batch, i+1)
		}
	}
}

func TestRenderProgressive_NilScene(t *testing.T) {
	t.Parallel()

	err := RenderProgressive(context.Background(), nil, ProgressiveConfig{}, func(TierResult) {})
	if err == nil {
		t.Fatal("expected error for nil scene")
	}
}

func TestRenderProgressive_RejectsNilContextAndUpdate(t *testing.T) {
	t.Parallel()

	sc := progressiveTestScene()
	cfg := progressiveTestConfig()

	err := RenderProgressive(nil, sc, cfg, func(TierResult) {}) //nolint:staticcheck // nil context is the input under test
	if err == nil || !strings.Contains(err.Error(), "context is nil") {
		t.Fatalf("nil context error = %v, want context is nil", err)
	}

	err = RenderProgressive(context.Background(), sc, cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "update callback is nil") {
		t.Fatalf("nil update error = %v, want update callback is nil", err)
	}
}

func TestRenderProgressive_ValidatesBeforeFirstUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		scene   *scene.Scene
		cfg     ProgressiveConfig
		wantErr string
	}{
		{
			name: "invalid scene",
			scene: func() *scene.Scene {
				sc := progressiveTestScene()
				sc.SampleRate = 0

				return sc
			}(),
			cfg:     progressiveTestConfig(),
			wantErr: "validate scene",
		},
		{
			name:  "invalid config",
			scene: progressiveTestScene(),
			cfg: func() ProgressiveConfig {
				cfg := progressiveTestConfig()
				cfg.Render.DurationSeconds = 0

				return cfg
			}(),
			wantErr: "render duration must be positive",
		},
		{
			name:  "negative defaultable config value",
			scene: progressiveTestScene(),
			cfg: func() ProgressiveConfig {
				cfg := progressiveTestConfig()
				cfg.SpeedOfSound = -1

				return cfg
			}(),
			wantErr: "speed of sound must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			updates := 0

			err := RenderProgressive(context.Background(), tt.scene, tt.cfg, func(TierResult) {
				updates++
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("RenderProgressive() error = %v, want %q", err, tt.wantErr)
			}

			if updates != 0 {
				t.Fatalf("update called %d times for invalid input, want 0", updates)
			}
		})
	}
}

func TestTier_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tier Tier
		want string
	}{
		{TierStatistical, "statistical"},
		{TierPreview, "preview"},
		{TierRefined, "refined"},
		{TierFinal, "final"},
		{Tier(99), "tier(99)"},
	}

	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.want {
			t.Errorf("Tier(%d).String() = %q, want %q", int(tt.tier), got, tt.want)
		}
	}
}
