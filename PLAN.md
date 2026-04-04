# PLAN.md — `algo-acoustics` Implementation Roadmap

> **Architecture in one sentence:**
> `scene → propagation engines → event stream → IR/binaural renderer → export/metrics`

For implementation ideas, check <https://github.com/reuk/wayverb/>.

---

## Phase 0 — Scaffolding

### Milestone: empty-but-buildable repo

> Everything needed to start writing real code: module, CI, linting, directory skeleton, and docs skeleton.

### 0.1 Repository initialization

- [x] Create `go.mod` with module path `github.com/cwbudde/algo-acoustics`
  - Go 1.25 (current toolchain)
- [x] Add `.gitignore` (Go standard: binaries, `vendor/`, test cache)
- [x] Create `LICENSE` file (MIT)
- [x] Initialize `README.md` with one-paragraph description and a placeholder badge row

### 0.2 CI pipeline

- [x] Add `.github/workflows/tests.yml` — triggers unit tests, lint, and format check jobs
  - [x] Job: `go test ./...` (matrix: Go 1.24.x, 1.25.x)
  - [x] Job: `golangci-lint` via `golangci-lint-action@v6`
  - [x] Job: `treefmt` format check
- [x] Create `.golangci.yml` — `default: all` with acoustics-appropriate disables
- [x] Create `treefmt.toml` — gofumpt + gci + prettier formatters
- [x] Add `justfile` with targets: `build`, `test`, `lint`, `fmt`, `ci`, `clean`

### 0.3 Directory skeleton

- [x] Create the top-level package directories, each with a single `doc.go` placeholder so the package boundaries exist before implementation starts:
  - `acoustics/`
  - `geometry/`
  - `scene/`
  - `directivity/`
  - `hrtf/`
  - `ir/`
  - `ism/`
  - `raytrace/`
  - `pde/`
  - `hybrid/`
  - `metrics/`
  - `export/`
- [x] Create the CLI stub directories that will later hold the first commands:
  - `cmd/roomir/`
  - `cmd/roomplot/`
  - `cmd/roombench/`
- [x] Create the example workspace with one directory per scenario:
  - `examples/shoebox_mono/`
  - `examples/shoebox_binaural/`
  - `examples/gll_source/`
  - `examples/hybrid_lowfreq/`
- [x] Create the test fixture layout used by later validation and integration tests:
  - `testdata/rooms/`
  - `testdata/gll/`
  - `testdata/sofa/`
- [x] Keep this phase free of implementation logic; the only files created here should be placeholders and future-ready folder scaffolding.
- [x] Confirm the tree is ready for Phase 1 by ensuring the directory names match the package and command names used throughout the rest of the plan.

### 0.4 External dependency wiring

- [x] Phase 0.3 complete; next up is wiring external dependencies.
- [x] Add `github.com/cwbudde/algo-dsp` to `go.mod`
- [x] Add `github.com/cwbudde/algo-pde` to `go.mod`
- [x] Add `github.com/cwbudde/gll-tools` to `go.mod`
- [x] Add `github.com/cwbudde/wav` to `go.mod`
- [x] Run `go mod tidy` and commit `go.sum`
- [x] Add a `tools/tools.go` file (`//go:build tools`) declaring CLI tool imports (staticcheck, golangci-lint wrapper)

> **Insight:** Keep `go-sofa` out of `go.mod` for now. It will be wired behind an interface in Phase 7 once the adapter boundary is clear.

### 0.5 Documentation skeleton

- [x] Create `docs/` directory
- [x] Create stub files (one sentence each):
  - `docs/architecture.md`
  - `docs/scene-format.md`
  - `docs/hybrid-rendering.md`
  - `docs/directivity-gll.md`
  - `docs/hrtf-sofa.md`
  - `docs/validation.md`
- [x] Add `PLAN.md` link in `README.md`

---

## Phase 1 — Domain Model and Scene Representation

### Milestone A (partial): `scene`, `acoustics/bands`, `geometry` types compile and validate

> No propagation yet. Just the types, constants, and validation that every later phase depends on.

### 1.1 Physical constants and units (`acoustics/`)

- [x] Add `constants.go`
  - [x] `SpeedOfSound` (343.0 m/s at 20 °C)
  - [x] `AirDensity` (1.204 kg/m³)
  - [x] `ReferencePressurePa` (20 µPa)
- [x] Add `units.go`
  - [x] `DecibelToLinear(dB float64) float64`
  - [x] `LinearToDecibel(lin float64) float64`
  - [x] `MetersToSamples(m, c float64, sampleRate int) int`
  - [x] `SamplesToSeconds(n, sampleRate int) float64`
- [x] Add `air.go`
  - [x] `SpeedOfSoundAt(tempCelsius float64) float64` (Cramer formula)
  - [x] `CharacteristicImpedance(tempCelsius float64) float64`
- [x] Add `bands.go`
  - [x] Define `BandSpec` struct (center freqs, lower/upper edges)
  - [x] Provide `Octave6` preset (125, 250, 500, 1k, 2k, 4k Hz)
  - [x] Provide `Octave8` preset (63 Hz – 8 kHz)
  - [x] `BandSpec.BandCount() int` (method on struct)
  - [x] Unit tests for all helpers (18 tests, all passing)

### 1.2 Core math / geometry types (`geometry/`)

- [x] Add `vec3.go` — `Vec3`, `Add/Sub/Scale/Dot/Cross/Norm/Normalize/Distance/Neg`, `Vec3Zero/One`
- [x] Add `ray.go` — `Ray`, `NewRay` (normalises direction), `Ray.At`
- [x] Add `plane.go` — `Plane`, `NewPlaneFromPointNormal`, `SideOf`, `Reflect`
- [x] Add `box.go` — `Box`, `NewBox` (normalises corners), `Contains`, `Center`, `Dimensions`, `Volume`
- [x] Add `intersect.go` — `RayPlane`, `RayBox` (slab method; AABB merged here, no separate file)
- [x] Add `triangle.go` — `Triangle`, `Normal`, `Centroid`, `Area`, `RayTriangle` (Möller–Trumbore, double-sided)
- [x] Add `mesh.go` — `Mesh{Triangles}`, `BoundingBox` (BVH/OBJ deferred to Phase 10)
- [x] Unit tests: 35 tests across all types, intersections, and edge cases

> **Insight:** Keep geometry types value types (structs, not pointers) for cache efficiency in hot loops. Reuse existing `algo-pde` naming conventions where they overlap so both libraries feel consistent.

### 1.3 Quaternion type (`geometry/`)

- [x] Add `quaternion.go` — `Quaternion{W,X,Y,Z}`, `QuatIdentity`, `QuatFromAxisAngle`, `Rotate`, `Mul`, `Conj`
- [x] Unit tests: identity, 90°/180° rotation, length preservation, Mul identity, round-trip

### 1.4 Scene model (`scene/`)

- [x] Add `room.go`
  - [x] `RoomKind` enum: `RoomKindShoebox`, `RoomKindMesh`
  - [x] `Shoebox` struct: `Width, Depth, Height float64`; `WallMaterials [6]string`
  - [x] `Room` struct: `Kind RoomKind`, `Shoebox *Shoebox`, `Mesh *geometry.Mesh`
- [x] Add `material.go`
  - [x] `Material` struct: `Name string`, `AbsorptionByBand []float64`, `ScatteringByBand []float64`
  - [x] `Material.AbsorptionAt(bandIndex int) float64`
  - [x] Default `MaterialFullyAbsorptive()` and `MaterialFullyReflective()` constructors
- [x] Add `source.go`
  - [x] `Source` struct: `Position Vec3`, `Orientation Quaternion`, `GainDB float64`, `Directivity directivity.Model`
  - [x] `Source.DirectionTo(target Vec3) Vec3` (transforms into source-local frame)
- [x] Add `receiver.go`
  - [x] `ReceiverType` enum: `ReceiverOmni`, `ReceiverBinaural`
  - [x] `Receiver` struct: `Position Vec3`, `Orientation Quaternion`, `Type ReceiverType`, `HRTF hrtf.Dataset`
- [x] Add `scene.go`
  - [x] `Scene` struct tying `Room`, `Materials map[string]Material`, `Sources []Source`, `Receivers []Receiver`, `BandSpec acoustics.BandSpec`, `SampleRate int`
- [x] Add `validate.go`
  - [x] `Validate(s *Scene) error`
  - [x] Check: room dimensions > 0
  - [x] Check: all referenced material names exist in `Materials`
  - [x] Check: `AbsorptionByBand` length matches `BandSpec` band count
  - [x] Check: all source positions inside room bounding box
  - [x] Check: all receiver positions inside room bounding box
  - [x] Check: binaural receivers have non-nil HRTF
  - [x] Check: `SampleRate` > 0
  - [x] Unit tests for each validation rule, both passing and failing cases

> **Insight:** Returning a multi-error slice (not just the first error) from `Validate` will be friendlier for the CLI `validate` command.

### 1.5 JSON scene loading (`scene/`)

- [x] Add `load.go`
  - [x] `LoadScene(r io.Reader) (*Scene, error)` using `encoding/json`
  - [x] `LoadSceneFile(path string) (*Scene, error)`
  - [x] JSON struct tags on all scene types
- [x] Create `testdata/rooms/shoebox_simple.json` — example 6×4.5×2.8 m room
- [x] Unit test: round-trip marshal/unmarshal preserves all fields

### 1.6 Directivity interface (stub, `directivity/`)

