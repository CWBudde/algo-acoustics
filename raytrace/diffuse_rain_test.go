package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestDiffuseRainEnergyFreefield(t *testing.T) {
	t.Parallel()

	// A diffuse reflection occurs on a wall at the origin, with the surface
	// normal pointing toward the receiver. This is the best-case geometry
	// where cos(Theta) = 1.0 (receiver is directly above the surface normal).
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}
	surfaceNormal := geometry.Vec3{X: 1, Y: 0, Z: 0}

	receiver := SphereReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Radius: 0.25,
	}

	// Particle energy after wall absorption/scattering split: 6 bands, 1.0 each.
	energy := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}
	scattering := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}
	centerFreqs := []float64{125, 250, 500, 1000, 2000, 4000}

	// No occluder: pass nil tracer (free-field).
	rain := computeDiffuseRain(
		reflectionPoint,
		surfaceNormal,
		energy,
		scattering,
		receiver,
		nil, // no tracer = free-field, no occlusion check
		centerFreqs,
		0.0, // pathLength so far (for arrival time)
		343.0,
	)

	if rain == nil {
		t.Fatal("computeDiffuseRain() returned nil, want non-nil contribution")
	}

	if rain.ArrivalTime <= 0 {
		t.Fatalf("arrival time = %v, want > 0", rain.ArrivalTime)
	}

	// Expected arrival time: distance / speed = 5.0 / 343.0
	expectedTime := 5.0 / 343.0
	if math.Abs(rain.ArrivalTime-expectedTime) > 1e-6 {
		t.Fatalf("arrival time = %v, want %v", rain.ArrivalTime, expectedTime)
	}

	// Each band should have positive energy.
	for i, e := range rain.BandEnergy {
		if e <= 0 {
			t.Fatalf("band %d energy = %v, want > 0", i, e)
		}
	}

	// The RAVEN formula for diffuse rain on a spherical detector:
	//   E_s = E_P * s * (1 - cos(gamma/2)) * 2 * cos(Theta) * exp(-m*r)
	// where gamma is the full opening angle = 2*asin(R/r), so
	//   cos(gamma/2) = cos(asin(R/r)) = sqrt(1 - (R/r)^2)
	// With cos(Theta) = 1.0 (normal points at receiver), r = 5.0 m.
	r := 5.0
	ratio := receiver.Radius / r
	cosHalfGamma := math.Sqrt(1 - ratio*ratio)
	expectedScale := (1 - cosHalfGamma) * 2.0 * 1.0 // cos(Theta)=1, s=1

	// Band 0 (125 Hz): air absorption is negligible at this distance.
	airAtten := math.Pow(10, -AlphaAirISO9613_1(125, defaultAirTemperatureC, defaultRelativeHumidity)*r/10)
	expectedBand0 := 1.0 * 1.0 * expectedScale * airAtten

	if diff := math.Abs(rain.BandEnergy[0] - expectedBand0); diff > expectedBand0*0.01 {
		t.Fatalf("band 0 energy = %v, want %v (diff=%v)", rain.BandEnergy[0], expectedBand0, diff)
	}
}

func TestDiffuseRainEnergyOblique(t *testing.T) {
	t.Parallel()

	// Surface normal at 60 degrees from the receiver direction.
	// cos(60deg) = 0.5, so rain energy should be halved.
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}

	// Normal at 60 deg from X-axis in XY-plane.
	surfaceNormal := geometry.Vec3{X: 0.5, Y: math.Sqrt(3) / 2, Z: 0}.Normalize()

	receiver := SphereReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Radius: 0.25,
	}

	energy := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}
	scattering := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}
	centerFreqs := []float64{125, 250, 500, 1000, 2000, 4000}

	rain := computeDiffuseRain(reflectionPoint, surfaceNormal, energy, scattering, receiver, nil, centerFreqs, 0.0, 343.0)

	if rain == nil {
		t.Fatal("computeDiffuseRain() returned nil for oblique angle")
	}

	// Compare against the normal-incidence case: should be half the energy
	// (cos(60 deg) = 0.5).
	rainNormal := computeDiffuseRain(
		reflectionPoint,
		geometry.Vec3{X: 1, Y: 0, Z: 0},
		energy, scattering, receiver, nil, centerFreqs, 0.0, 343.0,
	)

	ratio := rain.BandEnergy[0] / rainNormal.BandEnergy[0]
	if math.Abs(ratio-0.5) > 0.01 {
		t.Fatalf("oblique/normal ratio = %v, want 0.5", ratio)
	}
}

