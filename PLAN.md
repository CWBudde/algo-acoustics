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
>
> Interim: rather than leave the affected symmetries rendering silently wrong,
> the adapter now looks measurements up through the encoding those grid helpers
> actually read (`repairSymmetryCode`, `directivity/gll.go`) — the test fixture
> itself is Quarter-symmetric, so the defect was live in our own tests.
> `TestGLLToolsSymmetryWorkaroundStillNeeded` pins the dependency version and
> fails when the pin moves, so the remapping cannot survive into the release
> that fixes this upstream. **Remove it as part of the `go.mod` bump.**

#### 24.2 SOFA file loading (`hrtf/`)

- [x] SOFA file reader: parse NetCDF-based SimpleFreeFieldHRIR convention
      (positions, HRIRs, sample rate, delays) — via `github.com/cwbudde/go-sofa`
      v0.1.0 over the pure-Go `go-hdf5`, rather than a reader written here
- [x] Build `MeasurementGrid` from loaded data
- [x] `sofa.Load(path string) (*hrtf.NearestNeighborDataset, error)` convenience
      function
- [x] Support CIPIC, LISTEN, and ARI datasets
- [x] Unit test: load small SOFA file, verify measurement count and known HRIR
- [x] Integration test: binaural IR with SOFA HRTFs, verify ITD for lateral source

> **Packaging, and an API removal.** The reader lives in a new `hrtf/sofa`
> package, and the `sofa` build tag is gone along with `hrtf.LoadSOFA` and
> `hrtf.SOFAAdapter`. `go-hdf5` is pure Go, so nothing forced a build tag; and
> because the linker only pulls a package's dependencies into binaries that
> import it, a subpackage keeps HDF5 out of the WASM demo and `roomir` without
> one — while letting the tests run in the ordinary `just test` pass. The tag
> never had a CI job, so its code path was untested by construction.
>
> Removing those two exported symbols means **the next release must be a minor
> bump**. Call-site rewrite: `hrtf.LoadSOFA(path)` (which only ever returned an
> error) → `sofa.Load(path)` from `github.com/cwbudde/algo-acoustics/hrtf/sofa`,
> returning `*hrtf.NearestNeighborDataset`.
>
> Scene JSON deliberately cannot name a `.sofa` path: `scene` is compiled into
> the browser bundle, so importing the loader there would drag HDF5 in with it.
>
> **Upstream first.** `go-sofa` had no tags at all and did not expose the
> `Type`/`Units` attributes naming each position dataset's coordinate system —
> without which spherical `SimpleFreeFieldHRIR` positions are indistinguishable
> from cartesian ones, and reading them wrong misplaces every measurement
> silently. Both were added upstream and released as `go-sofa` v0.1.0 before
> this repo took the dependency, per the ordering rule in AGENTS.md.
>
> `Load` refuses rather than approximates: non-FIR data, receiver counts other
> than two, an unknowable coordinate system, and a missing `Data.SamplingRate`
> all fail with a message naming the cause. That last one is not hypothetical —
> the ARI file in the go-sofa corpus omits the field entirely, and there is no
> safe default. Per-ear `Data.Delay` is folded in by keeping the common delay
> and baking the difference into the later ear's HRIR, since `Lookup` returns a
> single delay for both ears. See `docs/hrtf-sofa.md`.
>
> Dataset support is verified against real files without committing any:
> `ALGO_SOFA_CORPUS=<dir> go test ./hrtf/sofa/` loads every `.sofa` in a
> directory and requires each to either produce a usable dataset or fail for a
> stated reason.

---

### Phase 25 — Multi-Room Acoustic Scene Graph

> Full ASG/PST/PPG infrastructure for complex multi-room environments. Builds on Phase 21's basic portal support. See `docs/raven.md` Section 5 and Section 10.

#### 25.1 Acoustic Scene Graph (`scene/`)

- [x] `AcousticSceneGraph`: nodes = Room, edges = Portal, with counter-portal
      pointers — `PortalView.Counter()` produces the opposite view on demand
      rather than storing a second object per room
