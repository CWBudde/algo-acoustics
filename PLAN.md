# PLAN.md — `algo-acoustics` Implementation Roadmap

> **Architecture in one sentence:**
> `scene → propagation engines → event stream → IR/binaural renderer → export/metrics`

For implementation ideas, check <https://github.com/reuk/wayverb/>.

---

## Completed Phases

> All items below are done. Condensed for reference — see git history for details.

### Phase 0 — Scaffolding

Go module, CI pipeline (tests, lint, treefmt), directory skeleton (12 packages + 3 CLIs + examples + testdata), external dependency wiring (`algo-dsp`, `algo-pde`, `gll-tools`, `wav`), documentation stubs.

### Phase 1 — Domain Model and Scene Representation

Physical constants and units (`acoustics/`). Core geometry types: Vec3, Ray, Plane, Box, Triangle, Mesh, Quaternion (`geometry/`). Scene model: Room (shoebox/mesh), Material, Source, Receiver, validation (`scene/`). JSON scene loading. Directivity interface with Omni and Cardioid models (`directivity/`). HRTF interface stub (`hrtf/`). First CLI: `roomir validate`.

### Phase 2 — Mono Shoebox Image-Source Engine

IR event types and buffer (`ir/`). ISM solver: image source generation, audibility checks, direct path, wall absorption, distance attenuation, directivity, sorting (`ism/`). Mono IR renderer with band-wise rendering and normalization. WAV export (`export/`). CLI: `roomir render`. Analytical ISM tests (reciprocity, inverse-square, reflection geometry).

### Phase 3 — Metrics and Regression Harness

Acoustic metrics: T60, EDT, T20, T30, C50, C80, D50 (`metrics/`). Metric comparison framework. JSON/CSV export for events and metrics. Regression test fixtures with golden tests. Property tests (decay, reciprocity, inverse-square). `roombench` CLI with run/compare/report. CLI: `roomir dump-events`.

### Phase 4 — Ray-Traced Late Field

Ray launch (Fibonacci sphere, stratified directions). Shoebox tracer with 6-plane intersection. Scatter/absorption per bounce (specular, diffuse, cosine-weighted hemisphere). Sphere receiver hit model. Energy histogram accumulation. Ray tracer integration. Late IR synthesis from histogram (`hybrid/`).

### Phase 5 — Hybrid Early + Late IR

Hybrid combine with time/order/energy-based crossover modes. Crossover alignment and weighting (linear fade, Hann window). Band-wise hybrid rendering. High-level `Renderer` API with `EarlyEngine`, `LateEngine`, `LowFreqEngine` interfaces. CLI: `roomir render --mode early|late|hybrid`. Regression tests for crossover continuity.

### Phase 6 — GLL-Based Source Directivity

GLL adapter wrapping `gll-tools` with azimuth/elevation interpolation (`directivity/`). Source coordinate frame transforms. Directivity application in ISM (direct + reflected paths) and ray tracer (initial ray power). Example: `gll_source`. CLI: `roomplot source-directivity`.

### Phase 7 — Binaural / HRTF Rendering

Nearest-neighbor HRTF lookup. Spherical barycentric interpolation. SOFA adapter behind `Dataset` interface. Binaural IR renderer with HRIR convolution and ITD. Head orientation support. CLI: `roomir render-stereo`. Example: `shoebox_binaural`.

### Phase 8 — Low-Frequency Shoebox Solver

Transfer function type with FFT-based time-domain conversion (`pde/`). Frequency sweep using `algo-pde` Helmholtz solver. Analytical modal analysis. Crossover between PDE and geometric (Hann-windowed blend). Hybrid integration: HP/LP filter sum. PDE engine wired into `Renderer`. Example: `hybrid_lowfreq`. Validation: modal frequencies, crossover smoothness.

### Phase 9 — Calibration and Validation

Analytical shoebox validation (path lengths, inverse-square). Modal frequency validation (axial/tangential/oblique modes). Directivity coordinate frame validation. HRTF lookup validation. Benchmark corpus (tiny, control, lecture, PA rooms). Report generator (`roombench report --format markdown`).

### Phase 10 — Mesh Geometry and Non-Shoebox Scenes

