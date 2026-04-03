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
Scene → validate → EventEngine.Generate() → []ir.Event (sparse)
                                                ↓
                              hybrid.Combine(early, late) → merged events
                                                ↓
                              ir.RenderMono/RenderBinaural → ir.Buffer (dense samples)
                                                ↓
                              LowFreqEngine.Transfer() → pde.TransferFunction
                                                ↓
                              hybrid.BlendLowFreq() → final IR
                                                ↓
                              export.WriteMonoWAV/WriteStereoWAV → output
```

Orchestrated by `Renderer` in `renderer.go` at the module root.

### Key Interfaces

- **`EventEngine`** (`renderer.go`) — generates sparse acoustic events from a scene. Implemented by `ism.ISMSolver` (early specular reflections) and `raytrace.RayTracer` (late diffuse energy).
- **`LowFreqEngine`** (`renderer.go`) — produces a frequency-domain transfer function for modal blending. Implemented by `pde.ShoeboxSolver`.
- **`directivity.Model`** — source directivity (omni, cardioid, GLL balloon). Single method: `GainLinear(freqHz, direction)`.
- **`hrtf.Dataset`** — binaural HRTF lookup. `Lookup(direction) → (left, right, delay)`.

### Package Roles

| Package | Role |
|---------|------|
| `acoustics` | Physical constants, octave band specs (`Octave6`, `Octave8`) |
| `geometry` | Vec3, Ray, Plane, Triangle, BVH, intersection tests, quaternions |
| `scene` | Room definition (shoebox/mesh), materials, sources, receivers, validation |
| `directivity` | Source directivity models behind `Model` interface |
| `hrtf` | HRTF dataset interface, SOFA adapter |
| `ir` | Sparse `Event` → dense `Buffer` rendering, band gain aggregation, normalization |
| `ism` | Image-source method solver (early specular reflections) |
| `raytrace` | Monte Carlo ray tracer (late-field diffuse energy histograms) |
| `pde` | Helmholtz solver for low-frequency modal content |
| `hybrid` | Crossover blending of early/late events and geometric/modal IRs |
| `metrics` | IR comparison, acoustic metrics |
| `export` | WAV/JSON/CSV output |
| `cmd/roomir` | Main CLI (validate, render, render-stereo, dump-events) |
| `cmd/roombench` | Regression benchmark runner |
| `web/wasm` | WASM entry point exposing `renderScene()` to JavaScript |

### Data Flow Types

- `ir.Event` — sparse: time, amplitude, direction, distance, per-band gains, phase, kind (Direct/Specular/Diffuse/PDE)
- `ir.Buffer` — dense: `[]float64` samples at a sample rate
- `scene.Scene` — root container: room geometry, materials, sources, receivers, band spec, sample rate
- `pde.TransferFunction` — complex H(f), convertible to time-domain via IFFT

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
- **JSON struct tags** must use `snake_case` (enforced by tagliatelle linter).
- **Magic numbers are allowed** — this is acoustics/DSP code; the mnd linter is disabled.
- **Short variable names are idiomatic** — `varnamelen` is disabled for math-heavy code.
- **Error wrapping**: use `fmt.Errorf("context: %w", err)`, no sentinel errors.
- **Formatting**: `gofumpt` + `gci` for Go, `prettier` for markdown/YAML/JSON. Run `just fmt`.

## Lint Relaxations

Solver packages (`ism/`, `raytrace/`, `pde/`) are exempt from `funlen`, `cyclop`, and `gocognit` because acoustic solvers are inherently complex single functions. Test files have broad relaxations. See `.golangci.yml` for full details.
