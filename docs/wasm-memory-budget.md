# WASM Memory Budget

The browser demo targets a peak of **512 MiB** of WebAssembly linear memory, so it runs on
mid-range phones and tablets rather than only on development machines. This document records
why that number is hard to hit, what enforces it, and the measurements the limits are drawn
from.

## Why the peak is what matters

A Go/WASM heap only ever grows. The browser never hands pages back, so the highest point any
render reaches stays resident for the life of the worker. A single expensive render early in a
session sets the tab's footprint for the rest of it. The only thing that reclaims the memory is
destroying the worker, which `RenderWorkerController.replace()` (`web/render-worker-controller.mjs`)
does when a render is cancelled.

The live set is never the problem. After a render the demo holds a few megabytes. What drives
the peak is **allocation churn**: a mono render at maximum settings allocates roughly 800 MiB in
total, and a connected-room render roughly 1.4 GiB, while keeping only single-digit megabytes
alive. Peak memory is a measure of how far the heap ran ahead of the collector.

Because `js/wasm` is single-threaded, the collector cannot preempt a long allocating call. That
makes the peak a _poor_ function of the request: neighbouring configurations were measured four
times apart, and raising the ray count sometimes lowered the peak because the extra allocation
triggered more collection cycles. Any scheme that tries to predict the peak in bytes from the
request parameters will be wrong. The budget is defended by measurement instead.

## What enforces the budget

Three mechanisms, all in `web/wasm/memory.go`:

1. **A GC soft limit.** `configureDemoMemory` sets `debug.SetMemoryLimit` to 64 MiB at startup.
   The value is far below the budget on purpose — it is a target for the collector, not a cap on
   the process. On the worst connected-room render it cut peak linear memory from ~1.8 GiB to
   ~160 MiB at no cost in wall-clock time.

   `debug.SetGCPercent` was tried alongside it and made things consistently _worse_ (portal at
   order 4 went from 207 MiB to 750 MiB). It is deliberately not used.

2. **Collection points between stages.** `releaseDemoMemory` runs a collection at render stage
   boundaries — between the two portal responses, before the heatmap, and after the result is
   built. Without them each stage inherits the heap goal the previous one reached.

3. **A bounded request envelope.** `applyDemoMemoryBudget` reduces quality knobs until the
   request fits, appending a warning for each reduction. Every warning is returned to the page
   on `result.warnings`.

Structural inputs are capped with errors rather than downgraded, because shrinking geometry
changes which room is simulated rather than how well: `maxDemoMeshTriangles` (20,000),
`minDemoRoomMeters`/`maxDemoRoomMeters` (2–50 m), and `maxDemoMaterials` (128).

## The envelope

| Setting          | Mono renders | Connected-room (portal) renders |
| ---------------- | ------------ | ------------------------------- |
| Reflection order | 1–12         | 1–5                             |
| Rays             | 128–16,384   | 128–4,096                       |
| Duration         | 0.25–3.0 s   | 0.25–1.2 s                      |

Mono renders need no restriction: the full slider range measures 61–135 MiB. Only portal
renders, which trace two complete binaural responses, need a tighter envelope.

## Measurements

Peak `runtime.MemStats.Sys` under `go_js_wasm_exec`, one render per process unless noted.

### The enforced envelope

Rendered in sequence in a single process, which is what a browsing session looks like — the
figures are cumulative, since linear memory never shrinks:

| Configuration                                               | Peak    |
| ----------------------------------------------------------- | ------- |
| Default preset                                              | 61 MiB  |
| Mono hybrid, order 12, 16,384 rays, 3.0 s                   | 119 MiB |
| Mono late, order 12, 16,384 rays, 3.0 s                     | 119 MiB |
| Mono early, order 12, 16,384 rays, 3.0 s                    | 127 MiB |
| 50 m room, order 12, 16,384 rays, 3.0 s                     | 127 MiB |
| Portal, requested at maximum, clamped to the envelope       | 134 MiB |
| Mesh room at 20,000 triangles, order 12, 16,384 rays, 3.0 s | 135 MiB |

`TestDemoRenderStaysUnderMemoryBudget` in `web/wasm/memory_test.go` asserts this.

### Why the portal envelope stops where it does

Portal renders, 3 runs per cell, with all mitigations active:

| Order | 2,048 rays @ 1.2 s  | 4,096 rays @ 1.2 s  |
| ----- | ------------------- | ------------------- |
| 1     | 68 / 101 / 61 MiB   | 59 / 74 / 62 MiB    |
| 3     | 98 / 98 / 99 MiB    | 72 / 73 / 73 MiB    |
| 5     | 138 / 191 / 138 MiB | 258 / 137 / 260 MiB |

One step beyond the envelope the peak jumps and turns erratic:

| Configuration                                | Peak                   |
| -------------------------------------------- | ---------------------- |
| Order 1–5, 8,192 rays, 1.5 s                 | 412–475 MiB            |
| Order 3, 8,192 rays, 2.0 s                   | 572 MiB                |
| Order 3, 8,192 rays, 3.0 s                   | 938 MiB                |
| Order 8, 4,096 rays, 3.0 s (no mitigations)  | 2,936 MiB              |
| Order 12, 4,096 rays, 3.0 s (no mitigations) | out of memory at 4 GiB |

### Why mesh rooms are not restricted

Subdividing a surface multiplies triangles without adding reflecting planes. A 20,000-triangle
room at maximum order yields a single audible image source, and its cost is geometry rather than
image sources:

| Triangles | Peak (order 12, 16,384 rays, 3.0 s) |
| --------- | ----------------------------------- |
| 12        | 110 MiB                             |
| 500       | 111 MiB                             |
| 5,000     | 111 MiB                             |
| 20,000    | 207 MiB                             |

The triangle cap exists to bound BVH construction and heatmap probing, not the image-source
solve. Mesh heatmap probes are separately capped at `maxMeshHeatmapProbes` (256), since each
probe costs a full image-source solve.

## Retention

Peak memory also depends on what survives a render. `demoAPIState.storeResult`
(`web/wasm/api.go`) keeps only the impulse response and the scalars `getParameters` reads; the
sample array, encoded WAV, heatmap, and portal responses are dropped. Holding them would keep
every render's output resident until the next one replaced it.
`TestRepeatedRendersDoNotRatchetMemory` covers this — six consecutive renders leave the live
heap flat at 3.1 MiB.

## Reporting

Every render result carries a `memory` object — `heapBytes`, `sysBytes`, `peakSysBytes`,
`budgetBytes`, `estimateBytes` — and a `warnings` array. `web/app.js` logs both to the console
after each render and exposes them on `window.algoAcousticsDemoLastRender`; downgrade warnings
are also appended to the on-page render log, because the sliders still show what was asked for
rather than what was rendered.

## A note on the underlying cost

The envelope bounds the symptom. The cause is that a binaural late-field render churns roughly
700 MiB per pass — mostly in `ir.RenderBinauralPoisson`, which convolves per histogram bin
across 72 directivity groups. Reducing that churn (buffer reuse in the synthesis loop) would let
the portal envelope widen back toward the mono one. That is a change to the `ir` package rather
than to the demo, and is not part of this work.
