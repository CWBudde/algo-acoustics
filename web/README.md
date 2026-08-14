# Web Demo

This folder contains a small browser demo for `algo-acoustics`.

- Go/WASM performs the actual early, late, or hybrid impulse-response render.
- Plain HTML, modern CSS, and ES modules handle the UI and 3D preview.
- The demo focuses on a shoebox room with editable wall materials, source and receiver placement, and render controls.
- Each render also samples a broadband first-order ISM field at the room boundary. The optional SPL heatmap displays those surface samples on a normalized `-30…0 dB rel.` scale; it is relative because demo source gains are not calibrated sound-power levels.

Rendering runs in a dedicated worker. Canceling a synchronous WASM render
terminates that worker, starts a fresh worker, waits for its ready message, and
ignores results from the retired generation. Cancellation therefore stops the
computation rather than merely hiding a late result.

## Memory budget

The demo targets a peak of 512 MiB of WASM linear memory. A Go/WASM heap only
grows, so the highest point any render reaches is the footprint the tab keeps
until its worker is replaced. Requests that would exceed the budget have their
quality settings reduced automatically, and each reduction is reported on
`result.warnings`, in the browser console, and in the on-page render log.
Connected-room (portal) renders carry a tighter envelope than mono ones because
they trace two full binaural responses.

Every result also carries a `memory` object (`peakSysBytes`, `budgetBytes`, and
friends), logged to the console and exposed on
`window.algoAcousticsDemoLastRender`.

See [docs/wasm-memory-budget.md](../docs/wasm-memory-budget.md) for the measured
envelope and the reasoning behind the limits.

## Local run

```bash
./web/build-wasm.sh
python3 -m http.server 8080 -d web
```

Open <http://localhost:8080>.

Or run:

```bash
just web-demo
```
