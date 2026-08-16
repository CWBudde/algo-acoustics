package sofa

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	gosofa "github.com/cwbudde/go-sofa"
)

// recognizedRejections are the reasons Load is allowed to turn a file away.
// Anything else means the loader broke on data it claimed to handle.
var recognizedRejections = []string{
	"want \"FIR\"",
	"want 2",
	"no measurements",
	"no source positions",
	"no Data.SamplingRate",
	"want a positive value",
	"not a whole number",
	"varies across measurements",
	"coordinate system is unknown",
	"want \"spherical\" or \"cartesian\"",
	"at the origin",
	"source positions",
	"impulse responses",
}

// TestLoadCorpus runs Load across a directory of real SOFA files, which is how
// support for the measured datasets — CIPIC, LISTEN, ARI — is verified without
// committing megabytes of binaries to this repository. Point it at a corpus:
//
//	ALGO_SOFA_CORPUS=/path/to/sofa/files go test ./hrtf/sofa/
//
// Every file must either load into a usable dataset or be rejected for a
// reason this package states outright. Neither a panic nor a silently wrong
// answer is acceptable.
func TestLoadCorpus(t *testing.T) {
	dir := os.Getenv("ALGO_SOFA_CORPUS")
	if dir == "" {
		t.Skip("set ALGO_SOFA_CORPUS to a directory of .sofa files to run this test")
	}

	paths, err := filepath.Glob(filepath.Join(dir, "*.sofa"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	if len(paths) == 0 {
		t.Fatalf("no .sofa files in %s", dir)
	}

	var loaded int

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			// Open separately from the conversion. A file the HDF5 reader
			// cannot parse at all says nothing about this package, so it is
			// skipped rather than failed; only what reaches datasetFromFile
			// is this package's responsibility.
			file, err := gosofa.Open(path)
			if err != nil {
				t.Skipf("unreadable by the SOFA reader, nothing for this package to do: %v", err)
			}
			defer file.Close()

			dataset, err := datasetFromFile(file)
			if err != nil {
				assertRecognizedRejection(t, file, err)

				return
			}

			loaded++

			assertUsable(t, dataset.SampleRateHz, dataset.Grid.Directions, dataset.Grid.LeftHRIRs, dataset.Grid.RightHRIRs)
		})
	}

	if loaded == 0 {
		t.Errorf("no file in %s loaded successfully; the corpus exercises nothing", dir)
	} else {
		t.Logf("loaded %d of %d files in %s", loaded, len(paths), dir)
	}
}

// assertRecognizedRejection fails unless the error is one this package raises
// deliberately, and reports what the file actually held so an unexpected
// rejection is diagnosable.
func assertRecognizedRejection(t *testing.T, file *gosofa.File, err error) {
	t.Helper()

	for _, want := range recognizedRejections {
		if strings.Contains(err.Error(), want) {
			t.Logf("rejected as expected: %v", err)

			return
		}
	}

	t.Fatalf("datasetFromFile() error = %v; file has DataType=%q convention=%q M=%d R=%d N=%d",
		err, file.DataType, file.SOFAConventions, file.M, file.R, file.N)
}

func assertUsable(t *testing.T, sampleRate int, directions []geometry.Vec3, left, right [][]float64) {
	t.Helper()

	if sampleRate < 8000 || sampleRate > 768000 {
		t.Errorf("SampleRateHz = %d, want a plausible audio rate", sampleRate)
	}

	if len(directions) == 0 {
		t.Fatal("dataset has no directions")
	}

	if len(left) != len(directions) || len(right) != len(directions) {
		t.Fatalf("got %d directions but %d left and %d right HRIRs",
			len(directions), len(left), len(right))
	}

	for i, dir := range directions {
		length := math.Sqrt(dir.X*dir.X + dir.Y*dir.Y + dir.Z*dir.Z)
		if math.Abs(length-1) > 1e-9 {
			t.Fatalf("direction %d has length %v, want 1", i, length)
		}
	}

	for i := range left {
		if len(left[i]) == 0 || len(right[i]) == 0 {
			t.Fatalf("measurement %d has an empty HRIR (left %d, right %d)",
				i, len(left[i]), len(right[i]))
		}
	}
}
