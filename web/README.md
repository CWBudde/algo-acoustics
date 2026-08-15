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

## Limits and progressive rendering

A render is synchronous inside its worker, so the demo reports as it goes rather
than leaving the page blank. A statistical estimate of the room's decay time
appears within milliseconds, a coarse preview waveform within a few hundred, and
the full result replaces both when it finishes.

Each render has a 10 s budget. A render that will not fit is not started; the
preview is returned instead as a complete, playable result carrying a warning,
and the badge reads "Partial render" rather than "Render complete".

The request envelope — 16,384 rays, reflection order 12, 3 s at 48 kHz, 50
distinct room surfaces — is defined once in `web/wasm/limits.go`, enforced there,
and published to the page so the sliders are sized from the values that are
actually enforced.

See [docs/web-demo-limits.md](../docs/web-demo-limits.md) for the measurements
behind each limit and why the ray cap is 16,384 rather than the 50,000 originally
planned.

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
just web-demo
```

Open <http://localhost:8080>.

That builds the module and serves `web/` through `web/devserver`, a small static
server that types `.wasm` as `application/wasm` and applies the demo's cache
policy. The MIME type is not cosmetic: the worker loads the module with
`WebAssembly.instantiateStreaming`, which rejects any other type, so a generic
static server (`python3 -m http.server` on a host without a wasm entry in
`/etc/mime.types`) fails to start the demo.

The equivalent by hand:

```bash
./web/build-wasm.sh
go run ./web/devserver -dir web -addr :8080
```

Pass `-coi` to add COOP/COEP headers. The demo does not need them — it renders in
a plain worker and never uses `SharedArrayBuffer` — but the flag makes the policy
testable.

## Deployment

`web/_headers` covers Netlify and Cloudflare Pages. GitHub Pages, where the demo
is deployed, sends the right headers on its own and accepts no overrides. See
[docs/web-deployment.md](../docs/web-deployment.md) for the full policy, host
configuration snippets, and how to verify a deployment.
