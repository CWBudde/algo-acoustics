package export

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
)

// WriteEventsJSON writes sparse IR events to a JSON file.
func WriteEventsJSON(path string, events []ir.Event) error {
	return writeJSONFile(path, events)
}

// WriteMetricsJSON writes metric comparison results to a JSON file.
func WriteMetricsJSON(path string, results []metrics.MetricResult) error {
	return writeJSONFile(path, results)
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	err = os.WriteFile(path, append(data, '\n'), 0o600)
	if err != nil {
		return fmt.Errorf("write json file: %w", err)
	}

	return nil
}
