package scene

import (
	"path/filepath"
	"testing"
)

func TestInteropScenesLoadAndValidate(t *testing.T) {
	t.Parallel()

	fixtures := []string{
		filepath.Join("..", "testdata", "interop", "atrium_shoebox.json"),
		filepath.Join("..", "testdata", "interop", "mesh_gallery.json"),
	}

	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			t.Parallel()

			sc, err := LoadSceneFile(fixture)
			if err != nil {
				t.Fatalf("LoadSceneFile() error = %v", err)
			}

			if err := Validate(sc); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
