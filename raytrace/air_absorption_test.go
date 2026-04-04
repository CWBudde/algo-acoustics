package raytrace

import (
	"math"
	"testing"
)

func TestAlphaAirISO9613_1ReferencePoint(t *testing.T) {
	t.Parallel()

	got := AlphaAirISO9613_1(4000, 20, 0.5)
	want := 0.19922361038201827

	if diff := math.Abs(got-want) / want; diff > 0.05 {
		t.Fatalf("AlphaAirISO9613_1(4kHz, 20C, 50%%RH) = %g, want %g within 5%%", got, want)
	}
}

func TestAirAbsorptionAttenuatesOverDistance(t *testing.T) {
	t.Parallel()

	energy := []float64{1}

	attenuated := attenuateEnergyByAir(energy, []float64{4000}, 50, 20, 0.5)
	if len(attenuated) != 1 {
		t.Fatalf("attenuated length = %d, want 1", len(attenuated))
	}

	attenuationDB := 10 * math.Log10(energy[0]/attenuated[0])

	want := 9.961180519100914
	if diff := math.Abs(attenuationDB-want) / want; diff > 0.05 {
		t.Fatalf("attenuation at 4kHz over 50m = %g dB, want %g dB within 5%%", attenuationDB, want)
	}
}
