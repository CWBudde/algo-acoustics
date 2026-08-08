import test from "node:test";
import assert from "node:assert/strict";

import { makeSlopedRoofMesh, makeTriangularPrismMesh } from "./app-presets.js";
import { computeMeshReflectionPaths } from "./mesh-reflections.mjs";

const WIDTH = 6;
const DEPTH = 4;

const MESHES = {
  "triangular prism": {
    mesh: makeTriangularPrismMesh(WIDTH, DEPTH, 3),
    // Well inside the prism, whose footprint narrows toward y = DEPTH.
    interior: { x: WIDTH / 2, y: DEPTH / 4, z: 1.4 },
  },
  "sloped roof": {
    mesh: makeSlopedRoofMesh(WIDTH, DEPTH, 2.4, 3.6),
    interior: { x: WIDTH / 2, y: DEPTH / 2, z: 1.1 },
  },
};

function sub(a, b) {
  return { x: a.x - b.x, y: a.y - b.y, z: a.z - b.z };
}

function cross(a, b) {
  return {
    x: a.y * b.z - a.z * b.y,
    y: a.z * b.x - a.x * b.z,
    z: a.x * b.y - a.y * b.x,
  };
}

function dot(a, b) {
  return a.x * b.x + a.y * b.y + a.z * b.z;
}

function triangleNormal(t) {
  return cross(sub(t.v1, t.v0), sub(t.v2, t.v0));
}

// The Go mesh image-source solver only mirrors the source across planes it lies
// in front of (ism/mesh_image.go), so an outward-wound face silently produces
// no specular reflections. Both presets are convex, so "faces the interior
// point" is an exact per-triangle test of that contract.
for (const [name, { mesh, interior }] of Object.entries(MESHES)) {
  test(`${name} mesh triangles are wound with inward normals`, () => {
    mesh.triangles.forEach((triangle, index) => {
      const facing = dot(triangleNormal(triangle), sub(interior, triangle.v0));
      assert.ok(facing > 0, `triangle ${index} of the ${name} mesh points outward`);
    });
  });

  test(`${name} mesh has consistent winding and encloses a volume`, () => {
    // Signed volume of a closed shell: negative for inward-facing normals.
    // Zero or positive would mean an open or inconsistently wound shell.
    const signedSixVolume = mesh.triangles.reduce(
      (total, t) => total + dot(t.v0, cross(t.v1, t.v2)),
      0,
    );
    assert.ok(signedSixVolume < 0, `${name} mesh is not a closed inward-wound shell`);

    // Every edge must be shared by exactly two triangles in opposite directions.
    const edges = new Map();
    const key = (a, b) => `${a.x},${a.y},${a.z}->${b.x},${b.y},${b.z}`;

    for (const t of mesh.triangles) {
      for (const [a, b] of [
        [t.v0, t.v1],
        [t.v1, t.v2],
        [t.v2, t.v0],
      ]) {
        edges.set(key(a, b), (edges.get(key(a, b)) ?? 0) + 1);
      }
    }

    for (const [edge, count] of edges) {
      const [from, to] = edge.split("->");
      assert.equal(count, 1, `edge ${edge} is traversed more than once`);
      assert.equal(edges.get(`${to}->${from}`), 1, `edge ${edge} has no opposite twin`);
    }
  });

  test(`${name} mesh yields reflections off every surface`, () => {
    const source = { x: interior.x - 1, y: interior.y, z: interior.z };
    const receiver = { x: interior.x + 1, y: interior.y, z: interior.z + 0.3 };
    const paths = computeMeshReflectionPaths(mesh.triangles, source, receiver, [1]);

    // One first-order path per distinct plane of the room.
    const planeCount = name === "triangular prism" ? 5 : 6;
    assert.equal(paths.length, planeCount);
  });
}