- [x] Room groups: connected components of rooms joined by open portals
- [x] `UpdateRoomGroups()` on portal state changes; per-group BVH and simulation
      context via `GroupGeometry`/`GroupBVH`/`GroupScene`. Group geometry cuts
      each open aperture from **both** adjacent rooms' walls; caches are keyed
      on `Scene.RoomGroupHash` rather than the group ID, because opening a
      portal renumbers the groups
- [x] Unit test: 8-room office floor with 10 portals, various open/closed configs

> Two constraints surfaced while building this. Mesh rooms carried a single
> material, so merging shoeboxes would have collapsed six wall materials into
> one; `Room.TriangleMaterials` now carries a per-triangle table through the
> ISM mesh solver and the ray tracer. And a merged group is deliberately not
> edge-manifold — two rooms sharing a partition contribute two coincident
> sheets, one per material — so group volume is the sum of room volumes rather
> than `Mesh.EnclosedVolume`, and closedness uses a group-local even-edge rule.
> Shoebox portals must be rectangular in their wall plane; mesh apertures must
> be an edge loop of the authored mesh. See `docs/scene-graph.md`.

#### 25.2 Path search and source elimination (`scene/`, `hybrid/`)

- [x] Depth-first path search with cycle detection (LIFO stack) and R_w pruning
      — `(*AcousticSceneGraph).SearchPaths` walks the **group** graph, so every
      edge is a closed portal. Marking the groups on the current path is the
      cycle detection, so the search enumerates simple paths only
- [x] Source elimination: skip fully-attenuated frequency bands per propagation
      path — `PathNode.ActiveBands`, floored at -60 dB by default
- [x] Convert PST to PPG: filter functions for portals, RIR/BRIR edges for rooms
      — `hybrid.BuildPPG`, with portal nodes keyed on (portal, direction) so
      shared prefixes converge and a group renders once per entry/exit pair
- [x] Unit test: 3-room chain finds direct path and confirms flanking paths

> RAVEN defines neither how R_w composes along a chain nor an audibility floor.
> We multiply the per-band coefficients (exact for cascaded intensity ratios)
> and recompute a weighted single-number index from that product; summing
> per-portal indices would be wrong when portals differ spectrally.
> `WeightedReductionIndexDB` is explicitly **not** ISO 717-1 and is used only
> for pruning. Portal area, room absorption, and distance are excluded, which
> makes pruning conservative. `PathSearchTree.Truncated` must be surfaced by
> callers or a large building silently under-renders. PPG node sharing costs
> exact per-node band masks, so they are widened to the union/maximum across
> contributing paths — conservative in the same direction.

#### 25.3 Filter network rendering (`hybrid/`, `ir/`)

- [x] Per-path transfer function: `H_PP = H_PS * prod(H_Portal) * prod(H_RoomGroup) * H_R`
      — `hybrid.PathChain` folds the factors by per-band convolution
- [x] Four path types: PS2R, PS2P, SS2R, SS2P with binaural/monaural selection
- [x] Sum all path contributions for final BRIR
- [x] Integration test: office floor with correct level drop per room —
      monotonic decay plus an analytic check against
      `metrics.ApparentSoundReductionIndex`

> The one-hop model could not simply be extended. `ism.SolveSecondary` runs a
> full solve per emitted event, so an N-hop chain costs O(events^N). The
> network runs one simulation per hop and composes by convolution instead;
> since convolving impulse trains is exactly their cartesian product, the two
> agree to floating-point precision, pinned against the Phase 21 renderer at
> image-source orders 0-2. `TransmissionRenderer` is untouched and
> `NewCrossRoomEngine` still selects it for the exact Phase 21 shape.
>
> PS2R/PS2P/SS2R/SS2P are named only in this plan, never in `docs/raven.md`;
> the expansion is ours and is documented as such. Alignment runs once on the
> summed fields, never per path, because per-path early-to-late ratios carry
> the information that makes flanking audible. Multi-room low-frequency
> blending is also unblocked for mono, computed on the receiver's group and
> excited at its entry portal. Remaining limits: one receiver, point-source
> portals, and no directionality on intermediate hops.

