package sofa

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	gosofa "github.com/cwbudde/go-sofa"
)

const testSampleRate = 48000

// cardinalDirections are the six axis directions the fixtures measure, paired
// with their spherical (azimuth, elevation) form in degrees.
var cardinalDirections = []struct {
	name      string
	unit      geometry.Vec3
	azimuth   float64
	elevation float64
}{
	{"front", geometry.Vec3{X: 1}, 0, 0},
	{"left", geometry.Vec3{Y: 1}, 90, 0},
	{"back", geometry.Vec3{X: -1}, 180, 0},
	{"right", geometry.Vec3{Y: -1}, 270, 0},
	{"up", geometry.Vec3{Z: 1}, 0, 90},
	{"down", geometry.Vec3{Z: -1}, 0, -90},
}

// fixture describes a SOFA file to synthesize for a test.
type fixture struct {
	convention   string
	dataType     string
	receivers    int
	samples      int
	sampleRate   float64
	positionType string
	units        string
	positions    []gosofa.Vector3
	receiverPos  []gosofa.Vector3
	receiverType string
	delay        []float64
	irs          [][][]float64
}

// sphericalFixture builds the default six-direction fixture in the given
// coordinate system.
func sphericalFixture() fixture {
	positions := make([]gosofa.Vector3, len(cardinalDirections))
	for i, d := range cardinalDirections {
		positions[i] = gosofa.Vector3{X: d.azimuth, Y: d.elevation, Z: 1.5}
	}

	return fixture{
		convention:   "SimpleFreeFieldHRIR",
		dataType:     "FIR",
		receivers:    2,
		samples:      4,
		sampleRate:   testSampleRate,
		positionType: gosofa.CoordinateSpherical,
		units:        gosofa.UnitsSphericalDegrees,
		positions:    positions,
		receiverPos:  []gosofa.Vector3{{Y: 0.09}, {Y: -0.09}},
		receiverType: gosofa.CoordinateCartesian,
	}
}

func cartesianFixture() fixture {
	f := sphericalFixture()
	f.positionType = gosofa.CoordinateCartesian
	f.units = gosofa.UnitsCartesianMetres

	f.positions = make([]gosofa.Vector3, len(cardinalDirections))
	for i, d := range cardinalDirections {
		// Scale by a radius to prove the loader normalizes.
		f.positions[i] = gosofa.Vector3{X: d.unit.X * 1.5, Y: d.unit.Y * 1.5, Z: d.unit.Z * 1.5}
	}

	return f
}

// build turns a fixture into a go-sofa File, filling in impulse responses that
// identify their measurement and ear if the fixture supplies none.
func (f fixture) build() *gosofa.File {
	measurements := len(f.positions)
	if measurements == 0 {
		measurements = 1
	}

	irs := f.irs
	if irs == nil {
		irs = make([][][]float64, measurements)
		for m := range irs {
			irs[m] = make([][]float64, f.receivers)
			for r := range irs[m] {
				samples := make([]float64, f.samples)
				// A distinct first sample per (measurement, ear) makes a
				// mis-mapped ear or measurement visible.
				samples[0] = float64(m+1) + float64(r+1)/10
				irs[m][r] = samples
			}
		}
	}

	return &gosofa.File{
		Conventions:            "SOFA",
		Version:                "1.0",
		SOFAConventions:        f.convention,
		SOFAConventionsVersion: "1.0",
		DataType:               f.dataType,
		M:                      measurements,
		R:                      f.receivers,
		E:                      1,
		N:                      f.samples,
		ImpulseResponses:       irs,
		SamplingRate:           []float64{f.sampleRate},
		Delay:                  f.delay,
		ListenerPositions:      []gosofa.Vector3{{}},
		ReceiverPositions:      f.receiverPos,
		ReceiverPositionType:   f.receiverType,
		SourcePositions:        f.positions,
		SourcePositionType:     f.positionType,
		SourcePositionUnits:    f.units,
		EmitterPositions:       []gosofa.Vector3{{}},
	}
}

// save writes the fixture to a temporary file and returns its path, so tests
// exercise the real Open path rather than only the in-memory conversion.
func (f fixture) save(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fixture.sofa")

	err := f.build().Save(path)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	return path
}

