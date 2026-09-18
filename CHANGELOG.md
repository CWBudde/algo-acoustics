# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed — breaking

The root package `algoacoustics` held three unrelated jobs and 85 exported
declarations, only 11 of which were used outside it. It now holds only the
renderer and its engine contracts (18 declarations). Two groups moved out.

**Multi-room propagation moved to `github.com/cwbudde/algo-acoustics/crossroom`.**
It stays public. Symbols were de-stuttered for the new package name:

| Before (`algoacoustics.`) | After (`crossroom.`) |
| --- | --- |
| `NewCrossRoomEngine` | `NewEngine` |
| `CrossRoomEngineConfig` | `EngineConfig` |
| `CrossRoomEngine` | `Engine` |
| `CrossRoomLateEngine` | `LateEngine` |
| `TransmissionEarlyEngine` | `EarlyEngine` |
| `NetworkRenderer` | `Network` |
| `NewNetworkRenderer` | `NewNetwork` |
| `NetworkRendererConfig` | `NetworkConfig` |
| `NetworkPlan` | `Plan` |
| `NetworkTruncation` | `Truncation` |
| `TransmissionRenderer` | `OneHop` |
| `NewTransmissionRenderer` | `NewOneHop` |
| `TransmissionRendererConfig` | `OneHopConfig` |
| `GroupResponseCache` | `ResponseCache` |
| `NewGroupResponseCache` | `NewResponseCache` |
| `DefaultGroupResponseCacheBytes` | `DefaultCacheBytes` |
| `LowFreqSceneForMultiRoom` | `LowFreqScene` |
| `MultiRoomLowFreq` | `LowFreq` |

Call-site rewrite:

```go
// before
engine := algoacoustics.NewCrossRoomEngine(sc, algoacoustics.CrossRoomEngineConfig{...})
// after
engine := crossroom.NewEngine(sc, crossroom.EngineConfig{...})
```

`algoacoustics.CrossRoomEngine` still exists in the root package as an alias for
`BinauralLateBufferEngine`, documenting the `Renderer.Transmission` field.
`crossroom.Engine` declares the same method set independently, so an engine from
either package satisfies the other without an import between them.

**Removed from the public API** — these were exported only for cross-file use
inside the old root package and could not be meaningfully constructed by a
caller: `GroupResponseKey`, `GroupFactor`, and `GroupResponseCache`'s `Get`,
`Put` and `InvalidateSignature`. `ResponseCache.Stats` and `CacheStats` remain.

**Progressive preview machinery moved to `internal/preview`** and is no longer
public: `RenderProgressive`, `ProgressiveConfig`, `Tier`, `TierResult`,
`UpdateFunc`, `StatisticalMetrics`, `ComputeStatisticalMetrics`, `QualityPreset`,
`PresetConfig`, `StatisticalTailConfig`, `SynthesizeStatisticalTail`,
`ReplaceStatisticalTail`, `Debouncer` and `NewDebouncer`. It served the WASM demo
rather than the rendering contract, and no caller outside this module used it.

**`RaytraceEngineConfig` moved to `raytrace.EngineConfig`.**
`algoacoustics.RaytraceEngineConfig` remains as a type alias, so existing call
sites compile unchanged. The new home lets the root adapter and the cross-room
engines share one config without depending on each other. The direction-group
defaulting, previously duplicated in three places, is now
`raytrace.EngineConfig.DirectionGroupCounts`.

This is pure code motion: `just test-regression` and `just test-integration`
produce identical output before and after.

## [v0.1.0]

First tagged release. Everything below was already on `main`; this entry records
what that amounts to for a consumer. algo-acoustics computes room impulse
responses in pure Go: from a `scene.Scene` — shoebox or triangle-mesh geometry,
band-dependent materials, sources, receivers — it runs an image-source solver for
the early specular field, a Monte Carlo ray tracer for the diffuse late field,
and a Helmholtz sweep for the modal region below the crossover, then blends the
three into one `ir.Buffer` that can be written as a mono or binaural WAV. The
geometric engines are exposed behind interfaces (`EventEngine`,
`LateBufferEngine`, `LowFreqEngine`) so a caller can replace any stage without
replacing the pipeline.

### Added

