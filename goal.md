# goal.md

- **`algo-acoustics`** owns acoustics-domain modeling, scene handling, propagation engines, hybrid rendering, and orchestration.
- **`algo-dsp`** remains algorithm-centric and transport-agnostic, which matches its stated scope boundaries. ([GitHub][1])
- **`algo-pde`** is used where its current strengths apply: fast repeated Poisson/Helmholtz solves on **regular grids** with reusable plans. ([GitHub][2])
- **`gll-tools`** provides loudspeaker directivity/balloon ingestion and export capabilities you can adapt into a source-directivity layer. ([GitHub][3])
- **`wav`** is used as an external codec/IO dependency, which also fits `algo-dsp`’s “no file codecs” boundary. ([GitHub][1])

I could not verify a public `github.com/cwbudde/go-sofa` repository; direct opens returned 404. So I treat `go-sofa` as either local/private/unpublished and design the plan around a stable HRTF adapter interface, with your existing SOFA reader plugged in behind it.

## 1. Project goal

Build a Go-native room acoustics toolkit that can grow in stages:

1. **shoebox / rectangular room simulation**
2. **specular early reflections**
3. **late-field statistical / ray-based response**
4. **hybrid low-frequency + geometric response**
5. **optional binaural rendering using SOFA**
6. **optional loudspeaker directivity from GLL**

That order aligns well with your current libraries: `algo-pde` already supports regular-grid Helmholtz/Poisson solves, while `gll-tools` already decodes balloon/directivity information and `wav` already handles WAV encoding/decoding. ([GitHub][4])

## 2. Non-goals for v1

Do **not** start with:

- arbitrary watertight mesh wave simulation
- GPU/OpenCL porting
- JUCE/desktop UI
- direct code translation from Wayverb
- full diffraction
- real-time rendering

This keeps v1 aligned with what your current stack can support well today: regular-grid PDE solves, offline DSP processing, and structured acoustics modeling. ([GitHub][4])

## 3. Repository shape

Suggested module:

`github.com/cwbudde/algo-acoustics`

Suggested top-level layout:

```text
algo-acoustics/
  README.md
  PLAN.md
  go.mod

  acoustics/
    constants.go
    units.go
    air.go
    bands.go

  geometry/
    vec3.go
    ray.go
    plane.go
    box.go
    triangle.go
    aabb.go
    mesh.go
    intersect.go

  scene/
    scene.go
    material.go
    source.go
    receiver.go
    room.go
    validate.go

  directivity/
    directivity.go
    omni.go
    cardioid.go
    gll.go

  hrtf/
    dataset.go
    lookup.go
    interpolate.go
    sofa_adapter.go

  ir/
    event.go
    buffer.go
    render.go
    normalize.go
    bandrender.go

  ism/
    image.go
    solver.go
    audibility.go

  raytrace/
    tracer.go
    launch.go
    scatter.go
    accumulate.go
    receiver.go

  pde/
    shoebox.go
    modal.go
    sweep.go
    crossover.go

  hybrid/
    combine.go
    align.go
    weighting.go

  metrics/
    metrics.go
    compare.go

  export/
    wav.go
    json.go
    csv.go

  cmd/
    roomir/
    roomplot/
    roombench/

  examples/
    shoebox_mono/
    shoebox_binaural/
    gll_source/
    hybrid_lowfreq/
```

## 4. External dependencies and roles

### `algo-dsp`

Use it for:

- convolution
- IR post-processing
- filtering / weighting
- FFT-domain helpers
- acoustic metrics where already available

This matches its current “production-quality DSP algorithms” scope and its explicit exclusion of file codecs and UI. ([GitHub][5])

### `algo-pde`

Use it for:

- rectangular-room low-frequency transfer functions
- modal field evaluation
- frequency sweeps over receiver points
- future 2D/3D field visualization support

Its current feature set is a very good fit for repeated Helmholtz solves on fixed rectangular grids, especially with its reusable plans and mixed boundary conditions. ([GitHub][4])

### `gll-tools`

Use it for:

- source directivity import
- frequency-dependent polar/balloon lookup
- optional loudspeaker preset metadata
- optional export/inspection CLI during development

Its README explicitly mentions decoding directivity balloon data, frequency responses, CSV export, and 3D balloon mesh export. That makes it a natural source-directivity backend. ([GitHub][6])

### `go-sofa`

Use it for:

- SOFA HRTF/HRIR dataset loading
- metadata and measurement position access
- IR lookup by direction

Since I could not verify the public repo, keep the coupling loose through a narrow interface.

### `wav`

Use it only in:

