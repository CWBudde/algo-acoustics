# Scene Format

Scenes describe the room, materials, sources, and receivers in JSON so
validation and rendering share one stable input shape. For a task-oriented
walkthrough and runnable files, start with [scene authoring](scene-authoring.md).

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

The public JSON field names intentionally retain their established casing
(`bandSpec`, `sampleRate`, `gainDb`, and uppercase vector components). Do not
derive scene JSON from the repository's general snake-case convention.

## Multi-room scenes

The legacy `room` field remains the canonical single-room representation.
Scenes that model transmission between spaces use `rooms` instead, with
at least two entries and world-space shoebox placement supplied by `origin`.
Do not emit a one-element `rooms` array. A portal is an ordered planar polygon
shared by two room boundary walls. Its winding points from the first
`roomIndices` entry toward the second.

```json
{
  "rooms": [
    {
      "kind": "shoebox",
      "shoebox": {
        "width": 6,
        "depth": 4,
        "height": 3,
        "wallMaterials": ["wall", "wall", "wall", "wall", "wall", "wall"]
      }
    },
    {
      "kind": "shoebox",
      "shoebox": {
        "width": 6,
        "depth": 4,
        "height": 3,
        "origin": { "X": 6, "Y": 0, "Z": 0 },
        "wallMaterials": ["wall", "wall", "wall", "wall", "wall", "wall"]
      }
    }
  ],
  "portals": [
    {
      "roomIndices": [0, 1],
      "polygon": [
        { "X": 6, "Y": 1, "Z": 0.5 },
        { "X": 6, "Y": 3, "Z": 0.5 },
        { "X": 6, "Y": 3, "Z": 2.5 },
        { "X": 6, "Y": 1, "Z": 2.5 }
      ],
      "material": "door",
      "state": "closed"
    }
  ]
}
```

Material transmission may be authored as linear energy coefficients in
`transmissionByBand` or as decibel values in `soundReductionIndex`. When both
are present they must agree. Closed portals apply their material; open portals
are fully transmissive in every band.

The Phase 21 renderer supports exactly one source and one receiver in two
different, directly connected shoebox rooms. It combines all portals between
that room pair. Portal chains, mesh-room transmission, and sources and
receivers located in the same room require a different rendering path or the
later acoustic-scene-graph work.

See [external-tool-compatibility.md](external-tool-compatibility.md) for the
scene conventions and validation workflow used for desktop authoring tools.