- `scene.Scene` as the single root container for a simulation: room geometry,
  a material table, sources, receivers, the octave band spec, and the sample
  rate. Rooms are either an analytic `scene.Shoebox` or a triangle
  `geometry.Mesh` loaded from OBJ, with per-triangle material assignment.
  `scene.LoadSceneFile` reads the JSON form, `scene.Validate` rejects a scene
  before any engine touches it, and `scene.Summary` prints a normalized view of
  what was actually parsed.
- `scene.Material` carries absorption, scattering, and transmission as
  per-band slices rather than single numbers, with `scene.MaterialLibrary` for
  named common surfaces and `LoadMaterialJSON`/`LoadMaterialCSV` for measured
  data. Scattering can be estimated from surface relief depth
  (`EstimateScatteringFromDepth`), and transmission converts to and from the
  ISO sound reduction index (`SoundReductionIndexFromTransmission` and its
  inverse) so partition data can be entered in whichever form it was published.
- Image-source method in `ism`, for both room kinds. `GenerateImageSources`
  mirrors a source through shoebox walls; `GenerateMeshImageSources` does the
  same against arbitrary mesh planes and validates each candidate against a BVH
  before it is allowed to emit. Generation is split from evaluation:
  `EvaluateShoebox` and `EvaluateMesh` replay an already-computed set of image
  sources against current materials, so changing an absorption coefficient does
  not re-run the geometric search. `ShoeboxCache` and `MeshCache` hold that
  geometry, keyed on `(*scene.Scene).GeometryHash`, which is what makes it safe
  to reuse across renders — the cache invalidates itself when the geometry
  moves.
- Monte Carlo ray tracing in `raytrace`, producing a banded
  `EnergyHistogram` rather than sparse events, because collapsing the late field
  to one pressure event per time bin would throw away its band-energy
  distribution. Rays launch on a Fibonacci sphere or stratified directions,
  reflect specularly or by Lambert scattering according to the per-band
  scattering coefficient, and lose energy to material absorption and to ISO
  9613-1 atmospheric attenuation (`AlphaAirISO9613_1`). Detection is by
  `SphereReceiver`, by `SurfaceReceiver` for planar diffuse rain, or by
  diffuse rain from every reflection point. The same trace/evaluate split as
  the ISM applies: `TracePaths` walks geometry only and `EvaluatePaths` replays
  a `PathCache` against materials.
- `DirectivityGroup` bins the late field by arrival direction (default 12
  azimuth by 6 elevation), which is what lets the same histogram serve both the
  mono buffer and the binaural synthesis instead of requiring a second trace.
- Low-frequency modal content in `pde`. `PDELowFreqEngine` sweeps the Helmholtz
  equation over the modal region (20–300 Hz across 48 points by default) and
  returns a `pde.TransferFunction`, with rigid or impedance wall boundary
  conditions. `ShoeboxModes` gives the analytic axial/tangential/oblique mode
  list for comparison. The package also carries the time-domain FDTD
  machinery — `ClassifyGrid` and `IBMStencil` implement an immersed-boundary
  classification for convex rooms whose walls do not fall on grid planes — but
  the shipped `LowFreqEngine` itself still requires a shoebox and returns an
  error for anything else.
- `Renderer` in the module root, wiring an `EventEngine` (early), a
  `LateBufferEngine` (late), and a `LowFreqEngine` into `RenderMono` and
  `RenderStereo`. The shipped adapters are `NewISMEngine`, which takes one or
  more sources and exactly one receiver, and `NewRaytraceEngine`, which takes
  exactly one of each and deliberately does not implement `EventEngine`.
- Crossover blending in `hybrid`. `HybridConfig` selects a time-based or
  reflection-order-based early/late split with an optional smoothed fade
  window; `AlignLateTail` positions the histogram-derived tail against the
  early events rather than assuming both start at `t = 0`; `BlendLowFreq`
  merges the modal transfer function into the geometric IR at 200 Hz by
  default, overridable through `LowFreqCrossoverProvider`.
