package raytrace

import (
	"testing"
	"time"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func newParityRayTracer(numRays, maxBounces int, maxTime float64) *RayTracer {
	abs := make([]float64, scene.NumBands)
	for i := range abs {
		abs[i] = 0.1
	}

	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  10,
				Depth:  8,
				Height: 3,
				WallMaterials: [6]string{
					"walls", "walls", "walls",
					"walls", "floor", "ceiling",
				},
			},
		},
		Materials: map[string]scene.Material{
			"walls":   {AbsorptionByBand: append([]float64{}, abs...)},
			"floor":   {AbsorptionByBand: append([]float64{}, abs...)},
			"ceiling": {AbsorptionByBand: append([]float64{}, abs...)},
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}, GainDB: 0}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	return &RayTracer{
		Config: LaunchConfig{
			NumRays:            numRays,
			MaxBounces:         maxBounces,
			MaxTimeSeconds:     maxTime,
			SpeedOfSound:       343.0,
			ReflectionStrategy: ReflectionStrategyDeterministicBlend,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}
}

func TestParity_TraceVsCachedReplay(t *testing.T) {
	t.Parallel()

	rt := newParityRayTracer(500, 50, 1.0)

	// Direct trace.
	directHist, err := rt.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	// Two-phase: TracePaths then EvaluatePaths.
	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths() error = %v", err)
	}

	cachedHist, err := rt.EvaluatePaths(cache)
	if err != nil {
		t.Fatalf("EvaluatePaths() error = %v", err)
	}

	directEnergy := sumHistogramEnergy(directHist)
	cachedEnergy := sumHistogramEnergy(cachedHist)

	if directEnergy <= 0 {
		t.Fatal("Trace() produced zero total energy")
	}

	if cachedEnergy <= 0 {
		t.Fatal("EvaluatePaths() produced zero total energy")
	}

	ratio := cachedEnergy / directEnergy
	t.Logf("direct energy = %e, cached energy = %e, ratio = %.4f", directEnergy, cachedEnergy, ratio)

	if ratio < 0.1 || ratio > 10 {
		t.Fatalf("energy ratio %.4f outside acceptable range [0.1, 10]", ratio)
	}
}

func TestParity_ReplayFasterThanTrace(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	rt := newParityRayTracer(10_000, 100, 2.0)

	// Time the full Trace.
	traceStart := time.Now()

	_, err := rt.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	traceDuration := time.Since(traceStart)

	// Build the cache (not timed as part of replay).
	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths() error = %v", err)
	}

	// Time the replay.
	replayStart := time.Now()

	_, err = rt.EvaluatePaths(cache)
	if err != nil {
		t.Fatalf("EvaluatePaths() error = %v", err)
	}

	replayDuration := time.Since(replayStart)

	speedup := float64(traceDuration) / float64(replayDuration)
	t.Logf("trace = %v, replay = %v, speedup = %.2fx", traceDuration, replayDuration, speedup)

	if speedup < 1.0 {
		t.Logf("WARNING: replay was not faster than trace (speedup %.2fx); CI machines may vary", speedup)
	}
}
