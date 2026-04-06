package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestSurfaceReceiverRainNormalIncidence(t *testing.T) {
	t.Parallel()

	// A surface detector centered at (5,0,0) facing the -X direction (toward
	// the reflection point). The reflection occurs at the origin with wall
	// normal pointing toward the detector (+X). This is the ideal case:
	// cos(Theta) = 1, cos(Psi) = 1.
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}
	surfaceNormal := geometry.Vec3{X: 1, Y: 0, Z: 0}

	detector := SurfaceReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Normal: geometry.Vec3{X: -1, Y: 0, Z: 0}, // Facing the reflection
		Area:   4.0,                                // 2m x 2m
	}

	energy := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}
	scattering := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0}
	centerFreqs := []float64{125, 250, 500, 1000, 2000, 4000}

	rain := computeSurfaceRain(
		reflectionPoint,
		surfaceNormal,
		energy,
		scattering,
		detector,
		nil,
		centerFreqs,
		0.0,
		343.0,
	)

	if rain == nil {
		t.Fatal("computeSurfaceRain() returned nil, want contribution")
	}

	// Expected: E * s * (A / (2*pi*r^2)) * cos(Psi) * cos(Theta) * airAtten
	// With cos(Theta) = 1, cos(Psi) = 1, r = 5, A = 4.
	r := 5.0
	expectedScale := 4.0 / (2 * math.Pi * r * r) * 1.0 * 1.0

	airAtten := math.Pow(10, -AlphaAirISO9613_1(125, defaultAirTemperatureC, defaultRelativeHumidity)*r/10)
	expectedBand0 := 1.0 * 1.0 * expectedScale * airAtten

	if diff := math.Abs(rain.BandEnergy[0] - expectedBand0); diff > expectedBand0*0.01 {
		t.Fatalf("band 0 energy = %v, want %v (diff=%v)", rain.BandEnergy[0], expectedBand0, diff)
	}

	// Arrival time: distance / speed.
	expectedTime := 5.0 / 343.0
	if math.Abs(rain.ArrivalTime-expectedTime) > 1e-6 {
		t.Fatalf("arrival time = %v, want %v", rain.ArrivalTime, expectedTime)
	}
}

func TestSurfaceReceiverRainObliqueDetector(t *testing.T) {
	t.Parallel()

	// Detector tilted 60 degrees from the connection vector. cos(Psi) = 0.5.
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}
	surfaceNormal := geometry.Vec3{X: 1, Y: 0, Z: 0}

	// Normal at 60 deg from -X axis in XY-plane.
	detectorNormal := geometry.Vec3{X: -0.5, Y: math.Sqrt(3) / 2, Z: 0}.Normalize()

	normalDetector := SurfaceReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Normal: geometry.Vec3{X: -1, Y: 0, Z: 0},
		Area:   4.0,
	}

	obliqueDetector := SurfaceReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Normal: detectorNormal,
		Area:   4.0,
	}

	energy := []float64{1.0}
	scattering := []float64{1.0}
	centerFreqs := []float64{125}

	rainNormal := computeSurfaceRain(reflectionPoint, surfaceNormal, energy, scattering, normalDetector, nil, centerFreqs, 0.0, 343.0)
	rainOblique := computeSurfaceRain(reflectionPoint, surfaceNormal, energy, scattering, obliqueDetector, nil, centerFreqs, 0.0, 343.0)

	if rainNormal == nil || rainOblique == nil {
		t.Fatal("got nil rain")
	}

	// Oblique detector: cos(Psi) = 0.5 → half the energy.
	ratio := rainOblique.BandEnergy[0] / rainNormal.BandEnergy[0]
	if math.Abs(ratio-0.5) > 0.01 {
		t.Fatalf("oblique/normal ratio = %v, want 0.5", ratio)
	}
}

func TestSurfaceReceiverRainBackfacingDetector(t *testing.T) {
	t.Parallel()

	// Detector normal points away from the reflection point. cos(Psi) < 0 → no rain.
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}
	surfaceNormal := geometry.Vec3{X: 1, Y: 0, Z: 0}

	detector := SurfaceReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Normal: geometry.Vec3{X: 1, Y: 0, Z: 0}, // Facing away
		Area:   4.0,
	}

	energy := []float64{1.0}
	scattering := []float64{1.0}
	centerFreqs := []float64{125}

	rain := computeSurfaceRain(reflectionPoint, surfaceNormal, energy, scattering, detector, nil, centerFreqs, 0.0, 343.0)

	if rain != nil {
		t.Fatalf("computeSurfaceRain() should return nil for backfacing detector, got %v", rain.BandEnergy)
	}
}

func TestSurfaceReceiverRainBackfacingWall(t *testing.T) {
	t.Parallel()

	// Wall normal points away from detector → no rain.
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}
	surfaceNormal := geometry.Vec3{X: -1, Y: 0, Z: 0} // Away from detector

	detector := SurfaceReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Normal: geometry.Vec3{X: -1, Y: 0, Z: 0},
		Area:   4.0,
	}

	energy := []float64{1.0}
	scattering := []float64{1.0}
	centerFreqs := []float64{125}

	rain := computeSurfaceRain(reflectionPoint, surfaceNormal, energy, scattering, detector, nil, centerFreqs, 0.0, 343.0)

	if rain != nil {
		t.Fatalf("computeSurfaceRain() should return nil for backfacing wall, got %v", rain.BandEnergy)
	}
}

