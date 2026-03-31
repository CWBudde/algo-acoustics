package scene

import (
	"encoding/json"
	"io"
	"os"
)

// LoadScene decodes a scene from JSON.
func LoadScene(r io.Reader) (*Scene, error) {
	var sc Scene
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&sc); err != nil {
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

	return LoadScene(file)
}
