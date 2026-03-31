# algo-acoustics

[![CI](https://github.com/cwbudde/algo-acoustics/actions/workflows/ci.yml/badge.svg)](https://github.com/cwbudde/algo-acoustics/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cwbudde/algo-acoustics)](https://goreportcard.com/report/github.com/cwbudde/algo-acoustics)
[![Go Reference](https://pkg.go.dev/badge/github.com/cwbudde/algo-acoustics.svg)](https://pkg.go.dev/github.com/cwbudde/algo-acoustics)

A Go-native room acoustics toolkit for offline simulation of sound propagation,
early reflections, diffuse late reverberation, and optional binaural rendering —
built around a scene → propagation engines → event stream → IR/export pipeline.

## Status

Work in progress. See [PLAN.md](PLAN.md) for the phased implementation roadmap.

## Docs

- [Architecture](docs/architecture.md)
- [Scene format](docs/scene-format.md)
- [Hybrid rendering](docs/hybrid-rendering.md)
- [Directivity and GLL](docs/directivity-gll.md)
- [HRTF and SOFA](docs/hrtf-sofa.md)
- [Validation](docs/validation.md)

## Dependencies

| Package                                             | Role                                            |
| --------------------------------------------------- | ----------------------------------------------- |
| [`algo-dsp`](https://github.com/cwbudde/algo-dsp)   | Convolution, FFT, filtering, acoustic metrics   |
| [`algo-pde`](https://github.com/CWBudde/algo-pde)   | Low-frequency Helmholtz solves on regular grids |
| [`gll-tools`](https://github.com/CWBudde/gll-tools) | Loudspeaker directivity balloon ingestion       |
| [`wav`](https://github.com/cwbudde/wav)             | WAV file encoding/decoding for export           |

## License

MIT