func TestSurfaceReceiverRainAreaScaling(t *testing.T) {
	t.Parallel()

	// Doubling the detector area should double the rain energy.
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}
	surfaceNormal := geometry.Vec3{X: 1, Y: 0, Z: 0}

	small := SurfaceReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Normal: geometry.Vec3{X: -1, Y: 0, Z: 0},
		Area:   2.0,
	}

	large := SurfaceReceiver{
		Center: geometry.Vec3{X: 5, Y: 0, Z: 0},
		Normal: geometry.Vec3{X: -1, Y: 0, Z: 0},
		Area:   4.0,
	}

	energy := []float64{1.0}
	scattering := []float64{1.0}
	centerFreqs := []float64{125}

	rainSmall := computeSurfaceRain(reflectionPoint, surfaceNormal, energy, scattering, small, nil, centerFreqs, 0.0, 343.0)
	rainLarge := computeSurfaceRain(reflectionPoint, surfaceNormal, energy, scattering, large, nil, centerFreqs, 0.0, 343.0)

	if rainSmall == nil || rainLarge == nil {
		t.Fatal("got nil rain")
	}

	ratio := rainLarge.BandEnergy[0] / rainSmall.BandEnergy[0]
	if math.Abs(ratio-2.0) > 0.01 {
		t.Fatalf("large/small area ratio = %v, want 2.0", ratio)
	}
}

func TestSurfaceReceiverRainOccluded(t *testing.T) {
	t.Parallel()

	// Use a shoebox tracer. Reflection on X=0.001 wall, detector at X=9.
	// Inside a shoebox, the path is unoccluded.
	room := geometry.Box{
		Min: geometry.Vec3{X: 0, Y: 0, Z: 0},
		Max: geometry.Vec3{X: 10, Y: 5, Z: 3},
	}
	tracer := ShoeboxTracer{Bounds: room}
	tracer.Walls[0] = geometry.NewPlaneFromPointNormal(geometry.Vec3{X: room.Min.X}, geometry.Vec3{X: -1})
	tracer.Walls[1] = geometry.NewPlaneFromPointNormal(geometry.Vec3{X: room.Max.X}, geometry.Vec3{X: 1})
	tracer.Walls[2] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Y: room.Min.Y}, geometry.Vec3{Y: -1})
	tracer.Walls[3] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Y: room.Max.Y}, geometry.Vec3{Y: 1})
	tracer.Walls[4] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Z: room.Min.Z}, geometry.Vec3{Z: -1})
	tracer.Walls[5] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Z: room.Max.Z}, geometry.Vec3{Z: 1})

	reflectionPoint := geometry.Vec3{X: 0.001, Y: 2.5, Z: 1.5}
	surfaceNormal := geometry.Vec3{X: 1, Y: 0, Z: 0}

	detector := SurfaceReceiver{
		Center: geometry.Vec3{X: 9, Y: 2.5, Z: 1.5},
		Normal: geometry.Vec3{X: -1, Y: 0, Z: 0},
		Area:   4.0,
	}

	energy := []float64{1.0}
	scattering := []float64{1.0}
	centerFreqs := []float64{125}

	rain := computeSurfaceRain(reflectionPoint, surfaceNormal, energy, scattering, detector, tracer, centerFreqs, 0.0, 343.0)

	// The X=10 wall hit is at distance ~10 from origin, but detector center
	// is at distance ~9. Since the detector is a surface (no radius to
	// subtract), we check that the wall hit is beyond the detector center.
	if rain == nil {
		t.Fatal("computeSurfaceRain() returned nil, want non-nil (detector is visible)")
	}

	if rain.BandEnergy[0] <= 0 {
		t.Fatal("expected positive energy")
	}
}

func TestSurfaceReceiverRainMatchesSphereOrder(t *testing.T) {
	t.Parallel()

	// At a given distance, a spherical detector with cross-sectional area A
	// and a surface detector with the same area A facing the reflection
	// should produce energies within the same order of magnitude.
	reflectionPoint := geometry.Vec3{X: 0, Y: 0, Z: 0}
	surfaceNormal := geometry.Vec3{X: 1, Y: 0, Z: 0}

	r := 5.0
	sphereRadius := 0.25
	// Cross-sectional area of the sphere: pi * r^2
	sphereArea := math.Pi * sphereRadius * sphereRadius

	sphere := SphereReceiver{
		Center: geometry.Vec3{X: r, Y: 0, Z: 0},
		Radius: sphereRadius,
	}

	surface := SurfaceReceiver{
		Center: geometry.Vec3{X: r, Y: 0, Z: 0},
		Normal: geometry.Vec3{X: -1, Y: 0, Z: 0},
		Area:   sphereArea,
	}

	energy := []float64{1.0}
	scattering := []float64{1.0}
	centerFreqs := []float64{500}

	sphereRain := computeDiffuseRain(reflectionPoint, surfaceNormal, energy, scattering, sphere, nil, centerFreqs, 0.0, 343.0)
	surfaceRain := computeSurfaceRain(reflectionPoint, surfaceNormal, energy, scattering, surface, nil, centerFreqs, 0.0, 343.0)

	if sphereRain == nil || surfaceRain == nil {
		t.Fatal("got nil rain")
	}

	// The formulas are different approximations but should agree within
	// an order of magnitude for small detectors at large distances.
	ratio := surfaceRain.BandEnergy[0] / sphereRain.BandEnergy[0]
	if ratio < 0.1 || ratio > 10 {
		t.Fatalf("surface/sphere ratio = %v, want within [0.1, 10]", ratio)
	}
}
