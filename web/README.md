# Web Demo

This folder contains a small browser demo for `algo-acoustics`.

- Go/WASM performs the actual early, late, or hybrid impulse-response render.
- Plain HTML, modern CSS, and ES modules handle the UI and 3D preview.
- The demo focuses on a shoebox room with editable wall materials, source and receiver placement, and render controls.

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