Triangle mesh ingestion with OBJ loader. BVH acceleration (SAH split). Mesh-capable ray tracer implementing `Tracer` interface. Scene mesh support with validation. Integration tests (cube OBJ, boundary checks, decay comparison).

### Phase 11 — Frequency-Dependent Scattering

Per-octave-band scattering coefficients on materials with default estimator. Materials library (concrete, brick, glass, carpet, diffuser, etc.). Lambert cosine-weighted hemisphere sampling (26.3M samples/sec). Energy splitting at reflections (specular/diffuse per band). ISO 9613-1 air absorption per frequency band. Validation: scattering sensitivity, ray count convergence, PTB round-robin comparison.

> **Remaining:** A/B listening comparison (same room with/without scattering) — low priority, deferred.

### Phase 12 — Edge Diffraction / UTD

Diffracting edge extraction from meshes. Fresnel transition function (three-regime approximation). Kouyoumjian-Pathak 4-term diffraction coefficient with spreading factor. Diffraction path finding (Fermat's principle). Combined reflection-diffraction paths. ISM diffraction accumulation (frequency-dependent, -60 dB culling). Ray-tracer edge diffraction (Keller cone spawning with spatial index). Validation: infinite wedge, Maekawa barrier insertion loss, mesh-cube smoke test, performance benchmark.

### Phase 13 — Non-Rectangular Wave-Based Solver / IBM

Convex room geometry with half-plane containment test. Grid classification (interior/boundary/exterior bitmask). Modified FDTD stencil for boundary nodes (Hamilton-Bilbao interpolation, impedance BC, ADE). Source injection for arbitrary positions. Validation: rectangular room regression (<0.1%), equilateral triangle modes, circular room Bessel modes, Sabine energy decay. Performance: sparse active-node iteration, <15% overhead vs shoebox.

### Phase 14 — Interoperability and Asset Exchange

JSON Schema for scenes. Schema validation. `roomir inspect` command. Canonical JSON export. Material/asset library interchange. `roomplot materials` and `roomplot scene-summary` commands. Comparison exports (`roomir compare`). Golden tests for output stability. External tool compatibility documentation and fixtures.

### Phase 15 — Diffuse Rain and Poisson Late-Field Synthesis

Diffuse rain spherical detector with BVH visibility checks. Surface detector variant (`SurfaceReceiver` for portal preparation). Diffuse rain integration with hybrid model (solid angle fix, energy calibration). Poisson noise process with rate-capped Dirac delta sequence. Band-filtered Poisson RIR construction (default for late-field). Binaural Poisson RIR with direction-dependent HRIRs and overlap-add. A/B regression tests and listening fixtures.

### Phase 16 — Directivity Groups for Binaural Ray Tracing

Directivity group definition (72 DGs: 12 azimuth x 6 elevation). Ray-tracer DG binning (detector hits + diffuse rain). DG hit probability computation. Binaural RIR construction with DG-weighted HRIRs (top-N blend). A/B regression and listening test fixtures.

### Phase 17 — GPU Acceleration

CPU profiling and standalone CUDA kernels (FDTD 23× speedup, ray-BVH 511× vs single-core). Subprocess integration architecture: Go + CUDA server via Unix socket and `/dev/shm`. FDTD GPU integration with CUDA streams and pinned host memory for overlapped compute/transfer. Ray tracing GPU integration. Production hardening: graceful CPU fallback on GPU OOM / kernel launch failure / driver crash, auto-detect GPU absence at startup, CI/CD with CUDA build and GPU runners, deployment documentation. See `docs/profiling-baseline.md`, `docs/profiling-gpu-kernels.md`, and `docs/gpu-deployment.md`.

### Phase 18 — Real-Time Preview

Trace/evaluate separation for ray tracer and ISM: cached geometry-only paths with material replay for <100 ms material updates (`raytrace/`, `ism/`). Statistical pre-computation: Sabine/Eyring RT60, critical distance, C80, D50 estimators (<5 ms, within 15% of full simulation) (`metrics/`). Four-tier progressive rendering pipeline (statistical → preview → refined batches → final) with `context.WithCancel` cancellation and 50 ms debounce coalescing (`progressive.go`, `debounce.go`). Incremental `CachedISMSolver` with per-source image-source caching and `scene.RoomHash()` for fast receiver/material re-evaluation. Quality presets (Draft/Preview/Final) with per-field override (`preset.go`). Hybrid statistical tail: Eyring-driven exponential decay noise tail for preview tiers with cosine crossfade to ray-traced late field for Tier 4 (`statistical_tail.go`).

---

## Active / Remaining Phases

> Ordered for serial execution. Each phase lists only open tasks.

### Phase 19 — Browser Demo Completion

> Most of Phase 19 is done: WASM build pipeline, Go/WASM API surface, HTML/JS scaffold, Three.js room visualization, Web Audio auralization with ConvolverNode, demo presets, GitHub Pages deployment. Remaining items below.

#### 19.2 Go/WASM API surface — remaining

- [ ] Memory budget: target < 512 MB peak for broad device compatibility

#### 19.4 Three.js visualization — remaining

- [ ] Optional: SPL heatmap texture on surfaces from simulation results

#### 19.5 Web Audio API — remaining

- [x] Load dry audio samples: speech, clap, music (bundled as small MP3/OGG files)
- [x] On IR update: crossfade between old and new `ConvolverNode` to avoid clicks

#### 19.6 Deployment — remaining

- [ ] Correct MIME type for `.wasm` (`application/wasm`), cache headers for WASM binary
- [ ] If using `SharedArrayBuffer` for Web Workers: set COOP/COEP headers

#### 19.7 Performance constraints — remaining

- [ ] Define demo limits: max 50 surfaces, max 50k rays, IR up to 3 s at 48 kHz
- [ ] Use progressive rendering from Phase 18 — show Tier 1/2 results immediately, refine in background (depends on Phase 18)
- [ ] Fallback: if computation exceeds 10 s timeout, return partial result with warning

---

### Phase 20 — Release Engineering and Maintenance

#### 20.1 Release artifacts

- [ ] Add release targets for CLI binaries, web demo assets, and regression bundles
- [ ] Publish versioned tarball/zip archives with example scenes and docs
- [ ] Add build metadata to binaries (version, commit, build date)

#### 18.2 CI and test coverage gaps

- [ ] Add CI jobs for all supported platforms (unit tests, integration tests, formatting checks)
- [ ] Add dedicated regression job for `testdata/regression/` and `cmd/roombench`
- [ ] Add smoke test for WASM demo build
- [ ] Add smoke test rendering mono, stereo, and low-frequency scenes in CI

#### 18.3 Documentation and examples

- [ ] Add pages for: scene authoring, HRTF usage, directivity usage, hybrid rendering, regression workflow
- [ ] Add a "compare against another tool" guide
- [ ] Keep example scenes in sync with current CLI flags and library interfaces

#### 18.4 Maintenance budget

- [ ] Quarterly dependency audit for `algo-dsp`, `algo-fft`, `algo-pde`, `gll-tools`, `wav`
- [ ] Benchmark baseline update procedure
- [ ] Track new format/solver features with small fixtures before expanding

---

### Phase 21 — Sound Transmission Between Rooms

> Basic sound transmission through walls and portals between adjacent rooms. Implements the secondary source model from the RAVEN dissertation (Section 5). See `docs/raven2.md` Section 5.

#### 21.1 Material transmission coefficients (`scene/`)

- [ ] Add `TransmissionByBand []float64` to `Material` struct
- [ ] Add `SoundReductionIndex []float64` as alternative input (convert via `tau = 10^(-R/10)`)
- [ ] Validation: `0 <= tau <= 1` and `alpha + tau <= 1` (energy conservation)
- [ ] Add transmission data to materials library (concrete ~50 dB, plasterboard ~35 dB, wooden door ~25 dB, glass ~30 dB, open doorway 0 dB)
- [ ] Unit test: round-trip between `SoundReductionIndex` and `TransmissionByBand`

#### 21.2 Multi-room scene definition (`scene/`)

- [ ] Extend `Scene` to support `Rooms []Room` instead of single `Room`
- [ ] Define `Portal` struct: two room indices, shared surface polygon, material reference, state (Open/Closed)
- [ ] Add `Portals []Portal` to `Scene` with validation (valid room indices, coplanar walls, open = tau 1.0)
- [ ] JSON serialization for multi-room scenes
- [ ] Unit test: two adjacent shoeboxes sharing a wall with one portal

#### 21.3 Secondary source model (`raytrace/`, `ism/`)

- [ ] When ray tracer detects energy at a portal's surface receiver, spawn secondary point source at portal center on receiving-room side
- [ ] Secondary source spectrum filtered by portal transmission: `SS = S * sum(H_S,x * H_x,y)`
- [ ] ISM: mirror secondary source in receiving room with transmitted spectrum as initial energy
- [ ] RT: launch particles from secondary source with energy proportional to transmitted histogram
- [ ] Unit test: two identical rooms with portal — level difference matches `D_n = L_S - L_R + 10*log(S/A_R)` within 3 dB

#### 21.4 Apparent sound reduction index (`metrics/`)

- [ ] Implement `ApparentSoundReductionIndex(sourceLevel, receiverLevel, partitionArea, receiverAbsorptionArea float64) float64`
- [ ] Flanking-aware variant: `R' = -10*log(sum(tau_ij))`
- [ ] Validate: two 90 m^3 rooms, 16 m^2 partition — simulated R matches input within 2.5 dB (300 Hz-16 kHz)

#### 21.5 Portal interaction for real-time/web demo (`hybrid/`)

- [ ] Cache BRIRs for open and closed portal states
- [ ] Crossfade function: `y(x) = x^(1/n)`, x in [0,1] mapping aperture to interpolation weight
- [ ] Hard switch to merged room group BRIR once fully open
- [ ] Unit test: crossfade produces monotonically increasing energy; no discontinuities

---

### Phase 22 — Higher-Order Diffraction and DAPDF

> Extend diffraction beyond first-order UTD: second-order via edge-to-edge paths (BTME) and replace Keller-cone sampling with DAPDF. See `docs/raven2.md` Sections 2.5 and 3.3.

#### 22.1 Second-order edge diffraction (`ism/`, `geometry/`)

- [ ] Enumerate edge-to-edge diffraction paths with mutual visibility checks
- [ ] Implement `SecondOrderDiffractionPaths`: Fermat-principle points on both edges, intermediate receiver, cascaded diffraction coefficients
- [ ] `MaxDiffractionOrder int` config (default: 1; set to 2 to enable)
- [ ] Contribution culling: skip pairs below -60 dB
- [ ] Unit tests: L-shaped corridor (two corners), double-doorway
- [ ] Validation against RAVEN BTME reference

#### 22.2 Combined reflection-diffraction paths (`ism/`)

- [ ] Enumerate source->reflect->diffract->receiver and source->diffract->reflect->receiver up to second total order
- [ ] Reuse IS tree for reflection segments; diffraction via `FindDiffractionPoint`
- [ ] Path construction using subpath descriptors: S2D, D2D, D2R

#### 22.3 DAPDF implementation (`raytrace/`)

- [ ] Implement `DAPDF(v, v0, D0 float64) float64` with piecewise definition
- [ ] Implement the six closed-form energy integrals (Eqs. 5.29-5.34 in raven.md)
- [ ] Unit tests: integral = 1.0 for any b > 0; shape matches published plots

#### 22.4 Deflection cylinders (`raytrace/`, `geometry/`)

- [ ] `DeflectionCylinder` struct: edge, frequency-dependent radius (r = 7\*lambda)
- [ ] Ray-cylinder collision test
- [ ] DAPDF energy integration for outgoing energy distribution
- [ ] Forward diffracted energy to visible detectors ("diffracted rain"), recursive up to configurable depth (default: 2)
- [ ] Unit test and benchmark vs Keller-cone approach

#### 22.5 Validation

- [ ] Single finite wedge: compare against Svensson Edge Diffraction Toolbox at 50 Hz, 500 Hz, 5 kHz, 10 kHz
- [ ] View/shadow zone transition smoothness for both IS (BTME) and RT (DAPDF)
- [ ] L-shaped room: second-order improves shadow-zone by 1-3 dB
- [ ] Regression: existing first-order tests still pass at `MaxDiffractionOrder = 1`

---

### Phase 23 — Plane-Polygon Map and ISM Optimizations

> For rooms with many coplanar polygons, mirror across distinct planes instead of individual triangles: IS count drops from `n(n-1)^(i-1)` to `p(p-1)^(i-1)` where `p << n`. See `docs/raven2.md` Sections 2.4 and 4.2.

#### 23.1 Plane-Polygon Map (`ism/`, `geometry/`)

- [ ] `PlanePolygonMap`: group triangles by plane equivalence (normal + distance tolerance)
- [ ] Modify `GenerateMeshImageSources()` to mirror across planes rather than individual triangles
- [ ] Point-in-polygon test on coplanar set during audibility checks
- [ ] Unit test: 12 polygons on 8 planes -> 8 distinct planes
- [ ] Performance test: 4th-order IS generation ~4x fewer image sources with PPM

#### 23.2 Per-particle hybrid detection logic (`raytrace/`)

- [ ] Add tracking fields to `RayState`: `HasDiffuseHistory`, `PreEDReflOrder`, `EDReflOrder`, `EDOrder`
- [ ] Update tracking in reflection/scatter/diffraction handlers
- [ ] Implement `DetectionAllowedHybrid(state, config)` replacing time/order-based crossover
- [ ] Unit tests: specular particle at order 3 with MaxISOrder=3 not detected (order 4 is); scattered particle at order 2 is detected
- [ ] Energy conservation test: IS + RT total equals source energy minus absorption within 2%

---

### Phase 24 — Extended Source Directivity and SOFA Loading

> Frequency-dependent source directivity and complete SOFA file loading for professional-grade auralization. See `docs/raven2.md` Section 12.3.

#### 24.1 Frequency-dependent directivity models (`directivity/`)

- [ ] `FrequencyDependentCardioid`: per-band cardioid orders (wide at low freq, narrow at high freq) with interpolation
- [ ] `BalloonDirectivity`: tabulated gain on spherical grid per frequency band, nearest-neighbor or bilinear lookup
- [ ] Integration with GLL loader: extract per-band balloons from GLL data
- [ ] Unit tests: frequency-dependent behavior, grid-point exactness
- [ ] Regression: GLL source produces same ISM events as before

#### 24.2 SOFA file loading (`hrtf/`)

- [ ] SOFA file reader: parse NetCDF-based SimpleFreeFieldHRIR convention (positions, HRIRs, sample rate, delays)
- [ ] Build `MeasurementGrid` from loaded data
- [ ] `LoadSOFA(path string) (*NearestNeighborDataset, error)` convenience function
- [ ] Support CIPIC, LISTEN, and ARI datasets
- [ ] Unit test: load small SOFA file, verify measurement count and known HRIR
- [ ] Integration test: binaural IR with SOFA HRTFs, verify ITD for lateral source

---

### Phase 25 — Multi-Room Acoustic Scene Graph

> Full ASG/PST/PPG infrastructure for complex multi-room environments. Builds on Phase 21's basic portal support. See `docs/raven2.md` Section 5 and Section 10.

#### 25.1 Acoustic Scene Graph (`scene/`)

- [ ] `AcousticSceneGraph`: nodes = Room, edges = Portal, with counter-portal pointers
- [ ] Room groups: connected components of rooms joined by open portals
- [ ] `UpdateRoomGroups()` on portal state changes; per-group BVH and simulation context
- [ ] Unit test: 8-room office floor with 10 portals, various open/closed configs

#### 25.2 Path search and source elimination (`scene/`, `hybrid/`)

- [ ] Depth-first path search with cycle detection (LIFO stack) and R_w pruning
- [ ] Source elimination: skip fully-attenuated frequency bands per propagation path
- [ ] Convert PST to PPG: filter functions for portals, RIR/BRIR edges for rooms
- [ ] Unit test: 3-room chain finds direct path and confirms flanking paths

#### 25.3 Filter network rendering (`hybrid/`, `ir/`)

- [ ] Per-path transfer function: `H_PP = H_PS * prod(H_Portal) * prod(H_RoomGroup) * H_R`
- [ ] Four path types: PS2R, PS2P, SS2R, SS2P with binaural/monaural selection
- [ ] Sum all path contributions for final BRIR
- [ ] Integration test: office floor with correct level drop per room

#### 25.4 Dynamic portal interaction

- [ ] Recompute room groups and PST/PPG on portal state change
- [ ] Cache BRIRs per room group; only resimulate affected groups
- [ ] Reuse Phase 21.5 crossfade for smooth open/close transitions
- [ ] Performance target: portal change to updated BRIR < 2 s for 4-room scenario

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