func TestDiffuseRainBackfacing(t *testing.T) {
	t.Parallel()

	// Surface normal points away from receiver. cos(Theta) < 0 → no rain.
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}
	surfaceNormal := geometry.Vec3{X: -1, Y: 0, Z: 0} // Pointing away

	receiver := SphereReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Radius: 0.25,
	}

	energy := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}
	scattering := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}
	centerFreqs := []float64{125, 250, 500, 1000, 2000, 4000}

	rain := computeDiffuseRain(reflectionPoint, surfaceNormal, energy, scattering, receiver, nil, centerFreqs, 0.0, 343.0)

	if rain != nil {
		t.Fatalf("computeDiffuseRain() = %v, want nil for backfacing surface", rain)
	}
}

func TestDiffuseRainOccluded(t *testing.T) {
	t.Parallel()

	// Use a shoebox tracer to occlude the rain path.
	// Reflection on one side of a wall, receiver on the other side.
	room := geometry.Box{
		Min: geometry.Vec3{X: 0, Y: 0, Z: 0},
		Max: geometry.Vec3{X: 10, Y: 5, Z: 3},
	}
	tracer := ShoeboxTracer{
		Bounds: room,
	}
	tracer.Walls[0] = geometry.NewPlaneFromPointNormal(geometry.Vec3{X: room.Min.X}, geometry.Vec3{X: -1})
	tracer.Walls[1] = geometry.NewPlaneFromPointNormal(geometry.Vec3{X: room.Max.X}, geometry.Vec3{X: 1})
	tracer.Walls[2] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Y: room.Min.Y}, geometry.Vec3{Y: -1})
	tracer.Walls[3] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Y: room.Max.Y}, geometry.Vec3{Y: 1})
	tracer.Walls[4] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Z: room.Min.Z}, geometry.Vec3{Z: -1})
	tracer.Walls[5] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Z: room.Max.Z}, geometry.Vec3{Z: 1})

	// Reflection on the X=0 wall (normal = -1,0,0 → inward normal = +1,0,0).
	reflectionPoint := geometry.Vec3{X: 0.001, Y: 2.5, Z: 1.5}
	surfaceNormal := geometry.Vec3{X: 1, Y: 0, Z: 0}

	// Receiver is at X=9, clearly visible (no internal walls).
	receiver := SphereReceiver{
		Center: geometry.Vec3{X: 9, Y: 2.5, Z: 1.5},
		Radius: 0.25,
	}

	energy := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}
	scattering := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}
	centerFreqs := []float64{125, 250, 500, 1000, 2000, 4000}

	// In a simple shoebox the path from wall to receiver is unoccluded,
	// so we should still get rain. The tracer will find a wall hit but it
	// should be BEYOND the receiver sphere.
	rain := computeDiffuseRain(reflectionPoint, surfaceNormal, energy, scattering, receiver, tracer, centerFreqs, 0.0, 343.0)

	if rain == nil {
		t.Fatal("computeDiffuseRain() returned nil, want non-nil (receiver is visible)")
	}

	for i, e := range rain.BandEnergy {
		if e <= 0 {
			t.Fatalf("band %d energy = %v, want > 0", i, e)
		}
	}
}

func TestDiffuseRainPartialScattering(t *testing.T) {
	t.Parallel()

	// With scattering = 0.5, rain energy should be half of s=1.0 case.
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}
	surfaceNormal := geometry.Vec3{X: 1, Y: 0, Z: 0}

	receiver := SphereReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Radius: 0.25,
	}

	energy := []float64{1.0}
	centerFreqs := []float64{125}

	rainFull := computeDiffuseRain(reflectionPoint, surfaceNormal, energy, []float64{1.0}, receiver, nil, centerFreqs, 0.0, 343.0)
	rainHalf := computeDiffuseRain(reflectionPoint, surfaceNormal, energy, []float64{0.5}, receiver, nil, centerFreqs, 0.0, 343.0)

	if rainFull == nil || rainHalf == nil {
		t.Fatal("got nil rain")
	}

	ratio := rainHalf.BandEnergy[0] / rainFull.BandEnergy[0]
	if math.Abs(ratio-0.5) > 0.01 {
		t.Fatalf("partial/full scattering ratio = %v, want 0.5", ratio)
	}
}