- Progressive rendering through `RenderProgressive`, which reports four tiers
  in increasing cost — `TierStatistical` (Sabine/Eyring estimates from geometry
  and absorption alone, no simulation), `TierPreview`, `TierRefined` (ray
  batches reported as they land), `TierFinal` — so an interactive caller can
  show something immediately and refine in place. `PresetConfig` maps the
  `QualityDraft`/`QualityPreview`/`QualityFinal` presets onto concrete ISM
  orders, ray counts, IR lengths, and band resolutions.
  `SynthesizeStatisticalTail` and `ReplaceStatisticalTail` splice a statistical
  tail onto a partially traced response.
- Binaural rendering: `ir.RenderBinaural` convolves sparse events through an
  `hrtf.Dataset` in head coordinates (`scene.Receiver.WorldToHeadDir`), and
  `ir.RenderBinauralPoisson` synthesizes the late field from the directional
  groups as a Poisson-distributed reflection sequence. `hrtf` provides
  `NearestNeighborDataset`, barycentric `InterpolatingDataset`, and
  `NoopDataset`; `metrics.IACC` measures the result.
  `Renderer.RenderStereo` deliberately omits the low-frequency blend — one
  monaural transfer function cannot carry ear-specific HRTF information.
- `hrtf/sofa.Load` reads measured HRTFs from SOFA (AES69) files —
  `SimpleFreeFieldHRIR` and the CIPIC, LISTEN, and ARI datasets built on it —
  into a `hrtf.NearestNeighborDataset`, over `github.com/cwbudde/go-sofa` and a
  pure-Go HDF5 backend, so no cgo is involved. It refuses rather than
  approximates: non-FIR data, a receiver count other than two, an unknowable
  coordinate system, or a missing `Data.SamplingRate` each produce a named
  error. Per-ear `Data.Delay` is folded in by keeping the common delay and
  baking the difference into the later ear's HRIR as leading zeros, since
  `Lookup` returns one delay for both ears.
- Source directivity behind `directivity.Model`, a single
  `GainLinear(freqHz, direction)` method. Implementations: `OmniModel`,
  `CardioidModel`, `FrequencyDependentCardioid` (per-band order, so a
  loudspeaker can narrow with frequency), tabulated `BalloonDirectivity`, and
  `LoadGLL`, which extracts per-band balloons from a GLL loudspeaker file for a
  named preset via `gll-tools`. `SampleBalloon` bakes any model down to a
  tabulated grid.
- Edge diffraction, opt-in through `ism.ISMConfig.EnableDiffraction` and
  `MaxDiffractionOrder`. First order finds the interior Fermat point on each
  audible finite edge and evaluates `geometry.BTMETransfer`, the finite-edge
  Biot-Tolstoy-Medwin expression, at each band center; second order enumerates
  ordered edge pairs plus the two mixed reflection-diffraction paths and
  composes them with RAVEN's midpoint approximation. Diffraction sources
  activate only where the corresponding direct or specular path is occluded, so
  the geometric and diffracted fields are not counted twice at a shadow
  boundary. The late field has a stochastic counterpart in the DAPDF deflection
  model. The BTME result is checked against 60 values generated by Svensson's
  official EDB2 toolbox (15 receiver angles at 50 Hz, 500 Hz, 5 kHz, 10 kHz);
  the fixture and its generator are in `testdata/diffraction`. Diffraction
  requires mesh edges — shoebox rooms do not acquire synthetic ones.
- Multi-room scenes. `scene.Portal` joins two rooms across a planar polygon
  with its own material and an open/closed state;
  `scene.NewAcousticSceneGraph` merges rooms connected by open portals into
  groups the single-room engines can then simulate as one space;
  `scene.PathSearchTree` and `hybrid.BuildPPG` enumerate and prune propagation
  paths between groups. Two renderers consume that: `TransmissionRenderer`, the
  fast one-hop secondary-source model for exactly two adjacent shoeboxes and
  one portal, and `NewNetworkRenderer`, which composes each path as a product
  of separately simulated group transfer functions convolved per band. The
  filter-network form is why an arbitrary room chain is tractable — re-emitting
  events costs a full solve per event per hop and is exponential in hop count,
  while convolution costs one simulation per hop however many events arrive.
  `GroupResponseCache` (256 MiB by default) retains those group responses, and
  a portal opening or closing re-simulates only the affected groups.
