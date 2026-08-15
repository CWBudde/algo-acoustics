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

## Related

- [Sound transmission](sound-transmission.md) — the Phase 21 one-hop model.
- [Scene format](scene-format.md) — `rooms`, `portals`, and `triangleMaterials`.
- [RAVEN notes](raven.md) — sections 5 and 10.
