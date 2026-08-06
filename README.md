# algo-acoustics

[![CI](https://github.com/cwbudde/algo-acoustics/actions/workflows/tests.yml/badge.svg)](https://github.com/cwbudde/algo-acoustics/actions/workflows/tests.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cwbudde/algo-acoustics)](https://goreportcard.com/report/github.com/cwbudde/algo-acoustics)
[![Go Reference](https://pkg.go.dev/badge/github.com/cwbudde/algo-acoustics.svg)](https://pkg.go.dev/github.com/cwbudde/algo-acoustics)

A Go-native room acoustics toolkit for offline simulation of sound propagation,
early reflections, diffuse late reverberation, low-frequency modal blending, and
optional binaural rendering. The shipped `NewISMEngine` produces sparse early
events; `NewRaytraceEngine` preserves banded late-field energy as a dense mono
or directional binaural buffer.

## Status

Work in progress. The core mono, binaural, hybrid, progressive, and validation
paths are implemented; [PLAN.md](PLAN.md) records completed and future phases.

## Docs

- [Architecture](docs/architecture.md)
- [Scene format](docs/scene-format.md)
- [Hybrid rendering](docs/hybrid-rendering.md)
- [Directivity and GLL](docs/directivity-gll.md)
- [HRTF and SOFA](docs/hrtf-sofa.md)
- [Validation](docs/validation.md)

## Web Demo

A small browser demo lives in [web/README.md](web/README.md). It uses Go/WASM for the actual render path and a static HTML/CSS/JS shell for the UI, similar to the lightweight browser stacks used in the sibling `algo-dsp` and `gll-tools` repositories.

## Dependencies

| Package                                             | Role                                            |
| --------------------------------------------------- | ----------------------------------------------- |
| [`algo-dsp`](https://github.com/cwbudde/algo-dsp)   | Convolution, FFT, filtering, acoustic metrics   |
| [`algo-pde`](https://github.com/CWBudde/algo-pde)   | Low-frequency Helmholtz solves on regular grids |
| [`gll-tools`](https://github.com/CWBudde/gll-tools) | Loudspeaker directivity balloon ingestion       |
| [`wav`](https://github.com/cwbudde/wav)             | WAV file encoding/decoding for export           |

## License

MIT
