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
paths are implemented, along with mesh geometry, edge diffraction, multi-room
transmission, GPU acceleration, and the browser demo. See the
[documentation index](docs/README.md) for what each subsystem does and where its
limits are; open work is tracked in
[issues](https://github.com/cwbudde/algo-acoustics/issues).

## Docs

- [Documentation index](docs/README.md)
- [Scene authoring](docs/scene-authoring.md)
- [HRTF usage](docs/hrtf-sofa.md)
- [Directivity usage](docs/directivity-gll.md)
- [Hybrid rendering](docs/hybrid-rendering.md)
- [Sound transmission between rooms](docs/sound-transmission.md)
- [Regression workflow](docs/regression-workflow.md)
- [Comparing against another tool](docs/compare-another-tool.md)

Runnable Go and JSON examples are indexed in [examples/README.md](examples/README.md).

## Web Demo

A browser demo lives in [`web/`](web/README.md). It runs the real early, late,
and hybrid render paths through Go/WASM in a dedicated worker, with a static
HTML/CSS/ES-module shell for the UI — similar to the lightweight browser stacks
used in the sibling `algo-dsp` and `gll-tools` repositories.

It covers a shoebox room with editable wall materials, source and receiver
placement, a 3D preview, an optional SPL heatmap on the room boundary, and
auralization of sample audio with the rendered impulse response.

Build and serve it locally:

```bash
just web-demo      # builds the WASM bundle, then serves web/ on :8080
```

Then open <http://localhost:8080>. See [web/README.md](web/README.md) for
details.

## Dependencies

| Package                                             | Role                                            |
| --------------------------------------------------- | ----------------------------------------------- |
| [`algo-dsp`](https://github.com/cwbudde/algo-dsp)   | Convolution, FFT, filtering, acoustic metrics   |
| [`algo-fft`](https://github.com/cwbudde/algo-fft)   | FFT operations used by DSP and modal paths      |
| [`algo-pde`](https://github.com/CWBudde/algo-pde)   | Low-frequency Helmholtz solves on regular grids |
| [`gll-tools`](https://github.com/CWBudde/gll-tools) | Loudspeaker directivity balloon ingestion       |
| [`wav`](https://github.com/cwbudde/wav)             | WAV file encoding/decoding for export           |

## License

MIT
