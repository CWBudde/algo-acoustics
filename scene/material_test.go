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
	if err := json.Unmarshal(encoded, &decoded); err != nil {
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