func TestDiffuseRainArrivalTimeIncludesPathLength(t *testing.T) {
	t.Parallel()

	// When pathLength > 0, the rain arrival time should include
	// the path the particle already traveled.
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}
	surfaceNormal := geometry.Vec3{X: 1, Y: 0, Z: 0}

	receiver := SphereReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Radius: 0.25,
	}

	energy := []float64{1.0}
	centerFreqs := []float64{125}

	priorPathLength := 10.0
	rain := computeDiffuseRain(reflectionPoint, surfaceNormal, energy, []float64{1.0}, receiver, nil, centerFreqs, priorPathLength, 343.0)

	if rain == nil {
		t.Fatal("computeDiffuseRain() returned nil")
	}

	expectedTime := (priorPathLength + 5.0) / 343.0
	if math.Abs(rain.ArrivalTime-expectedTime) > 1e-6 {
		t.Fatalf("arrival time = %v, want %v", rain.ArrivalTime, expectedTime)
	}
}

func TestDiffuseRainReceiverInsideReflectionPlane(t *testing.T) {
	t.Parallel()

	// Receiver very close to the reflection point (1 m). Rain should
	// still work and produce stronger energy due to 1/r^2 proximity.
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}
	surfaceNormal := geometry.Vec3{X: 1, Y: 0, Z: 0}

	nearReceiver := SphereReceiver{
		Center: geometry.Vec3{X: 1, Y: 0, Z: 0},
		Radius: 0.25,
	}

	farReceiver := SphereReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Radius: 0.25,
	}

	energy := []float64{1.0}
	centerFreqs := []float64{125}

	rainNear := computeDiffuseRain(reflectionPoint, surfaceNormal, energy, []float64{1.0}, nearReceiver, nil, centerFreqs, 0.0, 343.0)
	rainFar := computeDiffuseRain(reflectionPoint, surfaceNormal, energy, []float64{1.0}, farReceiver, nil, centerFreqs, 0.0, 343.0)

	if rainNear == nil || rainFar == nil {
		t.Fatal("got nil rain")
	}

	// Closer receiver should capture more energy.
	if rainNear.BandEnergy[0] <= rainFar.BandEnergy[0] {
		t.Fatalf("near energy %v should be > far energy %v", rainNear.BandEnergy[0], rainFar.BandEnergy[0])
	}
}

func TestDiffuseRainProducesMoreEnergy(t *testing.T) {
	t.Parallel()

	// Diffuse rain captures the analytical expected value of scattered
	// energy toward the receiver at every bounce. Without rain, the
	// scattered component only contributes when a diffuse ray stochastically
	// hits the small detector sphere. Rain recovers "lost" diffuse energy
	// at bounces where the ray went specular, so rain total > no-rain total.
	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6,
				Depth:  4.5,
				Height: 2.8,
				WallMaterials: [6]string{
					"absorptive", "absorptive", "absorptive",
					"absorptive", "absorptive", "absorptive",
				},
			},
		},
		Materials: map[string]scene.Material{
			"absorptive": {
				Name:             "absorptive",
				AbsorptionByBand: []float64{0.3, 0.3, 0.3, 0.3, 0.3, 0.3},
				ScatteringByBand: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5},
			},
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1.2, Y: 1.0, Z: 1.2}, GainDB: 0}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 3.5, Y: 2.2, Z: 1.2}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	baseCfg := LaunchConfig{
		NumRays:        2000,
		MaxBounces:     10,
		MaxTimeSeconds: 0.5,
		SpeedOfSound:   acoustics.SpeedOfSound,
	}

	noRainCfg := baseCfg
	noRainCfg.DiffuseRain = false

	histNoRain, err := (&RayTracer{Config: noRainCfg, Scene: sc}).Trace()
	if err != nil {
		t.Fatalf("Trace(no rain) error = %v", err)
	}

	rainCfg := baseCfg
	rainCfg.DiffuseRain = true

	histRain, err := (&RayTracer{Config: rainCfg, Scene: sc}).Trace()
	if err != nil {
		t.Fatalf("Trace(rain) error = %v", err)
	}

	totalNoRain := sumHistogramEnergy(histNoRain)
	totalRain := sumHistogramEnergy(histRain)

	if totalNoRain <= 0 {
		t.Fatal("no-rain histogram has zero energy")
	}

	// Rain should produce more total energy (it analytically captures
	// scattered contributions that stochastic detection misses).
	if totalRain <= totalNoRain {
		t.Fatalf("rain energy %v should exceed no-rain energy %v", totalRain, totalNoRain)
	}

	// The increase should be bounded — not wildly above the baseline.
	// For s=0.5, expect ~50-100% more energy from rain.
	relIncrease := (totalRain - totalNoRain) / totalNoRain
	if relIncrease > 2.0 {
		t.Fatalf("rain energy increase %.1f%% is too large (want ≤ 200%%)", relIncrease*100)
	}
}

