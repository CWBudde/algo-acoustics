package directivity

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// InterpolationMode selects how BalloonDirectivity reads between grid points.
type InterpolationMode int

const (
	// NearestNeighbor snaps to the closest tabulated direction.
	NearestNeighbor InterpolationMode = iota
	// Bilinear blends the four surrounding directions.
	Bilinear
)

// minBalloonLevelDB floors tabulated levels so that a null in the pattern
// stays representable in dB instead of becoming negative infinity.
const minBalloonLevelDB = -200.0

// SphericalGrid is a regular sampling of directions in the source-local frame,
// using the same convention as the GLL adapter: azimuth is measured in the XY
// plane from +X, elevation from the XY plane towards +Z, and +X is on-axis.
type SphericalGrid struct {
	// AzimuthCount samples span [0, 360) degrees; the wrap point is implied.
	AzimuthCount int `json:"azimuth_count"`
	// ElevationCount samples span [-90, +90] degrees inclusive of both poles.
	ElevationCount int `json:"elevation_count"`
}

// Validate reports whether the grid can be sampled and interpolated.
func (g SphericalGrid) Validate() error {
	if g.AzimuthCount < 1 {
		return fmt.Errorf("azimuth count %d must be at least 1", g.AzimuthCount)
	}

	if g.ElevationCount < 2 {
		return fmt.Errorf("elevation count %d must be at least 2", g.ElevationCount)
	}

	return nil
}

// PointCount returns the number of directions in the grid.
func (g SphericalGrid) PointCount() int {
	return g.AzimuthCount * g.ElevationCount
}

// Index returns the flat offset of a grid point, elevation-major.
func (g SphericalGrid) Index(azIndex, elIndex int) int {
	return elIndex*g.AzimuthCount + azIndex
}

// Azimuth returns the azimuth of a column in radians.
func (g SphericalGrid) Azimuth(azIndex int) float64 {
	return 2 * math.Pi * float64(azIndex) / float64(g.AzimuthCount)
}

// Elevation returns the elevation of a row in radians.
func (g SphericalGrid) Elevation(elIndex int) float64 {
	return -math.Pi/2 + math.Pi*float64(elIndex)/float64(g.ElevationCount-1)
}

// DirectionAt returns the unit direction of a grid point. It is the exact
// inverse of the angle extraction used during lookup, so a direction produced
// here reads back the value stored at that point.
func (g SphericalGrid) DirectionAt(azIndex, elIndex int) geometry.Vec3 {
	azimuth := g.Azimuth(azIndex)
	elevation := g.Elevation(elIndex)

	return geometry.Vec3{
		X: math.Cos(elevation) * math.Cos(azimuth),
		Y: math.Cos(elevation) * math.Sin(azimuth),
		Z: math.Sin(elevation),
	}
}

// BalloonDirectivity is a directivity pattern tabulated on a spherical grid,
// one grid per frequency band. It is the general form the GLL adapter reduces
// to, and it can hold any measured or synthesised balloon.
//
// LevelsDB is band-major: band*Grid.PointCount() + Grid.Index(az, el), in dB
// relative to on-axis. Levels are interpolated in dB — the domain balloon data
// is published and interpolated in — and converted to linear gain on return.
type BalloonDirectivity struct {
	// Bands holds ascending centre frequencies in Hz.
	Bands []float64         `json:"bands"`
	Grid  SphericalGrid     `json:"grid"`
	Mode  InterpolationMode `json:"mode"`
	// LevelsDB holds one grid of levels per band.
	LevelsDB []float64 `json:"levels_db"`
}

// NewBalloonDirectivity validates the table and returns the model.
func NewBalloonDirectivity(
	bands []float64,
	grid SphericalGrid,
	mode InterpolationMode,
	levelsDB []float64,
) (*BalloonDirectivity, error) {
	if len(bands) == 0 {
		return nil, errors.New("balloon directivity has no bands")
	}

	err := grid.Validate()
	if err != nil {
		return nil, fmt.Errorf("balloon grid: %w", err)
	}

	previous := 0.0

	for index, freq := range bands {
		if freq <= 0 {
			return nil, fmt.Errorf("band %d frequency %v is not positive", index, freq)
		}

		if freq <= previous {
			return nil, fmt.Errorf("band %d frequency %v does not ascend past %v", index, freq, previous)
		}

		previous = freq
	}

	want := len(bands) * grid.PointCount()
	if len(levelsDB) != want {
		return nil, fmt.Errorf("balloon has %d levels, want %d for %d bands on a %dx%d grid",
			len(levelsDB), want, len(bands), grid.AzimuthCount, grid.ElevationCount)
	}

	return &BalloonDirectivity{
		Bands:    append([]float64(nil), bands...),
		Grid:     grid,
		Mode:     mode,
		LevelsDB: append([]float64(nil), levelsDB...),
	}, nil
}

