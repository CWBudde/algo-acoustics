package geometry

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"
)

const svenssonMagnitudeToleranceDB = 0.5

func TestSvenssonReferenceManifestHashes(t *testing.T) {
	manifestData, err := os.ReadFile("../testdata/diffraction/reference-manifest.json")
	if err != nil {
		t.Fatal(err)
	}

	manifest := struct {
		Generator struct {
			File   string `json:"file"`
			SHA256 string `json:"sha256"`
		} `json:"generator"`
		GeneratedCSVSHA256 string `json:"generated_csv_sha256"`
		Evaluation         struct {
			CSVFile string `json:"csv_file"`
		} `json:"evaluation"`
	}{}

	err = json.Unmarshal(manifestData, &manifest)
	if err != nil {
		t.Fatal(err)
	}

	assertReferenceHash(t, manifest.Generator.File, manifest.Generator.SHA256)
	assertReferenceHash(t, manifest.Evaluation.CSVFile, manifest.GeneratedCSVSHA256)
}

func TestBTMETransferMatchesSvenssonEDB2FiniteWedge(t *testing.T) {
	records := readSvenssonReference(t)
	edge := DiffractionEdge{
		Start:       Vec3{Z: -50},
		End:         Vec3{Z: 50},
		Direction:   Vec3{Z: 1},
		Length:      100,
		WedgeIndex:  350.0 / 180.0,
		FaceONormal: Vec3{X: 1},
	}
	source := referencePoint(90)

	for _, record := range records[1:] {
		receiverAzimuth := parseReferenceFloat(t, record, 0)
		frequency := parseReferenceFloat(t, record, 1)
		wantMagnitudeDB := parseReferenceFloat(t, record, 4)

		transfer, err := BTMETransfer(source, referencePoint(receiverAzimuth), edge, frequency, 343)
		if err != nil {
			t.Fatalf("BTMETransfer(receiver=%g deg, frequency=%g Hz): %v", receiverAzimuth, frequency, err)
		}

		gotMagnitudeDB := 20 * math.Log10(math.Hypot(real(transfer), imag(transfer)))
		if delta := math.Abs(gotMagnitudeDB - wantMagnitudeDB); delta > svenssonMagnitudeToleranceDB {
			t.Errorf(
				"BTME magnitude at receiver=%g deg, frequency=%g Hz = %.9f dB, EDB2 %.9f dB (|delta|=%.9f dB, tolerance=%g dB)",
				receiverAzimuth,
				frequency,
				gotMagnitudeDB,
				wantMagnitudeDB,
				delta,
				svenssonMagnitudeToleranceDB,
			)
		}
	}
}

func TestBTMEViewShadowZoneTransitionSmooth(t *testing.T) {
	edge := DiffractionEdge{
		Start:       Vec3{Z: -50},
		End:         Vec3{Z: 50},
		Direction:   Vec3{Z: 1},
		Length:      100,
		WedgeIndex:  350.0 / 180.0,
		FaceONormal: Vec3{X: 1},
	}
	const (
		frequency    = 500.0
		speedOfSound = 343.0
		boundary     = -90.0
		stepDegrees  = 0.1
	)
	source := referencePoint(90)

	var previous complex128

	for index := range 21 {
		azimuth := boundary - 1 + float64(index)*stepDegrees
		receiver := referencePoint(azimuth)

		diffraction, err := BTMETransfer(source, receiver, edge, frequency, speedOfSound)
		if err != nil {
			t.Fatalf("BTMETransfer(%g deg): %v", azimuth, err)
		}

		directDistance := source.Distance(receiver)
		direct := complex(1.0/directDistance, 0) * complex(
			math.Cos(-2*math.Pi*frequency*directDistance/speedOfSound),
			math.Sin(-2*math.Pi*frequency*directDistance/speedOfSound),
		)

		total := diffraction

		switch {
		case azimuth < boundary-1e-12:
			total += direct
		case math.Abs(azimuth-boundary) <= 1e-12:
			total += 0.5 * direct
		}

		if index > 0 {
			previousDB := 20 * math.Log10(math.Hypot(real(previous), imag(previous)))

			currentDB := 20 * math.Log10(math.Hypot(real(total), imag(total)))
			if jump := math.Abs(currentDB - previousDB); jump > 0.5 {
				t.Fatalf("view/shadow transition jumped %.6f dB between %.1f and %.1f degrees", jump, azimuth-stepDegrees, azimuth)
			}
		}

		previous = total
	}
}

func readSvenssonReference(t *testing.T) [][]string {
	t.Helper()

	file, err := os.Open("../testdata/diffraction/svensson_edb2_single_wedge.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 61 {
		t.Fatalf("Svensson fixture has %d rows including header, want 61", len(records))
	}

	return records
}

func parseReferenceFloat(t *testing.T, record []string, column int) float64 {
	t.Helper()

	if len(record) != 5 {
		t.Fatalf("Svensson fixture row has %d columns, want 5: %v", len(record), record)
	}

	value, err := strconv.ParseFloat(record[column], 64)
	if err != nil {
		t.Fatal(fmt.Errorf("parse Svensson fixture column %d: %w", column, err))
	}

	return value
}

func referencePoint(azimuthDegrees float64) Vec3 {
	const radius = 10.0

	azimuth := azimuthDegrees * math.Pi / 180

	return Vec3{X: radius * math.Cos(azimuth), Y: radius * math.Sin(azimuth)}
}

func assertReferenceHash(t *testing.T, filename, want string) {
	t.Helper()

	data, err := os.ReadFile("../testdata/diffraction/" + filename)
	if err != nil {
		t.Fatal(err)
	}

	digest := sha256.Sum256(data)
	if got := hex.EncodeToString(digest[:]); got != want {
		t.Fatalf("%s SHA-256 = %s, manifest wants %s", filename, got, want)
	}
}
