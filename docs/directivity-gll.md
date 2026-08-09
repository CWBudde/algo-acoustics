# Directivity and GLL

Source directivity is evaluated in the source's local frame for every frequency
band. The source quaternion defines that frame; the model axis then defines its
forward direction within the frame.

## Scene Models

Scene JSON supports `omni` and analytic `cardioid` models:

```json
{
  "position": { "X": 2, "Y": 3, "Z": 1.5 },
  "orientation": { "W": 1, "X": 0, "Y": 0, "Z": 0 },
  "gainDb": -6,
  "directivity": {
    "type": "cardioid",
    "axis": { "X": 1, "Y": 0, "Z": 0 },
    "orderN": 1
  }
}
```

`axis` is normalized during gain evaluation. `orderN` controls sharpness:
larger non-negative values narrow the forward lobe. With the identity source
orientation, the example points toward world `+X`. See
[examples/scenes/shoebox_mono.json](../examples/scenes/shoebox_mono.json) for a
complete scene.

In Go, assign any `directivity.Model` to `scene.Source.Directivity`:

```go
source.Directivity = directivity.CardioidModel{
	Axis:   geometry.Vec3{X: 1},
	OrderN: 1,
}
```

A nil directivity is treated as unity gain by the solvers, but using
`OmniModel{}` makes intent explicit and survives scene serialization.

## GLL Balloons

`gll-tools` is the backend for importing loudspeaker balloon data. GLL models
are available through the Go API, not the scene-JSON directivity union:

```go
model, err := directivity.LoadGLL("speaker.gll", "preset-name")
if err != nil {
	return err
}
source.Directivity = model
```

An empty preset selects the first source definition with balloon data. A
non-empty preset matches a source-definition key or label case-insensitively.
Gains are normalized to the balloon's on-axis response and interpolated by the
GLL backend at the requested direction; the nearest available frequency band is
used.

Inspect an azimuth slice before rendering:

```bash
go run ./cmd/roomplot source-directivity speaker.gll \
  --preset preset-name \
  --freq 1000 \
  --elevation 0 \
  --step-deg 5 \
  --format csv \
  --output /tmp/directivity.csv
```

The runnable [GLL source example](../examples/gll_source) compares front and
rear energy and renders a hybrid IR:

```bash
go run ./examples/gll_source --output /tmp/gll.wav
```

When adding a new serializable directivity type, update the scene marshal and
unmarshal union, JSON Schema, validation tests, and a small scene fixture before
adding a large manufacturer dataset.