- [x] Add `directivity.go`
  - [x] `Model` interface: `GainLinear(freqHz float64, dir geometry.Vec3) float64`
- [x] Add `omni.go`
  - [x] `OmniModel` struct implementing `Model` — always returns 1.0
- [x] Add `cardioid.go`
  - [x] `CardioidModel` struct: `Axis Vec3`, `OrderN float64`
  - [x] Implements `Model` via `((1 + cos θ)/2)^N`
  - [x] Unit test: on-axis = 1.0, rear = 0.0 for N=1

### 1.7 HRTF interface (stub, `hrtf/`)

- [x] Add `dataset.go`
  - [x] `Dataset` interface: `SampleRate() int`, `Lookup(direction geometry.Vec3) (left, right []float64, delaySeconds float64, err error)`
- [x] Add `lookup.go` (stub — nearest-neighbor scaffold, impl in Phase 7)

### 1.8 First CLI: `roomir validate`

- [x] Add `cmd/roomir/main.go` with `cobra` or `flag`-based CLI
  - [x] Sub-command `validate <scene.json>`
  - [x] Prints ✓ or error list to stdout
  - [x] Exit code 1 on validation failure
- [x] Add `cmd/roomir/validate.go` wiring `scene.LoadSceneFile` + `scene.Validate`
- [x] Smoke test in CI: `roomir validate testdata/rooms/shoebox_simple.json`

> **Insight:** Cobra is a good fit now that the first subcommand exists and the command wiring is testable. Keep the surface minimal until more subcommands justify additional structure.

---

## Phase 2 — Mono Shoebox Image-Source Engine

### Milestone A (complete): "first audible result"

> Produce a usable sparse event list from the ISM and render to a mono IR WAV file.

### 2.1 IR event types (`ir/`)

- [x] Add `event.go`
  - [x] `EventKind` enum: `EventDirect`, `EventSpecular`, `EventDiffuse`, `EventPDE`
  - [x] `Event` struct: `TimeSeconds float64`, `Amplitude float64`, `Direction geometry.Vec3`, `DistanceMeters float64`, `BandGain []float64`, `PhaseRadians float64`, `Kind EventKind`
- [x] Add `buffer.go`
  - [x] `Buffer` struct wrapping `[]float64` with `SampleRate int`
  - [x] `Buffer.Len() int`, `Buffer.Duration() float64`
  - [x] `NewBuffer(sampleRate int, durationSeconds float64) *Buffer`

### 2.2 ISM shoebox image-source method (`ism/`)

- [x] Add `image.go`
  - [x] `ImageSource` struct: `Position geometry.Vec3`, `Order int`, `WallMask uint8`
  - [x] `GenerateImageSources(src geometry.Vec3, room *scene.Shoebox, maxOrder int) []ImageSource`
    - [x] Enumerate reflection orders up to `maxOrder` along ±X, ±Y, ±Z axes
    - [x] Return image-source positions for all combinations
- [x] Add `audibility.go`
  - [x] `IsAudible(imgSrc ImageSource, receiver geometry.Vec3) bool`
    - [x] Ensure reflection path does not pass through room boundaries
    - [x] Degenerate path filter (source == receiver edge case)
- [x] Add `solver.go`
  - [x] `ISMConfig` struct: `MaxOrder int`, `SpeedOfSound float64`, `BandSpec acoustics.BandSpec`
  - [x] `ISMSolver` struct
  - [x] `ISMSolver.Solve(sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error)`
    - [x] Compute direct-path event
    - [x] For each image source at each order:
      - [x] Compute path length and time-of-flight
      - [x] Compute distance attenuation (1/r)
      - [x] Accumulate band-dependent wall absorption from hit walls
      - [x] Apply source directivity gain
      - [x] Emit `ir.Event` if audible
    - [x] Sort events by `TimeSeconds`

> **Insight:** For shoebox ISM, the image-source at reflection order `(nx, ny, nz)` has position:
> `x_img = (-1)^nx * src.X + nx * 2*room.Width` (and analogously for Y, Z).
> Enumerate by integer triples `(nx, ny, nz)` with `|nx|+|ny|+|nz| <= maxOrder`.

### 2.3 Mono IR renderer (`ir/`)

- [x] Add `render.go`
  - [x] `RenderConfig` struct: `SampleRate int`, `DurationSeconds float64`, `BandSpec acoustics.BandSpec`
  - [x] `RenderMono(events []ir.Event, cfg RenderConfig) (*ir.Buffer, error)`
    - [x] Convert each event's `TimeSeconds` to sample index
    - [x] Add amplitude to buffer (additive sparse-to-dense)
    - [x] Apply per-band gains by summing across bands (mono sum)
- [x] Add `normalize.go`
  - [x] `NormalizePeak(buf *ir.Buffer) float64` — normalize to peak = 1.0, return scale factor
  - [x] `NormalizeRMS(buf *ir.Buffer, targetRMS float64) float64`

### 2.4 WAV export (`export/`)

- [x] Add `wav.go`
  - [x] `WriteMonoWAV(path string, buf *ir.Buffer) error` using `github.com/cwbudde/wav`
  - [x] `WriteStereoWAV(path string, left, right *ir.Buffer) error`
  - [x] `Float64ToInt16(samples []float64) []int16` helper
- [x] Unit test: write and re-read 100 samples, verify round-trip fidelity

### 2.5 Bandwise IR rendering (`ir/`)

- [x] Add `bandrender.go`
  - [x] `RenderBand(events []ir.Event, bandIndex int, cfg RenderConfig) (*ir.Buffer, error)`
  - [x] `SumBands(bands []*ir.Buffer) *ir.Buffer`

### 2.6 Example: `shoebox_mono`

- [x] Add `examples/shoebox_mono/main.go`
  - [x] Hardcoded 6×4.5×2.8 m shoebox scene
  - [x] Source at (1.2, 1.0, 1.2), receiver at (3.5, 2.2, 1.2)
  - [x] Run ISM up to order 3
  - [x] Render mono IR, write `output.wav`
- [x] Verify output file is non-silent and has a direct-path spike at the expected sample

### 2.7 CLI: `roomir render`

- [x] Add `cmd/roomir/render.go`
  - [x] Sub-command `render <scene.json> -o <output.wav> --max-order 3 --duration 1.5`
  - [x] Validate scene, run ISM, render mono, export WAV
  - [x] Print event count and render duration to stderr

### 2.8 Analytical tests for ISM

- [x] Test: direct path time equals `distance / speed_of_sound`
- [x] Test: first reflection from floor at `(0, 0, -src.Z)` image source — path length matches geometry
- [x] Test: source and receiver swap produces same path lengths (reciprocity)
- [x] Test: doubling room dimensions doubles first reflection arrival time
- [x] Test: zero absorption → amplitude sum matches theoretical reverberant level

---

## Phase 3 — Metrics and Regression Harness

### Milestone B (partial): measurable and regression-testable output

> Add acoustic metrics, export helpers, and a golden-test harness before adding complexity.

### 3.1 Acoustic metrics (`metrics/`)

- [x] Add `metrics.go`
  - [x] `T60FromDecaySlope(buf *ir.Buffer) (float64, error)` — linear regression on energy decay
  - [x] `EDT(buf *ir.Buffer) (float64, error)` — early decay time (0 to −10 dB)
  - [x] `T20(buf *ir.Buffer) (float64, error)` — −5 to −25 dB decay
  - [x] `T30(buf *ir.Buffer) (float64, error)` — −5 to −35 dB decay
  - [x] `C50(buf *ir.Buffer) (float64, error)` — clarity 50 ms
  - [x] `C80(buf *ir.Buffer) (float64, error)` — clarity 80 ms
  - [x] `D50(buf *ir.Buffer) (float64, error)` — definition

> **Insight:** Delegate convolution and filter-bank operations to `algo-dsp`; implement only the metric extraction formulas here. Check `algo-dsp` for existing EDT/T30 helpers before writing new ones.

- [x] Add `compare.go`
  - [x] `MetricResult` struct: `Name string`, `Expected, Actual, Tolerance float64`, `Pass bool`
  - [x] `CompareMetric(name string, expected, actual, tolerance float64) MetricResult`
  - [x] `CompareAll(results []MetricResult) bool`
  - [x] `PrintReport(results []MetricResult, w io.Writer)`

### 3.2 JSON and CSV export (`export/`)

- [x] Add `json.go`
  - [x] `WriteEventsJSON(path string, events []ir.Event) error`
  - [x] `WriteMetricsJSON(path string, results []metrics.MetricResult) error`
- [x] Add `csv.go`
  - [x] `WriteEventsCSV(path string, events []ir.Event) error`
  - [x] `WriteMetricsCSV(path string, results []metrics.MetricResult) error`

### 3.3 Regression test fixtures

- [x] Create `testdata/rooms/shoebox_absorptive.json` (high absorption walls)
- [x] Create `testdata/rooms/shoebox_symmetric.json` (source and receiver equidistant from center)
- [x] Create `testdata/rooms/shoebox_livelier.json` (very low absorption)
- [x] Golden test: run ISM on each fixture, serialize events to JSON, compare against committed baseline
  - [x] Tolerance: arrival time ±0.05 ms, amplitude ±0.5 dB

### 3.4 Property tests

- [x] Property: absorptive room has shorter T60 than reflective room
- [x] Property: larger room has longer first reflection time
- [x] Property: source–receiver distance increase reduces direct path amplitude by 6 dB/double
- [x] Property: swapping source and receiver positions yields same set of path lengths (reciprocity)
- [x] Property: monotonic decay — energy in each time window is non-increasing on average after direct sound

