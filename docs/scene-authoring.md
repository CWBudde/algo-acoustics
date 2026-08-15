# Scene Authoring

A scene is a JSON document containing room geometry, a material palette,
sources, receivers, a frequency-band specification, and a sample rate. Use
`room` for a single space or `rooms` plus `portals` for transmission between at
least two spaces. Start from the small fixtures in
[examples/scenes](../examples/scenes) instead of assembling the band edges by
hand. The canonical field reference is
[scene-schema.json](scene-schema.json).

## Coordinate and Unit Conventions

- Positions and room dimensions are in meters in a right-handed coordinate
  system with `+Z` up.
- A legacy shoebox starts at `(0, 0, 0)`. Multi-room shoeboxes use `origin` as
  their world-space minimum corner and end at `origin + (width, depth, height)`.
- Quaternions are `{ "W", "X", "Y", "Z" }`. The identity orientation is
  `{ "W": 1, "X": 0, "Y": 0, "Z": 0 }`.
- Band arrays follow `bandSpec.CenterFreqs`; coefficients are linear values in
  `[0, 1]`.
- `sampleRate` must be high enough that every upper band edge is below Nyquist.

## Authoring Checklist

1. Choose `room.kind` as `shoebox` or `mesh` for a single room. For transmission,
   define at least two `rooms`; a one-element `rooms` array is invalid. A mesh
   scene should use a relative `meshPath`; it is resolved relative to the scene
   file.
2. Define every material name referenced by the six shoebox wall entries. For a
   mesh-wide material, set `room.meshMaterial`. To vary the material across a
   mesh, add `room.triangleMaterials`: one entry per triangle, in mesh order,
   where an empty entry falls back to `meshMaterial`.
3. Keep source and receiver positions inside the room bounds. Add explicit
   identity orientations when no rotation is intended.
4. Use an `omni` receiver for mono output. A `binaural` receiver also requires
   an HRTF whose sample rate equals the scene sample rate.
5. Validate and inspect before rendering.

```bash
go run ./cmd/roomir validate examples/scenes/shoebox_mono.json
go run ./cmd/roomir inspect examples/scenes/shoebox_mono.json
go run ./cmd/roomir render examples/scenes/shoebox_mono.json \
  --output /tmp/shoebox_mono.wav \
  --mode hybrid \
  --duration 1.5 \
  --max-order 3 \
  --num-rays 4096
```

The renderer interfaces impose stricter cardinality than scene validation. The
shipped ISM adapter accepts one or more sources and exactly one receiver; late,
binaural, and progressive rendering require exactly one source and one
receiver. Phase 21 transmission additionally requires shoebox rooms with one
source and one receiver in different rooms joined directly by at least one
portal.

## Materials and Walls

`wallMaterials` contains six names in the order used by `scene.Shoebox`:
negative X, positive X, negative Y, positive Y, floor, ceiling. Keep
`absorptionByBand` and `scatteringByBand` the same length as the band
specification. Absorption removes energy; scattering transfers reflected energy
from specular into diffuse propagation. Transmission can be specified as
linear energy coefficients in `transmissionByBand` or decibels in
`soundReductionIndex`; singleton arrays apply to every band. Each band must
satisfy `absorption + transmission <= 1`.

## Mesh Rooms

OBJ geometry must form an enclosed, positive-volume room. Keep the OBJ beside
the JSON or below it so the pair remains portable:

```json
{
  "room": {
    "kind": "mesh",
    "meshPath": "geometry/room.obj",
    "meshMaterial": "concrete"
  }
}
```

Run `validate` after moving either file because path resolution happens while
loading. For interchange details, see
[external-tool compatibility](external-tool-compatibility.md).

## Choosing a Render Path

- `--mode early` is deterministic ISM direct/specular output.
- `--mode late` synthesizes the ray-traced diffuse field.
- `--mode hybrid` crossfades early and late paths and is the normal mono path.
- `render-stereo` requires a binaural receiver; see [HRTF usage](hrtf-sofa.md).
- `--enable-lowfreq` adds the mono PDE transfer below the selected frequency;
  see [hybrid rendering](hybrid-rendering.md).
