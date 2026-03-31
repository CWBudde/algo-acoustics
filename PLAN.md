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

- [ ] Add `metrics.go`
  - [ ] `T60FromDecaySlope(buf *ir.Buffer) (float64, error)` — linear regression on energy decay
  - [ ] `EDT(buf *ir.Buffer) (float64, error)` — early decay time (0 to −10 dB)
  - [ ] `T20(buf *ir.Buffer) (float64, error)` — −5 to −25 dB decay
  - [ ] `T30(buf *ir.Buffer) (float64, error)` — −5 to −35 dB decay
  - [ ] `C50(buf *ir.Buffer) (float64, error)` — clarity 50 ms
  - [ ] `C80(buf *ir.Buffer) (float64, error)` — clarity 80 ms
  - [ ] `D50(buf *ir.Buffer) (float64, error)` — definition

> **Insight:** Delegate convolution and filter-bank operations to `algo-dsp`; implement only the metric extraction formulas here. Check `algo-dsp` for existing EDT/T30 helpers before writing new ones.

- [ ] Add `compare.go`
  - [ ] `MetricResult` struct: `Name string`, `Expected, Actual, Tolerance float64`, `Pass bool`
  - [ ] `CompareMetric(name string, expected, actual, tolerance float64) MetricResult`
  - [ ] `CompareAll(results []MetricResult) bool`
  - [ ] `PrintReport(results []MetricResult, w io.Writer)`

### 3.2 JSON and CSV export (`export/`)

- [ ] Add `json.go`
  - [ ] `WriteEventsJSON(path string, events []ir.Event) error`
  - [ ] `WriteMetricsJSON(path string, results []metrics.MetricResult) error`
- [ ] Add `csv.go`
  - [ ] `WriteEventsCSV(path string, events []ir.Event) error`
  - [ ] `WriteMetricsCSV(path string, results []metrics.MetricResult) error`

### 3.3 Regression test fixtures

- [ ] Create `testdata/rooms/shoebox_absorptive.json` (high absorption walls)
- [ ] Create `testdata/rooms/shoebox_symmetric.json` (source and receiver equidistant from center)
- [ ] Create `testdata/rooms/shoebox_livelier.json` (very low absorption)
- [ ] Golden test: run ISM on each fixture, serialize events to JSON, compare against committed baseline
  - Tolerance: arrival time ±0.05 ms, amplitude ±0.5 dB

### 3.4 Property tests

- [ ] Property: absorptive room has shorter T60 than reflective room
- [ ] Property: larger room has longer first reflection time
- [ ] Property: source–receiver distance increase reduces direct path amplitude by 6 dB/double
- [ ] Property: swapping source and receiver positions yields same set of path lengths (reciprocity)
- [ ] Property: monotonic decay — energy in each time window is non-increasing on average after direct sound

### 3.5 `roombench` CLI

- [ ] Add `cmd/roombench/main.go`
  - [ ] Sub-command `run` — runs all regression fixtures, prints pass/fail
  - [ ] Sub-command `compare <baseline.json> <current.json>` — diff two metric reports
  - [ ] Sub-command `report` — generates a text summary table
- [ ] Wire `roombench run` into CI as a separate job

### 3.6 CLI: `roomir dump-events`

- [ ] Add sub-command `dump-events <scene.json> -o events.json --format json|csv`
- [ ] Useful for debugging and generating regression baselines

---

## Phase 4 — Ray-Traced Late Field

### Milestone B (partial): smooth statistical reverb tail

> Add a scalable Monte Carlo late-energy model. No PDE yet.

### 4.1 Ray launch (`raytrace/`)

- [ ] Add `launch.go`
  - [ ] `LaunchConfig` struct: `NumRays int`, `MaxBounces int`, `MaxTimeSeconds float64`, `SpeedOfSound float64`
  - [ ] `FibonacciSphere(n int) []geometry.Vec3` — uniformly distributed unit vectors
  - [ ] `StratifiedDirections(n int) []geometry.Vec3` — jittered stratified grid on sphere
  - [ ] `LaunchRays(src geometry.Vec3, cfg LaunchConfig) []geometry.Ray`