### 3.5 `roombench` CLI

- [x] Add `cmd/roombench/main.go`
  - [x] Sub-command `run` — runs all regression fixtures, prints pass/fail
  - [x] Sub-command `compare <baseline.json> <current.json>` — diff two metric reports
  - [x] Sub-command `report` — generates a text summary table
- [x] Wire `roombench run` into CI as a separate job

### 3.6 CLI: `roomir dump-events`

- [x] Add sub-command `dump-events <scene.json> -o events.json --format json|csv`
- [x] Useful for debugging and generating regression baselines

---

## Phase 4 — Ray-Traced Late Field

### Milestone B (complete): smooth statistical reverb tail

> Add a scalable Monte Carlo late-energy model. No PDE yet.

### 4.1 Ray launch (`raytrace/`)

- [x] Add `launch.go`
  - [x] `LaunchConfig` struct: `NumRays int`, `MaxBounces int`, `MaxTimeSeconds float64`, `SpeedOfSound float64`
  - [x] `FibonacciSphere(n int) []geometry.Vec3` — uniformly distributed unit vectors
  - [x] `StratifiedDirections(n int) []geometry.Vec3` — jittered stratified grid on sphere
  - [x] `LaunchRays(src geometry.Vec3, cfg LaunchConfig) []geometry.Ray`

> **Insight:** Fibonacci sphere formula: `θ = 2π·i/φ²`, `z = 1 − (2i+1)/n` where `φ = (1+√5)/2`. This gives a near-uniform distribution without random noise.

### 4.2 Scene intersection for ray tracing (`raytrace/`)

- [x] Add `tracer.go`
  - [x] `ShoeboxTracer` struct: holds room walls as 6 planes
  - [x] `ShoeboxTracer.NextHit(r geometry.Ray) (geometry.Vec3, geometry.Vec3, int, bool)` — returns hit point, normal, wall index, hit flag
  - [x] `Tracer` interface: `NextHit(r geometry.Ray) (hitPoint, normal Vec3, wallIdx int, ok bool)`

### 4.3 Scatter and absorption per bounce (`raytrace/`)

- [x] Add `scatter.go`
  - [x] `ScatterConfig` struct: per-band absorption and scattering coefficients
  - [x] `SpecularReflect(dir, normal geometry.Vec3) geometry.Vec3`
  - [x] `DiffuseReflect(normal geometry.Vec3, rng *rand.Rand) geometry.Vec3` — cosine-weighted hemisphere
  - [x] `SelectReflection(scatterCoeff float64, dir, normal geometry.Vec3, rng *rand.Rand) geometry.Vec3`
  - [x] `AbsorbedFraction(absorptionByBand []float64) []float64` — per-band energy remaining after bounce

### 4.4 Receiver hit model (`raytrace/`)

- [x] Add `receiver.go`
  - [x] `SphereReceiver` struct: `Center geometry.Vec3`, `Radius float64`
  - [x] `SphereReceiver.Intersects(r geometry.Ray, tMin, tMax float64) (t float64, hit bool)`
  - [x] `SphereReceiver.AngularWeight(dir geometry.Vec3) float64` — optional cosine weighting

### 4.5 Energy accumulation (`raytrace/`)

- [x] Add `accumulate.go`
  - [x] `HistogramBin` struct: per-band energy, time
  - [x] `EnergyHistogram` struct: `Bins []HistogramBin`, `BinDuration float64`, `BandCount int`
  - [x] `NewEnergyHistogram(duration, binDuration float64, bandCount int) *EnergyHistogram`
  - [x] `EnergyHistogram.Add(timeSeconds float64, bandEnergy []float64)`
  - [x] `EnergyHistogram.ToLateMono(sampleRate int) *ir.Buffer` — convert histogram to stochastic late IR with shaped noise

### 4.6 Ray tracer integration (`raytrace/`)

- [x] Add top-level `raytrace/tracer.go` (rename/extend earlier stub)
  - [x] `RayTracer` struct: `Config LaunchConfig`, `Scene *scene.Scene`
  - [x] `RayTracer.Trace() (*EnergyHistogram, error)`
    - [x] Launch rays from source
    - [x] For each ray: propagate bounces until max bounces or max time
    - [x] Check receiver hit at each segment
    - [x] Accumulate energy
- [x] Unit test: 10,000 rays in a shoebox with zero absorption → flat decay (conservation check)

### 4.7 Late IR synthesis from histogram

- [x] In `hybrid/` add `late_from_rays.go`
  - [x] `HistogramToEvents(h *raytrace.EnergyHistogram, rng *rand.Rand) []ir.Event` — optional sparse path
  - [x] `HistogramToBuffer(h *raytrace.EnergyHistogram, sampleRate int) *ir.Buffer` — direct noise shaping

### 4.8 Example: `shoebox_late` (inline, not separate dir yet)

- [x] Add a test-main in `raytrace/` package or inline example
- [x] Verify smooth decay plot by printing energy per 10 ms bin to stdout/CSV

---

## Phase 5 — Hybrid Early + Late IR

### Milestone B (complete): first public release candidate

> Combine ISM early reflections with ray-traced late tail into a single realistic mono IR.

### 5.1 Hybrid combine (`hybrid/`)

- [x] Add `combine.go`
  - [x] `HybridConfig` struct:
    - [x] `CrossoverTimeSeconds float64`
    - [x] `CrossoverOrder int` (for order-based cutoff)
    - [x] `CrossoverMode CrossoverMode` enum: `TimeBased`, `OrderBased`, `EnergyBased`
    - [x] `SmoothenCrossover bool`
  - [x] `Combine(early, late []ir.Event, cfg HybridConfig) []ir.Event`
    - [x] Strip from `late` any events with time < crossover time (anti-double-counting)
    - [x] Merge and sort by time
  - [x] `CombineBuffers(early, late *ir.Buffer, cfg HybridConfig) *ir.Buffer`
    - [x] Fade early out and late in around crossover region

### 5.2 Crossover alignment (`hybrid/`)

- [x] Add `align.go`
  - [x] `AlignLateTail(late *ir.Buffer, earlyEvents []ir.Event, cfg HybridConfig) *ir.Buffer`
    - [x] Energy-match late tail amplitude at crossover point to early tail
- [x] Unit test: combined energy is continuous at crossover

### 5.3 Crossover weighting (`hybrid/`)

- [x] Add `weighting.go`
  - [x] `LinearFade(start, end int, n int) []float64` — fade window
  - [x] `HannFade(n int) []float64` — smoother Hann window for crossover
  - [x] `ApplyFade(buf *ir.Buffer, startSample, endSample int, fadeIn bool) *ir.Buffer`

### 5.4 Band-wise hybrid rendering (`ir/`)

- [x] Extend `bandrender.go`
  - [x] `RenderHybridBand(earlyEvents, lateEvents []ir.Event, bandIndex int, cfg ir.RenderConfig) (*ir.Buffer, error)`
  - [x] `SumBandsWeighted(bands []*ir.Buffer, weights []float64) *ir.Buffer`

### 5.5 High-level renderer API

- [x] Add `renderer.go` at root package or `acoustics/renderer.go`
  - [x] `Renderer` struct: `Early EarlyEngine`, `Late LateEngine`, `LowFreq LowFreqEngine`, `Hybrid hybrid.HybridConfig`
  - [x] Define engine interfaces:
    - [x] `EarlyEngine` interface: `Generate(sc *scene.Scene, cfg RenderConfig) ([]ir.Event, error)`
    - [x] `LateEngine` interface: `Generate(sc *scene.Scene, cfg RenderConfig) ([]ir.Event, error)`
    - [x] `LowFreqEngine` interface: `Transfer(sc *scene.Scene, cfg RenderConfig) (*TransferFunction, error)`
  - [x] `Renderer.RenderMono(sc *scene.Scene, cfg RenderConfig) ([]float64, error)`
  - [x] `Renderer.RenderStereo(sc *scene.Scene, cfg RenderConfig) (left, right []float64, err error)` (stub for Phase 7)

### 5.6 CLI: `roomir render` (full hybrid)

- [x] Extend existing `render` sub-command:
  - [x] `--mode early|late|hybrid` flag
  - [x] `--crossover-time` flag (seconds)
  - [x] `--num-rays` flag
  - [x] Progress reporting to stderr

### 5.7 Regression tests for hybrid

- [x] Test: hybrid IR length matches requested duration
- [x] Test: energy in 0–100 ms window dominated by early engine output
- [x] Test: energy in 500 ms+ window dominated by late engine output
- [x] Test: no energy discontinuity at crossover (< 3 dB jump in 10 ms window)

---

## Phase 6 — GLL-Based Source Directivity

### Milestone C: loudspeaker-aware simulation

> Wire `gll-tools` directivity balloons into the source model so radiation patterns affect early and late energy.

### 6.1 GLL adapter (`directivity/`)

- [x] Add `gll.go`
  - [x] `GLLModel` struct: holds loaded `gll-tools` directivity data
  - [x] `LoadGLL(path, preset string) (*GLLModel, error)` wrapping `gll-tools` loader
  - [x] `GLLModel.GainLinear(freqHz float64, dir geometry.Vec3) float64`
    - [x] Convert `dir` to azimuth/elevation
    - [x] Look up nearest frequency band in balloon data
    - [x] Bilinear interpolation over angle grid
- [x] Unit test: on-axis (θ=0) gain ≥ off-axis gain for a cardioid-like GLL preset
- [x] Unit test: `LoadGLL` returns error on missing file