- GPU offload in `gpu/`: a standalone CUDA server binary speaking a small
  protocol over a Unix socket with bulk data in POSIX shared memory, with
  kernels for FDTD stepping and BVH ray traversal. The Go side stays CUDA-free;
  `worker.StartIfAvailable` and `worker.Probe` report `ErrGPUUnavailable` when
  no server binary or no device is present, so callers fall back to CPU without
  special-casing. It requires Linux x86_64, an NVIDIA device of compute
  capability 75 or newer, and CUDA 12.x to build. Note that `gpu/worker` is a
  standalone package: nothing in `Renderer` reaches for it yet, so using it
  today means calling it directly.
- Browser demo under `web/`, compiled to WASM and run in a dedicated worker. It
  exposes `algoAcousticsDemo.renderScene()` and a small scene-editing API to
  JavaScript, and runs the real early, late, and hybrid paths — not a reduced
  model. It covers a shoebox with editable wall materials, source and receiver
  placement, a 3D preview, an optional SPL heatmap on the room boundary, and
  auralization of sample audio through the rendered IR. A request envelope and
  a memory budget bound each render, and the progressive tiers are what keep
  the UI responsive inside it.
- Three CLIs. `roomir` validates, inspects, renders (`render`,
  `render-stereo`), dumps sparse events as JSON or CSV, and compares two WAV
  files by peak, RMS, correlation, and per-band deltas. `roomplot` prints
  material band tables, scene summaries, and source directivity by azimuth.
  `roombench` runs the regression corpus in `testdata/` against checked-in
  metric baselines and reports drift. Its envelopes are internal determinism
  guards — they say the renderer has not changed, not that it matches a
  measurement or a third-party tool.
- Metrics in `metrics`: `T20`, `T30`, `EDT`, `C50`, `C80`, `D50`,
  `T60FromDecaySlope`, and `IACC` measured from an `ir.Buffer`, alongside
  closed-form `SabineRT60`, `EyringRT60`, `CriticalDistance`, `EstimateC80`,
  and `EstimateD50` from `RoomStats`, so a simulated value can be checked
  against the analytic expectation for the same room. Building isolation is
  covered by `ApparentSoundReductionIndex` and
  `FlankingApparentSoundReductionIndex`.
- Export in `export`: mono and stereo WAV to a file or to `[]byte` (the latter
  for the WASM demo, which has no filesystem), scene JSON, and events, metrics,
  and comparison tables as JSON, CSV, or Markdown.
- A release guard, `scripts/release-guard.sh`, wired to `just release-check`,
  `just tag-release`, `just check-deps`, and `just check-unreleased`. It refuses
  to tag a dirty tree, a version with no CHANGELOG section, stale
  `github.com/cwbudde/*` siblings, or an incompatible API change that the
  version number does not signal — a minor bump is required for any API break
  even on `v0.x`, where semver would permit a patch. The reasoning is in
  `AGENTS.md`.

### Removed

- `hrtf.LoadSOFA` and `hrtf.SOFAAdapter`, replaced by the `hrtf/sofa`
  subpackage.

  old: `hrtf.LoadSOFA(path string) (*hrtf.SOFAAdapter, error)`, which required
  the `sofa` build tag
  new: `sofa.Load(path string) (*hrtf.NearestNeighborDataset, error)`,
  importing `github.com/cwbudde/algo-acoustics/hrtf/sofa`

  ```go
  // before
  dataset, err := hrtf.LoadSOFA("subject_003.sofa") // built with -tags sofa

  // after
  dataset, err := sofa.Load("subject_003.sofa")
  ```

  This break is compile-time only, not behavioural. Both build-tag variants of
  `LoadSOFA` returned an error unconditionally — `"SOFA support requires the
  sofa build tag"` untagged, `"SOFA support is not wired to a concrete go-sofa
  API in this build"` tagged — so no caller could ever have held a working
  dataset to lose. Code that called it was, necessarily, code that handled its
  error.

- The `sofa` build tag itself. Nothing is conditional on it any more, so
  `-tags sofa` is now meaningless; drop it from build and test invocations. The
  subpackage is the replacement mechanism and a deliberate one: the HDF5
  dependency links only into binaries that import `hrtf/sofa`, so the WASM demo
  and the `roomir` CLI pay nothing for it, while the loader's tests run in the
  ordinary test pass instead of behind a tag that CI never set.
