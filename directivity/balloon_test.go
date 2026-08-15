package directivity

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// testBalloonLevel is an arbitrary but deterministic level pattern that varies
// along every axis of the table, so an indexing mistake cannot go unnoticed.
func testBalloonLevel(band, azIndex, elIndex int) float64 {
	return -float64(band) - 2*float64(azIndex) - 5*float64(elIndex)
}

func newTestBalloon(t *testing.T, mode InterpolationMode) *BalloonDirectivity {
	t.Helper()

	bands := []float64{125, 500, 2000}
	grid := SphericalGrid{AzimuthCount: 8, ElevationCount: 5}
	levels := make([]float64, len(bands)*grid.PointCount())

	for band := range bands {
		for elIndex := range grid.ElevationCount {
			for azIndex := range grid.AzimuthCount {
				levels[band*grid.PointCount()+grid.Index(azIndex, elIndex)] = testBalloonLevel(band, azIndex, elIndex)
			}
		}
	}

	balloon, err := NewBalloonDirectivity(bands, grid, mode, levels)
	if err != nil {
		t.Fatalf("NewBalloonDirectivity() error = %v", err)
	}

	return balloon
}

func TestSphericalGridValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		grid   SphericalGrid
		wantOK bool
	}{
		{name: "valid", grid: SphericalGrid{AzimuthCount: 8, ElevationCount: 5}, wantOK: true},
		{name: "minimal", grid: SphericalGrid{AzimuthCount: 1, ElevationCount: 2}, wantOK: true},
		{name: "no azimuths", grid: SphericalGrid{AzimuthCount: 0, ElevationCount: 5}},
		{name: "single elevation", grid: SphericalGrid{AzimuthCount: 8, ElevationCount: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.grid.Validate()
			if (err == nil) != tt.wantOK {
				t.Fatalf("Validate() error = %v, want ok = %v", err, tt.wantOK)
			}
		})
	}
}

func TestSphericalGridDirectionRoundTrip(t *testing.T) {
	t.Parallel()

	grid := SphericalGrid{AzimuthCount: 12, ElevationCount: 7}

	for elIndex := range grid.ElevationCount {
		for azIndex := range grid.AzimuthCount {
			dir := grid.DirectionAt(azIndex, elIndex)

			if length := dir.Norm(); math.Abs(length-1) > 1e-12 {
				t.Fatalf("DirectionAt(%d, %d) length = %v, want 1", azIndex, elIndex, length)
			}

			elevation, _ := directionToAngles(dir)
			if want := grid.Elevation(elIndex); math.Abs(elevation-want) > 1e-12 {
				t.Fatalf("DirectionAt(%d, %d) elevation = %v, want %v", azIndex, elIndex, elevation, want)
			}
		}
	}

	// The grid is built on the same convention as the GLL adapter: +X is on-axis.
	onAxis := grid.DirectionAt(0, (grid.ElevationCount-1)/2)
	if math.Abs(onAxis.X-1) > 1e-12 {
		t.Fatalf("equator column 0 = %v, want +X", onAxis)
	}
}

func TestBalloonGridPointExactness(t *testing.T) {
	t.Parallel()

	for _, mode := range []InterpolationMode{NearestNeighbor, Bilinear} {
		balloon := newTestBalloon(t, mode)

		for band, freq := range balloon.Bands {
			for elIndex := range balloon.Grid.ElevationCount {
				for azIndex := range balloon.Grid.AzimuthCount {
					dir := balloon.Grid.DirectionAt(azIndex, elIndex)
					want := math.Pow(10, testBalloonLevel(band, azIndex, elIndex)/20)

					got := balloon.GainLinear(freq, dir)
					if math.Abs(got-want) > 1e-9*want {
						t.Fatalf("mode %v: GainLinear(%v, az %d, el %d) = %v, want %v",
							mode, freq, azIndex, elIndex, got, want)
					}
				}
			}
		}
	}
}

func TestBalloonNearestNeighborSnaps(t *testing.T) {
	t.Parallel()

	balloon := newTestBalloon(t, NearestNeighbor)
	grid := balloon.Grid

	// A quarter step past column 1 must still read column 1.
	azimuth := grid.Azimuth(1) + 0.25*(grid.Azimuth(1)-grid.Azimuth(0))
	elevation := grid.Elevation(2)
	dir := geometry.Vec3{
		X: math.Cos(elevation) * math.Cos(azimuth),
		Y: math.Cos(elevation) * math.Sin(azimuth),
		Z: math.Sin(elevation),
	}

	want := math.Pow(10, testBalloonLevel(0, 1, 2)/20)
	if got := balloon.GainLinear(125, dir); math.Abs(got-want) > 1e-9*want {
		t.Fatalf("GainLinear() = %v, want the column-1 value %v", got, want)
	}
}