### 6.2 Coordinate frame transform

- [x] In `scene/source.go` extend `Source.DirectionTo`:
  - [x] Transform world-space direction into source-local frame using `Source.Orientation`
  - [x] Unit test: source looking along +X, target at +X from source → local dir = (1,0,0)
  - [x] Unit test: 90° rotation yields perpendicular local direction

### 6.3 Directivity application in ISM

- [x] In `ism/solver.go` apply source directivity:
  - [x] At direct path: look up gain for direction from source to receiver
  - [x] At each reflection path: look up gain for direction from source to first image-source reflection point
  - [x] Multiply into `Event.Amplitude` and `Event.BandGain`
- [x] Unit test: rotating source 180° changes event amplitudes for non-omnidirectional model

### 6.4 Directivity application in ray tracer

- [x] In `raytrace/launch.go` apply directivity to initial ray power:
  - [x] For each launched ray direction, multiply initial power by `source.Directivity.GainLinear(...)`
  - [x] Store initial band powers in ray state
- [x] Unit test: directional source → rays behind source have near-zero power

### 6.5 Example: `gll_source`

- [x] Add `examples/gll_source/main.go`
  - [x] Load a test GLL fixture from `testdata/gll/`
  - [x] Set up shoebox scene with GLL source
  - [x] Render hybrid mono IR
  - [x] Compare to omni source IR: frontal energy higher, rear energy lower
  - [x] Expose a wasm-friendly backend entrypoint for browser inputs
- [x] Add a minimal synthetic GLL test fixture to `testdata/gll/`

### 6.6 Diagnostic CLI: `roomplot source-directivity`

- [x] Add `cmd/roomplot/main.go` skeleton
- [x] Add `cmd/roomplot/directivity.go`
  - [x] Sub-command `source-directivity <gll-file> --freq 1000 --format csv`
  - [x] Prints azimuth vs. gain table to stdout or file

---

## Phase 7 — Binaural / HRTF Rendering

### Milestone D: binaural simulator

> Add SOFA-backed HRTF lookup and stereo BRIR export.

### 7.1 Nearest-neighbor HRTF lookup (`hrtf/`)

- [x] Add `lookup.go`
  - [x] `MeasurementGrid` struct: list of measurement directions + associated HRIRs
  - [x] `NearestNeighbor(grid *MeasurementGrid, dir geometry.Vec3) int` — returns index of closest measurement
  - [x] `LookupNearest(grid *MeasurementGrid, dir geometry.Vec3) (left, right []float64, delay float64)`
- [x] Unit test: frontal direction → returns measurement closest to (1,0,0)

### 7.2 Spherical interpolation (`hrtf/`)

- [x] Add `interpolate.go`
  - [x] `BarycentricWeights(p geometry.Vec3, tri [3]geometry.Vec3) [3]float64`
  - [x] `InterpolateHRIR(grid *MeasurementGrid, dir geometry.Vec3) (left, right []float64, delay float64)`
    - [x] Find enclosing triangle on measurement sphere
    - [x] Barycentric blend of three HRIRs
- [x] Unit test: interpolated result at a measurement position equals the measurement itself

### 7.3 SOFA adapter (`hrtf/`)

(see ../go-sofa/)

- [x] Add `sofa_adapter.go`
  - [x] `SOFAAdapter` struct implementing `Dataset` interface
  - [x] `LoadSOFA(path string) (*SOFAAdapter, error)` — wraps `go-sofa` loader behind interface
  - [x] Guard with build tag or optional import so `go-sofa` is not required for non-binaural builds
- [x] Add `hrtf/noop.go`
  - [x] `NoopDataset` struct implementing `Dataset` that returns identity (Dirac delta, no delay)
  - [x] Useful for testing the binaural rendering pipeline without a real SOFA file

### 7.4 Binaural IR renderer (`ir/`)

- [x] Add `render_binaural.go`
  - [x] `RenderBinaural(events []ir.Event, hrtf hrtf.Dataset, cfg RenderConfig) (left, right *ir.Buffer, err error)`
    - [x] For each event:
      - [x] Look up HRIR pair for event direction
      - [x] Convolve event amplitude with HRIR (via `algo-dsp` convolution)
      - [x] Add delay offset from HRTF
      - [x] Accumulate to left/right buffers
- [x] Unit test with `NoopDataset`: binaural output = 2× mono output (both channels identical)
- [x] Unit test with synthetic asymmetric HRTF: left/right differ for lateral source

### 7.5 Head orientation support

- [x] In `scene/receiver.go`:
  - [x] `Receiver.WorldToHeadDir(worldDir geometry.Vec3) geometry.Vec3` using `Orientation` quaternion
- [x] Apply head orientation transform before HRTF lookup in `render_binaural.go`
- [x] Unit test: 90° head rotation correctly rotates incoming direction

### 7.6 CLI: `roomir render-stereo`

- [x] Add sub-command `render-stereo <scene.json> -o output_stereo.wav`
  - [x] Validates that scene has binaural receiver(s)
  - [x] Runs ISM + late ray + hybrid pipeline
  - [x] Renders BRIR per receiver
  - [x] Exports stereo WAV

### 7.7 Example: `shoebox_binaural`

- [x] Add `examples/shoebox_binaural/main.go`
  - [x] Uses `NoopDataset` if no SOFA file present (graceful degradation)
  - [x] Writes stereo WAV
  - [x] Prints binaural result stats (L/R peak, delay difference)

---

## Phase 8 — Low-Frequency Shoebox Solver via `algo-pde`

### Milestone E: physics-enhanced low end

> Add a mode-accurate low-frequency engine using `algo-pde` Helmholtz solves (see ../algo-pde/).

### 8.1 Transfer function type (`pde/`)

- [x] Add `shoebox.go`
  - [x] `TransferFunction` struct: `Freqs []float64`, `H []complex128`
  - [x] `TF.Magnitude(i int) float64`
  - [x] `TF.PhaseRad(i int) float64`
  - [x] `TF.ToTimeDomain(sampleRate int, nFFT int) []float64` via inverse FFT (use `algo-dsp` FFT)

### 8.2 Frequency sweep (`pde/`)

- [x] Add `sweep.go`
  - [x] `SweepConfig` struct: `FreqMin, FreqMax float64`, `NumPoints int`, `BoundaryCondition string`
  - [x] `SweepShoebox(room *scene.Shoebox, src, rcv geometry.Vec3, cfg SweepConfig) (*TransferFunction, error)`
    - [x] For each frequency in sweep:
      - [x] Assemble source excitation on `algo-pde` regular grid
      - [x] Run Helmholtz solve via `algo-pde`
      - [x] Sample pressure at receiver grid point
      - [x] Store complex transfer function value

> **Insight:** `algo-pde` uses reusable plans. Create the plan once outside the frequency loop and call `.Solve()` repeatedly. This mirrors the performance pattern already established in `algo-pde`.

### 8.3 Modal analysis (`pde/`)

- [x] Add `modal.go`
  - [x] `ShoeboxModes(room *scene.Shoebox, maxOrder int) []ModalFrequency`
    - Analytical formula: `f = c/2 * sqrt((nx/Lx)² + (ny/Ly)² + (nz/Lz)²)`
  - [x] `ModalFrequency` struct: `Freq float64`, `Nx, Ny, Nz int`
  - [x] Sort by frequency
- [x] Unit test: compare PDE sweep peaks against analytical modal frequencies
  - Tolerance: ±2% of analytical value

### 8.4 Crossover between PDE and geometric (`pde/`)

- [x] Add `crossover.go`
  - [x] `CrossoverConfig` struct: `FreqHz float64`, `BandwidthOctaves float64`
  - [x] `SplitTF(tf *TransferFunction, cfg CrossoverConfig) (low, high *TransferFunction)`
  - [x] `BlendTF(low, high *TransferFunction, cfg CrossoverConfig) *TransferFunction`
    - Hann-windowed blend in crossover band

### 8.5 Hybrid integration (`hybrid/`)

- [x] Add `crossover.go`
  - [x] `BlendLowFreq(lowIR []float64, geoIR *ir.Buffer, crossoverHz float64, sampleRate int) *ir.Buffer`
    - [x] HP-filter geometric IR above crossover
    - [x] LP-filter PDE IR below crossover
    - [x] Sum both (use `algo-dsp` filter helpers)

### 8.6 PDE engine wiring

- [x] Implement `PDELowFreqEngine` satisfying `LowFreqEngine` interface
- [x] Wire into `Renderer.LowFreq` field
- [x] Add `--enable-lowfreq` flag to `roomir render`

### 8.7 Example: `hybrid_lowfreq`

- [x] Add `examples/hybrid_lowfreq/main.go`
  - [x] Small room (3×2.5×2.2 m) to emphasize modal behavior
  - [x] Sweep 20–300 Hz, blend with ISM above 200 Hz
  - [x] Export WAV and CSV of transfer function magnitude

### 8.8 Validation tests

- [x] Verify first axial mode frequency matches `c/(2·Lx)` within 2%
- [x] Verify smooth magnitude response at crossover (< 3 dB discontinuity)
- [x] Verify PDE-only IR has meaningful energy above 50 ms in a live room

---

## Phase 9 — Calibration and Validation

### Milestone: trustworthy and reproducible results

> Systematic validation against analytical expectations and known room-acoustics behavior.

### 9.1 Analytical shoebox validation

