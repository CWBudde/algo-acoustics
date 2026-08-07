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
