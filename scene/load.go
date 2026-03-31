package scene

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// LoadScene decodes a scene from JSON.
func LoadScene(r io.Reader) (*Scene, error) {
	return loadScene(r, "")
}

func loadScene(r io.Reader, baseDir string) (*Scene, error) {
	var sc Scene

	decoder := json.NewDecoder(r)

	err := decoder.Decode(&sc)
	if err != nil {
		return nil, err
	}

	err = resolveRoomMesh(&sc, baseDir)
	if err != nil {
		return nil, err
	}

	return &sc, nil
}

// LoadSceneFile loads a scene from a JSON file on disk.
func LoadSceneFile(path string) (*Scene, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return loadScene(file, filepath.Dir(path))
}

func resolveRoomMesh(sc *Scene, baseDir string) error {
	if sc == nil || !sc.Room.IsMesh() || sc.Room.Mesh != nil || sc.Room.MeshPath == "" {
		return nil
	}

	meshPath := sc.Room.MeshPath
	if baseDir != "" && !filepath.IsAbs(meshPath) {
		meshPath = filepath.Join(baseDir, meshPath)
	}

	mesh, err := geometry.LoadOBJ(meshPath)
	if err != nil {
		return fmt.Errorf("load room mesh %q: %w", sc.Room.MeshPath, err)
	}

	sc.Room.Mesh = mesh

	return nil
}