func TestLoadReadsCardinalDirections(t *testing.T) {
	tests := []struct {
		name    string
		fixture fixture
	}{
		{"spherical degrees", sphericalFixture()},
		{"cartesian", cartesianFixture()},
		{"spherical radians", func() fixture {
			f := sphericalFixture()
			//nolint:dupword // The SOFA units triple repeats the angular unit.
			f.units = "radian, radian, metre"

			f.positions = make([]gosofa.Vector3, len(cardinalDirections))
			for i, d := range cardinalDirections {
				f.positions[i] = gosofa.Vector3{
					X: d.azimuth * math.Pi / 180,
					Y: d.elevation * math.Pi / 180,
					Z: 1.5,
				}
			}

			return f
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataset, err := Load(tt.fixture.save(t))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if dataset.SampleRateHz != testSampleRate {
				t.Errorf("SampleRateHz = %d, want %d", dataset.SampleRateHz, testSampleRate)
			}

			if got := len(dataset.Grid.Directions); got != len(cardinalDirections) {
				t.Fatalf("got %d directions, want %d", got, len(cardinalDirections))
			}

			for i, want := range cardinalDirections {
				got := dataset.Grid.Directions[i]
				if !vecNear(got, want.unit, 1e-9) {
					t.Errorf("direction %d (%s) = %v, want %v", i, want.name, got, want.unit)
				}
			}
		})
	}
}

