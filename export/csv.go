package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
)

// WriteEventsCSV writes sparse IR events to a CSV file.
func WriteEventsCSV(path string, events []ir.Event) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"index", "timeSeconds", "amplitude", "directionX", "directionY", "directionZ", "distanceMeters", "bandGain", "phaseRadians", "kind"}); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for index, event := range events {
		bandGain, err := json.Marshal(event.BandGain)
		if err != nil {
			return fmt.Errorf("marshal band gain: %w", err)
		}

		row := []string{
			strconv.Itoa(index),
			strconv.FormatFloat(event.TimeSeconds, 'f', -1, 64),
			strconv.FormatFloat(event.Amplitude, 'f', -1, 64),
			strconv.FormatFloat(event.Direction.X, 'f', -1, 64),
			strconv.FormatFloat(event.Direction.Y, 'f', -1, 64),
			strconv.FormatFloat(event.Direction.Z, 'f', -1, 64),
			strconv.FormatFloat(event.DistanceMeters, 'f', -1, 64),
			string(bandGain),
			strconv.FormatFloat(event.PhaseRadians, 'f', -1, 64),
			strconv.Itoa(int(event.Kind)),
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv writer: %w", err)
	}

	return nil
}

// WriteMetricsCSV writes metric comparison results to a CSV file.
func WriteMetricsCSV(path string, results []metrics.MetricResult) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"name", "expected", "actual", "tolerance", "pass"}); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, result := range results {
		row := []string{
			result.Name,
			strconv.FormatFloat(result.Expected, 'f', -1, 64),
			strconv.FormatFloat(result.Actual, 'f', -1, 64),
			strconv.FormatFloat(result.Tolerance, 'f', -1, 64),
			strconv.FormatBool(result.Pass),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv writer: %w", err)
	}

	return nil
}
