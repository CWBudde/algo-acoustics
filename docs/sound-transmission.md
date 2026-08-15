# Sound Transmission Between Rooms

Phase 21 models transmission between two directly adjacent shoebox rooms. A
portal is a shared wall polygon: the source-room field is measured at that
surface, filtered by the portal material, and emitted from the receiving-room
side as a secondary source. ISM uses pressure transmission `sqrt(tau)` while
ray tracing uses energy transmission `tau`.

Start from the runnable
[`two_room_transmission.json`](../examples/scenes/two_room_transmission.json)
fixture:

```bash
go run ./cmd/roomir validate examples/scenes/two_room_transmission.json
go run ./cmd/roomir render examples/scenes/two_room_transmission.json \
  --output /tmp/two_room.wav --mode hybrid \
  --duration 1.5 --max-order 3 --num-rays 4096
```

## Authoring

Use top-level `rooms` instead of legacy `room`. Shoebox `origin` values place
rooms in world coordinates. `roomIndices: [a, b]` and polygon winding establish
the portal direction from room `a` toward room `b`. Every polygon vertex must
lie on a boundary wall shared by both rooms.

A closed portal uses its material's `transmissionByBand`, or derives it from
`soundReductionIndex` with:

```text
tau = 10^(-R/10)
```

One value broadcasts across all bands. When both forms are present they must
agree. Validation also enforces `0 <= tau <= 1` and `absorption + tau <= 1` in
every band. An open portal is exactly transmissive (`tau = 1`) without mutating
the referenced closed-state material.

Source and receiver room membership is inferred from world-space positions.
Each point must belong to exactly one room, so avoid shared boundary planes.

## Supported Rendering Scope

The built-in transmission renderer supports:

- one source and one receiver in directly adjacent rooms;
- one or more parallel portals connecting that same room pair;
- early ISM, late ray tracing, hybrid mono, and hybrid binaural output;
- closed materials and the fully transmissive open state.

This renderer stays the fast path for exactly that shape, so its output is
unchanged. Anything beyond it — portal chains above all — is handled by the
multi-room filter network below, which `NewCrossRoomEngine` selects
automatically.

## Metrics and Interactive Aperture

`metrics.ApparentSoundReductionIndex` evaluates
`Ls - Lr + 10*log10(S/Ar)`. The flanking-aware helper combines parallel path
energy coefficients as `-10*log10(sum(tau_ij))`.

## Multi-Room Filter Network

`NetworkRenderer` renders propagation across any number of rooms as the filter
network of `docs/raven.md` section 5.2: a path is the product

```text
H_PP = H_PS * prod(H_Portal) * prod(H_RoomGroup) * H_R
```

of separately simulated factors, one per room group the path passes through.

**Why not extend the one-hop model.** Phase 21 composes a hop by re-emitting
every incident event and running a fresh image-source solve per emission. That
is one solve per event per hop, so an N-hop chain costs O(events^N) — an
order-3 shoebox yields 60 to 120 events, putting hop three around a million
solves. The network instead runs **one simulation per hop** and composes by
per-band convolution. Convolving two impulse trains is exactly their cartesian
product, so the two formulations agree to floating-point precision; the
regression test pins that against the Phase 21 renderer at image-source orders
0, 1, and 2.

### Path types

`PLAN.md` names four path types that `raven.md` never expands. **The expansion
below is ours, not the reference's.** It follows the structure of `H_PP`
directly: only the source and receiver factors are marked complex and binaural
there, so a hop ending at a portal is scalar per band while a hop ending at the
receiver carries direction through the HRTF.

| Type   | Meaning                      | Renders                                             |
| ------ | ---------------------------- | --------------------------------------------------- |
| `PS2P` | primary source to portal     | the first hop, out of the source's own group        |
| `SS2P` | secondary source to portal   | an intermediate hop, portal to portal               |
| `SS2R` | secondary source to receiver | the terminal hop, binaural                          |
| `PS2R` | primary source to receiver   | the zero-hop case, source and receiver in one group |

As in Phase 21 the portal filter is `sqrt(tau)` in the pressure domain and
`tau` in the energy domain.

### Alignment

Path contributions are summed first, and the early-to-late alignment runs
**once on the summed fields**. Aligning each path separately would be wrong:
per-path early-to-late ratios are physically meaningful, and a long flanking
path legitimately arrives with a different direct-to-reverberant ratio than the
direct path. Aligning them individually would flatten exactly the information
that makes flanking audible.

Cross-path time alignment needs nothing extra. Every factor is causal from its
own emission instant and the portal handoff adds no delay, so convolution sums
the delays automatically.

### Limits

- One source and one receiver. The ray tracer detects one receiver per trace,
  so several receivers would need one full render each.
- Intermediate hops compose in the energy domain, which carries no direction,
  so an intermediate room group acts as a scalar-per-band filter. Only the
  terminal group keeps true directionality.
- A portal is treated as a point source at its centre, which is an
  approximation for large apertures.
- `MaxPaths` caps how many paths are rendered, strongest first, because
  convolution assembly dominates the cost.

Low-frequency PDE blending now works for multi-room mono renders. The modal
response is computed for the receiver's own room group and excited at the
portal that admits sound to it, since the solver needs a shoebox containing a
source. That captures the receiving room's modes and where they are driven from,
not the coupled modal behaviour of two volumes sharing an aperture; it is
refused rather than approximated when the receiver's group is not a single
shoebox. Stereo still omits it, because one monaural transfer function cannot
preserve ear-specific HRTF information.

## Interactive Aperture

The browser demo caches its two binaural endpoint responses and interpolates
them with `x^(1/n)` (square root by default), so dragging the aperture control
triggers no new simulation.

The open endpoint is now a **physically merged room group**, not the `tau = 1`
all-pass stand-in it used to be. Nothing in the demo changed to achieve that:
`NewCrossRoomEngine` routes any open portal to the filter network, because the
Phase 21 renderer models "open" as a fully transmissive partition with the two
rooms still geometrically separate, while the scene graph cuts the aperture out
of both walls and merges the volumes into one cavity.

`hybrid.PortalBRIRCache` can hold all three states — closed, the all-pass
portal filter, and the merged room group — through
`NewPortalBRIRCacheWithFilter`. `AtApertureMerged` follows the sequencing of
`raven.md` section 5.3 — crossfade from closed toward the all-pass filter, and
only at that endpoint does the merged room group take over — but the last step
is a crossfade rather than the hard switch the reference describes. Two
independently simulated responses have different reflection times, so replacing
one with the other in a single buffer is a discontinuity no matter how well
their broadband levels match. The merged response therefore fades in over the
last 5 % of aperture, and the constructor additionally rejects an all-pass and
merged pair differing by more than 1.5 dB, since a fade that short cannot
disguise a large level step.