> **Insight:** Fibonacci sphere formula: `θ = 2π·i/φ²`, `z = 1 − (2i+1)/n` where `φ = (1+√5)/2`. This gives a near-uniform distribution without random noise.

### 4.2 Scene intersection for ray tracing (`raytrace/`)

- [ ] Add `tracer.go`
  - [ ] `ShoeboxTracer` struct: holds room walls as 6 planes
  - [ ] `ShoeboxTracer.NextHit(r geometry.Ray) (geometry.Vec3, geometry.Vec3, int, bool)` — returns hit point, normal, wall index, hit flag
  - [ ] `Tracer` interface: `NextHit(r geometry.Ray) (hitPoint, normal Vec3, wallIdx int, ok bool)`

### 4.3 Scatter and absorption per bounce (`raytrace/`)

- [ ] Add `scatter.go`
  - [ ] `ScatterConfig` struct: per-band absorption and scattering coefficients
  - [ ] `SpecularReflect(dir, normal geometry.Vec3) geometry.Vec3`
  - [ ] `DiffuseReflect(normal geometry.Vec3, rng *rand.Rand) geometry.Vec3` — cosine-weighted hemisphere
  - [ ] `SelectReflection(scatterCoeff float64, dir, normal geometry.Vec3, rng *rand.Rand) geometry.Vec3`
  - [ ] `AbsorbedFraction(absorptionByBand []float64) []float64` — per-band energy remaining after bounce

### 4.4 Receiver hit model (`raytrace/`)

- [ ] Add `receiver.go`
  - [ ] `SphereReceiver` struct: `Center geometry.Vec3`, `Radius float64`
  - [ ] `SphereReceiver.Intersects(r geometry.Ray, tMin, tMax float64) (t float64, hit bool)`
  - [ ] `SphereReceiver.AngularWeight(dir geometry.Vec3) float64` — optional cosine weighting

### 4.5 Energy accumulation (`raytrace/`)

- [ ] Add `accumulate.go`
  - [ ] `HistogramBin` struct: per-band energy, time
  - [ ] `EnergyHistogram` struct: `Bins []HistogramBin`, `BinDuration float64`, `BandCount int`
  - [ ] `NewEnergyHistogram(duration, binDuration float64, bandCount int) *EnergyHistogram`
  - [ ] `EnergyHistogram.Add(timeSeconds float64, bandEnergy []float64)`
  - [ ] `EnergyHistogram.ToLateMono(sampleRate int) *ir.Buffer` — convert histogram to stochastic late IR with shaped noise

### 4.6 Ray tracer integration (`raytrace/`)

- [ ] Add top-level `raytrace/tracer.go` (rename/extend earlier stub)
  - [ ] `RayTracer` struct: `Config LaunchConfig`, `Scene *scene.Scene`
  - [ ] `RayTracer.Trace() (*EnergyHistogram, error)`
    - [ ] Launch rays from source
    - [ ] For each ray: propagate bounces until max bounces or max time
    - [ ] Check receiver hit at each segment
    - [ ] Accumulate energy
- [ ] Unit test: 10,000 rays in a shoebox with zero absorption → flat decay (conservation check)

### 4.7 Late IR synthesis from histogram

- [ ] In `hybrid/` add `late_from_rays.go`
  - [ ] `HistogramToEvents(h *raytrace.EnergyHistogram, rng *rand.Rand) []ir.Event` — optional sparse path
  - [ ] `HistogramToBuffer(h *raytrace.EnergyHistogram, sampleRate int) *ir.Buffer` — direct noise shaping

### 4.8 Example: `shoebox_late` (inline, not separate dir yet)

- [ ] Add a test-main in `raytrace/` package or inline example
- [ ] Verify smooth decay plot by printing energy per 10 ms bin to stdout/CSV

---

## Phase 5 — Hybrid Early + Late IR

### Milestone B (complete): first public release candidate

> Combine ISM early reflections with ray-traced late tail into a single realistic mono IR.

### 5.1 Hybrid combine (`hybrid/`)