- `export/wav.go`
- CLI tools/examples
- tests that write reference IR files

The `wav` package supports both decoding and encoding and exposes round-trip metadata/chunk APIs, so it is fine for offline export without polluting the core acoustics packages. ([Go Packages][7])

## 5. Core architecture

The repo should be organized around one central idea:

**scene → propagation engines → event stream → IR/binaural renderer → export/metrics**

### 5.1 Scene model

Core types:

```go
type Scene struct {
    Room      Room
    Materials map[string]Material
    Sources   []Source
    Receivers []Receiver
}

type Room struct {
    Kind     RoomKind // Shoebox, Mesh
    Shoebox  *Shoebox
    Mesh     *Mesh
}

type Material struct {
    Name              string
    AbsorptionByBand  []float64
    ScatteringByBand  []float64
}

type Source struct {
    Position     Vec3
    Orientation  Quaternion
    Directivity  directivity.Model
    GainDB       float64
}

type Receiver struct {
    Position      Vec3
    Orientation   Quaternion
    Type          ReceiverType // Omni, Binaural
    HRTF          hrtf.Dataset
}
```

Keep materials octave-band or fractional-octave-band based from day one. That gives you a stable bridge between geometric acoustics, GLL directivity, and later low-frequency blending.

### 5.2 Event-based IR pipeline

Represent propagation outputs as sparse events first:

```go
type Event struct {
    TimeSeconds     float64
    Amplitude       float64
    Direction       Vec3
    DistanceMeters  float64
    BandGain        []float64
    PhaseRadians    float64
    Kind            EventKind // Direct, Specular, Diffuse, PDE
}
```

Then convert these events into:

- mono IR
- stereo BRIR
- per-band energy envelopes
- JSON/debug output

This lets every engine feed the same renderer.

## 6. Development phases

## Phase 1 — repo bootstrap and domain model

### Goal

Get a clean foundation with no propagation yet.

### Tasks

- Create repo and CI.
- Add package skeleton.
- Define core math/vector types or pick a tiny internal math package.
- Implement `Scene`, `Room`, `Material`, `Source`, `Receiver`.
- Add validation:
  - positive room dimensions
  - absorption/scattering length equals band count
  - source/receiver inside room
  - HRTF presence only for binaural receivers

- Write README and package docs.

### Deliverables

- `scene` package
- `acoustics/bands`
- validation tests
- first CLI: `roomir validate scene.json`

### Exit criteria

- You can define a shoebox room in JSON and validate it successfully.

## Phase 2 — mono shoebox image-source engine

### Goal

Produce useful early reflections quickly.

### Tasks

- Implement shoebox-only image-source method.
- Support:
  - direct sound
  - specular reflections up to configurable order
  - band-dependent wall absorption
  - source/receiver distance attenuation
  - optional source directivity factor

- Add audibility/path filtering to avoid duplicate/impossible paths.
- Emit sparse `[]Event`.
- Render events to mono IR.

### Deliverables

- `ism/solver.go`
- `ir/render.go`
- example `shoebox_mono`

### Exit criteria

- Generate plausible direct + first/second-order reflection IRs for rectangular rooms.
- Compare time-of-flight results to analytical expectations.

### Notes

This should be your first audible milestone.

## Phase 3 — metrics and regression harness

### Goal

Make the acoustics output measurable before adding complexity.

### Tasks

- Add optional integration with `algo-dsp` IR metrics.
- Build a small regression suite:
  - empty shoebox
  - absorptive shoebox
  - symmetry cases
  - source/receiver swap invariance where applicable

- Export IRs as WAV via `wav`.
- Create golden metrics:
  - direct arrival
  - peak ordering
  - EDT/T20/T30 when meaningful
  - C50/C80 for synthetic tests

### Deliverables

- `metrics/compare.go`
- test fixtures
- `cmd/roombench`

### Exit criteria

- Stable tests that fail when path timing or gain breaks. `algo-dsp` already positions itself as a reusable DSP library, so reusing it for this validation layer is a good fit. ([GitHub][5])

## Phase 4 — ray-traced late field

### Goal

Add a scalable late-energy model without touching hard PDE problems yet.

### Tasks

- Implement ray launch over sphere:
  - Fibonacci sphere or stratified directions

- Add triangle/plane/box intersections
- Add per-bounce:
  - absorption
  - scattering
  - diffuse redistribution

- Add receiver hit model:
  - sphere receiver
  - capture radius / angular weighting

- Accumulate either:
  - impulse events, or
  - energy histograms per time bin and band

- Convert histogram to stochastic late IR.