- [x] Compare ISM path lengths against hand-calculated geometry for specific cases
- [x] Validate all 6 first-order wall reflections for a symmetric shoebox
- [x] Validate second-order reflections for 3 edge cases
- [x] Test inverse-square attenuation: doubling distance → −6 dB

### 9.2 Modal frequency validation

- [x] Generate all axial, tangential, and oblique modes up to 300 Hz for 3 room sizes
- [x] Compare `pde/modal.go` analytical vs. PDE sweep peak frequencies
- [x] Flag any mode mismatch > 2% as test failure

### 9.3 Directivity coordinate frame validation

- [x] Test synthetic cardioid: on-axis gain = 1.0, rear = 0.0
- [x] Test GLL synthetic pattern: rotated source by 90°, 180°, 270° with known gains
- [x] Test `Source.DirectionTo` against hand-calculated quaternion rotation cases

### 9.4 HRTF lookup validation

- [x] Test: frontal measurement position returned for (1,0,0) direction
- [x] Test: left ear delay > 0 for sound from left for upright head orientation
- [x] Test: head rotation by 90° swaps lateral directions

### 9.5 Benchmark corpus

- [x] Create `testdata/rooms/tiny_room.json` (2×1.5×1.2 m — strong modes)
- [x] Create `testdata/rooms/control_room.json` (5×4×2.5 m)
- [x] Create `testdata/rooms/lecture_room.json` (12×8×4 m)
- [x] Create `testdata/rooms/pa_room.json` (10×6×3 m — GLL directional source)
- [x] Add `cmd/roombench/corpus_test.go`: run all rooms, compute T60/EDT/C80, smoke-test the benchmark corpus flow

### 9.6 Report generator

- [x] `roombench report --format markdown` — writes `bench_report.md`
- [x] Include: room name, T60, EDT, C80, expected range, pass/fail
- [x] Add report generation as optional CI artifact

---

## Phase 10 — Mesh Geometry and Non-Shoebox Scene Support

### Milestone F: geometry expansion

> Open the simulator to arbitrary triangulated room shapes for ray tracing.

### 10.1 Triangle mesh ingestion (`geometry/`)

- [x] Complete `triangle.go`
  - [x] `Triangle.Centroid() Vec3`
  - [x] `Triangle.Area() float64`
  - [x] `RayTriangle(r Ray, tri Triangle) (t float64, hit bool)` — Möller–Trumbore algorithm
- [x] Complete `mesh.go`
  - [x] `Mesh.BoundingBox() Box`
  - [x] `Mesh.Validate() error` — checks for degenerate triangles, non-watertight warnings
  - [x] `LoadOBJ(path string) (*Mesh, error)` — minimal OBJ loader (vertices + faces only)

> **Insight:** The Möller–Trumbore algorithm is the standard for ray-triangle intersection. It avoids computing the plane equation separately and is efficient for batch processing.

### 10.2 BVH acceleration structure (`geometry/`)

- [x] Add `bvh.go`
  - [x] `BVHNode` struct: `AABB Box`, `Left, Right *BVHNode`, `Triangles []int`
  - [x] `BuildBVH(mesh *Mesh) *BVHNode` — surface area heuristic (SAH) or midpoint split
  - [x] `BVHNode.Intersect(r Ray) (t float64, triIdx int, hit bool)`
- [x] Unit test: BVH on 1000 random triangles — all ray hits match brute force
- [x] Benchmark: BVH vs. brute force on 10k triangle mesh

### 10.3 Mesh-capable ray tracer (`raytrace/`)

- [x] Add `mesh_tracer.go`
  - [x] `MeshTracer` struct: `Mesh *geometry.Mesh`, `BVH *geometry.BVHNode`, `Materials []*scene.Material`
  - [x] Implements `Tracer` interface
  - [x] `MeshTracer.NextHit(r Ray) (hitPoint, normal Vec3, wallIdx int, ok bool)`

### 10.4 Scene mesh support (`scene/`)

- [x] Update `room.go`
  - [x] `Room.IsMesh() bool`
  - [x] `Room.IsValid() bool` — checks that mesh is non-nil when `Kind == RoomKindMesh`
- [x] Update `validate.go` to accept mesh rooms
- [x] Update JSON loader to handle mesh room with OBJ path reference

### 10.5 Integration test

- [x] Load a simple cube OBJ file
- [x] Trace 1000 rays — verify all bounces stay inside bounding box
- [x] Compare late-field decay to equivalent shoebox: should be similar for cube mesh

---

## Phase 11 — Frequency-Dependent Scattering

### Milestone G: physically accurate surface scattering

> Add per-octave-band scattering coefficients to materials, splitting reflected energy between specular and diffuse components using Lambert cosine-weighted hemisphere sampling. This is one of the most impactful accuracy improvements for late reverberation.

### 11.1 Material scattering data model (`scene/`)

- [x] Extend `Material` struct with per-octave-band scattering coefficients `Scattering [NumBands]float64` (125 Hz – 4 kHz, 6 bands minimum)
- [x] Add validation: each `s(f)` in `[0, 1]`, scattering must be monotonically non-decreasing with frequency (warn if not)
- [x] Update JSON scene loader to accept optional `"scattering"` array per material
- [x] Provide default scattering estimator from structural depth: `s(f) = 1 - exp(-k * (f / f0)^2)` where `f0 = c / (2 * depth)`
- [x] Update `material_test.go` with scattering coefficient round-trips

### 11.2 Material library with scattering data (`scene/`)

- [x] Create `materials_library.go` with published absorption + scattering data for common surfaces:
  - Painted concrete, exposed brick, plasterboard, glass, carpet, wooden floor
  - Audience seating (occupied/unoccupied), stage curtain
  - QRD diffuser, bookshelf
- [x] Source data from ISO 17497 measurements, Cox & D'Antonio (2017), Vorländer (2020)
- [x] Unit test: all library materials have valid coefficient ranges

### 11.3 Lambert diffuse reflection (`raytrace/`)

- [x] Implement `LambertDirection(normal Vec3, rng *rand.Rand) Vec3` — cosine-weighted hemisphere sampling via `theta = arccos(sqrt(r1))`, `phi = 2*pi*r2`
- [x] Build local coordinate frame from surface normal (tangent/bitangent construction)
- [x] Unit test: statistical distribution of 100k samples matches cos(theta) PDF within chi-squared tolerance
- [x] Benchmark: Lambert sampling throughput (target: > 10M samples/sec) — **achieved 26.3M samples/sec** ✅

### 11.4 Energy splitting at reflections (`raytrace/`)

- [x] Extend ray struct to carry energy per frequency band (`Energy [NumBands]float64`)
- [x] At each reflection, per band: `E_specular(f) = (1 - alpha(f)) * (1 - s(f)) * E_in(f)`, `E_diffuse(f) = (1 - alpha(f)) * s(f) * E_in(f)`
- [x] Implement hybrid direction strategy: use mean scattering coefficient across bands to decide ray direction (specular vs. Lambert), then weight energy per-band independently
- [x] Add configurable strategy: probabilistic split, deterministic blend, or full ray splitting with Russian roulette
- [x] Update ray termination: kill ray when max energy across all bands drops below threshold

### 11.5 Air absorption per frequency band (`raytrace/`)

- [x] Implement ISO 9613-1 atmospheric absorption: `alpha_air(f, T, h)` in dB/m as a function of frequency, temperature, and relative humidity
- [x] Apply per-band air absorption to ray energy after each path segment: `E(f) *= 10^(-alpha_air(f) * dist / 10)`
- [x] Unit test: attenuation at 4 kHz over 50 m at 20°C / 50% RH matches published ISO tables within 5%

### 11.6 Validation and sensitivity testing

- [x] Shoebox room comparison: run with `s=0` (all specular) and `s=1` (all diffuse), verify EDT and late-tail energy shift as scattering changes
- [x] Ray count convergence test: verify T30 stabilizes within 2% as ray count increases from 1k to 100k
- [x] Compare predicted T30, C80, D50 against published round-robin results (Bork 2000/2005 PTB study)
- [ ] A/B comparison: same room geometry with/without scattering, listen for plausibility of late reverberation

> **References:** ISO 17497-1/2 (scattering measurement), Vorländer "Auralization" (2020), Cox & D'Antonio "Acoustic Absorbers and Diffusers" (3rd ed., 2017), Mommertz (1995), PTB round-robin (Bork 2000/2005).

---

## Phase 12 — Edge Diffraction (UTD)

### Milestone H: diffraction around edges and barriers

> Implement the Uniform Theory of Diffraction (Kouyoumjian & Pathak 1974) to model sound bending around finite edges and wedges. First-order diffraction typically improves shadow-zone accuracy by 3–6 dB over ISM-only simulation.

### 12.1 Diffracting edge extraction (`geometry/`)

- [x] Add `diffraction.go` with `DiffractionEdge` struct:
  - `Start, End Vec3` — edge endpoints
  - `Direction Vec3` — unit vector along edge
  - `Length float64`
  - `WedgeIndex float64` — `n = exterior_angle / pi` (1 < n ≤ 2)
  - `FaceONormal, FaceNNormal Vec3` — normals of the two adjacent faces
  - `FaceOID, FaceNID int`
  - `LocalBasis [3]Vec3` — edge-local coordinate frame for angle computation
- [x] Implement `ExtractDiffractionEdges(mesh *Mesh) []DiffractionEdge`:
  - Build edge-adjacency map from triangle mesh (half-edge or edge-face lookup)
  - Compute dihedral angle between adjacent face normals
  - Classify: convex edges (exterior angle > π) are diffracting; concave and coplanar are skipped
  - Merge adjacent colinear diffracting edges sharing the same two planes