func TestLoadMapsEarsAndMeasurements(t *testing.T) {
	dataset, err := Load(sphericalFixture().save(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	for m := range cardinalDirections {
		wantLeft := float64(m+1) + 0.1
		wantRight := float64(m+1) + 0.2

		if got := dataset.Grid.LeftHRIRs[m][0]; math.Abs(got-wantLeft) > 1e-9 {
			t.Errorf("left HRIR %d starts at %v, want %v", m, got, wantLeft)
		}

		if got := dataset.Grid.RightHRIRs[m][0]; math.Abs(got-wantRight) > 1e-9 {
			t.Errorf("right HRIR %d starts at %v, want %v", m, got, wantRight)
		}
	}
}

// TestLoadHonorsReversedReceiverOrder checks that receiver positions decide
// which ear is which, since a file may store the right ear first.
func TestLoadHonorsReversedReceiverOrder(t *testing.T) {
	f := sphericalFixture()
	f.receiverPos = []gosofa.Vector3{{Y: -0.09}, {Y: 0.09}}

	dataset, err := Load(f.save(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Receiver 1 is now the left ear, so the left HRIR carries the .2 marker.
	if got := dataset.Grid.LeftHRIRs[0][0]; math.Abs(got-1.2) > 1e-9 {
		t.Errorf("left HRIR starts at %v, want 1.2 (receiver 1)", got)
	}

	if got := dataset.Grid.RightHRIRs[0][0]; math.Abs(got-1.1) > 1e-9 {
		t.Errorf("right HRIR starts at %v, want 1.1 (receiver 0)", got)
	}
}

// TestLoadBakesPerEarDelay checks that a per-ear Data.Delay becomes leading
// zeros on the later ear, with only the common part left as a delay.
func TestLoadBakesPerEarDelay(t *testing.T) {
	f := sphericalFixture()
	f.positions = f.positions[:1]
	// [M][R] = one measurement, left at 5 samples, right at 12.
	f.delay = []float64{5, 12}
	f.irs = [][][]float64{{{1, 0, 0, 0}, {1, 0, 0, 0}}}

	dataset, err := Load(f.save(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantCommon := 5.0 / testSampleRate
	if got := dataset.Grid.Delays[0]; math.Abs(got-wantCommon) > 1e-12 {
		t.Errorf("common delay = %v, want %v", got, wantCommon)
	}

	if got := len(dataset.Grid.LeftHRIRs[0]); got != 4 {
		t.Errorf("left HRIR length = %d, want 4 (no padding)", got)
	}

	// The right ear trails by 7 samples, which must appear as leading zeros.
	right := dataset.Grid.RightHRIRs[0]
	if len(right) != 11 {
		t.Fatalf("right HRIR length = %d, want 11 (7 samples of padding)", len(right))
	}

	for i := range 7 {
		if right[i] != 0 {
			t.Errorf("right HRIR sample %d = %v, want 0", i, right[i])
		}
	}

	if right[7] != 1 {
		t.Errorf("right HRIR onset at sample 7 = %v, want 1", right[7])
	}
}

// TestLoadReadsAmbiguousDelayAsPerReceiver pins the one shape of Data.Delay
// that cannot be inferred from its length alone. With two measurements and two
// receivers, a length-2 delay matches both M and R; SOFA declares the field as
// [I][R] or [M][R] and never as [M], so it must be read per receiver.
func TestLoadReadsAmbiguousDelayAsPerReceiver(t *testing.T) {
	f := sphericalFixture()
	f.positions = f.positions[:2]
	f.delay = []float64{0, 8}
	f.irs = [][][]float64{
		{{1, 0, 0, 0}, {1, 0, 0, 0}},
		{{1, 0, 0, 0}, {1, 0, 0, 0}},
	}

	dataset, err := Load(f.save(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Per receiver: both measurements delay the right ear by 8 samples. Read
	// per measurement, measurement 1 would instead delay both ears equally and
	// leave no padding at all.
	for m := range 2 {
		if got := len(dataset.Grid.LeftHRIRs[m]); got != 4 {
			t.Errorf("left HRIR %d length = %d, want 4", m, got)
		}

		if got := len(dataset.Grid.RightHRIRs[m]); got != 12 {
			t.Errorf("right HRIR %d length = %d, want 12 (8 samples of padding)", m, got)
		}

		if got := dataset.Grid.Delays[m]; got != 0 {
			t.Errorf("common delay %d = %v, want 0", m, got)
		}
	}
}

func TestLoadScalarSourcePositionAppliesToAllMeasurements(t *testing.T) {
	f := sphericalFixture()
	f.positions = []gosofa.Vector3{{X: 90, Y: 0, Z: 1.5}}
	f.irs = [][][]float64{
		{{1, 0, 0, 0}, {1, 0, 0, 0}},
		{{2, 0, 0, 0}, {2, 0, 0, 0}},
	}

	file := f.build()
	file.M = 2

	dataset, err := datasetFromFile(file)
	if err != nil {
		t.Fatalf("datasetFromFile() error = %v", err)
	}

	for m := range 2 {
		if !vecNear(dataset.Grid.Directions[m], geometry.Vec3{Y: 1}, 1e-9) {
			t.Errorf("direction %d = %v, want +Y", m, dataset.Grid.Directions[m])
		}
	}
}

// TestLoadClonesImpulseResponses guards against the aliasing in go-sofa's
// reshape, where every measurement views one flat backing array.
func TestLoadClonesImpulseResponses(t *testing.T) {
	dataset, err := Load(sphericalFixture().save(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	before := dataset.Grid.RightHRIRs[0][0]
	dataset.Grid.LeftHRIRs[0][0] = 99

	if got := dataset.Grid.RightHRIRs[0][0]; got != before {
		t.Errorf("mutating the left HRIR changed the right one: %v became %v", before, got)
	}
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*gosofa.File)
		wantSub string
	}{
		{
			name:    "frequency-domain data",
			mutate:  func(f *gosofa.File) { f.DataType = "TF" },
			wantSub: "want \"FIR\"",
		},
		{
			name:    "single receiver",
			mutate:  func(f *gosofa.File) { f.R = 1 },
			wantSub: "want 2",
		},
		{
			name:    "no measurements",
			mutate:  func(f *gosofa.File) { f.M = 0 },
			wantSub: "no measurements",
		},
		{
			name:    "zero sample rate",
			mutate:  func(f *gosofa.File) { f.SamplingRate = []float64{0} },
			wantSub: "want a positive value",
		},
		{
			name:    "fractional sample rate",
			mutate:  func(f *gosofa.File) { f.SamplingRate = []float64{44100.5} },
			wantSub: "not a whole number",
		},
		{
			name: "sample rate varying across measurements",
			mutate: func(f *gosofa.File) {
				f.SamplingRate = []float64{48000, 44100}
			},
			wantSub: "varies across measurements",
		},
		{
			name: "unknown coordinate system",
			mutate: func(f *gosofa.File) {
				f.SourcePositionType = ""
				f.SOFAConventions = "GeneralFIR"
			},
			wantSub: "coordinate system is unknown",
		},
		{
			name:    "unrecognized coordinate type",
			mutate:  func(f *gosofa.File) { f.SourcePositionType = "polar" },
			wantSub: "want \"spherical\" or \"cartesian\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := sphericalFixture().build()
			tt.mutate(file)

			_, err := datasetFromFile(file)
			if err == nil {
				t.Fatalf("datasetFromFile() error = nil, want one containing %q", tt.wantSub)
			}

			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("datasetFromFile() error = %q, want it to contain %q", err, tt.wantSub)
			}
		})
	}
}

// TestLoadDefaultsToSphericalForHRIRConventions checks the one case where a
// missing Type attribute is safe to fill in: conventions that mandate
// spherical positions.
func TestLoadDefaultsToSphericalForHRIRConventions(t *testing.T) {
	f := sphericalFixture()
	f.positionType = ""
	f.units = ""

	file := f.build()

	dataset, err := datasetFromFile(file)
	if err != nil {
		t.Fatalf("datasetFromFile() error = %v", err)
	}

	if !vecNear(dataset.Grid.Directions[1], geometry.Vec3{Y: 1}, 1e-9) {
		t.Errorf("direction 1 = %v, want +Y", dataset.Grid.Directions[1])
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "absent.sofa"))
	if err == nil {
		t.Fatal("Load() error = nil, want a failure for a missing file")
	}
}

func vecNear(got, want geometry.Vec3, tol float64) bool {
	return math.Abs(got.X-want.X) < tol &&
		math.Abs(got.Y-want.Y) < tol &&
		math.Abs(got.Z-want.Z) < tol
}