#### 25.4 Dynamic portal interaction

- [x] Recompute room groups and PST/PPG on portal state change —
      `(*NetworkRenderer).Prepare` and `(*NetworkPlan).Apply`
- [x] Cache BRIRs per room group; only resimulate affected groups —
      `GroupResponseCache`, keyed on `GroupSignature` rather than the group ID,
      because opening a portal renumbers every group
- [x] Reuse Phase 21.5 crossfade for smooth open/close transitions —
      `AtApertureMerged` follows raven.md 5.3 up to the last step: crossfade to
      the all-pass filter, then fade on into the merged group over the final
      5 % of aperture rather than switching in one buffer
- [x] Performance target: portal change to updated BRIR < 2 s for 4-room
      scenario — **0.55 s warm binaural** (0.39 s mono) against 2.14 s cold on
      an i7-1255U; see `docs/profiling-baseline.md`

> The target holds only in the warm case, and the benchmark says so: the cold
> benchmark builds a fresh renderer each iteration so it cannot inherit the
> warm cache. WASM is not covered — single-threaded WASM is several times
> slower, which is what `DynamicRays` exists for.
>
> raven.md calls the hard switch at full aperture artifact-free without
> conditions. Level-matching is necessary but not sufficient: the all-pass and
> merged responses are separate simulations with different reflection times, so
> a single-buffer swap is discontinuous whatever their levels.
> `AtApertureMerged` therefore crossfades into the merged response over the last
> 5 % of aperture, and `NewPortalBRIRCacheWithFilter` still rejects a pair
> differing by more than 1.5 dB, which such a short fade could not disguise.
>
> The browser demo gained a genuinely merged open endpoint with no demo-side
> change: `NewCrossRoomEngine` routes any open portal to the filter network,
> since the Phase 21 renderer models "open" as a transmissive partition rather
> than merged geometry.

---

### Phase 26 — IBM Boundary Robustness and Cross-Platform Determinism

> A wall that lands exactly on a grid node plane is currently resolved by a
> floating-point tie, so the IBM grid classifies the same room differently on
> amd64 and arm64. This is a correctness defect in shipped Phase 13 code, not a
> flaky test: it makes `TestIBMValidation_EquilateralTriangle` fail on macOS CI
> today. Independent of Phases 24 and 25 — recommended ahead of 25.

#### 26.1 Node-plane degeneracy in grid classification (`pde/`)

Diagnosis, verified locally under `qemu-aarch64` (which reproduces the macOS
result exactly — 19 peaks, 3 matched):

- `ClassifyGrid` computes `origin.Z = center.Z - float64(halfNz)*h`
  (`pde/ibm_grid.go:168`). arm64 contracts that into a single FMA, amd64 does
  not, so the origin differs by 18 ulps (`-0.10000000000000053` vs
  `-0.10000000000000028`). Rebuilding the arm64 binary with
  `-gcflags=all=-d=fmahash=…` makes the classification dump byte-identical,
  which pins FMA contraction as the source.
- The triangle fixture's ceiling sits at `z = 10.0` with `h = 0.1`, so it falls
  exactly on a node plane and the nodes one cell below land exactly at `frac == 1`.
- `fracToWall` ends with `if frac > 1 { return 0 }` (`pde/ibm_grid.go:337`).
  amd64 computes `1.0000000000000142` (→ `0`, "no wall within a cell"), arm64
  computes `0.9999999999999964` (→ sub-cell wall). The two branches mean
  opposite things, so **375 ceiling nodes get a different boundary condition per
  architecture** — pressure-release versus rigid.
- Ruled out: instability (a one-ulp seed grows only to 2.7e-13 over 3126 steps)
  and small cut cells (smallest fraction 0.036).