- [ ] Add `combine.go`
  - [ ] `HybridConfig` struct:
    - [ ] `CrossoverTimeSeconds float64`
    - [ ] `CrossoverOrder int` (for order-based cutoff)
    - [ ] `CrossoverMode CrossoverMode` enum: `TimeBased`, `OrderBased`, `EnergyBased`
    - [ ] `SmoothenCrossover bool`
  - [ ] `Combine(early, late []ir.Event, cfg HybridConfig) []ir.Event`
    - [ ] Strip from `late` any events with time < crossover time (anti-double-counting)
    - [ ] Merge and sort by time
  - [ ] `CombineBuffers(early, late *ir.Buffer, cfg HybridConfig) *ir.Buffer`
    - [ ] Fade early out and late in around crossover region

### 5.2 Crossover alignment (`hybrid/`)

- [ ] Add `align.go`
  - [ ] `AlignLateTail(late *ir.Buffer, earlyEvents []ir.Event, cfg HybridConfig) *ir.Buffer`
    - Energy-match late tail amplitude at crossover point to early tail
- [ ] Unit test: combined energy is continuous at crossover

### 5.3 Crossover weighting (`hybrid/`)

- [ ] Add `weighting.go`
  - [ ] `LinearFade(start, end int, n int) []float64` — fade window
  - [ ] `HannFade(n int) []float64` — smoother Hann window for crossover
  - [ ] `ApplyFade(buf *ir.Buffer, startSample, endSample int, fadeIn bool) *ir.Buffer`

### 5.4 Band-wise hybrid rendering (`ir/`)

- [ ] Extend `bandrender.go`
  - [ ] `RenderHybridBand(earlyEvents, lateEvents []ir.Event, bandIndex int, cfg ir.RenderConfig) (*ir.Buffer, error)`
  - [ ] `SumBandsWeighted(bands []*ir.Buffer, weights []float64) *ir.Buffer`

### 5.5 High-level renderer API

- [ ] Add `renderer.go` at root package or `acoustics/renderer.go`
  - [ ] `Renderer` struct: `Early EarlyEngine`, `Late LateEngine`, `LowFreq LowFreqEngine`, `Hybrid hybrid.HybridConfig`
  - [ ] Define engine interfaces:
    - [ ] `EarlyEngine` interface: `Generate(sc *scene.Scene, cfg RenderConfig) ([]ir.Event, error)`
    - [ ] `LateEngine` interface: `Generate(sc *scene.Scene, cfg RenderConfig) ([]ir.Event, error)`
    - [ ] `LowFreqEngine` interface: `Transfer(sc *scene.Scene, cfg RenderConfig) (*TransferFunction, error)`
  - [ ] `Renderer.RenderMono(sc *scene.Scene, cfg RenderConfig) ([]float64, error)`
  - [ ] `Renderer.RenderStereo(sc *scene.Scene, cfg RenderConfig) (left, right []float64, err error)` (stub for Phase 7)

### 5.6 CLI: `roomir render` (full hybrid)

- [ ] Extend existing `render` sub-command:
  - [ ] `--mode early|late|hybrid` flag
  - [ ] `--crossover-time` flag (seconds)
  - [ ] `--num-rays` flag
  - [ ] Progress reporting to stderr

### 5.7 Regression tests for hybrid

- [ ] Test: hybrid IR length matches requested duration
- [ ] Test: energy in 0–100 ms window dominated by early engine output
- [ ] Test: energy in 500 ms+ window dominated by late engine output
- [ ] Test: no energy discontinuity at crossover (< 3 dB jump in 10 ms window)

---

## Phase 6 — GLL-Based Source Directivity

### Milestone C: loudspeaker-aware simulation

> Wire `gll-tools` directivity balloons into the source model so radiation patterns affect early and late energy.

### 6.1 GLL adapter (`directivity/`)

- [ ] Add `gll.go`
  - [ ] `GLLModel` struct: holds loaded `gll-tools` directivity data
  - [ ] `LoadGLL(path, preset string) (*GLLModel, error)` wrapping `gll-tools` loader
  - [ ] `GLLModel.GainLinear(freqHz float64, dir geometry.Vec3) float64`
    - [ ] Convert `dir` to azimuth/elevation
    - [ ] Look up nearest frequency band in balloon data
    - [ ] Bilinear interpolation over angle grid