### Deliverables

- `raytrace/`
- `hybrid/late_from_rays.go`
- example `shoebox_late`

### Exit criteria

- Smooth late decay that changes plausibly with absorption and room size.

## Phase 5 — hybrid early + late IR

### Goal

Make the first genuinely useful room simulator.

### Tasks

- Combine:
  - direct sound
  - early specular reflections from ISM
  - late diffuse/ray tail

- Add transition control:
  - time-based crossover
  - order-based crossover
  - energy-based crossover

- Add anti-double-counting rules:
  - early reflections removed from ray bins
  - optional smoothing in crossover region

- Add bandwise rendering.

### Deliverables

- `hybrid/combine.go`
- CLI `roomir render`
- reference examples and plots

### Exit criteria

- One-command generation of a realistic mono room IR for a shoebox room.

This should be the first public release candidate.

## Phase 6 — GLL-based source directivity

### Goal

Make the simulator source-aware for real loudspeaker radiation patterns.

### Tasks

- Define directivity interface:

```go
type Model interface {
    GainLinear(freqHz float64, dir Vec3) float64
}
```

- Build `directivity/gll.go` adapter around `gll-tools`.
- Map GLL balloon data to:
  - azimuth/elevation lookup
  - frequency interpolation
  - optional mesh-based visualization data

- Add source-orientation transform from room coordinates into source local coordinates.
- Use directivity in:
  - direct path
  - reflected path launch weighting
  - ray launch power distribution

### Deliverables

- `directivity/gll.go`
- example `gll_source`
- tests for coordinate-frame correctness

### Exit criteria

- Rotating a loudspeaker changes the IR and reflected energy in expected directions.

`gll-tools` already exposes directivity balloon and frequency-response concepts, which makes this integration concrete rather than speculative. ([GitHub][6])

## Phase 7 — binaural/HRTF rendering

### Goal

Turn events and IRs into BRIR output.

### Tasks

- Define minimal HRTF dataset interface:

```go
type Dataset interface {
    SampleRate() int
    Lookup(direction Vec3) (left, right []float64, delaySeconds float64, err error)
}
```

- Wrap your existing SOFA loader behind that interface.
- Add nearest-neighbor lookup first.
- Add optional interpolation later:
  - spherical interpolation
  - barycentric interpolation on measurement mesh

- Render each event into stereo by convolving with HRIR pair.
- Add head orientation support.
- Resample HRIR or event-render buffer as needed through DSP helpers.

### Deliverables

- `hrtf/sofa_adapter.go`
- `ir/render_binaural.go`
- example `shoebox_binaural`

### Exit criteria

- BRIR export from a shoebox scene to stereo WAV.

Because the `go-sofa` repo was not publicly verifiable, keep this package boundary deliberately narrow and avoid leaking its internal types into the main repo API.

## Phase 8 — low-frequency shoebox solver via `algo-pde`

### Goal

Add a physically grounded low-frequency engine where `algo-pde` is strongest.

### Tasks

- Implement a shoebox low-frequency module using `algo-pde` Helmholtz solves.
- Sweep frequencies across a configurable range, for example 20–300 Hz.
- For each frequency:
  - assemble source excitation on regular grid
  - solve field under chosen boundary conditions
  - sample receiver transfer

- Create a complex transfer function `H(f)`.
- Convert to time-domain low-frequency IR with inverse FFT / hermitian reconstruction.
- Align and blend with geometric IR above crossover.

### Deliverables

- `pde/shoebox.go`
- `pde/sweep.go`
- `hybrid/crossover.go`
- example `hybrid_lowfreq`

### Exit criteria

- Clear modal behavior in rectangular rooms.
- Smooth crossover to geometric acoustics above threshold.

This is exactly the sort of repeated regular-grid Helmholtz use case `algo-pde` is already built for. ([GitHub][4])

## Phase 9 — calibration and validation against known room-acoustics behavior

### Goal

Make the simulator trustworthy.

### Tasks

- Validate shoebox modal frequencies against analytical formulas.
- Validate image-source path timing analytically.
- Validate directivity rotations with synthetic patterns.
- Validate HRTF lookup using known frontal/lateral positions.
- Compare hybrid decay trends against expected changes from absorption and volume.
- Build benchmark corpus:
  - tiny room
  - control room
  - lecture room
  - strongly directional PA source case

### Deliverables

- `testdata/rooms/`
- `testdata/gll/`
- `testdata/sofa/`
- benchmark report generator

### Exit criteria

- Reproducible benchmark suite with target tolerances.

## Phase 10 — mesh geometry and non-shoebox scene support

