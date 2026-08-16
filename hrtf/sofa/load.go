// Package sofa loads measured HRTFs from SOFA (AES69) files into the
// measurement grids the hrtf package renders with.
//
// It is a separate package on purpose. The SOFA reader pulls in an HDF5
// implementation, and the linker only includes that in binaries that import
// this package — so the WASM demo and the roomir CLI, which do not, pay
// nothing for it.
package sofa

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	gosofa "github.com/cwbudde/go-sofa"
)

// Conventions whose positions are spherical by definition, used only when a
// file omits the Type attribute that would say so outright.
var sphericalByConvention = map[string]bool{
	"simplefreefieldhrir": true,
	"simplefreefieldhrtf": true,
}

// Load reads a SOFA file into a nearest-neighbor HRTF dataset.
//
// The file must hold time-domain impulse responses (DataType "FIR") for
// exactly two receivers, which is what SimpleFreeFieldHRIR and the measured
// datasets built on it — CIPIC, LISTEN, ARI — provide. Frequency-domain and
// second-order-section files are rejected rather than approximated.
func Load(path string) (*hrtf.NearestNeighborDataset, error) {
	file, err := gosofa.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open SOFA file %q: %w", path, err)
	}
	defer file.Close()

	dataset, err := datasetFromFile(file)
	if err != nil {
		return nil, fmt.Errorf("read SOFA file %q: %w", path, err)
	}

	return dataset, nil
}

// datasetFromFile converts an open SOFA file into a dataset. It is separate
// from Load so that tests can drive it with hand-built files.
func datasetFromFile(file *gosofa.File) (*hrtf.NearestNeighborDataset, error) {
	err := checkRenderable(file)
	if err != nil {
		return nil, err
	}

	sampleRate, err := sampleRateHz(file)
	if err != nil {
		return nil, err
	}

	leftIndex, rightIndex := earIndices(file)

	dirs, err := measurementDirections(file)
	if err != nil {
		return nil, err
	}

	grid := &hrtf.MeasurementGrid{
		Directions: dirs,
		LeftHRIRs:  make([][]float64, file.M),
		RightHRIRs: make([][]float64, file.M),
		Delays:     make([]float64, file.M),
	}

	for m := range file.M {
		// Clone: go-sofa reshapes one flat backing array into [M][R][N], so
		// the per-measurement slices alias each other.
		left := cloneSamples(file.ImpulseResponses[m][leftIndex])
		right := cloneSamples(file.ImpulseResponses[m][rightIndex])

		leftDelay := delaySeconds(file, m, leftIndex, sampleRate)
		rightDelay := delaySeconds(file, m, rightIndex, sampleRate)
		common := math.Min(leftDelay, rightDelay)

		// Lookup returns one delay for both ears, so only the common part can
		// travel as a delay. The per-ear excess becomes leading zeros in that
		// ear's HRIR, which is what preserves the ITD.
		grid.LeftHRIRs[m] = padFront(left, (leftDelay-common)*float64(sampleRate))
		grid.RightHRIRs[m] = padFront(right, (rightDelay-common)*float64(sampleRate))
		grid.Delays[m] = common
	}

	return &hrtf.NearestNeighborDataset{SampleRateHz: sampleRate, Grid: grid}, nil
}

// checkRenderable rejects files this package cannot turn into a binaural
// measurement grid, naming what the file actually contains.
func checkRenderable(file *gosofa.File) error {
	if file.DataType != "FIR" {
		return fmt.Errorf(
			"DataType is %q, want \"FIR\": only time-domain impulse responses can be loaded, "+
				"not frequency-domain or filter-coefficient data",
			file.DataType,
		)
	}

	if file.R != 2 {
		return fmt.Errorf("file has %d receivers, want 2: binaural rendering needs one HRIR per ear", file.R)
	}

	if file.M < 1 {
		return errors.New("file has no measurements")
	}

	if len(file.ImpulseResponses) != file.M {
		return fmt.Errorf("file declares %d measurements but carries %d impulse responses",
			file.M, len(file.ImpulseResponses))
	}

	return nil
}

// sampleRateHz returns the file's sample rate, rejecting anything that cannot
// be represented as the integer rate the rest of the pipeline uses.
func sampleRateHz(file *gosofa.File) (int, error) {
	// AES69 requires Data.SamplingRate for FIR data, but files in the wild
	// omit it, and there is no safe default: guessing mistimes every HRIR.
	if len(file.SamplingRate) == 0 {
		return 0, errors.New("file has no Data.SamplingRate, so its sample rate is unknown")
	}

	rate := file.SamplingRateScalar()
	if rate <= 0 {
		return 0, fmt.Errorf("sample rate is %v, want a positive value", rate)
	}

	// A per-measurement rate is legal in SOFA but has no representation in a
	// single dataset, so require the measurements to agree.
	for i, other := range file.SamplingRate {
		if other != rate {
			return 0, fmt.Errorf("sample rate varies across measurements (%v at index 0, %v at index %d)",
				rate, other, i)
		}
	}

	if rate != math.Trunc(rate) {
		return 0, fmt.Errorf("sample rate %v is not a whole number of Hz", rate)
	}

	return int(rate), nil
}

