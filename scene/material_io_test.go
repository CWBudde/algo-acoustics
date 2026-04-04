package scene_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/scene"
)

func TestLoadMaterialFileJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "glass.json")

	err := os.WriteFile(path, []byte(`{
  "name": "glass",
  "absorptionByBand": [0.03, 0.03, 0.04, 0.04, 0.05, 0.05],
  "scatteringByBand": [0.05, 0.05, 0.06, 0.08, 0.10, 0.12]
}`), 0o600)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	material, err := scene.LoadMaterialFile(path)
	if err != nil {
		t.Fatalf("LoadMaterialFile() error = %v", err)
	}

	if material.Name != "glass" {
		t.Fatalf("Name = %q, want glass", material.Name)
	}

	if got, want := len(material.AbsorptionByBand), scene.NumBands; got != want {
		t.Fatalf("AbsorptionByBand length = %d, want %d", got, want)
	}

	if material.AbsorptionByBand[0] != 0.03 || material.ScatteringByBand[5] != 0.12 {
		t.Fatalf("loaded material has wrong coefficients: %#v", material)
	}
}

func TestLoadMaterialFileCSV(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plasterboard.csv")

	err := os.WriteFile(path, []byte(strings.TrimSpace(`
band,center_hz,absorption,scattering
0,125,0.08,0.04
1,250,0.09,0.05
2,500,0.12,0.06
3,1000,0.15,0.08
4,2000,0.12,0.10
5,4000,0.10,0.12
`)), 0o600)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	material, err := scene.LoadMaterialFile(path)
	if err != nil {
		t.Fatalf("LoadMaterialFile() error = %v", err)
	}

	if material.Name != "plasterboard" {
		t.Fatalf("Name = %q, want plasterboard", material.Name)
	}

	if material.AbsorptionByBand[3] != 0.15 {
		t.Fatalf("AbsorptionByBand[3] = %v, want 0.15", material.AbsorptionByBand[3])
	}

	if material.Scattering[5] != 0.12 {
		t.Fatalf("Scattering[5] = %v, want 0.12", material.Scattering[5])
	}
}
