package directivity

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	ggll "github.com/cwbudde/gll-tools/pkg/gll"
)

func gllFixturePath() string {
	return filepath.Join("..", "testdata", "gll", "synthetic_ls.gll")
}

// The fixture is measured with Quarter symmetry, so its horizontal pattern has
// to come back four-fold symmetric about the 0°/90° axes. Reading the balloon
// through the gll-tools grid helpers without repairSymmetryCode unfolds the
// meridian with the Vertical rule instead and collapses everything past 90°
// onto a clamped value (0.885 at 135°, 180° and 225° alike) — wrong, and silent.
func TestGLLModelGainLinearHonoursQuarterSymmetry(t *testing.T) {
	t.Parallel()

	model, err := LoadGLL(gllFixturePath(), "")
	if err != nil {
		t.Fatalf("LoadGLL() error = %v", err)
	}

	if model.Symmetry != ggll.SymmetryQuarter {
		t.Fatalf("fixture symmetry = %s, want Quarter", model.Symmetry)
	}

	horizontalGain := func(deg float64) float64 {
		rad := deg * math.Pi / 180

		return model.GainLinear(1000, geometry.Vec3{X: math.Cos(rad), Y: math.Sin(rad), Z: 0})
	}

	// Distinct meridians inside the measured quadrant, so the mirror checks
	// below cannot pass by everything collapsing onto one value.
	for _, deg := range []float64{45, 90} {
		if math.Abs(horizontalGain(deg)-horizontalGain(0)) < 1e-6 {
			t.Fatalf("gain(%.0f°) = gain(0°) = %v, want a direction-dependent pattern", deg, horizontalGain(0))
		}
	}

	for _, deg := range []float64{0, 30, 45, 90} {
		want := horizontalGain(deg)

		for _, mirror := range []float64{180 - deg, 180 + deg, 360 - deg} {
			got := horizontalGain(mirror)
			if math.Abs(got-want) > 1e-9 {
				t.Fatalf("gain(%.0f°) = %v, want %v from quarter mirror of %.0f°", mirror, got, want, deg)
			}
		}
	}
}

// TestLoadGLLHydratesBalloonResponses guards the loader against regressing to
// the state where the balloon parsed but its measurements never did. gll-tools
// defers response loading to a file offset, so closing the file before reading
// them leaves a model that silently radiates omnidirectionally.
func TestLoadGLLHydratesBalloonResponses(t *testing.T) {
	t.Parallel()

	model, err := LoadGLL(gllFixturePath(), "")
	if err != nil {
		t.Fatalf("LoadGLL() error = %v", err)
	}

	balloon := model.SourceDefinition.BalloonData
	if balloon.ResponseCount <= 0 {
		t.Fatalf("fixture declares %d responses, want a positive count", balloon.ResponseCount)
	}

	if got, want := len(balloon.Responses), int(balloon.ResponseCount); got != want {
		t.Fatalf("loaded %d balloon responses, want %d", got, want)
	}
}

func TestGLLModelBandCenterFrequencies(t *testing.T) {
	t.Parallel()

	model, err := LoadGLL(gllFixturePath(), "")
	if err != nil {
		t.Fatalf("LoadGLL() error = %v", err)
	}

	bands := model.BandCenterFrequencies()
	if len(bands) < 2 {
		t.Fatalf("BandCenterFrequencies() returned %d bands, want a populated grid", len(bands))
	}

	for index := 1; index < len(bands); index++ {
		if bands[index] <= bands[index-1] {
			t.Fatalf("band %d frequency %v does not ascend past %v", index, bands[index], bands[index-1])
		}
	}

	// The measured range has to bracket the octave bands the renderer uses,
	// otherwise extraction would be pure extrapolation.
	if bands[0] > 125 || bands[len(bands)-1] < 4000 {
		t.Fatalf("measured range %v..%v Hz does not bracket 125..4000 Hz", bands[0], bands[len(bands)-1])
	}
}