// earIndices returns the receiver indices of the left and right ears. SOFA
// orders receivers left, right; where receiver positions are present they
// settle it outright, since a file may store them either way round.
func earIndices(file *gosofa.File) (left, right int) {
	if len(file.ReceiverPositions) != 2 {
		return 0, 1
	}

	first := file.ReceiverPositions[0]

	switch file.ReceiverPositionType {
	case gosofa.CoordinateCartesian:
		// +Y is left in the SOFA listener frame.
		if first.Y < 0 {
			return 1, 0
		}
	case gosofa.CoordinateSpherical:
		// Azimuth is measured counter-clockwise from front, so the left ear
		// lies between 0 and 180 degrees.
		if azimuth := normalizeDegrees(first.X); azimuth > 180 {
			return 1, 0
		}
	}

	return 0, 1
}

// measurementDirections converts each measurement's source position into a
// unit vector in the receiver's head frame: azimuth from +X in the XY plane,
// elevation toward +Z, matching the convention the rest of the library uses.
func measurementDirections(file *gosofa.File) ([]geometry.Vec3, error) {
	if len(file.SourcePositions) == 0 {
		return nil, errors.New("file has no source positions")
	}

	// SOFA permits a scalar position that applies to every measurement.
	if len(file.SourcePositions) != file.M && len(file.SourcePositions) != 1 {
		return nil, fmt.Errorf("file declares %d measurements but carries %d source positions",
			file.M, len(file.SourcePositions))
	}

	spherical, err := positionsAreSpherical(file)
	if err != nil {
		return nil, err
	}

	radians := isRadians(file.SourcePositionUnits)

	out := make([]geometry.Vec3, file.M)
	for m := range file.M {
		position := file.SourcePositions[0]
		if len(file.SourcePositions) > 1 {
			position = file.SourcePositions[m]
		}

		if spherical {
			out[m] = fromSpherical(position.X, position.Y, radians)
			continue
		}

		direction := geometry.Vec3{X: position.X, Y: position.Y, Z: position.Z}.Normalize()
		if direction == geometry.Vec3Zero {
			return nil, fmt.Errorf("source position %d is at the origin, giving no direction", m)
		}

		out[m] = direction
	}

	return out, nil
}

// positionsAreSpherical decides how to read SourcePositions. A file that names
// its coordinate system is taken at its word; one that does not is only
// assumed spherical for conventions that mandate it, because reading spherical
// data as cartesian misplaces every measurement without any visible failure.
func positionsAreSpherical(file *gosofa.File) (bool, error) {
	switch file.SourcePositionType {
	case gosofa.CoordinateSpherical:
		return true, nil
	case gosofa.CoordinateCartesian:
		return false, nil
	case "":
		if sphericalByConvention[strings.ToLower(strings.TrimSpace(file.SOFAConventions))] {
			return true, nil
		}

		return false, fmt.Errorf(
			"SourcePosition has no Type attribute and convention %q does not imply one, "+
				"so its coordinate system is unknown",
			file.SOFAConventions,
		)
	default:
		return false, fmt.Errorf("SourcePosition Type is %q, want %q or %q",
			file.SourcePositionType, gosofa.CoordinateSpherical, gosofa.CoordinateCartesian)
	}
}

// fromSpherical converts an azimuth and elevation into a unit vector.
func fromSpherical(azimuth, elevation float64, radians bool) geometry.Vec3 {
	if !radians {
		azimuth *= math.Pi / 180
		elevation *= math.Pi / 180
	}

	cosElevation := math.Cos(elevation)

	return geometry.Vec3{
		X: cosElevation * math.Cos(azimuth),
		Y: cosElevation * math.Sin(azimuth),
		Z: math.Sin(elevation),
	}
}

// delaySeconds returns Data.Delay for one measurement and receiver.
//
// The field's shape has to be inferred from its length, and the candidates
// overlap: for a two-measurement binaural file, M, R, and a per-receiver array
// are all length 2. SOFA declares Data.Delay as [I][R] or [M][R] and never as
// [M] alone, so per-receiver is checked before per-measurement. The [M] case
// remains as a lenient fallback for files that stray from the spec.
func delaySeconds(file *gosofa.File, measurement, receiver, sampleRate int) float64 {
	var samples float64

	// Cases are evaluated in order, which is what makes the overlap resolve
	// the right way round.
	switch len(file.Delay) {
	case 0:
		return 0
	case 1:
		samples = file.Delay[0]
	case file.M * file.R:
		samples = file.Delay[measurement*file.R+receiver]
	case file.R:
		samples = file.Delay[receiver]
	case file.M:
		samples = file.Delay[measurement]
	default:
		return 0
	}

	return samples / float64(sampleRate)
}

// padFront prepends whole samples of silence. The sub-sample remainder is
// dropped: the measured datasets this targets carry their ITD inside the
// HRIRs with Data.Delay zero, so the rounding only touches files that encode
// delay separately.
func padFront(samples []float64, delaySamples float64) []float64 {
	pad := int(math.Round(delaySamples))
	if pad <= 0 {
		return samples
	}

	out := make([]float64, pad+len(samples))
	copy(out[pad:], samples)

	return out
}

func cloneSamples(samples []float64) []float64 {
	out := make([]float64, len(samples))
	copy(out, samples)

	return out
}

// isRadians reports whether a Units attribute names radians. SOFA spells the
// units as a triple such as "degree, degree, metre", so only the leading
// angular unit matters.
func isRadians(units string) bool {
	return strings.HasPrefix(strings.TrimSpace(units), "radian")
}

func normalizeDegrees(degrees float64) float64 {
	wrapped := math.Mod(degrees, 360)
	if wrapped < 0 {
		wrapped += 360
	}

	return wrapped
}
