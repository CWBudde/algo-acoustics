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

## Balloon Response Loading

`gll-tools` `Parse` records only the file offset of the balloon responses and
leaves `BalloonData.Responses` empty. The measurements have to be hydrated with
`gll.LoadBalloonResponses` while the reader is still open. `LoadGLL` and
`LoadGLLReader` now do this before returning the model.

Previously the file was closed immediately after parsing, so
`GetResponseAtAngle` always returned nil and `GLLModel.GainLinear` returned 1
for every direction: a GLL source was silently omnidirectional. This is a
behaviour change. Scenes that use a GLL source now produce genuinely
directional results, and their output differs from earlier runs.

`LoadGLLFile` adapts an already-parsed file and has no reader, so it cannot
hydrate the balloon. Its doc comment says so; prefer `LoadGLL` or
`LoadGLLReader` unless the responses are already populated.

## Frequency-Dependent Cardioid

`CardioidModel` uses one order for all frequencies. Real sources are wide at
low frequencies and narrow at high ones, so `FrequencyDependentCardioid`
tabulates the power-cardioid order per band and interpolates it linearly in log
frequency between band centres. Outside the tabulated range the nearest
endpoint order is held.

```go
model, err := directivity.NewFrequencyDependentCardioid(
	geometry.Vec3{X: 1},
	[]float64{125, 500, 2000, 8000},
	[]float64{0.2, 0.6, 1.5, 3.0},
)
if err != nil {
	return err
}
source.Directivity = model
```

The constructor requires ascending positive band frequencies, one order per
band, and non-negative orders. It copies both slices, so later edits to the
caller's slices do not affect the model. `OrderAt(freqHz)` exposes the
interpolated order, which is useful when plotting or comparing patterns. A
model with no bands is omnidirectional, matching `CardioidModel` at order 0.

## Tabulated Balloons

`BalloonDirectivity` holds a directivity pattern sampled on a spherical grid,
one grid per frequency band. It is the general form any measured or synthesised
balloon reduces to.

`SphericalGrid` uses the same convention as the GLL adapter: azimuth is
measured in the XY plane from `+X`, elevation from the XY plane towards `+Z`,
and `+X` is on-axis. The `AzimuthCount` samples span `[0, 360)` degrees with the
wrap point implied; the `ElevationCount` samples span `[-90, +90]` degrees
inclusive of both poles.

`LevelsDB` is band-major and indexed
`band*Grid.PointCount() + Grid.Index(az, el)`, in dB relative to on-axis.
Interpolation happens in the dB domain, which is how balloon data is published
and how GLL itself interpolates: across azimuth with wrapping, across elevation
clamped at the poles, and across frequency in log frequency between the two
bracketing bands, held flat outside the table. Levels are floored at -200 dB so
an exact null stays finite instead of becoming negative infinity.
`InterpolationMode` selects `NearestNeighbor` or `Bilinear` reads between grid
points.

`SampleBalloon` tabulates any `directivity.Model` onto a grid:

```go
grid := directivity.SphericalGrid{AzimuthCount: 72, ElevationCount: 37}

balloon, err := directivity.SampleBalloon(
	model,
	acoustics.Octave8.CenterFreqs,
	grid,
	directivity.Bilinear,
)
if err != nil {
	return err
}
source.Directivity = balloon
```

## Extracting a Balloon from a GLL File

`(*GLLModel).ExtractBalloon` reduces a loaded GLL balloon to a
`BalloonDirectivity`, normalised on-axis exactly as `GLLModel.GainLinear` is:

```go
gllModel, err := directivity.LoadGLL("speaker.gll", "preset-name")
if err != nil {
	return err
}

balloon, err := gllModel.ExtractBalloon(
	acoustics.Octave8.CenterFreqs,
	directivity.SphericalGrid{AzimuthCount: 72, ElevationCount: 37},
	directivity.Bilinear,
)
if err != nil {
	return err
}
source.Directivity = balloon
```

Passing nil bands extracts the file's native frequency grid, which
`BandCenterFrequencies` also reports. GLL files are typically measured at 1/24
octave with around 241 points, far finer than the octave bands the renderer
evaluates, so callers should normally pass `acoustics.Octave8.CenterFreqs`.

The extracted table no longer references the GLL file, so it can be held,
compared, or written out without keeping megabytes of measurement data alive.
Extraction fails with a clear error when the balloon has no loaded responses,
which is the case for a model built by `LoadGLLFile`.

## Known GLL Symmetry Limitation

With `gll-tools` v0.1.1, the version pinned in `go.mod`, balloon lookups are
incorrect for sources whose balloon uses Quarter, Vertical, Horizontal, or
Axial symmetry. The library's internal grid math cases on different symmetry
enum values than `pkg/gll.SymmetryType` defines.

The symptom is off-axis gains that rise above the on-axis level and then
flatline as indices are clamped past the end of the response array, and two
different sources that return identical patterns. Sources with `SymmetryNone`
are unaffected. A fix exists upstream and is expected in the next `gll-tools`
release.

This affects the GLL lookup only. `FrequencyDependentCardioid` and
`BalloonDirectivity` do not use the `gll-tools` grid math and are unaffected,
though a balloon extracted from an affected GLL file inherits the incorrect
values.

## Serialization Scope

The scene JSON directivity union supports only `omni` and `cardioid`.
`FrequencyDependentCardioid`, `BalloonDirectivity`, and `GLLModel` are Go-API
only: a scene that uses one of them cannot currently be marshalled, so build
such scenes in Go rather than loading them from JSON.

When adding a new serializable directivity type, update the scene marshal and
unmarshal union, JSON Schema, validation tests, and a small scene fixture before
adding a large manufacturer dataset.