### Goal

Open the door to more realistic geometry without pretending to have full wave physics yet.

### Tasks

- Add triangulated mesh ingestion.
- Add BVH or uniform-grid acceleration.
- Reuse mesh for ray tracing first.
- Optionally add image-source support for planar subsets only.
- Keep PDE low-frequency mode limited to shoebox/rectangular rooms until a real non-rectangular method is ready.

### Deliverables

- `geometry/mesh.go`
- `geometry/bvh.go`
- mesh-capable ray tracing
- `scene.RoomKindMesh`

### Exit criteria

- Ray-based late field works on imported room meshes.

## Phase 11 — future research track

Only after earlier phases succeed:

- diffraction
- frequency-dependent scattering models
- non-rectangular PDE / iterative Helmholtz
- voxel/immersed-boundary methods
- GPU acceleration
- real-time preview
- browser demo

`algo-pde` today is regular-grid oriented, so treat arbitrary-geometry low-frequency acoustics as future research, not baseline engineering. ([GitHub][4])

## 7. Recommended API design

Keep the public API small:

```go
type Renderer struct {
    Early   EarlyEngine
    Late    LateEngine
    LowFreq LowFreqEngine
    Hybrid  HybridConfig
}

func (r *Renderer) RenderMono(scene *scene.Scene, cfg RenderConfig) ([]float64, error)
func (r *Renderer) RenderStereo(scene *scene.Scene, cfg RenderConfig) (left, right []float64, err error)
```

Then define engine interfaces:

```go
type EarlyEngine interface {
    Generate(scene *scene.Scene, cfg RenderConfig) ([]ir.Event, error)
}

type LateEngine interface {
    Generate(scene *scene.Scene, cfg RenderConfig) ([]ir.Event, error)
}

type LowFreqEngine interface {
    Transfer(scene *scene.Scene, cfg RenderConfig) (*TransferFunction, error)
}
```

This lets you start with:

- `ISMEngine`
- `RayLateEngine`
- `PDELowFreqEngine`

without locking the repo to one method.

## 8. Configuration model

Use a JSON/YAML scene file for examples and CLI tools.

Example sketch:

```json
{
  "sampleRate": 48000,
  "room": {
    "kind": "shoebox",
    "dimensions": [6.0, 4.5, 2.8],
    "materials": {
      "xMin": "brick",
      "xMax": "brick",
      "yMin": "curtain",
      "yMax": "curtain",
      "zMin": "carpet",
      "zMax": "plaster"
    }
  },
  "materials": {
    "brick": { "absorption": [0.02, 0.02, 0.03, 0.04, 0.05, 0.07] },
    "curtain": { "absorption": [0.05, 0.1, 0.35, 0.55, 0.7, 0.7] },
    "carpet": { "absorption": [0.02, 0.06, 0.15, 0.25, 0.45, 0.65] },
    "plaster": { "absorption": [0.01, 0.02, 0.03, 0.04, 0.05, 0.05] }
  },
  "sources": [
    {
      "position": [1.2, 1.0, 1.2],
      "directivity": { "type": "gll", "file": "speaker.gll", "preset": "flat" }
    }
  ],
  "receivers": [
    {
      "position": [3.5, 2.2, 1.2],
      "type": "binaural",
      "hrtf": { "type": "sofa", "file": "hrtf.sofa" }
    }
  ]
}
```

## 9. Command-line tools

Start with three CLIs:

### `roomir`

Main offline renderer.
Commands:

- `validate`
- `render`
- `render-stereo`
- `dump-events`

### `roomplot`

Diagnostic plotting/export.
Commands:

- `paths`
- `energy-decay`
- `freq-response`
- `source-directivity`

### `roombench`

Benchmark and validation tool.
Commands:

- `run`
- `compare`
- `report`

Keep plotting/export optional so the core library stays clean.

## 10. Testing strategy

Use five layers of tests.

### Unit tests

- vector math
- ray-plane / ray-box / ray-triangle intersections
- material interpolation
- HRIR lookup
- GLL directional lookup

### Property tests

- symmetry in shoebox reflection timing
- monotonic decay with increasing absorption
- source rotation changes angular gain but not room geometry

### Analytical tests

- modal frequency checks in shoebox rooms
- image-source path lengths
- inverse-square attenuation sanity

### Golden tests

- rendered mono IR hashes/tolerant comparisons
- stereo BRIR regression
- CSV/JSON event dumps

### Performance tests

- allocations per render
- rays/sec
- solver latency across grid sizes

## 11. Performance priorities

Do not optimize everything at once. Prioritize:

