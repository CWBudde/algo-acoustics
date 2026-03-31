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
	if err := WriteEventsJSON(jsonPath, events); err != nil {
		t.Fatalf("WriteEventsJSON() error = %v", err)
	}
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var decodedEvents []ir.Event
	if err := json.Unmarshal(jsonData, &decodedEvents); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(decodedEvents) != len(events) || decodedEvents[0].Amplitude != events[0].Amplitude {
		t.Fatalf("decoded events = %#v, want %#v", decodedEvents, events)
	}

	csvPath := filepath.Join(tmpDir, "events.csv")
	if err := WriteEventsCSV(csvPath, events); err != nil {
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
	if err := WriteMetricsJSON(jsonPath, results); err != nil {
		t.Fatalf("WriteMetricsJSON() error = %v", err)
	}
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var decodedResults []metrics.MetricResult
	if err := json.Unmarshal(jsonData, &decodedResults); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(decodedResults) != len(results) || decodedResults[0].Name != results[0].Name {
		t.Fatalf("decoded metrics = %#v, want %#v", decodedResults, results)
	}

	csvPath := filepath.Join(tmpDir, "metrics.csv")
	if err := WriteMetricsCSV(csvPath, results); err != nil {
		t.Fatalf("WriteMetricsCSV() error = %v", err)
	}
	if data, err := os.ReadFile(csvPath); err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	} else if len(data) == 0 {
		t.Fatal("metrics CSV is empty")
	}
}