- [x] Compute edge-local coordinate system: edge direction as one axis, reference face normal defines `phi = 0`
- [x] Unit test: cube mesh produces 12 edges, all with `n = 1.5` (270° exterior = 1.5π)
- [x] Unit test: L-shaped room produces correct convex and concave edge classification

### 12.2 Fresnel transition function (`geometry/`)

- [x] Implement `FresnelTransition(x float64) complex128` — the UTD transition function `F(x) = 2j√x · e^(jx) · ∫_√x^∞ e^(-jt²) dt`
- [x] Use three regimes:
  - `x > 10`: asymptotic expansion `F(x) ≈ 1 + j/(2x) - 3/(4x²) - ...`
  - `x < 0.3`: small-argument power series
  - Intermediate: rational approximation or direct numerical integration
- [x] Unit test: validate against published tables (McNamara et al. 1990, Table 4.1)
- [x] Test boundary: `F(x) → 1` for large `x`, smooth transition near `x = 0`

### 12.3 Kouyoumjian–Pathak diffraction coefficient (`geometry/`)

- [x] Implement `WedgeDiffraction(phi, phiPrime, betaZero, n, k, L float64) complex128`
  - Four-term formula: incident shadow boundary, reflection shadow boundaries (face O and face N), second RSB
  - Each term: `D_i = (-e^(-jπ/4)) / (2n√(2πk)) · cot(α_i / (2n)) · F(kLa_i)`
  - Integer `N_i` selection to minimize `|2nπN ± β|`
- [x] Implement spreading factor `A = √(1 / (s · s'(s + s')))` for spherical wave incidence
- [x] Implement distance parameter `L = (s · s') / (s + s') · sin²(β₀)`
- [x] Unit test: half-plane diffraction (`n = 2`) matches classical Sommerfeld solution
- [x] Unit test: 90° wedge (`n = 1.5`) matches published values from Balanis (2012, Table 13.1)
- [x] Validate: coefficient remains finite near shadow and reflection boundaries; the total field continuity comes from the GO plus diffraction sum

### 12.4 Diffraction path finding (`geometry/`, `ism/`)

- [x] Implement `FindDiffractionPoint(source, receiver Vec3, edge DiffractionEdge) (point Vec3, t float64, ok bool)`:
  - Fermat's principle — minimize total path length `|S - P| + |P - R|`
  - Closed-form: project S and R onto plane perpendicular to edge, solve for `t` parameter
  - Reject if `t ∉ [0, 1]` (diffraction point outside finite edge)
- [x] Implement visibility testing: verify source-to-edge and edge-to-receiver paths are unoccluded (reuse existing BVH intersection)
- [x] Implement first-order path enumeration: for each source–receiver pair, iterate over all diffracting edges, find valid diffraction points, check visibility, compute contribution
- [x] Implement combined reflection–diffraction paths: use ISM image sources as virtual sources for diffraction (source→reflect→diffract→receiver and source→diffract→reflect→receiver)
- [x] Unit test: barrier between source and receiver — diffraction path found over the barrier top edge
- [x] Unit test: diffraction point on a wall corner edge is geometrically correct

### 12.5 ISM diffraction accumulation (`ism/`)

- [x] Add deterministic diffraction contribution accumulation to impulse response: `p_d = p_incident · D · A · e^(-jks) / √s`
- [x] Frequency-dependent: evaluate diffraction coefficient at each octave band center frequency
- [x] Implement contribution culling: skip edges whose estimated contribution is below –60 dB relative to direct sound
- [x] Integrate the diffraction pass with the ISM solver so direct/specular and diffraction events can be rendered together

### 12.6 Ray-tracer edge diffraction (`raytrace/`)

- [x] For ray tracer: when a traced ray passes near an edge (configurable angular threshold), spawn diffracted rays on the Keller cone — sample 8–16 directions around the cone
- [x] Add spatial index for edges (reuse BVH or build separate edge index) for efficient proximity queries

### 12.7 Validation

- [x] Canonical infinite wedge: regression coverage for wedge angles 90°, 180° (half-plane), 270°
- [x] Barrier insertion loss: compare against Maekawa chart / ISO 9613-2 for Fresnel numbers > 1, target ≤ 1 dB error
- [x] Mesh-cube diffraction smoke test: confirm branch spawning in a diffraction-rich mesh scene
- [x] Performance: first-order diffraction overhead benchmark on branch spawning / ray-trace hot path

> **References:** Kouyoumjian & Pathak (1974), Svensson et al. (1999) BTM model, Tsingos et al. (2001) UTD in virtual environments, McNamara et al. (1990) UTD textbook, Torres et al. (2001), Calamia & Svensson (2007) fast edge diffraction.
>
> **Known limitations:** UTD is a high-frequency approximation; accuracy degrades when `k · edge_length ≪ 1` (wavelength > edge length). For a 1 m edge, this means below ~170 Hz. Second-order diffraction gives diminishing returns (1–2 dB) at significant computational cost — implement first-order only initially.

---

## Phase 13 — Non-Rectangular Wave-Based Solver (IBM)

### Milestone I: wave-based acoustics for arbitrary convex rooms

> Extend `algo-pde` from rectangular-only to convex polyhedral rooms using the Immersed Boundary Method (IBM) on a regular Cartesian grid. This preserves the existing FDTD architecture while removing the shoebox geometry restriction for the low-frequency solver.

### 13.1 Convex room geometry module (`pde/`)

- [x] Add `convex.go` with `ConvexRoom` struct: ordered list of wall planes (normal + offset)
- [x] Implement convexity validation: all vertices on the correct side of all planes
- [x] Implement `PointInConvexRoom(p Vec3) bool` — half-plane intersection test (all dot products positive)
- [x] Implement `DistanceToNearestWall(p Vec3) (dist float64, normal Vec3, wallIdx int)` — for each grid node near the boundary
- [x] Construct axis-aligned bounding box for the convex room + PML padding
- [x] Unit test: point containment for cube, wedge-shaped room, truncated pyramid

### 13.2 Grid classification and boundary mapping (`pde/`)

- [x] Add `ibm_grid.go`:
  - For each grid node: classify as interior, boundary, or exterior
  - Boundary nodes: nodes inside the room with at least one exterior neighbor
  - For each boundary node: store fractional distance to wall along each axis, wall normal vector
- [x] Implement efficient classification using the convex half-plane tests (avoid per-node ray casting)
- [x] Store classification as a compact bitmask or enum grid (memory-efficient for large grids)
- [x] Unit test: rectangular room classification matches existing shoebox solver exactly
- [x] Unit test: 45° rotated square room produces correct boundary node pattern

### 13.3 Modified FDTD stencil for boundary nodes (`pde/`)

- [x] Implement interpolated boundary scheme: adjust FD coefficients based on sub-cell wall position
  - Option A (baseline): weighted reflection — mirror pressure at wall with appropriate reflection coefficient
  - Option B (higher accuracy): Hamilton–Bilbao coefficient modification based on fractional cell distances
- [x] Handle corner nodes where two walls meet (only convex corners for convex rooms)
- [x] Set exterior nodes to zero pressure (inactive)
- [x] Verify no stencil reads from uninitialized exterior data
- [x] Implement configurable wall boundary condition:
  - Rigid walls: `∂p/∂n = 0` (Neumann)
  - Impedance walls: frequency-independent real-valued reflection coefficient
  - Frequency-dependent impedance: auxiliary differential equation (ADE) approach at boundary nodes
- [x] CFL stability verification: empirically test that modified boundary stencils do not reduce stability limit below usable threshold

### 13.4 Source injection for arbitrary positions (`pde/`)

- [x] Point source injection at arbitrary position inside convex room
- [x] Verify source position is inside geometry before injection
- [x] Support both soft source (additive) and hard source (overwrite) modes
- [x] Gaussian pulse and sine burst source signals (reuse from shoebox solver)

### 13.5 Validation against analytical solutions

- [ ] **Rectangular room regression**: run IBM solver on a rectangular room, compare eigenfrequencies and IR against existing shoebox solver — must match within 0.1% for first 20 modes
- [ ] **Equilateral triangle (2D)**: compare against analytical eigenfrequencies `f_{m,n} = (c / 3L) · √(m² + mn + n²)` — target < 0.5% error for well-resolved modes
- [ ] **Circular room (2D)**: compare against Bessel function modes — tests curved-boundary IBM accuracy
- [ ] **Energy decay**: verify energy decay rate in convex room matches Sabine prediction within 10% for a room with known absorption

### 13.6 Performance optimization

- [ ] Sparse active-node iteration: only update interior + boundary nodes, skip exterior (avoid wasting compute on bounding-box padding)
- [ ] Profile: compare IBM solver throughput to shoebox solver — target < 15% overhead from boundary handling
- [ ] Memory: for rooms filling < 50% of bounding box, implement compressed storage of active nodes

> **References:** Botteldooren (1995) FDTD for room acoustics, Hamilton & Bilbao (2017) immersed boundary FDTD, Savioja & Svensson (2015) overview, Bilbao (2004) "Wave and Scattering Methods", Erlangga et al. (2004) shifted-Laplacian preconditioner.
>
> **Crossover frequency:** The IBM solver covers 0 Hz to a configurable crossover (default ~1000 Hz). Above the crossover, geometric acoustics (ray tracing + ISM from Phases 4–5) takes over. The hybrid combiner from Phase 5 handles the merge. Cost scaling is O(f_max⁴) in 3D — a 512³ grid at 1 kHz needs ~48 MB and runs in seconds; 4 kHz needs ~379 MB and minutes.
>
> **Future extension:** BEM (Boundary Element Method) is a strong alternative for convex rooms — surface-only discretization with O(N_boundary) DOF. Consider if IBM accuracy at oblique walls proves insufficient. FEM on unstructured meshes is the eventual path for non-convex rooms with fine geometric detail.

