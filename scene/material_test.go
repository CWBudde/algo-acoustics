package scene_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/cwbudde/algo-acoustics/scene"
)

func TestMaterialUnmarshalSupportsScatteringField(t *testing.T) {
	data := []byte(`{
		"name": "plaster",
		"absorptionByBand": [0.1, 0.1, 0.15, 0.2, 0.2, 0.25],
		"scattering": [0.02, 0.03, 0.04, 0.06, 0.08, 0.1]
	}`)

	var material scene.Material

	err := json.Unmarshal(data, &material)
	if err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}

	wantScattering := [scene.NumBands]float64{0.02, 0.03, 0.04, 0.06, 0.08, 0.1}
	if material.Scattering != wantScattering {
		t.Fatalf("Scattering = %#v, want %#v", material.Scattering, wantScattering)
	}

	if !reflect.DeepEqual(material.ScatteringByBand, []float64{0.02, 0.03, 0.04, 0.06, 0.08, 0.1}) {
		t.Fatalf("ScatteringByBand = %#v, want six-band copy", material.ScatteringByBand)
	}
}

func TestMaterialScatteringRoundTrip(t *testing.T) {
	original := scene.Material{
		Name:             "brick",
		AbsorptionByBand: []float64{0.02, 0.02, 0.03, 0.04, 0.05, 0.07},
		Scattering:       [scene.NumBands]float64{0.03, 0.04, 0.06, 0.08, 0.1, 0.12},
	}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}

	var decoded scene.Material

	err = json.Unmarshal(encoded, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}

	if decoded.Name != original.Name {
		t.Fatalf("Name = %q, want %q", decoded.Name, original.Name)
	}

	if !reflect.DeepEqual(decoded.AbsorptionByBand, original.AbsorptionByBand) {
		t.Fatalf("AbsorptionByBand = %#v, want %#v", decoded.AbsorptionByBand, original.AbsorptionByBand)
	}

	if decoded.Scattering != original.Scattering {
		t.Fatalf("Scattering = %#v, want %#v", decoded.Scattering, original.Scattering)
	}
}

func TestEstimateScatteringFromDepthMonotonic(t *testing.T) {
	scattering := scene.EstimateScatteringFromDepth(0.1)
	for i := range scattering {
		if scattering[i] < 0 || scattering[i] > 1 {
			t.Fatalf("scattering[%d] = %f, want within [0, 1]", i, scattering[i])
		}

		if i > 0 && scattering[i] < scattering[i-1] {
			t.Fatalf("scattering[%d] = %f is smaller than scattering[%d] = %f", i, scattering[i], i-1, scattering[i-1])
		}
	}
}

func TestMaterialLibraryCoefficientsValid(t *testing.T) {
	for name, material := range scene.MaterialLibrary {
		// Check absorption coefficients
		if len(material.AbsorptionByBand) != scene.NumBands {
			t.Errorf("material %q: AbsorptionByBand has %d bands, want %d",
				name, len(material.AbsorptionByBand), scene.NumBands)

			continue
		}

		for i, alpha := range material.AbsorptionByBand {
			if alpha < 0 || alpha > 1 {
				t.Errorf("material %q: AbsorptionByBand[%d] = %f, want within [0, 1]",
					name, i, alpha)
			}
		}

		// Check scattering coefficients and monotonicity
		for i := 0; i < scene.NumBands; i++ {
			s := material.Scattering[i]
			if s < 0 || s > 1 {
				t.Errorf("material %q: Scattering[%d] = %f, want within [0, 1]",
					name, i, s)
			}

			// Scattering should be monotonically non-decreasing with frequency
			if i > 0 && s < material.Scattering[i-1] {
				t.Errorf("material %q: Scattering[%d] = %f is less than Scattering[%d] = %f (should be non-decreasing)",
					name, i, s, i-1, material.Scattering[i-1])
			}
		}
	}
}

func TestMaterialLibraryRetrievable(t *testing.T) {
	tests := []string{
		"glass",
		"painted_concrete",
		"exposed_brick",
		"carpet_on_concrete",
		"stage_curtain",
		"audience_seated",
		"qrd_diffuser",
		"bookshelf_dense",
	}

	for _, name := range tests {
		mat := scene.Material{}

		retrieved, ok := mat.FromLibrary(name)
		if !ok {
			t.Errorf("FromLibrary(%q) returned false, want true", name)

			continue
		}

		if retrieved.Name != name {
			t.Errorf("FromLibrary(%q).Name = %q, want %q", name, retrieved.Name, name)
		}

		// Verify absorption data is present
		if len(retrieved.AbsorptionByBand) == 0 {
			t.Errorf("FromLibrary(%q): AbsorptionByBand is empty", name)
		}
	}

	// Test retrieval of non-existent material
	mat := scene.Material{}

	_, ok := mat.FromLibrary("nonexistent_material")
	if ok {
		t.Errorf("FromLibrary(\"nonexistent_material\") returned true, want false")
	}
}
