package export

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
)

func TestWriteEventsJSONAndCSV(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	events := []ir.Event{{
		TimeSeconds:    0.123,
		Amplitude:      0.5,
		Direction:      geometry.Vec3{X: 1, Y: 0, Z: 0},
		DistanceMeters: 4.2,
		BandGain:       []float64{1, 0.5},
		PhaseRadians:   0.25,
		Kind:           ir.EventSpecular,
	}}

	jsonPath := filepath.Join(tmpDir, "events.json")
	err := WriteEventsJSON(jsonPath, events)
	if err != nil {
		t.Fatalf("WriteEventsJSON() error = %v", err)
	}

	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var decodedEvents []ir.Event
	err = json.Unmarshal(jsonData, &decodedEvents)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(decodedEvents) != len(events) || decodedEvents[0].Amplitude != events[0].Amplitude {
		t.Fatalf("decoded events = %#v, want %#v", decodedEvents, events)
	}

	csvPath := filepath.Join(tmpDir, "events.csv")
	err = WriteEventsCSV(csvPath, events)
	if err != nil {
		t.Fatalf("WriteEventsCSV() error = %v", err)
	}

	file, err := os.Open(csvPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if got, want := len(rows), 2; got != want {
		t.Fatalf("csv rows = %d, want %d", got, want)
	}
}

func TestWriteMetricsJSONAndCSV(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	results := []metrics.MetricResult{{Name: "T60", Expected: 1, Actual: 0.98, Tolerance: 0.05, Pass: true}}

	jsonPath := filepath.Join(tmpDir, "metrics.json")
	err := WriteMetricsJSON(jsonPath, results)
	if err != nil {
		t.Fatalf("WriteMetricsJSON() error = %v", err)
	}

	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var decodedResults []metrics.MetricResult
	err = json.Unmarshal(jsonData, &decodedResults)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(decodedResults) != len(results) || decodedResults[0].Name != results[0].Name {
		t.Fatalf("decoded metrics = %#v, want %#v", decodedResults, results)
	}

	csvPath := filepath.Join(tmpDir, "metrics.csv")
	err = WriteMetricsCSV(csvPath, results)
	if err != nil {
		t.Fatalf("WriteMetricsCSV() error = %v", err)
	}

	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("metrics CSV is empty")
	}
}