// GainLinear returns the tabulated gain for the supplied direction, blended
// across the two bands bracketing freqHz and held flat outside the table.
func (m *BalloonDirectivity) GainLinear(freqHz float64, dir geometry.Vec3) float64 {
	if m == nil || len(m.Bands) == 0 || len(m.LevelsDB) == 0 {
		return 1
	}

	unitDir := dir.Normalize()
	if unitDir == geometry.Vec3Zero {
		return 0
	}

	elevation, azimuth := directionToAngles(unitDir)

	lower, upper, weight := bracketBand(m.Bands, freqHz)

	levelDB := m.levelAt(lower, azimuth, elevation)
	if upper != lower && weight > 0 {
		levelDB += weight * (m.levelAt(upper, azimuth, elevation) - levelDB)
	}

	return math.Pow(10, levelDB/20)
}

// levelAt reads one band's grid at the supplied angles, in dB.
func (m *BalloonDirectivity) levelAt(band int, azimuth, elevation float64) float64 {
	azPos := m.azimuthPosition(azimuth)
	elPos := m.elevationPosition(elevation)

	if m.Mode == NearestNeighbor {
		azIndex := int(math.Round(azPos)) % m.Grid.AzimuthCount
		elIndex := clampIndex(int(math.Round(elPos)), m.Grid.ElevationCount)

		return m.sample(band, azIndex, elIndex)
	}

	azLow := int(math.Floor(azPos))
	azFrac := azPos - float64(azLow)
	azIndex0 := ((azLow % m.Grid.AzimuthCount) + m.Grid.AzimuthCount) % m.Grid.AzimuthCount
	azIndex1 := (azIndex0 + 1) % m.Grid.AzimuthCount

	elLow := int(math.Floor(elPos))
	elFrac := elPos - float64(elLow)
	elIndex0 := clampIndex(elLow, m.Grid.ElevationCount)
	elIndex1 := clampIndex(elLow+1, m.Grid.ElevationCount)

	bottom := blend(m.sample(band, azIndex0, elIndex0), m.sample(band, azIndex1, elIndex0), azFrac)
	top := blend(m.sample(band, azIndex0, elIndex1), m.sample(band, azIndex1, elIndex1), azFrac)

	return blend(bottom, top, elFrac)
}

// azimuthPosition maps an azimuth in radians onto fractional column position.
func (m *BalloonDirectivity) azimuthPosition(azimuth float64) float64 {
	turns := azimuth / (2 * math.Pi)
	turns -= math.Floor(turns)

	return turns * float64(m.Grid.AzimuthCount)
}

// elevationPosition maps an elevation in radians onto fractional row position.
func (m *BalloonDirectivity) elevationPosition(elevation float64) float64 {
	normalized := (elevation + math.Pi/2) / math.Pi

	return normalized * float64(m.Grid.ElevationCount-1)
}

func (m *BalloonDirectivity) sample(band, azIndex, elIndex int) float64 {
	offset := band*m.Grid.PointCount() + m.Grid.Index(azIndex, elIndex)
	if offset < 0 || offset >= len(m.LevelsDB) {
		return minBalloonLevelDB
	}

	return m.LevelsDB[offset]
}

// SampleBalloon tabulates any directivity model onto a grid, producing a
// balloon that can be evaluated without the original model. This is how a
// measured GLL source is reduced to the octave bands the renderer uses.
func SampleBalloon(
	model Model,
	bands []float64,
	grid SphericalGrid,
	mode InterpolationMode,
) (*BalloonDirectivity, error) {
	if model == nil {
		return nil, errors.New("cannot sample a nil directivity model")
	}

	err := grid.Validate()
	if err != nil {
		return nil, fmt.Errorf("balloon grid: %w", err)
	}

	points := grid.PointCount()
	levels := make([]float64, len(bands)*points)

	for bandIndex, freq := range bands {
		for elIndex := range grid.ElevationCount {
			for azIndex := range grid.AzimuthCount {
				gain := model.GainLinear(freq, grid.DirectionAt(azIndex, elIndex))
				levels[bandIndex*points+grid.Index(azIndex, elIndex)] = levelToDB(gain)
			}
		}
	}

	return NewBalloonDirectivity(bands, grid, mode, levels)
}

// bracketBand returns the band indices surrounding freqHz and the blend weight
// towards the upper one, interpolating in log frequency.
func bracketBand(bands []float64, freqHz float64) (lower, upper int, weight float64) {
	last := len(bands) - 1
	if freqHz <= bands[0] {
		return 0, 0, 0
	}

	if freqHz >= bands[last] {
		return last, last, 0
	}

	upper = 1
	for upper < last && bands[upper] < freqHz {
		upper++
	}

	lower = upper - 1

	span := math.Log(bands[upper] / bands[lower])
	if span <= 0 {
		return lower, lower, 0
	}

	return lower, upper, math.Log(freqHz/bands[lower]) / span
}

func levelToDB(gain float64) float64 {
	if gain <= 0 {
		return minBalloonLevelDB
	}

	return math.Max(20*math.Log10(gain), minBalloonLevelDB)
}

func blend(a, b, weight float64) float64 {
	return a + weight*(b-a)
}

func clampIndex(index, count int) int {
	if index < 0 {
		return 0
	}

	if index >= count {
		return count - 1
	}

	return index
}
