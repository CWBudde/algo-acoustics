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

## Loading SOFA Files

`hrtf/sofa` reads measured HRTFs from SOFA (AES69) files into the grid
described above:

```go
import "github.com/cwbudde/algo-acoustics/hrtf/sofa"

dataset, err := sofa.Load("CIPIC_subject_003_hrir_final.sofa")
if err != nil {
	return err
}

receiver := scene.Receiver{
	Position:    geometry.Vec3{X: 4, Y: 2, Z: 1.2},
	Orientation: geometry.QuatIdentity(),
	Type:        scene.ReceiverBinaural,
	HRTF:        *dataset,
}
```

The loader reads `SimpleFreeFieldHRIR` and the measured datasets built on it —
CIPIC, LISTEN, ARI. Source positions are converted from the file's own
coordinate system into the head frame this library uses: azimuth from +X in the
XY plane, elevation toward +Z. Receiver positions decide which measurement is
the left ear, so a file that stores the right ear first still loads correctly.

It is a separate package on purpose. The reader pulls in an HDF5
implementation, and Go's linker only includes that in binaries importing it, so
the WASM demo and the `roomir` CLI pay nothing for it. That is also why scene
JSON cannot name a `.sofa` path: `scene` is compiled into the browser bundle,
and importing the loader there would drag HDF5 in with it. Load the file in Go
and assign the dataset to the receiver, as above.

### Limits

`Load` rejects rather than approximates, and says why:

- **Impulse responses only.** Frequency-domain (`TF`, `TF-E`) and
  second-order-section (`SOS`) files are refused, including SH-encoded HRTFs.
- **Two receivers.** Files with one receiver or a microphone array are refused.
- **No resampling.** The file's rate becomes the dataset's rate, and the scene
  sample rate must match it.
- **The coordinate system must be knowable.** A file that omits the
  `SourcePosition` `Type` attribute is only assumed spherical when its
  convention mandates it; otherwise loading fails rather than guess, because
  reading spherical positions as cartesian misplaces every measurement
  silently.
- **A sample rate must be present.** Some published files omit
  `Data.SamplingRate` entirely; there is no safe default.

`Data.Delay` is folded into the measurements. Since `Lookup` returns one delay
for both ears, the common part travels as the delay and the per-ear excess
becomes leading zeros in that ear's HRIR, preserving the ITD. The sub-sample
remainder is dropped, which is moot for the datasets above: they carry their
ITD inside the HRIRs with `Data.Delay` zero.

No triangle topology is built, so a loaded dataset does nearest-neighbor
lookup. `InterpolatingDataset` falls back to nearest without triangles anyway.

Low-frequency PDE blending is mono-only. `render-stereo` combines binaural ISM
and directional late-field buffers, but does not apply `Renderer.LowFreq`.