---

## Phase 14 — GPU Acceleration

### Milestone J: GPU-accelerated hot paths

> Profile CPU bottlenecks and offload the two heaviest workloads — FDTD stencil updates and batch ray tracing — to the GPU. Start with the subprocess model for clean separation, migrate to CGo + CUDA if IPC overhead matters.

### 14.1 CPU profiling baseline

- [ ] Profile full simulation pipeline with `go tool pprof` (CPU, memory, block profiles)
- [ ] Identify Amdahl fraction: what percentage of wall-clock time is in FDTD stencils vs. ray tracing vs. orchestration/I/O?
- [ ] Measure single-core vs. multi-core scaling of hot loops (`GOMAXPROCS` sweep: 1, 2, 4, 8)
- [ ] Document problem sizes: grid dimensions, ray counts per batch, total timesteps
- [ ] Compute Amdahl ceiling: `Speedup = 1 / ((1 - P) + P/S)` — if ceiling < 3×, GPU may not justify the complexity
- [ ] Estimate GPU memory requirements for target problem sizes, verify fit on target GPU (e.g., RTX 4090 = 24 GB)

### 14.2 Standalone CUDA kernel prototypes

- [ ] Write standalone CUDA FDTD stencil kernel (`.cu` file, no Go involvement):
  - 3D second-order finite-difference stencil with shared memory tiling
  - Benchmark: grid update throughput (cells/sec) for 256³, 512³, 1024³ grids
  - Compare against Go CPU baseline including transfer time
- [ ] Write standalone CUDA ray-BVH traversal kernel:
  - Evaluate NVIDIA OptiX for hardware RT-core acceleration vs. custom software BVH traversal
  - Benchmark: rays/sec for 100k, 1M, 10M rays against a 10k-triangle scene
  - Compare against Go CPU baseline including transfer time
- [ ] **Decision gate**: if GPU kernel + transfer is less than 5× faster than multi-core CPU for actual problem sizes, reconsider GPU investment

### 14.3 Integration architecture

- [ ] Choose integration approach:
  - **Subprocess model** (recommended first): Go orchestrates, GPU binary does compute via shared memory (`mmap`/`shm_open`) for bulk data + Unix domain socket for control
  - **CGo + CUDA**: thin C wrapper around kernels, called from Go — tighter coupling, harder build chain
- [ ] Define interface: what data crosses the boundary (geometry upload, field grids, ray buffers, results), in what format, how often
- [ ] Design memory lifecycle: upload-once (geometry, BVH, materials) vs. per-frame (ray origins) vs. persistent (field grids stay on GPU)
- [ ] Implement GPU worker pattern: single goroutine serializes GPU submissions via channel, avoids CGo thread-pool exhaustion

### 14.4 FDTD GPU integration (`pde/`)

- [ ] Implement GPU module (shared library or standalone binary) wrapping the FDTD stencil kernel
- [ ] Go-side integration: upload grid + materials once, run N thousand timesteps entirely on GPU, download receiver time series
- [ ] CUDA stream management: overlap compute and host↔device transfer (double-buffering)
- [ ] Pinned (page-locked) host memory for 2–3× faster transfers
- [ ] End-to-end benchmark: full PDE simulation with GPU, wall-clock time vs. CPU-only

### 14.5 Ray tracing GPU integration (`raytrace/`)

- [ ] Implement GPU ray tracing module:
  - Upload BVH + mesh once
  - Per-batch: upload ray origins/directions, download hit results (point, normal, triangle ID, distance)
  - Accumulate energy histograms on GPU (avoid per-ray download)
- [ ] End-to-end benchmark: full ray-traced IR with GPU, wall-clock time vs. CPU-only

### 14.6 Production hardening

- [ ] Error handling: GPU OOM, kernel launch failure, driver crash → graceful fallback to CPU
- [ ] CPU fallback path: detect GPU absence at startup, select CPU or GPU code path
- [ ] CI/CD: build pipeline with CUDA compilation, test on GPU-equipped runners (or skip GPU tests with build tag)
- [ ] Document GPU requirements and deployment prerequisites

> **References:** Mumax3 (`github.com/mumax/3`) — Go + CUDA for PDE stencils, closest architectural precedent. `gorgonia/cu` for CUDA driver API bindings from Go. NVIDIA OptiX for hardware ray tracing.
>
> **Key insight:** PCIe 4.0 bandwidth (~25 GB/s) is 30–100× slower than GPU memory bandwidth (~900 GB/s). Minimize host↔device transfers — keep field grids and energy accumulators on GPU across timesteps. A 512³ float32 grid is ~512 MB, well within modern GPU memory.
>
> **Expected speedups:** FDTD stencils: 20–50× over 8-core CPU. Ray-BVH traversal: 50–200× with OptiX RT cores. Total pipeline speedup depends on Amdahl fraction — profile first.

---

## Phase 15 — Real-Time Preview

### Milestone K: interactive parameter feedback

> Enable sub-second feedback when users change materials, source/receiver positions, or room dimensions. Separate geometric tracing from energy evaluation so material changes reuse cached ray paths.

### 15.1 Architecture: trace/evaluate separation (`raytrace/`, `ism/`)

- [ ] Refactor ray tracer to store ray paths as geometry-only data: sequence of surface IDs + hit points + path lengths (no energy)
- [ ] Implement "replay" function: given cached paths + material coefficients → energy histogram / IR
- [ ] Cache invalidation tags: geometry hash + source/receiver position hash + material hash
- [ ] Material-only change: reuse cached paths, recompute energy only (target < 100 ms for 10k paths)
- [ ] Geometry change: invalidate all cached paths, full re-trace required

### 15.2 Statistical pre-computation (`metrics/`)

- [ ] Instant estimates on any parameter change (< 5 ms):
  - Sabine RT60: `0.161 V / A` where `A = Σ(α_i · S_i)`
  - Eyring RT60: `0.161 V / (-S · ln(1 - ᾱ))`
  - Critical distance, estimated C80 and D50 from statistical formulas
- [ ] Display predicted parameters before simulation completes
- [ ] Unit test: statistical estimates match full simulation within 15% for a standard shoebox

### 15.3 Progressive rendering pipeline

- [ ] **Tier 1 — Instant (< 50 ms):** statistical estimates (Sabine/Eyring RT60, C80, D50)
- [ ] **Tier 2 — Fast preview (50–500 ms):** ISM order 2–3 + low ray count (1k–5k) + 3-band frequency resolution
- [ ] **Tier 3 — Refined (0.5–5 s):** full ISM order + progressive ray batches (1k rays per batch, update display after each)
- [ ] **Tier 4 — Final (background):** full ray count, all frequency bands, scattering, air absorption
- [ ] Implement `context.WithCancel` cancellation: new user input cancels current Tier 3/4 computation, restarts from Tier 1
- [ ] Debounce rapid parameter changes (slider dragging): coalesce within 50 ms window

### 15.4 Incremental ISM with caching (`ism/`)

- [ ] Cache image source tree (geometry-dependent only, no material data)
- [ ] On material change: re-evaluate energy along cached IS paths without rebuilding tree
- [ ] On geometry change: invalidate and rebuild affected branches
- [ ] On source/receiver move: rebuild paths from new position using existing IS tree structure

### 15.5 Quality presets and LOD controls

- [ ] Expose quality level setting: Draft / Preview / Final
- [ ] Map to concrete parameters:

| Parameter       | Draft  | Preview  | Final    |
| --------------- | ------ | -------- | -------- |
| ISM order       | 2      | 4        | 8+       |
| Ray count       | 1,000  | 10,000   | 100,000+ |
| Frequency bands | 3      | 6        | 8        |
| Max IR length   | 100 ms | 500 ms   | full T60 |
| Scattering      | off    | mid-band | per-band |

- [ ] Allow manual override of each parameter for advanced users

### 15.6 Hybrid statistical tail

- [ ] For preview tiers: compute early reflections exactly (first 50–100 ms), append exponential decay tail from Eyring formula
- [ ] Configurable crossover time (default 80 ms)
- [ ] Full computation (Tier 4) replaces statistical tail with ray-traced result
- [ ] Smooth crossfade between statistical tail and ray-traced tail to avoid artifacts

> **References:** ODEON "Quick Estimate" mode, CATT-Acoustic cone tracing with quality tiers, Treble cloud-based progressive rendering. All commercial tools separate quick statistical estimates from full computation.

---

## Phase 16 — Browser Demo (WASM)

### Milestone L: interactive web-based room acoustics demo

> Compile the core simulation engine to WebAssembly and build a browser-based demo with 3D room visualization, interactive parameter controls, and real-time auralization via Web Audio API.

### 16.1 WASM build pipeline

- [ ] Set up `GOOS=js GOARCH=wasm` build target in justfile
- [ ] Evaluate TinyGo: build core packages, compare binary size and numerical correctness against standard Go WASM
  - Standard Go: expect 8–15 MB raw, 2–4 MB brotli
  - TinyGo: expect 500 KB – 2 MB raw, 150–600 KB brotli
