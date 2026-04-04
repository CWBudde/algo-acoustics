package export

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/scene"
)

// WriteEventsJSON writes sparse IR events to a JSON file.
func WriteEventsJSON(path string, events []ir.Event) error {
	return writeJSONFile(path, events)
}

// WriteMetricsJSON writes metric comparison results to a JSON file.
func WriteMetricsJSON(path string, results []metrics.MetricResult) error {
	return writeJSONFile(path, results)
}

// WriteSceneJSON writes a canonical scene JSON file.
//
// Scenes loaded from disk may carry a resolved mesh payload for runtime use.
// Canonical interchange keeps the authored mesh reference and omits the
// in-memory mesh blob when meshPath is present.
func WriteSceneJSON(path string, sc *scene.Scene) error {
	data, err := SceneJSON(sc)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		return fmt.Errorf("write scene json: %w", err)
	}

	return nil
}

// SceneJSON returns canonical JSON bytes for a scene.
func SceneJSON(sc *scene.Scene) ([]byte, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	canonical := *sc
	if canonical.Room.MeshPath != "" {
		canonical.Room.Mesh = nil
	}

	data, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal scene json: %w", err)
	}

	return append(data, '\n'), nil
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
