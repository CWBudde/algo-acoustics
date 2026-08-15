# Acoustic Scene Graph

`scene.AcousticSceneGraph` is the multi-room structure of
[`raven.md`](raven.md) section 5.1: rooms are nodes, portals are edges, and
rooms joined by **open** portals collapse into a _room group_ that is simulated
as one acoustic space.

```go
graph, err := scene.NewAcousticSceneGraph(sc)
graph.SetPortalState(2, scene.PortalOpen) // recomputes the groups
sub, err := graph.GroupScene(groupID)     // a single-room scene
```

## Room groups

`UpdateRoomGroups` walks the connected components of rooms joined by open
portals. Rooms are visited in ascending index and groups are numbered in the
order their lowest-indexed room is reached, so a given portal configuration
always yields the same numbering.

A closed portal is not a group edge in the geometric sense — it stays a solid
wall — but it _is_ an edge of the group graph. `GroupPortalViews` lists exactly
those closed portals leading out of a group; open portals never appear, having
already been merged away.

`PortalView` is a portal seen from one of its two rooms. It replaces RAVEN's
counter-portal pointer: rather than storing a second object per room, the
opposite view comes from `Counter()`, and `Normal()` always points from the
view's own room toward its neighbour regardless of how the polygon was wound.

## Merged group geometry

`GroupScene` is the payoff. It returns an ordinary single-room `scene.Scene`, so
every existing engine — the image-source solvers, the ray tracer, the hybrid
combiner — simulates a group without knowing the graph exists.

- A group that is exactly one shoebox room with no open portal keeps its
  shoebox representation, so the analytic solvers stay in play and single-room
  scenes behave exactly as before.
- Any other group becomes a mesh room carrying the merged boundary and a
  per-triangle material table.

Building that boundary means cutting each open portal's aperture out of the
coincident wall of **both** adjacent rooms. Cutting only one side would leave
the neighbour's wall intact, so the merged cavity would carry a solid partition
straight across the opening and no ray would ever cross.

Closed portals contribute no aperture. They remain solid wall carrying the wall
material; their transmission belongs to the portal filter of a propagation
path, and cutting them here as well would double-count.

### Authoring rules

- **Shoebox rooms.** A portal must be an axis-aligned rectangle in its wall
  plane and lie within that wall. Doors flush with the floor, the ceiling, or a
  side wall are fully supported — that is the common case — as are several
  portals in one wall. A portal that is planar and correctly wound but not
  rectangular is rejected with an error naming the restriction, rather than
  silently walled shut.
- **Mesh rooms.** The aperture must be _triangle-aligned_: the portal outline
  must be an edge loop of the authored mesh, so opening the portal is exactly a
  deletion of the triangles that tile it. A mesh whose triangles cross the
  outline is rejected with a message asking for retriangulation. Arbitrary mesh
  clipping is deliberately out of scope; it belongs in the modelling tool.
- Rooms in one group must not overlap.

### Volume and closedness

Two rooms sharing a partition contribute two coincident sheets with opposing
normals. That is physically right — each side of a wall carries its own
material — but it means edges on that partition are used four times, so the
merged mesh is intentionally **not** edge-manifold.

Two consequences follow. `geometry.Mesh.EnclosedVolume` rejects such a mesh, so
a group's volume is the sum of its member room volumes instead. And closedness
is checked group-locally: every undirected edge must be used an even number of
times and at least twice. `geometry.Mesh.Validate` reports the non-manifold
edges only as a warning, so `raytrace.NewMeshTracer` still accepts merged
groups.

## Caching

Derived geometry and BVHs are keyed on `Scene.RoomGroupHash` — a hash of a
group's rooms and the portals incident to them — rather than on the group
identifier, because opening a portal renumbers the groups. Hashing per group is
what lets a door toggle in one part of a building leave every unaffected group
warm.

## Positions

`GroupOfPosition` is deliberately more forgiving than `Scene.RoomIndexAt`. A
point on a shared wall lies in two rooms and `RoomIndexAt` reports it as
ambiguous, but if those rooms share a group the point is unambiguous at group
level. Ambiguity between different groups remains an error.

## Path search

`SearchPaths` enumerates the propagation paths from a source group to every
group holding a receiver, as a depth-first search across the group graph
([`raven.md`](raven.md) section 5.1). It runs over **groups**, so every edge is
a closed portal — open portals have already been merged into their group.

The search uses an explicit LIFO stack and marks the groups on the current
path; that marking is the cycle detection, so the search enumerates simple
paths only. Branches are also cut by `MaxDepth`, `MaxNodes`, and the prune
floor. Any such cut sets `PathSearchTree.Truncated`, which callers must
surface: a silently truncated search under-renders a large building with no
other symptom.

### The reduction index is an approximation, deliberately

`WeightedReductionIndexDB` estimates a single-number reduction index as
`R = -10*log10(sum(w_b * tau_b))`. This is **not** ISO 717-1, which needs
third-octave data from 100 Hz to 3150 Hz and a shifting reference curve that
octave-band scene data cannot supply. It is used only to prune the search; the
filter network always works from the per-band coefficients.

Along a chain the per-band coefficients **multiply**, which is exact for
cascaded intensity transmission ratios, and the single-number index is
recomputed from that product. Summing per-portal indices instead would be wrong
whenever the portals differ in spectral shape.

Portal area, room absorption, and propagation distance are excluded on purpose.
All three attenuate further, so pruning on transmission alone is conservative
and never discards a path that would have been audible.

### Source elimination

Each node carries `ActiveBands`, marking the bands whose accumulated
transmission is still above the floor (default -60 dB, matching the diffraction
culling convention). A band the portals have already killed need not be
simulated at all. A branch with no surviving band is pruned outright.

## Propagation path graph

`hybrid.BuildPPG` converts the search tree into the directed acyclic graph that
serves as the construction plan for the filter network, using exactly the
mapping of [`raven.md`](raven.md) section 10: the tree root becomes the source
node, tree edges (portal traversals) become portal filter nodes, tree nodes
(group residencies) become transfer-function edges, and every leaf merges into
a single receiver node.

Portal nodes are keyed on `(portal index, traversal direction)`, so two paths
reaching the same portal the same way converge on one node. That is what turns
the tree into a genuine DAG, and it is why a group's transfer function can be
rendered once per entry/exit pair rather than once per path.

The cost of sharing is that a merged node has no single per-path band mask.
`ActiveBands` is therefore the union across contributing paths and
`Transmission` their per-band maximum — both conservative, so the filter network
may simulate a band it could have skipped but never skips one it needed. Exact
per-path products are recovered by walking the graph and multiplying the portal
filters along the way.

## Related

- [Sound transmission](sound-transmission.md) — the Phase 21 one-hop model.
- [Scene format](scene-format.md) — `rooms`, `portals`, and `triangleMaterials`.
- [RAVEN notes](raven.md) — sections 5 and 10.