- [ ] Strip debug info (`-ldflags="-s -w"`), avoid importing `fmt` / `encoding/json` in hot paths
- [ ] Automate: `just wasm` → `.wasm` + `wasm_exec.js` + brotli compression
- [ ] **Decision**: standard Go vs. TinyGo based on binary size vs. feature coverage trade-off

### 16.2 Go/WASM API surface (`cmd/wasm/`)

- [ ] Define exported functions callable from JS via `syscall/js`:
  - `createRoom(jsonGeometry) → roomID`
  - `setMaterial(surfaceID, absorptionCoeffs, scatteringCoeffs)`
  - `setSource(x, y, z)` / `setReceiver(x, y, z)`
  - `simulate(options) → Float32Array` (impulse response)
  - `getParameters() → {rt60, c80, d50, ...}`
- [ ] Data transfer: `js.CopyBytesToJS` for typed arrays (Float32Array for IR)
- [ ] Progress callback from WASM → JS for progress bar during simulation
- [ ] Memory budget: target < 512 MB peak for broad device compatibility

### 16.3 HTML/JS frontend scaffold

- [ ] Static HTML page: Three.js viewport + control sidebar + results panel
- [ ] Parameter sliders: per-surface material absorption, source/receiver XYZ position
- [ ] Result display: RT60, C80, D50, energy decay curve (Chart.js or uPlot)
- [ ] Audio player section with play/stop/export controls
- [ ] Responsive layout for desktop and tablet

### 16.4 Three.js 3D room visualization

- [ ] Room geometry renderer: wireframe + solid mode, color-coded surfaces by material/absorption
- [ ] Draggable source (sphere) and receiver (sphere) markers with `THREE.Raycaster` picking
- [ ] `OrbitControls` for camera (orbit, pan, zoom)
- [ ] Optional: ray path visualization (animated `THREE.Line` showing reflection sequences)
- [ ] Optional: SPL heatmap texture on surfaces from simulation results

### 16.5 Web Audio API auralization

- [ ] Load dry audio samples: speech, clap, music (bundled as small MP3/OGG files)
- [ ] Create `ConvolverNode` with IR from WASM simulation
- [ ] Playback controls: play/stop, dry/wet mix slider, gain control
- [ ] On IR update: crossfade between old and new `ConvolverNode` to avoid clicks
- [ ] "Export WAV" button using `OfflineAudioContext`

### 16.6 Demo presets and deployment

- [ ] 2–3 built-in room geometries: shoebox, simple hall, classroom
- [ ] Pre-selected material presets: concert hall, studio, bathroom
- [ ] "Reset to default" button
- [ ] URL parameter encoding for shareable room configurations
- [ ] Deploy as static site (GitHub Pages or Cloudflare Pages)
- [ ] Correct MIME type for `.wasm` (`application/wasm`), cache headers for WASM binary
- [ ] If using `SharedArrayBuffer` for Web Workers: set COOP/COEP headers

### 16.7 Performance constraints

- [ ] Define demo limits: max 50 surfaces, max 50k rays, IR up to 3 s at 48 kHz
- [ ] Simulation runs in Web Worker (keep UI responsive)
- [ ] Use progressive rendering from Phase 15 — show Tier 1/2 results immediately, refine in background
- [ ] Fallback: if computation exceeds 10 s timeout, return partial result with warning

> **References:** Go WASM documentation, TinyGo WASM target, Three.js (`threejs.org`), Web Audio API `ConvolverNode` spec.
>
> **Performance note:** WASM runs at ~40–70% of native Go speed for numerical computation. For a simple room (< 50 surfaces) with 10k rays, expect 1–3 s simulation time in browser. Progressive rendering (Phase 15) is essential for interactive feel.

---

## Phase 17 — Interoperability and Asset Exchange

### Milestone M: exchange scenes and assets with external authoring tools

> Make `algo-acoustics` a better computational backend for desktop authoring tools by hardening scene interchange, material libraries, and comparison exports.

### 17.1 Scene schema and import/export (`scene/`, `export/`)

- [ ] Define a machine-readable JSON Schema for `scene.Scene` and publish it in `docs/`
- [ ] Add `scene.ValidateSchema` support for structure-level checks before rendering
- [ ] Add `roomir inspect <scene.json>` to print a normalized summary: room kind, materials, sources, receivers, bands, and sample rate
- [ ] Add `export/json.go` helpers for canonical scene dumps with stable key ordering
- [ ] Add round-trip tests for shoebox, mesh, and binaural scenes

### 17.2 Material and asset library interchange (`scene/`, `cmd/roomplot/`)

- [ ] Add import helpers for simple material tables in JSON/CSV form so external editors can reuse the same absorption/scattering data
- [ ] Add `roomplot materials <file>` to print band tables for a material library entry
- [ ] Add `roomplot scene-summary <scene.json>` for quick visual and textual inspection of scene metadata
- [ ] Add fixtures for a mesh-authored room with named materials, source placement, and listener placement

### 17.3 Comparison exports and regression bundles (`metrics/`, `export/`)

- [ ] Add `roomir compare <a.wav> <b.wav>` to print peak, RMS, correlation, and bandwise deltas
- [ ] Export comparison tables as CSV and Markdown for sharing with GUI-based workflows
- [ ] Add golden tests for scene summaries and comparison reports so the textual output stays stable
- [ ] Add a small corpus of externally authored scenes to `testdata/interop/`

### 17.4 External tool compatibility pass

- [ ] Document the expected scene conventions for mesh-based authoring tools: coordinate system, units, material bands, and listener orientation
- [ ] Add one end-to-end fixture that can be authored in an external GUI, rendered here, and compared against a known reference IR
- [ ] Verify that exported metadata is sufficient for a desktop client to reconstruct the same room, sources, and receivers

## Phase 18 — Release Engineering and Long-Run Maintenance

### Milestone N: reproducible binaries, demos, and regression history

> Turn the toolkit into something that is easy to ship, easy to verify, and hard to regress.

### 18.1 Release artifacts

- [ ] Add release targets for CLI binaries, web demo assets, and regression bundles
- [ ] Publish versioned tarball/zip archives with example scenes and docs
- [ ] Add build metadata to binaries so users can report exact version, commit, and build date

### 18.2 CI and test coverage gaps

- [ ] Add CI jobs for all supported platforms with a split between unit tests, integration tests, and formatting checks
- [ ] Add a dedicated regression job for `testdata/regression/` and `cmd/roombench`
- [ ] Add a smoke test for the WASM demo build so browser regressions fail fast
- [ ] Add a smoke test that renders at least one mono, stereo, and low-frequency scene in CI

### 18.3 Documentation and examples

- [ ] Add one page each for scene authoring, HRTF usage, directivity usage, hybrid rendering, and regression workflow
- [ ] Add a short "compare against another tool" guide that explains the expected output formats and validation workflow
- [ ] Keep the example scenes in sync with the current CLI flags and library interfaces

### 18.4 Maintenance budget

- [ ] Add a quarterly dependency audit for `algo-dsp`, `algo-fft`, `algo-pde`, `gll-tools`, and `wav`
- [ ] Add a benchmark baseline update procedure so performance improvements do not accidentally become regressions
- [ ] Track any new format or solver feature with a small fixture before expanding it into a full phase

## Milestone Summary

| Milestone                          | Phases  | Deliverable                              |
| ---------------------------------- | ------- | ---------------------------------------- |
| **A — First audible result**       | 0, 1, 2 | Mono WAV IR from shoebox scene           |
| **B — Useful room simulator**      | 3, 4, 5 | Hybrid mono IR + metrics + regression    |
| **C — Loudspeaker-aware**          | 6       | GLL directivity source model             |
| **D — Binaural simulator**         | 7       | Stereo BRIR WAV export                   |
| **E — Physics-enhanced low end**   | 8       | `algo-pde` crossover hybrid IR           |
| **F — Geometry expansion**         | 9, 10   | Mesh scenes + validation corpus          |
| **G — Accurate scattering**        | 11      | Per-band scattering + air absorption     |
| **H — Diffraction**                | 12      | UTD edge diffraction in ISM + ray tracer |
| **I — Convex room wave solver**    | 13      | IBM-FDTD for non-rectangular rooms       |
| **J — GPU acceleration**           | 14      | GPU-offloaded FDTD + ray tracing         |
| **K — Interactive preview**        | 15      | Sub-second parameter feedback            |
| **L — Browser demo**               | 16      | WASM + Three.js + Web Audio demo         |
| **M — Interop and asset exchange** | 17      | External authoring tool compatibility    |
| **N — Release engineering**        | 18      | Reproducible artifacts + maintenance     |

---

## Dependency Map

```text
algo-acoustics
├── algo-dsp          (convolution, FFT, metrics, filtering)
├── algo-pde          (Helmholtz shoebox, Phase 8+)
├── gll-tools         (directivity balloons, Phase 6+)
├── wav               (export only, Phase 2+)
└── go-sofa           (HRTF/BRIR, Phase 7, behind interface)
```

---

## Testing Strategy Reference

| Layer       | Focus                                                                |
| ----------- | -------------------------------------------------------------------- |
| Unit        | vec3, intersections, material interpolation, HRIR lookup, GLL lookup |
| Property    | symmetry, monotonic decay, rotation invariance                       |
| Analytical  | modal frequencies, ISM path lengths, inverse-square attenuation      |
| Golden      | IR hashes, event JSON dumps, stereo BRIR regression                  |
| Performance | allocations/render, rays/sec, PDE solver latency                     |