- The shoebox path (3.0x2.5x2.0, `h = 0.05`) has zero flips today, but by luck:
  its fractions sit at `0.9999999999999992`, ~8e-16 from the same cliff. Any
  room whose dimensions are an exact multiple of `h` is exposed.

The measurement that settles it: counting only directions whose neighbour is
genuinely exterior (the earlier count conflated those with unwritten `Frac`
slots, which are also 0),

| fixture                    | ext-neighbour dirs | `frac == 0` | `frac == 1` | just below 1 | sub-cell |
| -------------------------- | ------------------ | ----------- | ----------- | ------------ | -------- |
| shoebox 3x2.5x2, h = 0.05  | 14206              | **2891**    | 0           | 11315        | 0        |
| rect 4x4x4, h = 0.5        | 294                | 0           | **294**     | 0            | 0        |
| triangle L=3 z=10, h = 0.1 | 11442              | **375**     | 0           | 375          | 10692    |

`rect 4x4x4` at `h = 0.5` is the control: origin and spacing are exact in binary,
nothing rounds, and every wall direction comes out at exactly 1 — which is why
`TestIBMStencil_UniformFieldLaplacianZero` passes on it. The shoebox's 2891 zeros
and the triangle's 375 are the same geometry rounding the other way. So `frac == 0`
on an exterior neighbour was never a semantics to be chosen, it was a bug, and
the exact fixture shows the intended value is 1. It also means the shoebox has
been modelling 20 % of its wall directions as pressure-release rather than rigid
since Phase 13 — a physics defect, not only a portability one.

- [x] Define the intended meaning of `BoundaryInfo.Frac == 0` for a direction whose neighbour is **exterior** — it is now unreachable. `Frac == 0` means only "the neighbour is an active node"; an exterior neighbour always carries `Frac` in `(0, 1]` (`pde/ibm_grid.go`)
- [x] Make the stencil consistent with that definition — no stencil change was needed once the sentinel became disjoint. `fracToWall` returns `(frac, ok)`, clamps a fraction above 1 instead of returning 0, snaps a near-tie to exactly 1 (`nodePlaneTol = 1e-9`) and floors a degenerate cut cell (`minWallFrac = 1e-6`)
- [x] Remove the dependence on rounding — `offsetFromOrigin` rounds `i*h` explicitly before adding, for both the origin and every node position, and `sideOf` is an FMA-free `Plane.SideOf` shared by `PointInside`, `DistanceToNearestWall` and `fracToWall` so the inside test and the wall distance cannot disagree. `geometry.Plane.SideOf` is left alone: it is the ISM/ray-tracing hot path and decides no ties
- [x] Regression: `ClassifyGrid` output byte-identical between amd64 and arm64 — `TestClassifyGridGolden`, verified under `qemu-aarch64`

> Because amd64 does not contract FMA today, the FMA-free rewrite reproduces the
> amd64 classification bit-for-bit and arm64 converges onto it: the class hashes
> are unchanged for all three fixtures. The only intended amd64 behaviour change
> is the frac clamp.
>
> One residual, deliberately not chased: the triangle fixture's own planes are
> built from `math.Sqrt(3)` in the test helper and differ by 2 ulps across
> architectures, so its _fraction_ hash is not portable. Its classification is,
> which is the point — a 2-ulp wall plane no longer flips 375 boundary
> conditions. The golden test therefore covers the exact-multiple fixtures, and
> `TestClassifyGridFracInvariant` covers the triangle.

#### 26.2 Triangle modal validation correctness (`pde/ibm_validation_test.go`)

Two defects that made the test a coin flip, which is why a 1-ulp difference could
decide pass/fail at all:

