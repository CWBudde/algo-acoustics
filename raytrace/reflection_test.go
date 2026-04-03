package raytrace

import (
	"math"
	"testing"
)

func TestSplitReflectionEnergyConservesBands(t *testing.T) {
	t.Parallel()

	in := []float64{1, 2, 3}
	absorption := []float64{0.25, 0.5, 0.75}
	scattering := []float64{0.2, 0.4, 0.6}

	specular, diffuse, remaining := splitReflectionEnergy(in, absorption, scattering)

	wantSpecular := []float64{0.6, 0.6, 0.3}
	wantDiffuse := []float64{0.15, 0.4, 0.45}
	wantRemaining := []float64{0.75, 1, 0.75}

	for i := range wantSpecular {
		if diff := math.Abs(specular[i] - wantSpecular[i]); diff > 1e-12 {
			t.Fatalf("specular[%d] = %g, want %g", i, specular[i], wantSpecular[i])
		}

		if diff := math.Abs(diffuse[i] - wantDiffuse[i]); diff > 1e-12 {
			t.Fatalf("diffuse[%d] = %g, want %g", i, diffuse[i], wantDiffuse[i])
		}

		if diff := math.Abs(remaining[i] - wantRemaining[i]); diff > 1e-12 {
			t.Fatalf("remaining[%d] = %g, want %g", i, remaining[i], wantRemaining[i])
		}

		if diff := math.Abs((specular[i] + diffuse[i]) - remaining[i]); diff > 1e-12 {
			t.Fatalf("band %d does not conserve energy: spec+diff=%g remaining=%g", i, specular[i]+diffuse[i], remaining[i])
		}
	}
}
