package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestDetectionAllowedHybrid(t *testing.T) {
	t.Parallel()

	limits := HybridDetectionConfig{
		Enabled:         true,
		MaxISOrder:      3,
		MaxPreEDISOrder: 2,
		MaxEDISOrder:    1,
		MaxEDOrder:      1,
	}

	tests := []struct {
		name  string
		state RayState
		cfg   HybridDetectionConfig
		want  bool
	}{
		{
			name:  "disabled always detects direct sound",
			state: RayState{},
			cfg:   HybridDetectionConfig{MaxISOrder: 3},
			want:  true,
		},
		{
			name:  "disabled always detects diffracted particle",
			state: RayState{EDOrder: 1, ReflectionOrder: 1, PreEDReflOrder: 1},
			cfg:   HybridDetectionConfig{MaxISOrder: 3},
			want:  true,
		},
		{
			name:  "direct sound is image-source covered",
			state: RayState{},
			cfg:   limits,
			want:  false,
		},
		{
			name:  "specular at max image-source order is covered",
			state: RayState{ReflectionOrder: 3, PreEDReflOrder: 3, AllowDetection: true},
			cfg:   limits,
			want:  false,
		},
		{
			name:  "specular beyond max image-source order is detected",
			state: RayState{ReflectionOrder: 4, PreEDReflOrder: 4, AllowDetection: true},
			cfg:   limits,
			want:  true,
		},
		{
			name:  "scattered and diffracted particle is detected",
			state: RayState{HasDiffuseHistory: true, EDOrder: 1, ReflectionOrder: 1, PreEDReflOrder: 1},
			cfg:   limits,
			want:  true,
		},
		{
			name:  "diffracted particle within all limits is covered",
			state: RayState{EDOrder: 1, ReflectionOrder: 3, PreEDReflOrder: 2, EDReflOrder: 1},
			cfg:   limits,
			want:  false,
		},
		{
			name:  "diffracted particle beyond pre-diffraction limit is detected",
			state: RayState{EDOrder: 1, ReflectionOrder: 4, PreEDReflOrder: 3, EDReflOrder: 1},
			cfg:   limits,
			want:  true,
		},
		{
			name:  "diffracted particle beyond post-diffraction limit is detected",
			state: RayState{EDOrder: 1, ReflectionOrder: 4, PreEDReflOrder: 2, EDReflOrder: 2},
			cfg:   limits,
			want:  true,
		},
		{
			name:  "diffracted particle beyond diffraction order limit is detected",
			state: RayState{EDOrder: 2, ReflectionOrder: 3, PreEDReflOrder: 2, EDReflOrder: 1},
			cfg:   limits,
			want:  true,
		},
		{
			name:  "scattered particle without diffraction follows reflection order",
			state: RayState{HasDiffuseHistory: true, ReflectionOrder: 2, PreEDReflOrder: 2},
			cfg:   limits,
			want:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := DetectionAllowedHybrid(test.state, test.cfg); got != test.want {
				t.Fatalf("DetectionAllowedHybrid() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAdvanceStateAfterReflectionRoutesCounters(t *testing.T) {
	t.Parallel()

	var state RayState

	advanceStateAfterReflection(&state, false)

	if state.ReflectionOrder != 1 || state.PreEDReflOrder != 1 || state.EDReflOrder != 0 {
		t.Fatalf("after specular reflection: %+v, want ReflectionOrder=1 PreEDReflOrder=1 EDReflOrder=0", state)
	}

	if !state.AllowDetection {
		t.Fatal("AllowDetection = false after specular reflection, want true")
	}

	if state.HasDiffuseHistory {
		t.Fatal("HasDiffuseHistory = true after specular reflection, want false")
	}

	advanceStateAfterReflection(&state, true)

	if state.ReflectionOrder != 2 || state.PreEDReflOrder != 2 || state.EDReflOrder != 0 {
		t.Fatalf("after scattered reflection: %+v, want ReflectionOrder=2 PreEDReflOrder=2 EDReflOrder=0", state)
	}

	if state.AllowDetection {
		t.Fatal("AllowDetection = true after scattering, want false")
	}

	if !state.HasDiffuseHistory {
		t.Fatal("HasDiffuseHistory = false after scattering, want true")
	}

	advanceStateAfterDiffraction(&state)

	if state.EDOrder != 1 || state.AllowDetection {
		t.Fatalf("after diffraction: %+v, want EDOrder=1 AllowDetection=false", state)
	}

	advanceStateAfterReflection(&state, false)

	if state.ReflectionOrder != 3 || state.PreEDReflOrder != 2 || state.EDReflOrder != 1 {
		t.Fatalf("after post-diffraction reflection: %+v, want ReflectionOrder=3 PreEDReflOrder=2 EDReflOrder=1", state)
	}

	if !state.AllowDetection {
		t.Fatal("AllowDetection = false after post-diffraction specular reflection, want true")
	}
}

func TestChildStatesInheritHybridCounters(t *testing.T) {
	t.Parallel()

	parent := RayState{
		HasDiffuseHistory: true,
		AllowDetection:    true,
		ReflectionOrder:   2,
		PreEDReflOrder:    1,
		EDReflOrder:       1,
		EDOrder:           1,
	}

	specular := reflectedChildState(parent, false)
	if specular.ReflectionOrder != 3 || specular.EDReflOrder != 2 || specular.PreEDReflOrder != 1 ||
		specular.EDOrder != 1 || !specular.HasDiffuseHistory || !specular.AllowDetection {
		t.Fatalf("reflectedChildState(specular) = %+v", specular)
	}

	diffuse := reflectedChildState(parent, true)
	if diffuse.ReflectionOrder != 3 || diffuse.EDReflOrder != 2 || diffuse.AllowDetection {
		t.Fatalf("reflectedChildState(diffuse) = %+v", diffuse)
	}

	diffracted := diffractedChildState(parent)
	if diffracted.EDOrder != 2 || diffracted.ReflectionOrder != 2 || diffracted.AllowDetection {
		t.Fatalf("diffractedChildState() = %+v", diffracted)
	}
}

// hybridPartitionTrace traces the same fully specular shoebox with a given
// hybrid gate and returns the per-bin band energy sums.
func hybridPartitionTrace(t *testing.T, cfg HybridDetectionConfig) []float64 {
	t.Helper()

	sc := newTestShoeboxScene(5, 4, 3)
	sc.Sources[0].Position = geometry.Vec3{X: 1.3, Y: 1.1, Z: 1.7}
	sc.Receivers[0].Position = geometry.Vec3{X: 3.4, Y: 2.6, Z: 1.4}

	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:            2048,
			MaxBounces:         6,
			MaxTimeSeconds:     0.15,
			SpeedOfSound:       acoustics.SpeedOfSound,
			ReflectionStrategy: ReflectionStrategyProbabilistic,
			HybridDetection:    cfg,
		},
		Scene:              sc,
		ReceiverRadius:     0.5,
		BinDurationSeconds: 0.005,
	}

	hist, err := rt.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	totals := make([]float64, len(hist.Bins))
	for index, bin := range hist.Bins {
		for _, energy := range bin.BandEnergy {
			totals[index] += energy
		}
	}

	return totals
}

func sumFloats(values []float64) float64 {
	var total float64
	for _, value := range values {
		total += value
	}

	return total
}

func TestHybridDetectionPartitionsEnergy(t *testing.T) {
	t.Parallel()

	ungated := hybridPartitionTrace(t, HybridDetectionConfig{})
	if sumFloats(ungated) <= 0 {
		t.Fatal("ungated trace produced no energy")
	}

	// A gate admitting every reflection order must reproduce the ungated run
	// exactly: gating skips deposits, it never perturbs the simulation.
	admitAll := hybridPartitionTrace(t, HybridDetectionConfig{Enabled: true, MaxISOrder: -1})
	for index := range ungated {
		if admitAll[index] != ungated[index] {
			t.Fatalf("bin %d: admit-all gate = %g, ungated = %g", index, admitAll[index], ungated[index])
		}
	}

	const maxOrder = 6

	gated := make([][]float64, maxOrder+1)
	for order := range gated {
		gated[order] = hybridPartitionTrace(t, HybridDetectionConfig{Enabled: true, MaxISOrder: order})
	}

	// The gate must only remove detections, never add or duplicate them, so the
	// captured energy is monotonically non-increasing in the image-source order.
	previous := ungated
	for order := range gated {
		for index := range previous {
			if gated[order][index] > previous[index]+1e-12 {
				t.Fatalf("order %d bin %d: gated = %g exceeds previous = %g", order, index, gated[order][index], previous[index])
			}
		}

		previous = gated[order]
	}

	// The per-order shells (energy admitted by gate n-1 but rejected by gate n)
	// plus the residual beyond the last order must recover the ungated total:
	// the gate partitions energy rather than losing it.
	complement := 0.0
	previous = ungated

	for order := range gated {
		for index := range previous {
			shell := previous[index] - gated[order][index]
			if shell < -1e-12 {
				t.Fatalf("order %d bin %d: negative shell energy %g", order, index, shell)
			}

			complement += shell
		}

		previous = gated[order]
	}

	total := sumFloats(ungated)
	recovered := complement + sumFloats(gated[maxOrder])

	if math.Abs(recovered-total) > 0.02*total {
		t.Fatalf("partitioned energy = %g, ungated total = %g (>2%% mismatch)", recovered, total)
	}

	// The partition must be non-trivial: the first-order gate has to reject a
	// measurable amount of image-source covered energy.
	if sumFloats(gated[0]) >= total {
		t.Fatalf("gate at order 0 captured %g, want less than the ungated total %g", sumFloats(gated[0]), total)
	}

	if sumFloats(gated[0]) <= 0 {
		t.Fatal("gate at order 0 captured no energy, want the late field to survive")
	}
}
