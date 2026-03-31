package directivity

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	ggll "github.com/cwbudde/gll-tools/pkg/gll"
)

type fakeBalloon struct {
	onAxis  *ggll.TransferFunction
	offAxis *ggll.TransferFunction
}

func (f fakeBalloon) GetResponseAtAngle(theta, phi float64) *ggll.TransferFunction {
	if math.Abs(theta) < 1e-9 && math.Abs(phi) < 1e-9 {
		return f.onAxis
	}

	return f.offAxis
}

func TestLoadGLLMissingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.gll")

	_, err := LoadGLL(path, "")
	if err == nil {
		t.Fatal("LoadGLL() succeeded for missing file")
	}
}

func TestGLLModelGainLinearFavorsOnAxis(t *testing.T) {
	t.Parallel()

	definition := ggll.LogSpectrumDefinition{BandsPerOctave: 1, StartFreq: 125, PointCount: 4}
	model := &GLLModel{
		balloon: fakeBalloon{
			onAxis: &ggll.TransferFunction{
				Definition: definition,
				Level:      []float64{0, 0, 0, 0},
			},
			offAxis: &ggll.TransferFunction{
				Definition: definition,
				Level:      []float64{-6, -6, -6, -6},
			},
		},
	}

	onAxis := model.GainLinear(1000, geometry.Vec3{X: 1, Y: 0, Z: 0})
	offAxis := model.GainLinear(1000, geometry.Vec3{X: 0, Y: 1, Z: 0})

	if onAxis < offAxis {
		t.Fatalf("on-axis gain %v should be >= off-axis gain %v", onAxis, offAxis)
	}
}

func TestGLLModelGainLinearZeroDirection(t *testing.T) {
	t.Parallel()

	model := &GLLModel{balloon: fakeBalloon{}}
	if got := model.GainLinear(1000, geometry.Vec3Zero); got != 0 {
		t.Fatalf("GainLinear() = %v, want 0", got)
	}
}
