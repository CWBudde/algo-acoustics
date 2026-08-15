# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

algo-acoustics is a Go library for room acoustics simulation. It computes impulse responses by combining geometric propagation (image-source method, Monte Carlo ray tracing) with modal analysis (Helmholtz PDE solver), blending them via a frequency-domain crossover. Outputs mono or binaural WAV files. Includes a WASM-based browser demo.

## Build & Test Commands

All commands use `just` (justfile-based task runner):

```bash
just build              # Build CLI binaries (roomir, roomplot, roombench)
just test               # Run all tests (go test -v ./...)
just test-race          # Tests with race detector
just lint               # golangci-lint (all linters enabled minus explicit exclusions)
just lint-fix           # Auto-fix lint issues
just fmt                # Format with treefmt (gofumpt + gci + prettier)
just ci                 # Full pipeline: format check + test + lint + tidy check
just bench              # Run benchmarks
```

Run a single test:

```bash
go test -v -run TestName ./package/...
```

Private dependencies require: `GOPRIVATE=github.com/cwbudde` (set in justfile).

## Architecture

### Rendering Pipeline

```
Scene → validate
  ├─ NewISMEngine → []ir.Event (sparse direct/specular early field)
  │                    └─ ir.RenderMono/RenderBinaural → ir.Buffer
  ├─ NewRaytraceEngine → energy histogram (dense late field)
  │                    ├─ mono buffer synthesis
  │                    └─ directional groups → binaural Poisson/HRTF synthesis
  └─ LowFreqEngine.Transfer() → pde.TransferFunction (mono only)
                                   ↓
                 time/frequency crossover → final IR → WAV/export
```

Orchestrated by `Renderer` in `renderer.go` at the module root.

### Key Interfaces

- **`EventEngine`** (`renderer.go`) — generates sparse pressure events. The shipped `NewISMEngine` adapter supports one or more sources and exactly one receiver.
- **`LateBufferEngine` / `BinauralLateBufferEngine`** (`renderer.go`) — dense late-field rendering. The shipped `NewRaytraceEngine` adapter requires exactly one source and receiver and preserves directional groups for binaural synthesis; it deliberately does not implement `EventEngine`.
- **`LowFreqEngine`** (`renderer.go`) — produces a frequency-domain transfer function for modal blending. Implemented by `pde.PDELowFreqEngine`.
- **`directivity.Model`** — source directivity (omni, cardioid, GLL balloon). Single method: `GainLinear(freqHz, direction)`.
- **`hrtf.Dataset`** — binaural HRTF lookup. `Lookup(direction) → (left, right, delay)`.

### Package Roles

| Package         | Role                                                                            |
| --------------- | ------------------------------------------------------------------------------- |
| `acoustics`     | Physical constants, octave band specs (`Octave6`, `Octave8`)                    |
| `geometry`      | Vec3, Ray, Plane, Triangle, BVH, intersection tests, quaternions                |
| `scene`         | Room definition (shoebox/mesh), materials, sources, receivers, validation       |
| `directivity`   | Source directivity models behind `Model` interface                              |
| `hrtf`          | HRTF dataset lookup/interpolation; tagged SOFA adapter is still a loading stub  |
| `ir`            | Sparse `Event` → dense `Buffer` rendering, band gain aggregation, normalization |
| `ism`           | Image-source method solver (early specular reflections)                         |
| `raytrace`      | Monte Carlo ray tracer (late-field diffuse energy histograms)                   |
| `pde`           | Helmholtz solver for low-frequency modal content                                |
| `hybrid`        | Crossover blending of early/late events and geometric/modal IRs                 |
| `metrics`       | IR comparison, acoustic metrics                                                 |
| `export`        | WAV/JSON/CSV output                                                             |
| `cmd/roomir`    | Main CLI (validate, render, render-stereo, dump-events)                         |
| `cmd/roombench` | Regression benchmark runner                                                     |
| `web/wasm`      | WASM entry point exposing `renderScene()` to JavaScript                         |

### Data Flow Types

- `ir.Event` — sparse: time, amplitude, direction, distance, per-band gains, phase, kind (Direct/Specular/Diffuse/PDE)
- `ir.Buffer` — dense: `[]float64` samples at a sample rate
- `scene.Scene` — root container: room geometry, materials, sources, receivers, band spec, sample rate
- `pde.TransferFunction` — complex H(f), converted and blended only for mono rendering

### Sibling Libraries

All under `github.com/cwbudde` (private):

- `algo-dsp` — convolution, FFT wrappers, acoustic metrics
- `algo-fft` — FFT operations
- `algo-pde` — Helmholtz/Poisson grid solves
- `gll-tools` — GLL loudspeaker balloon parsing
- `wav` — WAV codec

