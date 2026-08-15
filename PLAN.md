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

### Phase 19 — Browser Demo Completion

WASM build pipeline and Go/WASM API surface (`web/wasm/`). HTML/JS scaffold, Three.js room visualization with SPL heatmap texture, demo presets. Web Audio auralization: bundled dry samples (speech, clap, music), `ConvolverNode` crossfade on IR update. Rendering in a dedicated worker with real cancellation. Memory budget: 512 MiB peak target with automatic quality reduction (`docs/wasm-memory-budget.md`). Deployment: `web/devserver` typing `.wasm` as `application/wasm` with the demo's cache policy, `web/_headers` for Netlify/Cloudflare, COOP/COEP behind `-coi` but not required (plain worker, no `SharedArrayBuffer`); see `docs/web-deployment.md`. Request envelope defined once in `web/wasm/limits.go` and published to the page to size its sliders — 50 distinct surface planes, 3 s at 48 kHz, and a **16,384-ray cap rather than the 50,000 originally planned**, because wall-clock rather than memory binds. Progressive tiers (statistical → preview → full) sharing Phase 18's Tier 1 via `ComputeStatisticalMetrics`, plus a 10 s render budget that predicts from the timed preview and returns a complete partial result with a warning instead of overrunning; see `docs/web-demo-limits.md`.

### Phase 20 — Release Engineering and Maintenance

Release targets for CLI binaries, web demo assets, and regression bundles; versioned archives with example scenes and docs; build metadata (version, commit, date) in binaries. CI across supported platforms with dedicated jobs for regression baselines, `roombench`, the WASM demo build, and mono/stereo/low-frequency smoke renders. User and contributor documentation (`docs/`) kept in sync with CLI flags and library interfaces. Maintenance procedures: quarterly private-dependency audit, benchmark baseline updates, fixture-first feature workflow (`docs/maintenance.md`).

### Phase 21 — Sound Transmission Between Rooms

Material transmission: `TransmissionByBand` and `SoundReductionIndex` with `tau = 10^(-R/10)` conversion, energy-conservation validation, and transmission data in the materials library. Multi-room scenes: `Rooms []Room`, `Portal` struct with open/closed state, `Portals []Portal` validation, JSON serialization. Secondary source model — portal surface receivers spawn transmission-filtered secondary sources mirrored in the receiving room (ISM) and launched as particles (RT). `ApparentSoundReductionIndex` with flanking-aware variant, validated within 2.5 dB. Portal interaction for the demo: cached open/closed BRIRs with `y(x) = x^(1/n)` aperture crossfade and hard switch to the merged room group. See `docs/sound-transmission.md`.

### Phase 22 — Higher-Order Diffraction and DAPDF

Second-order edge diffraction: edge-to-edge path enumeration with mutual visibility, `SecondOrderDiffractionPaths` (Fermat points on both edges, cascaded coefficients), `MaxDiffractionOrder` config, -60 dB culling, validated against the RAVEN BTME reference. Combined reflection-diffraction paths up to second total order via S2D/D2D/D2R subpath descriptors. DAPDF replacing Keller-cone sampling: piecewise `DAPDF(v, v0, D0)` plus the six closed-form energy integrals. Deflection cylinders (`r = 7*lambda`) with ray-cylinder collision and recursive diffracted rain. Validation against the Svensson Edge Diffraction Toolbox, shadow-zone transition smoothness, and first-order regression at `MaxDiffractionOrder = 1`. See `docs/diffraction.md`.

### Phase 23 — Plane-Polygon Map and ISM Optimizations

`PlanePolygonMap` grouping triangles by plane equivalence; `GenerateMeshImageSources()` mirrors across distinct planes rather than triangles, with point-in-polygon audibility checks on the coplanar set (~4x fewer image sources at 4th order). Per-particle hybrid detection: `RayState` tracking fields (`HasDiffuseHistory`, `PreEDReflOrder`, `EDReflOrder`, `EDOrder`) and `DetectionAllowedHybrid` replacing the time/order-based crossover, with an IS + RT energy conservation test within 2%.

---

## Active / Remaining Phases

> Ordered for serial execution. Each phase lists only open tasks.

### Phase 24 — Extended Source Directivity and SOFA Loading

> Frequency-dependent source directivity and complete SOFA file loading for professional-grade auralization. See `docs/raven.md` Section 12.3.

#### 24.1 Frequency-dependent directivity models (`directivity/`)

- [x] `FrequencyDependentCardioid`: per-band cardioid orders (wide at low freq, narrow at high freq) with interpolation — orders interpolate linearly in log frequency, held flat outside the table
- [x] `BalloonDirectivity`: tabulated gain on spherical grid per frequency band, nearest-neighbor or bilinear lookup — levels stored and interpolated in dB (azimuth wraps, elevation clamps at the poles, frequency blends in log frequency), floored at -200 dB so nulls stay finite
- [x] Integration with GLL loader: extract per-band balloons from GLL data —
      `(*GLLModel).ExtractBalloon` plus the generic `SampleBalloon`. This first
      required fixing the loader: `gll-tools` defers balloon responses to a file
      offset, and `LoadGLL` closed the file before hydrating them, so **GLL
      sources were silently omnidirectional**. `LoadGLL`/`LoadGLLReader` now
      hydrate the balloon, which changes GLL-source output.
- [x] Unit tests: frequency-dependent behavior, grid-point exactness
- [x] Regression: GLL source produces same ISM events as before — pinned as
      structural equivalence (a GLL source yields the same paths, times, and
      distances as an omni source, differing only in band gain) plus a
      guard that the band gains are not all unity, rather than as absolute gain
      values: the "before" behaviour was the omnidirectional bug above, so
      pinning it would have pinned the defect. `ism/gll_directivity_test.go`
      also checks that a balloon extracted from the GLL source is a faithful
      stand-in for it inside the solver.

> **Open:** GLL balloon lookups are wrong for Quarter/Vertical/Horizontal/Axial
> symmetry in `gll-tools` v0.1.1 (its internal grid math cases on different
> symmetry enum values than `pkg/gll.SymmetryType` defines). Fixed on the
> `gll-tools` main branch; awaiting a release tag and a `go.mod` bump. The
> models above are unaffected. See `docs/directivity-gll.md`.

#### 24.2 SOFA file loading (`hrtf/`)

- [ ] SOFA file reader: parse NetCDF-based SimpleFreeFieldHRIR convention (positions, HRIRs, sample rate, delays)
- [ ] Build `MeasurementGrid` from loaded data
- [ ] `LoadSOFA(path string) (*NearestNeighborDataset, error)` convenience function
- [ ] Support CIPIC, LISTEN, and ARI datasets
- [ ] Unit test: load small SOFA file, verify measurement count and known HRIR
- [ ] Integration test: binaural IR with SOFA HRTFs, verify ITD for lateral source

---

### Phase 25 — Multi-Room Acoustic Scene Graph

> Full ASG/PST/PPG infrastructure for complex multi-room environments. Builds on Phase 21's basic portal support. See `docs/raven.md` Section 5 and Section 10.

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