- [ ] Unit test: on-axis (θ=0) gain ≥ off-axis gain for a cardioid-like GLL preset
- [ ] Unit test: `LoadGLL` returns error on missing file

### 6.2 Coordinate frame transform

- [ ] In `scene/source.go` extend `Source.DirectionTo`:
  - [ ] Transform world-space direction into source-local frame using `Source.Orientation`
  - [ ] Unit test: source looking along +X, target at +X from source → local dir = (1,0,0)
  - [ ] Unit test: 90° rotation yields perpendicular local direction

### 6.3 Directivity application in ISM

- [ ] In `ism/solver.go` apply source directivity:
  - [ ] At direct path: look up gain for direction from source to receiver
  - [ ] At each reflection path: look up gain for direction from source to first image-source reflection point
  - [ ] Multiply into `Event.Amplitude` and `Event.BandGain`
- [ ] Unit test: rotating source 180° changes event amplitudes for non-omnidirectional model

### 6.4 Directivity application in ray tracer

- [ ] In `raytrace/launch.go` apply directivity to initial ray power:
  - [ ] For each launched ray direction, multiply initial power by `source.Directivity.GainLinear(...)`
  - [ ] Store initial band powers in ray state
- [ ] Unit test: directional source → rays behind source have near-zero power

### 6.5 Example: `gll_source`

- [ ] Add `examples/gll_source/main.go`
  - [ ] Load a test GLL fixture from `testdata/gll/`
  - [ ] Set up shoebox scene with GLL source
  - [ ] Render hybrid mono IR
  - [ ] Compare to omni source IR: frontal energy higher, rear energy lower
- [ ] Add a minimal synthetic GLL test fixture to `testdata/gll/`

### 6.6 Diagnostic CLI: `roomplot source-directivity`

- [ ] Add `cmd/roomplot/main.go` skeleton
- [ ] Add `cmd/roomplot/directivity.go`
  - [ ] Sub-command `source-directivity <gll-file> --freq 1000 --format csv`
  - [ ] Prints azimuth vs. gain table to stdout or file

---

## Phase 7 — Binaural / HRTF Rendering

### Milestone D: binaural simulator

> Add SOFA-backed HRTF lookup and stereo BRIR export.

### 7.1 Nearest-neighbor HRTF lookup (`hrtf/`)

- [ ] Add `lookup.go`
  - [ ] `MeasurementGrid` struct: list of measurement directions + associated HRIRs
  - [ ] `NearestNeighbor(grid *MeasurementGrid, dir geometry.Vec3) int` — returns index of closest measurement
  - [ ] `LookupNearest(grid *MeasurementGrid, dir geometry.Vec3) (left, right []float64, delay float64)`
- [ ] Unit test: frontal direction → returns measurement closest to (1,0,0)

### 7.2 Spherical interpolation (`hrtf/`)

- [ ] Add `interpolate.go`
  - [ ] `BarycentricWeights(p geometry.Vec3, tri [3]geometry.Vec3) [3]float64`
  - [ ] `InterpolateHRIR(grid *MeasurementGrid, dir geometry.Vec3) (left, right []float64, delay float64)`
    - [ ] Find enclosing triangle on measurement sphere
    - [ ] Barycentric blend of three HRIRs
- [ ] Unit test: interpolated result at a measurement position equals the measurement itself

### 7.3 SOFA adapter (`hrtf/`)

- [ ] Add `sofa_adapter.go`
  - [ ] `SOFAAdapter` struct implementing `Dataset` interface
  - [ ] `LoadSOFA(path string) (*SOFAAdapter, error)` — wraps `go-sofa` loader behind interface
  - [ ] Guard with build tag or optional import so `go-sofa` is not required for non-binaural builds
- [ ] Add `hrtf/noop.go`
  - [ ] `NoopDataset` struct implementing `Dataset` that returns identity (Dirac delta, no delay)
  - [ ] Useful for testing the binaural rendering pipeline without a real SOFA file

### 7.4 Binaural IR renderer (`ir/`)

