# Web Demo Limits and Progressive Rendering

What the browser demo accepts, what it does while a render is running, and what
it does when a render will not finish in time. Implemented in
`web/wasm/limits.go` and `web/wasm/tiers.go`; the memory side is documented
separately in [WASM memory budget](wasm-memory-budget.md).

## Why a demo needs limits at all

A render in the browser is constrained by three different resources, and they
bind at different points:

| Resource   | Failure mode                            | Handled by                                    |
| ---------- | --------------------------------------- | --------------------------------------------- |
| Memory     | The tab dies and takes the page with it | `applyDemoMemoryBudget` reduces quality knobs |
| Wall-clock | The worker blocks; the page looks hung  | Progressive tiers plus the render deadline    |
| Structure  | Geometry too large to solve at all      | `validateDemoStructure` rejects the request   |

Only the third rejects anything. Memory pressure is answered by rendering
something cheaper, and time pressure by returning the best tier that finished —
because a coarse impulse response with an explanation is worth more to someone
trying a demo than an error is.

## The envelope

`web/wasm/limits.go` is the single definition. It is enforced in
`normalizeDemoRequest`, published to the page on the worker's ready message, and
used by `web/demo-limits.mjs` to size the render sliders — so the controls cannot
offer a setting the engine would silently clamp.

| Limit                    | Value        | Note                                      |
| ------------------------ | ------------ | ----------------------------------------- |
| Sample rate              | 48 kHz       | Fixed                                     |
| Impulse response length  | 0.25 – 3 s   | 3 s is 144,000 samples per channel        |
| Rays                     | 128 – 16,384 | See below                                 |
| Reflection order         | 1 – 12       | Order 12 is 2,925 image-source candidates |
| Distinct room surfaces   | 50           | Counted as planes, not triangles          |
| Mesh triangles           | 20,000       | BVH and heatmap probe cost                |
| Material library entries | 128          | Bounds a long editing session             |
| Room dimensions          | 2 – 50 m     | The page's sliders are tighter            |
| Render budget            | 10 s         | See "The render deadline"                 |

### Surfaces are counted as planes

`countDemoSurfaces` groups a mesh room's triangles onto the plane each lies in,
matching normals and offsets to a tolerance and treating a plane and its back
face as one surface.

Counting triangles instead would be the wrong measure twice over. Subdividing a
wall multiplies triangles without adding a single reflecting surface, so a
tessellated box would be rejected while an acoustically identical six-triangle
box passed. And it is the plane count, not the triangle count, that drives
image-source growth — the cost the limit exists to bound. A subdivided flat wall
of 3,200 triangles counts as one surface; a staircase of 51 differently-angled
triangles counts as 51 and is refused.

Triangles are still capped separately at 20,000, because they drive BVH size and
heatmap probe cost regardless of how many planes they form.

### Rays: 16,384, not the 50,000 originally planned

The original roadmap proposed a 50,000-ray ceiling. Memory allows it easily —
50,000 rays cost
about 25 MiB against a 512 MiB budget — but wall-clock does not.

Measured under `go_js_wasm_exec` on the default demo room, ray cost is close to
linear:

| Request                               | Elapsed |
| ------------------------------------- | ------- |
| 3,072 rays, order 4, 1.35 s (default) | 1.5 s   |
| 16,384 rays, order 4, 1.35 s          | 7.0 s   |
| 16,384 rays, order 12, 3 s            | 18.0 s  |

Extrapolating, 50,000 rays would take roughly 21 s at a 1.35 s response and
close to a minute at 3 s. That is not a demo, and the 10 s timeout cannot rescue
it into a good result either — it would simply mean every request at the top of
the slider came back as a preview. The cap is therefore set from the measured
time envelope rather than the memory one.

Note that even 16,384 rays at 3 s exceeds the 10 s budget. That is deliberate:
the budget is sized for what a render should normally take, and the fallback
below covers the tail rather than the limits being cut until the fallback never
fires.

## Progressive rendering

A WASM render is synchronous. It holds its worker from the first call to the
last, so without intermediate reporting the page shows nothing at all until the
whole thing finishes. The demo therefore runs Phase 18's progressive idea:
report what is already known, then refine.

