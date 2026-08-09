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

Phase 21 deliberately rejects portal chains, sources or receivers with
ambiguous room membership, and cross-room propagation involving mesh rooms.
Mesh rooms and portals can still be serialized for future scene-graph work.
Low-frequency PDE blending is also unavailable for multi-room renders. Portal
chains and true merged open-room simulation belong to Phase 25.

## Metrics and Interactive Aperture

`metrics.ApparentSoundReductionIndex` evaluates
`Ls - Lr + 10*log10(S/Ar)`. The flanking-aware helper combines parallel path
energy coefficients as `-10*log10(sum(tau_ij))`.

The browser demo caches closed and fully transmissive binaural endpoint
responses. Its aperture control interpolates them with `x^(1/n)` (square root
by default), so dragging the control does not trigger a new simulation. The
fully open Phase 21 endpoint is the all-pass portal response; physical merging
of the room geometry remains Phase 25 work.