- [ ] Add `render_binaural.go`
  - [ ] `RenderBinaural(events []ir.Event, hrtf hrtf.Dataset, cfg RenderConfig) (left, right *ir.Buffer, err error)`
    - [ ] For each event:
      - [ ] Look up HRIR pair for event direction
      - [ ] Convolve event amplitude with HRIR (via `algo-dsp` convolution)
      - [ ] Add delay offset from HRTF
      - [ ] Accumulate to left/right buffers
- [ ] Unit test with `NoopDataset`: binaural output = 2× mono output (both channels identical)
- [ ] Unit test with synthetic asymmetric HRTF: left/right differ for lateral source

### 7.5 Head orientation support

- [ ] In `scene/receiver.go`:
  - [ ] `Receiver.WorldToHeadDir(worldDir geometry.Vec3) geometry.Vec3` using `Orientation` quaternion
- [ ] Apply head orientation transform before HRTF lookup in `render_binaural.go`
- [ ] Unit test: 90° head rotation correctly rotates incoming direction

### 7.6 CLI: `roomir render-stereo`

- [ ] Add sub-command `render-stereo <scene.json> -o output_stereo.wav`
  - [ ] Validates that scene has binaural receiver(s)
  - [ ] Runs ISM + late ray + hybrid pipeline
  - [ ] Renders BRIR per receiver
  - [ ] Exports stereo WAV

### 7.7 Example: `shoebox_binaural`

- [ ] Add `examples/shoebox_binaural/main.go`
  - [ ] Uses `NoopDataset` if no SOFA file present (graceful degradation)
  - [ ] Writes stereo WAV
  - [ ] Prints binaural result stats (L/R peak, delay difference)

---

## Phase 8 — Low-Frequency Shoebox Solver via `algo-pde`

### Milestone E: physics-enhanced low end

> Add a mode-accurate low-frequency engine using `algo-pde` Helmholtz solves.

### 8.1 Transfer function type (`pde/`)

- [ ] Add `shoebox.go`
  - [ ] `TransferFunction` struct: `Freqs []float64`, `H []complex128`
  - [ ] `TF.Magnitude(i int) float64`
  - [ ] `TF.PhaseRad(i int) float64`
  - [ ] `TF.ToTimeDomain(sampleRate int, nFFT int) []float64` via inverse FFT (use `algo-dsp` FFT)

### 8.2 Frequency sweep (`pde/`)

- [ ] Add `sweep.go`
  - [ ] `SweepConfig` struct: `FreqMin, FreqMax float64`, `NumPoints int`, `BoundaryCondition string`
  - [ ] `SweepShoebox(room *scene.Shoebox, src, rcv geometry.Vec3, cfg SweepConfig) (*TransferFunction, error)`
    - [ ] For each frequency in sweep:
      - [ ] Assemble source excitation on `algo-pde` regular grid
      - [ ] Run Helmholtz solve via `algo-pde`
      - [ ] Sample pressure at receiver grid point
      - [ ] Store complex transfer function value

> **Insight:** `algo-pde` uses reusable plans. Create the plan once outside the frequency loop and call `.Solve()` repeatedly. This mirrors the performance pattern already established in `algo-pde`.

### 8.3 Modal analysis (`pde/`)

- [ ] Add `modal.go`
  - [ ] `ShoeboxModes(room *scene.Shoebox, maxOrder int) []ModalFrequency`
    - Analytical formula: `f = c/2 * sqrt((nx/Lx)² + (ny/Ly)² + (nz/Lz)²)`
  - [ ] `ModalFrequency` struct: `Freq float64`, `Nx, Ny, Nz int`
  - [ ] Sort by frequency
- [ ] Unit test: compare PDE sweep peaks against analytical modal frequencies
  - Tolerance: ±2% of analytical value

### 8.4 Crossover between PDE and geometric (`pde/`)

- [ ] Add `crossover.go`
  - [ ] `CrossoverConfig` struct: `FreqHz float64`, `BandwidthOctaves float64`
  - [ ] `SplitTF(tf *TransferFunction, cfg CrossoverConfig) (low, high *TransferFunction)`
  - [ ] `BlendTF(low, high *TransferFunction, cfg CrossoverConfig) *TransferFunction`
    - Hann-windowed blend in crossover band