- [x] `nSteps` now derives from the timestep the solver actually uses — `runFDTD` takes a duration in seconds and computes `nSteps` from `0.95*stencil.CFLLimit(c)`. The triangle runs 16504 steps rather than 3126, and all three modal tests realise a true 500 ms and a 2.00 Hz resolution
- [x] The inverted "tall z isolates the 2D modes" premise is gone: `prismEigenfreqs` combines any cross-section's 2D set with the axial family, and both the triangle and the cylinder are scored against their full 3D mode set. Against the 2D-only set the solver's recall lands _below_ chance (−4 pp triangle, −15 pp cylinder), i.e. the peaks avoid those frequencies — the 2D sets were actively wrong
- [x] Match tolerance is `max(0.5 %·f, 1.5·df)` with `df = 1/(nSteps·dt)`, the true resolution rather than the zero-padded bin spacing
- [x] Pass criterion retuned — see below

Retuning turned up two further problems, both of which had to be fixed before a
threshold could mean anything:

- **The peak-first count is uninformative.** `logChanceLevel` reports how much of
  the band the tolerance windows cover: 81 % for the shoebox, **97.5 %** for the
  prism. At that density every possible peak matches something, so 40/40 is
  arithmetic, not evidence. Scoring is now `requireModeRecall` — how many
  analytical modes the solver actually _produced_ — which gets harder, not
  easier, as the mode set thickens, and the test fails outright if recall ever
  falls to the chance level.
- **A sealed rigid room has nowhere to put the source's injected volume.** With
  the walls correctly rigid after 26.1, the receiver's time series settles on a
  DC offset whose bin is ~49x the strongest acoustic mode, and the peak
  threshold — 1 % of the _global_ maximum — then rejects nearly every mode: the
  shoebox dropped from 40 detected peaks to 5. This was masked before 26.1
  precisely because those 2891 pressure-release directions vented the enclosure.
  `extractPeakFreqs` now subtracts the mean, applies a Hann taper, and takes its
  threshold from the in-band maximum.

Final scores, identical on amd64 and arm64:

| test             | band      | recall | chance | threshold |
| ---------------- | --------- | ------ | ------ | --------- |
| shoebox          | 30–153 Hz | 67 %   | 39 %   | ≥ 60 %    |
| triangular prism | 30–187 Hz | 35 %   | 27 %   | ≥ 30 %    |
| cylinder         | 20–147 Hz | 36 %   | 28 %   | ≥ 30 %    |

> **Revised 2026-08-16 after PR review — the first table published here was
> wrong, and materially so.** It read shoebox 75 %, prism 58 % / 40 % chance over
> 30–500 Hz, cylinder 73 % / 57 % over 20–400 Hz. Three defects inflated it:
>
> - **Recall double-counted.** Each analytical mode scanned the whole peak list
>   independently, so one detected peak could satisfy dozens of modes. With ~40
>   peaks against the prism's 953 modes, most of that 58 % was a handful of peaks
>   counted over and over. Peaks are now consumed at most once, and modes closer
>   together than the resolution are clustered first, since they are not
>   separable even in principle.
> - **The triangle's analytical set was at half its true frequencies.**
>   `triangleEigenfreqs` used `c/(3L)`; Lamé's closed form for the equilateral
>   triangle gives `2c/(3L)·√(m²+mn+n²)`. Every transverse mode of the prism was
>   therefore compared against a frequency the room cannot produce.
> - **The cylinder's analytical set was truncated.** `besselPrimeZeros` was a
>   13-entry table stopping at angular order m = 4, so it omitted whole mode
>   families — including four roots inside its own range. The roots are now
>   computed to the band edge, giving 32 for this fixture rather than 13.
>
> With all three corrected, the prism and cylinder fall **below** their chance
> levels when scored over the full band — i.e. over 30–500 Hz those fixtures
> demonstrate nothing, and no threshold could have made them meaningful.
> Lengthening the run does not help: at 2 s the resolution improves to 0.5 Hz and
> the set resolves into 73 clusters, but the solver still yields only 40 peaks,
> capping recall at 55 % by construction.
>
> Both are therefore scored on their sparse low bands, where the tolerance
> windows cover ~27 % of the band instead of ~100 %, and both clear chance by
> about 8 points. That is a real but weak claim, and weaker than what this
> document previously asserted. The shoebox — 67 % against 39 % — remains the
> strong one. `requireModeRecall` fails outright if any of them falls back to its
> chance level, so that margin, not the threshold, is the real assertion.

