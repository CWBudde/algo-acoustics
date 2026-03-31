# Scene Format

Scenes describe the room, materials, sources, and receivers in JSON so validation and rendering can share the same stable input shape.

Mesh rooms use `"kind": "mesh"` and reference a Wavefront OBJ file via `"meshPath"`. When scenes are loaded from disk, relative mesh paths are resolved relative to the scene JSON file.

```json
{
	"room": {
		"kind": "mesh",
		"meshPath": "cube.obj"
	}
}
```