## Code Conventions

- **All floating-point is `float64`** throughout geometry, acoustics, and IR processing.
- **Frequency bands are processed in parallel arrays** (6 or 8 octave bands); combined at render time.
- **White-box tests** — test files live in the same package (not `_test` packages).
- **Table-driven tests** are the standard pattern.
- **Regression baselines** live in `testdata/regression/`; `roombench` tracks drift.
- **Benchmark corpus ranges** are internal deterministic regression envelopes, not measured or third-party reference data.
- **JSON struct tags** must use `snake_case` (enforced by tagliatelle linter).
- **Magic numbers are allowed** — this is acoustics/DSP code; the mnd linter is disabled.
- **Short variable names are idiomatic** — `varnamelen` is disabled for math-heavy code.
- **Error wrapping**: use `fmt.Errorf("context: %w", err)`, no sentinel errors.
- **Formatting**: `gofumpt` + `gci` for Go, `prettier` for markdown/YAML/JSON. Run `just fmt`.

## Lint Relaxations

Solver packages (`ism/`, `raytrace/`, `pde/`) are exempt from `funlen`, `cyclop`, and `gocognit` because acoustic solvers are inherently complex single functions. Test files have broad relaxations. See `.golangci.yml` for full details.

## Releasing, and Not Drifting

This module is part of the `github.com/cwbudde/algo-*` family, which is co-developed
across separate repositories. That arrangement failed once already, and the rules below
exist to stop it failing the same way twice.

**What went wrong (August 2026).** The family had drifted onto three different `algo-fft`
versions simultaneously — `algo-pde` on v0.6.15, `algo-dsp` on v0.7.3, `algo-acoustics` on
v0.6.11 — while `algo-fft`'s own `main` sat 97 commits past its latest tag and its
CHANGELOG documented a `v0.7.5` that had never been tagged. Because `algo-fft`'s generic
`PlanReal2D`/`PlanReal3D` had changed signature between the v0.6 and v0.7 lines, _no single
upgrade anywhere would compile_. Untangling it took a day and four coordinated releases.

Three separate mistakes combined to produce that. Each now has a check.

### 1. Do not let work pile up untagged

Work that only exists on `main` cannot be consumed. If you finish something a sibling repo
needs, tag it — do not wait for a milestone.

```bash
just check-unreleased     # how much is sitting past the latest tag?
```

A scheduled CI job (`.github/workflows/dep-drift.yml`) reports this weekly.

### 2. Do not sit on stale siblings

```bash
just check-deps           # are all github.com/cwbudde/* deps at their latest tags?
```

This is wired into the repo's aggregate check recipe, and the same scheduled job files a
GitHub issue when it starts failing. If a bump is _deliberately_ deferred, write down why in
`PLAN.md` — an undocumented old pin is indistinguishable from an forgotten one.

Renovate (`.github/renovate.json`) opens the bump PRs automatically and groups the whole
`cwbudde` family into a single PR on purpose: an incompatible `algo-fft` can reach a
consumer through two different dependency paths at once, so bumping them one PR at a time
produces intermediate combinations that never build.

### 3. Never remove or change exported API without the version saying so

Always release through the guard rather than by hand:

```bash
just tag-release v0.8.0       # runs every precondition, then tags and pushes
```

It refuses to tag when the tree is dirty, when `HEAD` is not a pushed default branch, when the tag
already exists or does not sort after the current one, when siblings are stale, when
`CHANGELOG.md` has no section for the version, or when the exported API changed
incompatibly without the version reflecting it.

**That last rule is stricter than semver, deliberately.** Semver exempts `v0.x` — "anything
MAY change at any time" — so `gorelease` will happily approve a _patch_ bump across a
removed symbol. Every module in this family is `v0.x`, so that exemption is exactly the hole
we fell through: `KernelEightStep` was removed and `PlanReal2D` became generic, and nothing
in the version numbers said so. The guard therefore requires a **minor** bump for any
incompatible change while on `v0.x`.

When you do break API, say so in the CHANGELOG in the form a consumer needs: the old
signature, the new signature, and the call-site rewrite. "Refactored plans" does not help
anyone; `NewPlanReal2D(rows, cols)` → `NewPlanReal2D64(rows, cols)` does.

### Order of operations for a cross-repo change

Releases must flow up the dependency graph, never sideways:

```
algo-vecmath ─┐
algo-approx  ─┼─→ algo-dsp ─┐
algo-fft ─────┴─→ algo-pde ─┴─→ algo-acoustics
```

Tag the dependency first, then bump and tag its consumers, then the consumers' consumers.
Bumping a consumer before its dependency is tagged is what forces pseudo-versions into
`go.mod`, and those are how a repo quietly ends up pinned to a commit nobody can find later.
