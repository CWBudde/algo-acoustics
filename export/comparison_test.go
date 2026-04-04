package export_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/metrics"
)

func TestWriteComparisonCSVAndMarkdown(t *testing.T) {
	t.Parallel()

	rows := []metrics.ComparisonRow{
		{Name: "peak amplitude", Expected: 1, Actual: 0.5, Delta: -0.5, Unit: "linear"},
		{Name: "correlation", Expected: 1, Actual: 0.99, Delta: -0.01, Unit: "coefficient"},
	}

	tmpDir := t.TempDir()

	csvPath := filepath.Join(tmpDir, "comparison.csv")

	err := export.WriteComparisonCSV(csvPath, rows)
	if err != nil {
		t.Fatalf("WriteComparisonCSV() error = %v", err)
	}

	file, err := os.Open(csvPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if got, want := len(records), 3; got != want {
		t.Fatalf("csv rows = %d, want %d", got, want)
	}

	if records[0][0] != "metric" || records[0][1] != "expected" || records[0][2] != "actual" {
		t.Fatalf("csv header = %v, want comparison header", records[0])
	}

	mdPath := filepath.Join(tmpDir, "comparison.md")

	err = export.WriteComparisonMarkdown(mdPath, rows)
	if err != nil {
		t.Fatalf("WriteComparisonMarkdown() error = %v", err)
	}

	content, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "| Metric | Expected | Actual | Delta | Unit |") {
		t.Fatalf("markdown header missing from %q", text)
	}

	if !strings.Contains(text, "peak amplitude") || !strings.Contains(text, "correlation") {
		t.Fatalf("markdown output missing rows:\n%s", text)
	}
}
