# Scene Format

Scenes describe the room, materials, sources, and receivers in JSON so validation and rendering can share the same stable input shape.

The canonical schema lives in [scene-schema.json](scene-schema.json).

Mesh rooms use `"kind": "mesh"` and reference a Wavefront OBJ file via `"meshPath"`. When scenes are loaded from disk, relative mesh paths are resolved relative to the scene JSON file.

```json
{
  "room": {
    "kind": "mesh",
    "meshPath": "cube.obj"
  }
}
```

See [external-tool-compatibility.md](external-tool-compatibility.md) for the scene conventions and validation workflow used for desktop authoring tools.