func TestBalloonBilinearBlendsNeighbors(t *testing.T) {
	t.Parallel()

	balloon := newTestBalloon(t, Bilinear)
	grid := balloon.Grid

	// Halfway between azimuth columns 1 and 2 on an exact elevation row.
	azimuth := 0.5 * (grid.Azimuth(1) + grid.Azimuth(2))
	elevation := grid.Elevation(3)
	dir := geometry.Vec3{
		X: math.Cos(elevation) * math.Cos(azimuth),
		Y: math.Cos(elevation) * math.Sin(azimuth),
		Z: math.Sin(elevation),
	}

	wantDB := 0.5 * (testBalloonLevel(0, 1, 3) + testBalloonLevel(0, 2, 3))
	want := math.Pow(10, wantDB/20)

	if got := balloon.GainLinear(125, dir); math.Abs(got-want) > 1e-9*want {
		t.Fatalf("GainLinear() = %v, want the dB mean %v", got, want)
	}
}

func TestBalloonBilinearWrapsAzimuth(t *testing.T) {
	t.Parallel()

	balloon := newTestBalloon(t, Bilinear)
	grid := balloon.Grid

	// Halfway between the last column and column 0, crossing the wrap point.
	azimuth := 0.5 * (grid.Azimuth(grid.AzimuthCount-1) + grid.Azimuth(grid.AzimuthCount))
	elevation := grid.Elevation(1)
	dir := geometry.Vec3{
		X: math.Cos(elevation) * math.Cos(azimuth),
		Y: math.Cos(elevation) * math.Sin(azimuth),
		Z: math.Sin(elevation),
	}

	wantDB := 0.5 * (testBalloonLevel(0, grid.AzimuthCount-1, 1) + testBalloonLevel(0, 0, 1))
	want := math.Pow(10, wantDB/20)

	if got := balloon.GainLinear(125, dir); math.Abs(got-want) > 1e-9*want {
		t.Fatalf("GainLinear() = %v, want the wrapped dB mean %v", got, want)
	}
}

func TestBalloonInterpolatesAcrossBands(t *testing.T) {
	t.Parallel()

	balloon := newTestBalloon(t, Bilinear)
	dir := balloon.Grid.DirectionAt(2, 2)

	// The geometric mean of two band centres is halfway in log frequency.
	freq := math.Sqrt(balloon.Bands[0] * balloon.Bands[1])
	wantDB := 0.5 * (testBalloonLevel(0, 2, 2) + testBalloonLevel(1, 2, 2))
	want := math.Pow(10, wantDB/20)

	if got := balloon.GainLinear(freq, dir); math.Abs(got-want) > 1e-9*want {
		t.Fatalf("GainLinear(%v) = %v, want the interpolated %v", freq, got, want)
	}

	// Outside the table the nearest band is held rather than extrapolated.
	lowWant := math.Pow(10, testBalloonLevel(0, 2, 2)/20)
	if got := balloon.GainLinear(10, dir); math.Abs(got-lowWant) > 1e-9*lowWant {
		t.Fatalf("below-table GainLinear() = %v, want %v", got, lowWant)
	}

	highWant := math.Pow(10, testBalloonLevel(len(balloon.Bands)-1, 2, 2)/20)
	if got := balloon.GainLinear(96000, dir); math.Abs(got-highWant) > 1e-9*highWant {
		t.Fatalf("above-table GainLinear() = %v, want %v", got, highWant)
	}
}

func TestBalloonDegenerate(t *testing.T) {
	t.Parallel()

	var nilBalloon *BalloonDirectivity
	if got := nilBalloon.GainLinear(1000, geometry.Vec3{X: 1}); got != 1 {
		t.Fatalf("nil balloon GainLinear() = %v, want 1", got)
	}

	balloon := newTestBalloon(t, Bilinear)
	if got := balloon.GainLinear(1000, geometry.Vec3Zero); got != 0 {
		t.Fatalf("zero-direction GainLinear() = %v, want 0", got)
	}
}