| Tier            | Cost                         | What the page does with it                        |
| --------------- | ---------------------------- | ------------------------------------------------- |
| **Statistical** | No simulation, ~1 ms         | Fills the "Est. RT60" chip                        |
| **Preview**     | ≤ 384 rays, order ≤ 2, ≤ 1 s | Draws a real waveform, notes it in the render log |
| **Full render** | The request as asked for     | Everything: audio, download, heatmap, metrics     |

Measured in Chrome for a 4,096-ray, order-4, 1.5 s hybrid render: the
statistical tier lands at about 200 ms, the preview at about 480 ms, and the
full result at about 4.4 s.

The statistical tier reuses the library's own Tier 1 computation through
`algoacoustics.ComputeStatisticalMetrics`, so the demo and
`RenderProgressive` cannot drift apart on what "statistical" means. It reports
nothing for a mesh room, whose volume the Sabine and Eyring estimators cannot
derive; the chip then simply stays empty rather than showing a fabricated number.

Tiers travel as their own worker message and never touch the page's `lastRender`,
the WAV download, or the auralization — those stay bound to the render that
finishes. A tier that fails is not fatal, because it is an optimisation of what
the user sees while the accurate render is still ahead.

The demo does not call `RenderProgressive` itself. That function covers a single
mono hybrid render, whereas the demo must also serve early-only, late-only, and
connected-room requests — and its final tier has to stay identical to what the
non-progressive path produced, which a differently-configured ray tracer would
not be.

### Why the preview tier is not itself deadline-checked

Its cost is bounded by construction, and it is the very result the timeout falls
back to. Aborting it because the budget had already expired would leave the
fallback with nothing to return. A user cancel still stops it, because
cancellation and the deadline are separate conditions.

## The render deadline

The budget is 10 s of wall-clock per render. It is checked against `time.Now`
rather than driven by a timer: js/wasm is single-threaded, so a Go timer
goroutine cannot preempt a running solver — the timer would not fire until the
computation it was meant to bound had already returned.

That same single-threadedness is why the deadline alone is not enough. Checks sit
at the stage boundaries, never inside a solver, and a late-field trace is one
uninterruptible call. Measured, the worst case inside the envelope overran a 10 s
budget by a further 8 s before any checkpoint noticed — 18.2 s to return a
result the user was promised in 10.

So the demo predicts as well as checks. The preview tier is timed, and
`projectFullRenderCost` scales that measurement by the ratio of ray counts and
response durations, the two knobs that dominate. If the projection cannot fit
what is left of the budget — with a 1.5× tolerance, because the model is crude
and being wrong high costs the user detail they asked for — the full render is
never started.

The prediction is measured on the user's own device, in their browser, on their
room, which is a far better basis than any constant fitted elsewhere.

Measured effect on that same worst case: the partial result now arrives in 1.4 s
instead of 19.4 s.

### What a truncated render looks like

The result is a complete, playable, downloadable impulse response — the preview
tier finished into a full result, heatmap and WAV included — carrying a warning:

```
render timeout: the full render projects to 48 s against 9.6 s left of the 10 s
demo budget; returning the preview result
```

or, when the budget ran out rather than being projected away:

```
render timeout: exceeded the 10 s demo budget after 18.2 s; returning the
preview result
```

The page shows the warning in its render log and sets the badge to **Partial
render** rather than "Render complete", so a coarser response is never presented
as the finished article.

Two cases have no preview to fall back on:

- A request already at or below the preview settings has nothing cheaper to
  return, so an expired budget is reported as an error. Such a request is also
  the cheapest the demo accepts, so in practice it does not time out.
- A connected-room render has no cheap preview — both endpoints are full binaural
  renders. If the budget expires after the closed-portal endpoint, that endpoint
  is returned for both, which leaves the aperture crossfade interpolating between
  identical responses. The warning says so: the aperture control has no effect on
  that result.

## Testing

- `web/wasm/limits_test.go` — surface counting, structural rejection, the quality
  envelope, and the limits object the page consumes.
- `web/wasm/tiers_test.go` — deadline arithmetic, preview derivation, cost
  projection, and the end-to-end fallback (an expired deadline must return the
  preview, not an error).
- `web/render-tiers.test.mjs` and `web/demo-limits.test.mjs` — the page's reading
  of tier messages and its slider sizing.

Run them with `just test-wasm` and `node --test web/*.test.mjs`.