#### 26.3 Cross-platform determinism guard

- [x] Golden test over `ClassifyGrid` — `TestClassifyGridGolden` (dims, class counts, FNV-64a fraction hash) over three fixtures whose dimensions are exact multiples of `h`, plus `TestClassifyGridFracInvariant` and `TestNodePositionsFMAFree` (`pde/ibm_determinism_test.go`)
- [x] Hazard documented in `docs/maintenance.md` — the contraction rule, the explicit-conversion suppression idiom, the qemu reproduction procedure and the `-d=fmahash=` bisection
- [x] arm64 in CI — `arm64-determinism` job in `.github/workflows/test-unit.yml`, `docker/setup-qemu-action` plus `GOARCH=arm64 go test ./pde/... ./geometry/...`. Scoped to the tie-sensitive packages because emulation is ~10x slower; the macOS job still covers the full portable suite on native arm64

> **Also open (unrelated dependency bug, same CI job) — resolved:** the macOS
> integration job failed to build `algo-vecmath@v0.1.1`, whose
> `arch/arm64/neon/dotproduct.s` used the ARM32 VFP mnemonics `VFMULD`/`VFADDD`
> in an arm64 file. Fixed upstream in v0.1.2 (scalar `FLDPD` pair loads with two
> `FMADDD` accumulators, not the vector form); `go.mod` is bumped and
> `GOARCH=arm64 go build ./...` now succeeds for the whole tree.
>
> Still open upstream, found while verifying the above: at v0.1.2
> `arch/arm64/neon/scale.s` branches to the scalar path _past_ the `ANDS` that
> initialises the loop counter, so `ScaleBlock`/`ScaleBlockInPlace` read off the
> end of the buffer for a single-element slice. Reproduced under qemu.
> `algo-acoustics` does not call those entry points, so it is not blocking here.
> **Resolved upstream in `algo-vecmath` v0.1.3** (2026-08-15), which fixes the
> out-of-bounds write and rewrites the NEON backend as true SIMD. `go.mod` is
> bumped; see Phase 27.

---

### Phase 27 — Dependency Currency Across the `algo-*` Stack (Complete)

> Not a feature phase. The four sibling modules had drifted onto three different
> `algo-fft` versions, which made it impossible to take an upstream fix in one
> without breaking another. This phase brings the whole stack onto current tags
> and keeps it there.

The state that triggered it (2026-08-15):

| module           | pinned `algo-fft` | own latest tag           |
| ---------------- | ----------------- | ------------------------ |
| `algo-pde`       | v0.6.15           | v0.2.1                   |
| `algo-dsp`       | v0.7.3            | v0.6.0, `main` untagged  |
| `algo-acoustics` | v0.6.11           | —                        |
| `algo-fft`       | —                 | v0.7.4, `main` +87 ahead |

`algo-fft`'s generic `PlanReal2D` / `PlanReal3D` changed signature between the
v0.6 and v0.7 lines, so code written against v0.6.x fails to compile with
`cannot use generic type PlanReal2D[...] without instantiation`. That is why
`algo-acoustics` could not simply take `algo-dsp` `main`: the bump pulls
`algo-fft` v0.7.x in, which `algo-pde` v0.2.1 does not build against.

- [x] Released `algo-fft` **v0.8.0** from `main` (97 commits since v0.7.4). A
      minor rather than patch bump because `KernelEightStep` was removed — it
      duplicated `KernelSixStep`, and enum value 4 is retired rather than reused.
      The orphaned `[0.7.5]` changelog section was given a retroactive `v0.7.5`
      tag at the commit that introduced it, so no release note claims a tag that
      does not exist.