func TestGLLModelExtractBalloonMatchesModel(t *testing.T) {
	t.Parallel()

	model, err := LoadGLL(gllFixturePath(), "")
	if err != nil {
		t.Fatalf("LoadGLL() error = %v", err)
	}

	bands := []float64{125, 250, 500, 1000, 2000, 4000}
	grid := SphericalGrid{AzimuthCount: 24, ElevationCount: 13}

	balloon, err := model.ExtractBalloon(bands, grid, Bilinear)
	if err != nil {
		t.Fatalf("ExtractBalloon() error = %v", err)
	}

	if got, want := len(balloon.LevelsDB), len(bands)*grid.PointCount(); got != want {
		t.Fatalf("extracted %d levels, want %d", got, want)
	}

	// The extracted table must reproduce the GLL model at the directions and
	// frequencies it was sampled at. Absolute levels are the library's to
	// define; what this pins is that extraction is faithful to the source.
	spread := 0.0

	for band, freq := range bands {
		for elIndex := range grid.ElevationCount {
			for azIndex := range grid.AzimuthCount {
				dir := grid.DirectionAt(azIndex, elIndex)

				want := model.GainLinear(freq, dir)
				got := balloon.GainLinear(freq, dir)

				if math.Abs(got-want) > 1e-9*math.Max(want, 1e-12) {
					t.Fatalf("band %v az %d el %d: extracted gain %v, want %v", freq, azIndex, elIndex, got, want)
				}

				spread = math.Max(spread, math.Abs(balloon.LevelsDB[band*grid.PointCount()+grid.Index(azIndex, elIndex)]))
			}
		}
	}

	// A loudspeaker balloon that is flat in every direction at every band means
	// the measurements never reached the model — the failure mode this whole
	// path exists to avoid.
	if spread < 1 {
		t.Fatalf("extracted balloon spans only %v dB, want a directional pattern", spread)
	}
}

func TestGLLModelExtractBalloonDefaultsToNativeBands(t *testing.T) {
	t.Parallel()

	model, err := LoadGLL(gllFixturePath(), "")
	if err != nil {
		t.Fatalf("LoadGLL() error = %v", err)
	}

	// A coarse grid keeps the native 1/24-octave extraction cheap.
	balloon, err := model.ExtractBalloon(nil, SphericalGrid{AzimuthCount: 4, ElevationCount: 3}, NearestNeighbor)
	if err != nil {
		t.Fatalf("ExtractBalloon() error = %v", err)
	}

	if got, want := len(balloon.Bands), len(model.BandCenterFrequencies()); got != want {
		t.Fatalf("extracted %d bands, want the native %d", got, want)
	}
}

func TestGLLModelExtractBalloonWithoutResponses(t *testing.T) {
	t.Parallel()

	// LoadGLLFile has no reader, so the deferred measurements cannot be read;
	// extraction must say so rather than return an omnidirectional table.
	file := &ggll.File{Database: &ggll.Database{SourceDefinitions: []ggll.SourceDefinitionItem{{
		Key: "main",
		Definition: &ggll.SourceDefinition{
			Label:       "Main Source",
			BalloonData: &ggll.BalloonData{ResponseCount: 4},
		},
	}}}}

	model, err := LoadGLLFile(file, "")
	if err != nil {
		t.Fatalf("LoadGLLFile() error = %v", err)
	}

	_, err = model.ExtractBalloon(nil, SphericalGrid{AzimuthCount: 4, ElevationCount: 3}, Bilinear)
	if err == nil {
		t.Fatal("ExtractBalloon() succeeded without loaded responses")
	}
}

func TestGLLModelExtractBalloonNilModel(t *testing.T) {
	t.Parallel()

	var model *GLLModel

	_, err := model.ExtractBalloon(nil, SphericalGrid{AzimuthCount: 4, ElevationCount: 3}, Bilinear)
	if err == nil {
		t.Fatal("ExtractBalloon() on a nil model succeeded")
	}
}
