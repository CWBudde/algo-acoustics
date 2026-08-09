# HRTF and SOFA

Binaural rendering depends on the narrow `hrtf.Dataset` interface: a sample
rate plus left/right HRIR lookup for a direction. The renderer transforms event
directions from world space into the receiver's head frame before lookup, so a
receiver quaternion rotates the listener rather than the room.

## Scene JSON

Set the receiver type to `binaural`, include an HRTF, and use
`render-stereo`:

```json
{
  "position": { "X": 4, "Y": 2, "Z": 1.2 },
  "orientation": { "W": 1, "X": 0, "Y": 0, "Z": 0 },
  "type": "binaural",
  "hrtf": {
    "type": "nearestNeighbor",
    "sampleRate": 48000
  }
}
```

```bash
go run ./cmd/roomir validate examples/scenes/shoebox_binaural.json
go run ./cmd/roomir render-stereo examples/scenes/shoebox_binaural.json \
  --output /tmp/shoebox_binaural.wav \
  --duration 1.5 \
  --max-order 3 \
  --num-rays 4096
```

The HRTF sample rate must equal the scene sample rate. The example intentionally
omits a measurement grid, which produces a centered identity impulse: it tests
the stereo path but does not create perceptual spatialization.

## Supplying Measurements

`NearestNeighborDataset` is the measured-grid implementation supported by scene
JSON (`"type": "nearestNeighbor"`). A grid contains unit direction vectors,
parallel left/right HRIR arrays, optional per-direction delays in seconds, and
optional triangle topology. For large measured datasets, construct the scene in
Go instead of embedding thousands of samples in JSON:

```go
dataset := hrtf.NearestNeighborDataset{
	SampleRateHz: 48000,
	Grid: &hrtf.MeasurementGrid{
		Directions: []geometry.Vec3{{X: 1}, {Y: 1}},
		LeftHRIRs:  [][]float64{{1, 0}, {0.8, 0.1}},
		RightHRIRs: [][]float64{{1, 0}, {0.6, 0.2}},
		Delays:     []float64{0, 0.0002},
	},
}
receiver := scene.Receiver{
	Position:    geometry.Vec3{X: 4, Y: 2, Z: 1.2},
	Orientation: geometry.QuatIdentity(),
	Type:        scene.ReceiverBinaural,
	HRTF:        dataset,
}
```

All measurement arrays should use matching indices. Normalize directions and
keep both ears' HRIR lengths consistent within the dataset.

Interpolation is explicit opt-in through `InterpolatingDataset` or
`InterpolateHRIR`. It uses only triangles supplied in
`MeasurementGrid.Triangles`, falls back to the nearest measurement when no
valid containing triangle exists, and is not currently a scene-JSON dataset
type.

## SOFA Status

SOFA file loading is not implemented. Without the `sofa` build tag,
`LoadSOFA` reports that the tag is required; tagged builds expose the adapter
but still return an error because no concrete SOFA reader is wired in. Convert
measurements into a `MeasurementGrid` in application code for now; do not treat
the tagged adapter as full SOFA ingestion.

Low-frequency PDE blending is mono-only. `render-stereo` combines binaural ISM
and directional late-field buffers, but does not apply `Renderer.LowFreq`.
