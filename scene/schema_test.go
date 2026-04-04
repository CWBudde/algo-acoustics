package scene_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSceneSchemaFileExistsAndIsValidJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "docs", "scene-schema.json")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var schema map[string]any

	err = json.Unmarshal(data, &schema)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if schema["$schema"] == "" {
		t.Fatalf("schema is missing $schema")
	}

	if schema["title"] == "" {
		t.Fatalf("schema is missing title")
	}
}
