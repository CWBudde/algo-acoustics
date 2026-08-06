package scene_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestSceneSchemaValidatesCanonicalFixtures(t *testing.T) {
	t.Parallel()

	schema := compileSceneSchema(t)
	patterns := []string{
		filepath.Join("..", "testdata", "rooms", "*.json"),
		filepath.Join("..", "testdata", "interop", "*.json"),
	}

	var paths []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("filepath.Glob(%q) error = %v", pattern, err)
		}

		paths = append(paths, matches...)
	}

	if len(paths) == 0 {
		t.Fatal("no canonical scene fixtures found")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			validateSceneFile(t, schema, path)
		})
	}
}

func TestSceneSchemaAcceptsInlineMesh(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "testdata", "rooms", "mesh_cube.json")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}

	var instance map[string]any

	err = json.Unmarshal(data, &instance)
	if err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", path, err)
	}

	room, ok := instance["room"].(map[string]any)
	if !ok {
		t.Fatalf("room in %q is not an object", path)
	}

	delete(room, "meshPath")

	inlineMesh := geometry.Mesh{
		Triangles: []geometry.Triangle{{
			V0: geometry.Vec3{X: 0, Y: 0, Z: 0},
			V1: geometry.Vec3{X: 1, Y: 0, Z: 0},
			V2: geometry.Vec3{X: 0, Y: 1, Z: 0},
		}},
	}

	meshData, err := json.Marshal(inlineMesh)
	if err != nil {
		t.Fatalf("json.Marshal(inline mesh) error = %v", err)
	}

	var meshInstance any

	err = json.Unmarshal(meshData, &meshInstance)
	if err != nil {
		t.Fatalf("json.Unmarshal(inline mesh) error = %v", err)
	}

	room["mesh"] = meshInstance

	err = compileSceneSchema(t).Validate(instance)
	if err != nil {
		t.Fatalf("inline mesh scene does not validate: %v", err)
	}
}

func compileSceneSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()

	path := filepath.Join("..", "docs", "scene-schema.json")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}

	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("jsonschema.UnmarshalJSON(%q) error = %v", path, err)
	}

	compiler := jsonschema.NewCompiler()

	err = compiler.AddResource("scene-schema.json", document)
	if err != nil {
		t.Fatalf("compiler.AddResource() error = %v", err)
	}

	schema, err := compiler.Compile("scene-schema.json")
	if err != nil {
		t.Fatalf("compiler.Compile() error = %v", err)
	}

	return schema
}

func validateSceneFile(t *testing.T, schema *jsonschema.Schema, path string) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open(%q) error = %v", path, err)
	}
	defer file.Close()

	instance, err := jsonschema.UnmarshalJSON(file)
	if err != nil {
		t.Fatalf("jsonschema.UnmarshalJSON(%q) error = %v", path, err)
	}

	err = schema.Validate(instance)
	if err != nil {
		t.Fatalf("schema.Validate(%q) error = %v", path, err)
	}
}
