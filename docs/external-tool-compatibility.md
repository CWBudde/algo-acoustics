# External Tool Compatibility

`algo-acoustics` treats scene JSON as the interchange surface for desktop authoring tools. External editors should preserve the authored room, source, receiver, and material metadata so scenes can round-trip through the CLI without losing intent.

## Conventions

- Coordinate system: right-handed, with `+Z` up.
- Units: meters for positions, room dimensions, and mesh coordinates.
- Source positions: expressed in world coordinates; source orientation is stored as a quaternion.
- Receiver positions: expressed in world coordinates; binaural receivers must preserve orientation and HRTF metadata.
- Materials: use the same octave-band absorption and scattering layout as `scene.Material`, with 125 Hz to 4 kHz as the default 6-band set.
- Mesh scenes: keep `meshPath` relative to the scene file when possible so external tools and the CLI can resolve the same asset graph.

## Recommended Workflow

1. Author the room and place sources/receivers in the external tool.
2. Export the scene as JSON with stable material names and mesh references.
3. Validate the JSON with `roomir validate`.
4. Inspect the normalized metadata with `roomir inspect`.
5. Render a reference IR and compare it against a stored baseline.

## Baseline Strategy

For mesh-based authoring workflows, prefer a deterministic late-field render with a fixed ray count and duration, then compare the resulting WAV against a committed reference IR. This keeps the compatibility check stable while still exercising the full scene import and render path.
