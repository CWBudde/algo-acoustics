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
		return nil, fmt.Errorf("decode scene JSON: %w", err)
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
		return nil, fmt.Errorf("open scene file %q: %w", path, err)
	}
	defer file.Close()

	return loadScene(file, filepath.Dir(path))
}

func resolveRoomMesh(sc *Scene, baseDir string) error {
	if sc == nil {
		return nil
	}

	for index := range sc.RoomCount() {
		room, ok := sc.RoomAt(index)
		if !ok {
			continue
		}

		err := resolveMesh(room, baseDir)
		if err != nil {
			return fmt.Errorf("resolve room[%d] mesh: %w", index, err)
		}
	}

	return nil
}

func resolveMesh(room *Room, baseDir string) error {
	if room == nil || !room.IsMesh() || room.Mesh != nil || room.MeshPath == "" {
		return nil
	}

	meshPath := room.MeshPath
	if baseDir != "" && !filepath.IsAbs(meshPath) {
		meshPath = filepath.Join(baseDir, meshPath)
	}

	mesh, err := geometry.LoadOBJ(meshPath)
	if err != nil {
		return fmt.Errorf("load room mesh %q: %w", room.MeshPath, err)
	}

	room.Mesh = mesh

	return nil
}