### 8.5 Hybrid integration (`hybrid/`)

- [ ] Add `crossover.go`
  - [ ] `BlendLowFreq(lowIR []float64, geoIR *ir.Buffer, crossoverHz float64, sampleRate int) *ir.Buffer`
    - [ ] HP-filter geometric IR above crossover
    - [ ] LP-filter PDE IR below crossover
    - [ ] Sum both (use `algo-dsp` filter helpers)

### 8.6 PDE engine wiring

- [ ] Implement `PDELowFreqEngine` satisfying `LowFreqEngine` interface
- [ ] Wire into `Renderer.LowFreq` field
- [ ] Add `--enable-lowfreq` flag to `roomir render`

### 8.7 Example: `hybrid_lowfreq`

- [ ] Add `examples/hybrid_lowfreq/main.go`
  - [ ] Small room (3×2.5×2.2 m) to emphasize modal behavior
  - [ ] Sweep 20–300 Hz, blend with ISM above 200 Hz
  - [ ] Export WAV and CSV of transfer function magnitude

### 8.8 Validation tests

- [ ] Verify first axial mode frequency matches `c/(2·Lx)` within 2%
- [ ] Verify smooth magnitude response at crossover (< 3 dB discontinuity)
- [ ] Verify PDE-only IR has meaningful energy above 50 ms in a live room

---

## Phase 9 — Calibration and Validation

### Milestone: trustworthy and reproducible results

> Systematic validation against analytical expectations and known room-acoustics behavior.

### 9.1 Analytical shoebox validation

- [ ] Compare ISM path lengths against hand-calculated geometry for specific cases
- [ ] Validate all 6 first-order wall reflections for a symmetric shoebox
- [ ] Validate second-order reflections for 3 edge cases
- [ ] Test inverse-square attenuation: doubling distance → −6 dB

### 9.2 Modal frequency validation

- [ ] Generate all axial, tangential, and oblique modes up to 300 Hz for 3 room sizes
- [ ] Compare `pde/modal.go` analytical vs. PDE sweep peak frequencies
- [ ] Flag any mode mismatch > 2% as test failure

### 9.3 Directivity coordinate frame validation

- [ ] Test synthetic cardioid: on-axis gain = 1.0, rear = 0.0
- [ ] Test GLL synthetic pattern: rotated source by 90°, 180°, 270° with known gains
- [ ] Test `Source.DirectionTo` against hand-calculated quaternion rotation cases

### 9.4 HRTF lookup validation

- [ ] Test: frontal measurement position returned for (1,0,0) direction
- [ ] Test: left ear delay > 0 for sound from left for upright head orientation
- [ ] Test: head rotation by 90° swaps lateral directions

### 9.5 Benchmark corpus

- [ ] Create `testdata/rooms/tiny_room.json` (2×1.5×1.2 m — strong modes)
- [ ] Create `testdata/rooms/control_room.json` (5×4×2.5 m)
- [ ] Create `testdata/rooms/lecture_room.json` (12×8×4 m)
- [ ] Create `testdata/rooms/pa_room.json` (10×6×3 m — GLL directional source)
- [ ] Add `cmd/roombench/corpus.go`: run all rooms, compute T60/EDT/C80, output comparison table

### 9.6 Report generator

- [ ] `roombench report --format markdown` — writes `bench_report.md`
- [ ] Include: room name, T60, EDT, C80, expected range, pass/fail
- [ ] Add report generation as optional CI artifact

---

## Phase 10 — Mesh Geometry and Non-Shoebox Scene Support

### Milestone F: geometry expansion

> Open the simulator to arbitrary triangulated room shapes for ray tracing.

### 10.1 Triangle mesh ingestion (`geometry/`)

- [ ] Complete `triangle.go`
  - [ ] `Triangle.Centroid() Vec3`
  - [ ] `Triangle.Area() float64`
  - [ ] `RayTriangle(r Ray, tri Triangle) (t float64, hit bool)` — Möller–Trumbore algorithm
