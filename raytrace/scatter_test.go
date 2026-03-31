package raytrace

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// chiSquaredTest performs a chi-squared goodness-of-fit test.
// Returns the chi-squared statistic and the DoF.
func chiSquaredTest(observed, expected []float64) (float64, int) {
	var chi2 float64

	for i := range observed {
		diff := observed[i] - expected[i]
		chi2 += (diff * diff) / expected[i]
	}

	return chi2, len(observed) - 1
}

// criticalChiSquared returns the approximate critical value for chi-squared test.
// For DoF > 0, uses Choi's approximation valid for p=0.05 significance level.
// See: "The median of the chi-squared distribution" by Choi (2008).
func criticalChiSquared(dof int) float64 {
	if dof <= 0 {
		return 0
	}
	// Approximation for p=0.05: chi2_crit ≈ dof * (1 - 2/(9*dof) + 1.96*sqrt(2/(9*dof)))^3
	x := 1.0 - 2.0/(9.0*float64(dof)) + 1.96*math.Sqrt(2.0/(9.0*float64(dof)))

	return float64(dof) * x * x * x
}

func TestSpecularReflect(t *testing.T) {
	t.Parallel()

	reflected := SpecularReflect(geometry.Vec3{X: 1}, geometry.Vec3{X: 1})
	if reflected != (geometry.Vec3{X: -1}) {
		t.Fatalf("SpecularReflect() = %#v, want -X", reflected)
	}
}

func TestDiffuseReflectStaysInHemisphere(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))
	for range 64 {
		reflected := DiffuseReflect(geometry.Vec3{Z: 1}, rng)
		if reflected.Z <= 0 {
			t.Fatalf("DiffuseReflect() produced vector in wrong hemisphere: %#v", reflected)
		}

		if diff := math.Abs(reflected.Norm() - 1); diff > 1e-12 {
			t.Fatalf("DiffuseReflect() norm = %g, want 1", reflected.Norm())
		}
	}
}

func TestAbsorbedFractionClampsRange(t *testing.T) {
	t.Parallel()

	got := AbsorbedFraction([]float64{-0.25, 0.5, 1.25})

	want := []float64{1, 0.5, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %g, want %g", i, got[i], want[i])
		}
	}
}

// TestLambertDirectionStaysInHemisphere verifies direction stays in correct hemisphere.
func TestLambertDirectionStaysInHemisphere(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(42))
	normal := geometry.Vec3{Z: 1}

	for range 1000 {
		dir := LambertDirection(normal, rng)

		// Verify unit length
		norm := dir.Norm()
		if diff := math.Abs(norm - 1.0); diff > 1e-12 {
			t.Fatalf("LambertDirection() norm = %g, want 1.0", norm)
		}

		// Verify direction is in hemisphere of normal (dot product > 0)
		dot := dir.Dot(normal)
		if dot <= 0 {
			t.Fatalf("LambertDirection() = %#v, dot with normal = %g (should be > 0)", dir, dot)
		}
	}
}

// TestLambertDirectionDistribution verifies the angular distribution matches cos(theta) PDF.
// Uses 100k samples and chi-squared goodness-of-fit test at p=0.05 level.
func TestLambertDirectionDistribution(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(12345))
	normal := geometry.Vec3{Z: 1}

	const (
		numSamples = 100_000
		numBins    = 20
	)

	// Bin by cos(theta) = z value, since that's what determines the Lambert weighting
	histogramCounts := make([]float64, numBins)

	for range numSamples {
		dir := LambertDirection(normal, rng)
		cosTheta := dir.Z // dot product with Z-normal

		// cosTheta = sqrt(r1), so it ranges from 0 to 1
		// Map to bin 0..numBins-1
		bin := int(cosTheta * float64(numBins))
		if bin >= numBins {
			bin = numBins - 1
		}

		histogramCounts[bin]++
	}

	// Expected distribution for Lambert sampling where cos(theta) = sqrt(r1)
	// The PDF of cos(theta) is p(c) = 2c on [0, 1]
	// For bin i covering cos ∈ [i/n, (i+1)/n], expected count is:
	// numSamples * ∫_{i/n}^{(i+1)/n} 2c dc = numSamples * [c²]_{i/n}^{(i+1)/n}
	// = numSamples * ((i+1)²/n² - i²/n²) = numSamples * (2i + 1) / n²
	expected := make([]float64, numBins)
	for j := range numBins {
		binProb := float64(2*j+1) / float64(numBins*numBins)
		expected[j] = binProb * float64(numSamples)
	}

	// Chi-squared test
	chi2, dof := chiSquaredTest(histogramCounts, expected)
	chi2Crit := criticalChiSquared(dof)

	if chi2 > chi2Crit {
		t.Fatalf("Lambert distribution chi-squared = %g, critical = %g (DoF=%d, p=0.05): distribution does not match expected PDF",
			chi2, chi2Crit, dof)
	}
}

// TestLambertDirectionArbitraryNormal verifies correct behaviour for arbitrary normal vectors.
func TestLambertDirectionArbitraryNormal(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(99))

	v45 := geometry.Vec3{X: 1, Y: 1, Z: 0}.Normalize()
	v111 := geometry.Vec3{X: 1, Y: 1, Z: 1}.Normalize()

	testNormals := []geometry.Vec3{
		{X: 1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 0},
		{X: 0, Y: 0, Z: 1},
		v45,
		v111,
	}

	for _, normal := range testNormals {
		for range 100 {
			dir := LambertDirection(normal, rng)

			// Verify unit length
			if diff := math.Abs(dir.Norm() - 1.0); diff > 1e-12 {
				t.Fatalf("LambertDirection(normal=%#v) norm = %g, want 1.0", normal, dir.Norm())
			}

			// Verify direction is in correct hemisphere
			dot := dir.Dot(normal)
			if dot <= 0 {
				t.Fatalf("LambertDirection(normal=%#v) produced %#v, dot = %g (should be > 0)",
					normal, dir, dot)
			}
		}
	}
}

// BenchmarkLambertDirection measures Lambert sampling throughput.
func BenchmarkLambertDirection(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	normal := geometry.Vec3{Z: 1}

	b.ResetTimer()

	for range b.N {
		_ = LambertDirection(normal, rng)
	}
}
