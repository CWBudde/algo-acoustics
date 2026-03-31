package directivity

import (
	"math"
	"testing"

	ggll "github.com/cwbudde/gll-tools/pkg/gll"

	"github.com/cwbudde/algo-acoustics/geometry"
)

type anglePatternBalloon struct{}

func (anglePatternBalloon) GetResponseAtAngle(theta, phi float64) *ggll.TransferFunction {
	_ = theta

	levelDB := patternLevelDB(phi)
	definition := ggll.LogSpectrumDefinition{BandsPerOctave: 1, StartFreq: 125, PointCount: 1}
	return &ggll.TransferFunction{Definition: definition, Level: []float64{levelDB}}
}

func patternLevelDB(phi float64) float64 {
	normalized := math.Mod(phi, 2*math.Pi)
	if normalized < 0 {
		normalized += 2 * math.Pi
	}

	switch {
	case almostAngle(normalized, 0):
		return 0
	case almostAngle(normalized, math.Pi/2):
		return -18
	case almostAngle(normalized, math.Pi):
		return -12
	case almostAngle(normalized, 3*math.Pi/2):
		return -6
	default:
		return -24
	}
}

func almostAngle(got, want float64) bool {
	return math.Abs(got-want) <= 1e-9
}

func TestGLLModelTracksSourceRotationAcrossQuadrants(t *testing.T) {
	t.Parallel()

	model := &GLLModel{balloon: anglePatternBalloon{}}
	rotations := []struct {
		name     string
		angle    float64
		wantGain float64
	}{
		{name: "0deg", angle: 0, wantGain: 1},
		{name: "90deg", angle: math.Pi / 2, wantGain: math.Pow(10, -6.0/20.0)},
		{name: "180deg", angle: math.Pi, wantGain: math.Pow(10, -12.0/20.0)},
		{name: "270deg", angle: 3 * math.Pi / 2, wantGain: math.Pow(10, -18.0/20.0)},
	}

	for _, tc := range rotations {
		t.Run(tc.name, func(t *testing.T) {
			direction := geometry.Vec3{X: math.Cos(tc.angle), Y: math.Sin(tc.angle), Z: 0}
			got := model.GainLinear(1000, direction)
			if math.Abs(got-tc.wantGain) > 1e-12 {
				t.Fatalf("gain = %v, want %v for rotation %v", got, tc.wantGain, tc.angle)
			}
		})
	}
}