func TestNewBalloonDirectivityValidation(t *testing.T) {
	t.Parallel()

	grid := SphericalGrid{AzimuthCount: 4, ElevationCount: 3}
	full := make([]float64, 2*grid.PointCount())

	tests := []struct {
		name   string
		bands  []float64
		grid   SphericalGrid
		levels []float64
		wantOK bool
	}{
		{name: "valid", bands: []float64{125, 250}, grid: grid, levels: full, wantOK: true},
		{name: "no bands", bands: nil, grid: grid, levels: nil},
		{name: "bad grid", bands: []float64{125}, grid: SphericalGrid{}, levels: nil},
		{name: "descending bands", bands: []float64{250, 125}, grid: grid, levels: full},
		{name: "non-positive band", bands: []float64{0, 125}, grid: grid, levels: full},
		{name: "level count mismatch", bands: []float64{125, 250}, grid: grid, levels: full[:1]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewBalloonDirectivity(tt.bands, tt.grid, Bilinear, tt.levels)
			if (err == nil) != tt.wantOK {
				t.Fatalf("NewBalloonDirectivity() error = %v, want ok = %v", err, tt.wantOK)
			}
		})
	}
}

func TestSampleBalloonReproducesSourceModel(t *testing.T) {
	t.Parallel()

	source, err := NewFrequencyDependentCardioid(
		geometry.Vec3{X: 1}, []float64{125, 1000, 8000}, []float64{0.5, 2, 5},
	)
	if err != nil {
		t.Fatalf("NewFrequencyDependentCardioid() error = %v", err)
	}

	bands := []float64{125, 1000, 8000}
	grid := SphericalGrid{AzimuthCount: 36, ElevationCount: 19}

	balloon, err := SampleBalloon(source, bands, grid, Bilinear)
	if err != nil {
		t.Fatalf("SampleBalloon() error = %v", err)
	}

	// At tabulated directions and band centres the balloon must reproduce the
	// model it was sampled from.
	for _, freq := range bands {
		for elIndex := range grid.ElevationCount {
			for azIndex := range grid.AzimuthCount {
				dir := grid.DirectionAt(azIndex, elIndex)
				want := source.GainLinear(freq, dir)

				got := balloon.GainLinear(freq, dir)

				// Gains at or below the dB floor are stored as the floor, so
				// all the table promises there is that they stay silent.
				// Everything above it must round-trip exactly.
				if floor := math.Pow(10, minBalloonLevelDB/20); want <= floor {
					if got > floor*(1+1e-9) {
						t.Fatalf("GainLinear(%v, az %d, el %d) = %v, want no louder than the %v floor",
							freq, azIndex, elIndex, got, floor)
					}

					continue
				}

				if math.Abs(got-want) > 1e-9*want {
					t.Fatalf("GainLinear(%v, az %d, el %d) = %v, want %v", freq, azIndex, elIndex, got, want)
				}
			}
		}
	}

	// Between grid points the tabulated balloon tracks the analytic model
	// closely, which is what makes the reduction usable in place of it.
	offGrid := geometry.Vec3{X: 0.6, Y: 0.5, Z: 0.3}
	want := source.GainLinear(1000, offGrid)

	if got := balloon.GainLinear(1000, offGrid); math.Abs(got-want) > 0.01 {
		t.Fatalf("off-grid GainLinear() = %v, want %v within 0.01", got, want)
	}
}

func TestSampleBalloonErrors(t *testing.T) {
	t.Parallel()

	grid := SphericalGrid{AzimuthCount: 4, ElevationCount: 3}

	_, err := SampleBalloon(nil, []float64{125}, grid, Bilinear)
	if err == nil {
		t.Fatal("SampleBalloon() with a nil model succeeded")
	}

	_, err = SampleBalloon(OmniModel{}, []float64{125}, SphericalGrid{}, Bilinear)
	if err == nil {
		t.Fatal("SampleBalloon() with an invalid grid succeeded")
	}
}

func TestSampleBalloonFloorsNulls(t *testing.T) {
	t.Parallel()

	// A first-order cardioid has an exact null at the rear, which has no dB
	// representation; the table floors it instead of storing -Inf.
	source := CardioidModel{Axis: geometry.Vec3{X: 1}, OrderN: 1}
	grid := SphericalGrid{AzimuthCount: 4, ElevationCount: 3}

	balloon, err := SampleBalloon(source, []float64{1000}, grid, Bilinear)
	if err != nil {
		t.Fatalf("SampleBalloon() error = %v", err)
	}

	for index, level := range balloon.LevelsDB {
		if math.IsInf(level, 0) || math.IsNaN(level) {
			t.Fatalf("level %d = %v, want a finite dB value", index, level)
		}

		if level < minBalloonLevelDB {
			t.Fatalf("level %d = %v, want it floored at %v", index, level, minBalloonLevelDB)
		}
	}

	rear := balloon.GainLinear(1000, geometry.Vec3{X: -1})
	if rear > 1e-9 {
		t.Fatalf("rear gain = %v, want it floored near zero", rear)
	}
}