- [ ] Complete `mesh.go`
  - [ ] `Mesh.BoundingBox() Box`
  - [ ] `Mesh.Validate() error` — checks for degenerate triangles, non-watertight warnings
  - [ ] `LoadOBJ(path string) (*Mesh, error)` — minimal OBJ loader (vertices + faces only)

> **Insight:** The Möller–Trumbore algorithm is the standard for ray-triangle intersection. It avoids computing the plane equation separately and is efficient for batch processing.

### 10.2 BVH acceleration structure (`geometry/`)

- [ ] Add `bvh.go`
  - [ ] `BVHNode` struct: `AABB Box`, `Left, Right *BVHNode`, `Triangles []int`
  - [ ] `BuildBVH(mesh *Mesh) *BVHNode` — surface area heuristic (SAH) or midpoint split
  - [ ] `BVHNode.Intersect(r Ray) (t float64, triIdx int, hit bool)`
- [ ] Unit test: BVH on 1000 random triangles — all ray hits match brute force
- [ ] Benchmark: BVH vs. brute force on 10k triangle mesh

### 10.3 Mesh-capable ray tracer (`raytrace/`)

- [ ] Add `mesh_tracer.go`
  - [ ] `MeshTracer` struct: `Mesh *geometry.Mesh`, `BVH *geometry.BVHNode`, `Materials []*scene.Material`
  - [ ] Implements `Tracer` interface
  - [ ] `MeshTracer.NextHit(r Ray) (hitPoint, normal Vec3, wallIdx int, ok bool)`

### 10.4 Scene mesh support (`scene/`)

- [ ] Update `room.go`
  - [ ] `Room.IsMesh() bool`
  - [ ] `Room.IsValid() bool` — checks that mesh is non-nil when `Kind == RoomKindMesh`
- [ ] Update `validate.go` to accept mesh rooms
- [ ] Update JSON loader to handle mesh room with OBJ path reference

### 10.5 Integration test

- [ ] Load a simple cube OBJ file
- [ ] Trace 1000 rays — verify all bounces stay inside bounding box
- [ ] Compare late-field decay to equivalent shoebox: should be similar for cube mesh

---

## Phase 11 — Future Research Track

### Post-milestone: research and experimentation only after Phases 0–10 are solid

> These items are explicitly deferred. Do not begin until Phases 1–10 have stable, tested implementations.

### 11.1 Diffraction (deferred)

- [ ] Review uniform theory of diffraction (UTD) literature
- [ ] Identify edge diffraction paths in mesh geometry
- [ ] Prototype wedge diffraction coefficient

### 11.2 Frequency-dependent scattering (deferred)

- [ ] Material scattering vs. frequency model
- [ ] Validate against published room acoustics data

### 11.3 Non-rectangular PDE / iterative Helmholtz (deferred)

- [ ] Evaluate `algo-pde` roadmap for non-regular grid support
- [ ] Prototype immersed boundary method for simple convex rooms

### 11.4 GPU acceleration (deferred)

- [ ] Profile hot loops: ray tracing and PDE sweeps
- [ ] Evaluate `go-cuda` or OpenCL binding feasibility

### 11.5 Real-time preview (deferred)

- [ ] Define latency budget (< 20 ms update for parameter change)
- [ ] Design incremental re-render strategy

### 11.6 Browser demo (deferred)

- [ ] Evaluate WASM compilation of core packages
- [ ] Minimal Web Audio API integration prototype

---

## Milestone Summary

| Milestone                        | Phases  | Deliverable                           |
| -------------------------------- | ------- | ------------------------------------- |
| **A — First audible result**     | 0, 1, 2 | Mono WAV IR from shoebox scene        |
| **B — Useful room simulator**    | 3, 4, 5 | Hybrid mono IR + metrics + regression |
| **C — Loudspeaker-aware**        | 6       | GLL directivity source model          |
| **D — Binaural simulator**       | 7       | Stereo BRIR WAV export                |
| **E — Physics-enhanced low end** | 8       | `algo-pde` crossover hybrid IR        |
| **F — Geometry expansion**       | 9, 10   | Mesh scenes + validation corpus       |
| **Research**                     | 11      | Experimental features                 |

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