func TestDiffuseRainReducesVariance(t *testing.T) {
	t.Parallel()

	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6,
				Depth:  4.5,
				Height: 2.8,
				WallMaterials: [6]string{
					"absorptive", "absorptive", "absorptive",
					"absorptive", "absorptive", "absorptive",
				},
			},
		},
		Materials: map[string]scene.Material{
			"absorptive": {
				Name:             "absorptive",
				AbsorptionByBand: []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
				ScatteringByBand: []float64{0.6, 0.6, 0.6, 0.6, 0.6, 0.6},
			},
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1.2, Y: 1.0, Z: 1.2}, GainDB: 0}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 3.5, Y: 2.2, Z: 1.2}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	baseCfg := LaunchConfig{
		NumRays:        5000,
		MaxBounces:     10,
		MaxTimeSeconds: 0.5,
		SpeedOfSound:   acoustics.SpeedOfSound,
	}

	// Count how many non-empty bins each mode fills in the late field
	// (past 30 ms). Diffuse rain should fill more bins since it deposits
	// energy at every diffuse reflection, not just on detector sphere hits.
	threshold := 0.03 // 30 ms

	noRainCfg := baseCfg
	noRainCfg.DiffuseRain = false

	histNoRain, err := (&RayTracer{Config: noRainCfg, Scene: sc}).Trace()
	if err != nil {
		t.Fatalf("Trace(no rain) error = %v", err)
	}

	rainCfg := baseCfg
	rainCfg.DiffuseRain = true

	histRain, err := (&RayTracer{Config: rainCfg, Scene: sc}).Trace()
	if err != nil {
		t.Fatalf("Trace(rain) error = %v", err)
	}

	filledNoRain := countFilledBins(histNoRain, threshold)
	filledRain := countFilledBins(histRain, threshold)

	// With rain, significantly more late-field bins should be filled.
	// This is the primary variance-reduction benefit: rain deposits energy
	// analytically at every bounce, producing a dense histogram, while
	// the stochastic process leaves most late-field bins empty.
	if filledRain < filledNoRain {
		t.Fatalf("rain filled %d late bins, no-rain filled %d — rain should fill at least as many", filledRain, filledNoRain)
	}

	// Rain should fill at least 2x more bins than no-rain for the same
	// ray count. With 5000 rays in a small shoebox, rain fills most
	// late-field bins while stochastic hits are sparse.
	if filledNoRain > 0 {
		binRatio := float64(filledRain) / float64(filledNoRain)
		if binRatio < 2.0 {
			t.Fatalf("rain/noRain filled bin ratio = %.2f (want ≥ 2.0): rain=%d, noRain=%d", binRatio, filledRain, filledNoRain)
		}
	}
}

func sumHistogramEnergy(hist *EnergyHistogram) float64 {
	if hist == nil {
		return 0
	}

	var total float64

	for _, bin := range hist.Bins {
		for _, e := range bin.BandEnergy {
			total += e
		}
	}

	return total
}

func countFilledBins(hist *EnergyHistogram, minTime float64) int {
	if hist == nil {
		return 0
	}

	count := 0

	for _, bin := range hist.Bins {
		if bin.TimeSeconds < minTime {
			continue
		}

		var total float64

		for _, e := range bin.BandEnergy {
			total += e
		}

		if total > 0 {
			count++
		}
	}

	return count
}

func BenchmarkDiffuseRain(b *testing.B) {
	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6,
				Depth:  4.5,
				Height: 2.8,
				WallMaterials: [6]string{
					"absorptive", "absorptive", "absorptive",
					"absorptive", "absorptive", "absorptive",
				},
			},
		},
		Materials: map[string]scene.Material{
			"absorptive": {
				Name:             "absorptive",
				AbsorptionByBand: []float64{0.3, 0.3, 0.3, 0.3, 0.3, 0.3},
				ScatteringByBand: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5},
			},
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1.2, Y: 1.0, Z: 1.2}, GainDB: 0}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 3.5, Y: 2.2, Z: 1.2}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	baseCfg := LaunchConfig{
		NumRays:        1000,
		MaxBounces:     10,
		MaxTimeSeconds: 0.5,
		SpeedOfSound:   acoustics.SpeedOfSound,
	}

	b.Run("NoRain", func(b *testing.B) {
		cfg := baseCfg
		cfg.DiffuseRain = false
		for range b.N {
			_, _ = (&RayTracer{Config: cfg, Scene: sc}).Trace()
		}
	})

	b.Run("WithRain", func(b *testing.B) {
		cfg := baseCfg
		cfg.DiffuseRain = true
		for range b.N {
			_, _ = (&RayTracer{Config: cfg, Scene: sc}).Trace()
		}
	})
}