1. zero-allocation hot loops for ray tracing
2. reusable scratch buffers
3. cache-friendly scene layout
4. plan reuse in `algo-pde`
5. optional parallelism for rays and frequency sweeps

The nice part is that `algo-pde` already emphasizes reusable plans and zero-allocation solve paths, so you can mirror that design language in `algo-acoustics`. ([GitHub][4])

## 12. Documentation plan

Create these docs early:

- `README.md` — what the repo does
- `PLAN.md` — roadmap
- `docs/architecture.md`
- `docs/scene-format.md`
- `docs/hybrid-rendering.md`
- `docs/directivity-gll.md`
- `docs/hrtf-sofa.md`
- `docs/validation.md`

This will matter because the repo spans acoustics, geometry, DSP, and PDEs.

## 13. Suggested milestone schedule

### Milestone A — “first audible result”

- scene model
- shoebox ISM
- mono IR renderer
- WAV export

### Milestone B — “useful room simulator”

- late ray tail
- hybrid mono IR
- metrics and regression

### Milestone C — “loudspeaker-aware simulation”

- GLL directivity
- orientation-aware source model

### Milestone D — “binaural simulator”

- SOFA-backed HRTF adapter
- stereo BRIR export

### Milestone E — “physics-enhanced low end”

- `algo-pde` shoebox low-frequency solver
- crossover blending

### Milestone F — “geometry expansion”

- mesh scenes
- accelerated ray tracing

## 14. First issues to open

I would create these issues immediately:

1. Bootstrap `algo-acoustics` repo and CI
2. Define scene/material/source/receiver model
3. Add shoebox validator and JSON schema
4. Implement event-based IR representation
5. Implement shoebox ISM direct + first-order reflections
6. Add mono IR renderer
7. Add WAV export via `github.com/cwbudde/wav`
8. Add regression tests for shoebox path timing
9. Define directivity interface
10. Implement omnidirectional source model
11. Add GLL directivity adapter prototype
12. Define HRTF dataset interface
13. Add SOFA adapter wrapper
14. Implement ray-based late field prototype
15. Implement hybrid early/late combiner
16. Implement low-frequency shoebox PDE prototype on `algo-pde`
17. Add benchmark suite and comparison report

## 15. My recommended implementation order

If you want the most value per week of work:

**Week 1–2**

- repo bootstrap
- scene model
- event IR abstraction
- shoebox ISM
- WAV export

**Week 3–4**

- metrics
- regression harness
- CLI polish
- first release

**Week 5–7**

- ray-traced late field
- hybrid combiner

**Week 8–9**

- GLL directivity integration

**Week 10–11**

- HRTF/SOFA binaural rendering

**Week 12+**

- `algo-pde` low-frequency module
- modal validation
- crossover tuning

That ordering gets you something useful before the hardest work begins.

## 16. Final recommendation

The cleanest version of this project is:

- **`algo-acoustics`**: acoustics orchestration and simulation
- **`algo-dsp`**: DSP kernels and metrics
- **`algo-pde`**: rectangular low-frequency physics
- **`gll-tools`**: source directivity adapter
- **`go-sofa`**: HRTF adapter behind an interface
- **`wav`**: offline file export only

That architecture respects the stated boundaries of `algo-dsp`, leverages the existing strengths of `algo-pde`, and directly incorporates the capabilities already present in `gll-tools` and `wav`. ([GitHub][5])

I can turn this into a concrete `PLAN.md` draft for the new repo next.

[1]: https://github.com/cwbudde/algo-dsp?utm_source=chatgpt.com "GitHub - CWBudde/algo-dsp: Production-quality DSP (Digital Signal ..."
[2]: https://github.com/CWBudde/algo-pde?utm_source=chatgpt.com "GitHub - CWBudde/algo-pde: WORK IN PROGRESS"
[3]: https://github.com/CWBudde/gll-tools?utm_source=chatgpt.com "GitHub - CWBudde/gll-tools: WORK IN PROGRESS"
[4]: https://github.com/CWBudde/algo-pde/blob/main/README.md "algo-pde/README.md at main · CWBudde/algo-pde · GitHub"
[5]: https://github.com/CWBudde/algo-dsp/blob/main/README.md "algo-dsp/README.md at main · CWBudde/algo-dsp · GitHub"
[6]: https://github.com/CWBudde/gll-tools/blob/main/README.md "gll-tools/README.md at main · CWBudde/gll-tools · GitHub"
[7]: https://pkg.go.dev/github.com/cwbudde/wav "wav package - github.com/cwbudde/wav - Go Packages"