- [x] Moved `algo-pde` to `algo-fft` v0.8.0 and released **v0.2.2**. The generic
      migration was `PlanRealT[F, C]` → `PlanReal[F, C]` plus instantiation of
      the 2D/3D plans. **The float32 → float64 promotion was deliberately not
      taken**: the new `NewPlanReal2D64`/`3D64` constructors would have silently
      deleted `WithFloat32`/`WithRealFFT`, which are a documented public accuracy
      trade (~1e-6 vs ~1e-14) with `UsedRealFFT()` reporting which path ran, and
      would have broken the test that asserts the two paths _differ_. Patch, not
      minor: algo-pde's own API and numerical output are unchanged. The stale
      `poisson/options.go` comment blaming the dependency for the float32 buffers
      was corrected.
- [x] Moved `algo-dsp` to `algo-fft` v0.8.0 and released **v0.7.0** — a pure
      dependency move, no source change (`KernelEightStep` appears nowhere, and
      the repo persists no wisdom files to invalidate). Minor rather than patch
      because the release carries the fused-AXPY `conv.DirectTo` rewrite, whose
      arm64 output is not bit-identical to v0.6.0's. Its benchmark guard reports
      identical allocation counts and `B/op` moving only downward.
- [x] Bumped all four here — algo-fft v0.6.11 → v0.8.0, algo-pde v0.2.1 → v0.2.2,
      algo-dsp v0.5.1 → v0.7.0, algo-vecmath already v0.1.3. **No source change
      was needed**: this repo only uses `NewPlan64`, `NewPlanReal64`,
      `*Plan[complex128]` and `Forward`/`Inverse`, none of which moved.
- [x] Confirmed the regression fixtures on **both** architectures. `roombench`
      reports 4/4 against the checked-in baselines on amd64 and, under
      `qemu-aarch64-static`, on arm64. `hybrid`, `ir`, `metrics`, `pde` and
      `geometry` all pass on arm64 too — `hybrid/network.go` is the caller that
      made this necessary, since it uses `conv.Convolve`/`conv.Direct` directly.
      So the predicted arm64 ulp shift does not move any fixture.
- [x] Added a standing guard, in this repo and the seven siblings:
      `scripts/release-guard.sh`, exposed as `just check-deps`,
      `just check-unreleased` and `just tag-release vX.Y.Z`, plus a weekly
      `dep-drift` workflow that files an issue, and a Renovate config that
      groups the whole `cwbudde` family into one bump PR. The gate refuses to
      tag on a dirty tree, an unpushed default branch, an out-of-order or
      existing tag, stale siblings, a missing CHANGELOG section, or an
      incompatible API change the version does not signal. **That last rule is
      deliberately stricter than semver**, which exempts `v0.x` — every module
      here is `v0.x`, so `gorelease` alone would have approved the `v0.7.6`
      patch bump across the `KernelEightStep` removal. Replaying that exact
      case, the guard refuses and demands `v0.8.0`. It immediately found two
      live drifts nobody had flagged: `algo-dsp` pinning `algo-approx v0.1.0`
      against `v0.2.0`, and `gll-tools` carrying a zero pseudo-version for
      `go-sofa` that only resolves through a local `replace`.

> **Pre-existing, found while verifying and deliberately not fixed here:**
> `gpu/worker` does not build for any non-Linux `GOOS`. `shm_linux.go` is
> `//go:build linux` but `fdtd.go` carries no tag, so it fails on `darwin/arm64`
> and `darwin/amd64` alike — confirmed identical at the pre-bump commit, so it is
> unrelated to this phase. `linux/arm64` builds fine, which is why the emulated
> arm64 CI job never caught it. It needs its own fix.

---

## Dependency Map

```text
algo-acoustics
├── algo-dsp          (convolution, FFT, metrics, filtering)
│   ├── algo-fft      (transitive; also a direct require here)
│   ├── algo-vecmath  (transitive; SIMD block kernels)
│   └── algo-approx   (transitive)
├── algo-pde          (Helmholtz shoebox, Phase 8+)
│   └── algo-fft      (transitive)
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
| Portability | grid classification identical on amd64/arm64 (FMA contraction)       |
