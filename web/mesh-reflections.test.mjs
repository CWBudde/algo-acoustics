import test from "node:test";
import assert from "node:assert/strict";

import {
  MAX_MESH_PATHS,
  computeMeshReflectionPaths,
  meshUniquePlanes,
  orientPlaneToward,
  sideOf,
} from "./mesh-reflections.mjs";

const WIDTH = 6;
const DEPTH = 4;
const HEIGHT = 3;

const SOURCE = { x: 1.5, y: 1, z: 1.2 };
const RECEIVER = { x: 4, y: 3, z: 1.7 };

function quad(a, b, c, d) {
  return [
    { v0: a, v1: b, v2: c },
    { v0: a, v1: c, v2: d },
  ];
}

// Axis-aligned box with inward-pointing normals, in acoustic coordinates.
function boxMesh() {
  const p = (x, y, z) => ({ x, y, z });

  return [
    // floor (z = 0), normal +z
    ...quad(p(0, 0, 0), p(WIDTH, 0, 0), p(WIDTH, DEPTH, 0), p(0, DEPTH, 0)),
    // ceiling (z = HEIGHT), normal -z
    ...quad(
      p(0, 0, HEIGHT),
      p(0, DEPTH, HEIGHT),
      p(WIDTH, DEPTH, HEIGHT),
      p(WIDTH, 0, HEIGHT),
    ),
    // south (y = 0), normal +y
    ...quad(p(0, 0, 0), p(0, 0, HEIGHT), p(WIDTH, 0, HEIGHT), p(WIDTH, 0, 0)),
    // north (y = DEPTH), normal -y
    ...quad(
      p(0, DEPTH, 0),
      p(WIDTH, DEPTH, 0),
      p(WIDTH, DEPTH, HEIGHT),
      p(0, DEPTH, HEIGHT),
    ),
    // west (x = 0), normal +x
    ...quad(p(0, 0, 0), p(0, DEPTH, 0), p(0, DEPTH, HEIGHT), p(0, 0, HEIGHT)),
    // east (x = WIDTH), normal -x
    ...quad(
      p(WIDTH, 0, 0),
      p(WIDTH, 0, HEIGHT),
      p(WIDTH, DEPTH, HEIGHT),
      p(WIDTH, DEPTH, 0),
    ),
  ];
}

function flipWinding(triangles) {
  return triangles.map((t) => ({ v0: t.v0, v1: t.v2, v2: t.v1 }));
}

// Alternate the winding face by face, reproducing the inconsistently wound
// meshes that used to make half the surfaces disappear from the overlay.
function mixWinding(triangles) {
  return triangles.map((t, index) =>
    index % 2 === 0 ? t : { v0: t.v0, v1: t.v2, v2: t.v1 },
  );
}

function signature(paths) {
  return paths
    .map(({ order, points }) =>
      [order, ...points.map((p) => `${round(p.x)},${round(p.y)},${round(p.z)}`)].join("|"),
    )
    .sort();
}

function round(value) {
  return Math.round(value * 1e9) / 1e9;
}

// Analytic first-order reflection point on an axis-aligned wall: the mirrored
// source and the receiver define the segment that crosses the wall.
function analyticFirstOrderHit(axis, coord) {
  const image = { ...SOURCE, [axis]: 2 * coord - SOURCE[axis] };
  const t = (coord - image[axis]) / (RECEIVER[axis] - image[axis]);

  return {
    x: image.x + (RECEIVER.x - image.x) * t,
    y: image.y + (RECEIVER.y - image.y) * t,
    z: image.z + (RECEIVER.z - image.z) * t,
  };
}

test("unique planes collapse coplanar triangles regardless of winding", () => {
  assert.equal(meshUniquePlanes(boxMesh()).length, 6);
  assert.equal(meshUniquePlanes(flipWinding(boxMesh())).length, 6);
  assert.equal(meshUniquePlanes(mixWinding(boxMesh())).length, 6);
});

test("orientPlaneToward flips the plane so its normal faces the point", () => {
  const [plane] = meshUniquePlanes(flipWinding(boxMesh()));
  assert.ok(sideOf(plane, SOURCE) < 0);

  const oriented = orientPlaneToward(plane, SOURCE);
  assert.ok(sideOf(oriented, SOURCE) > 0);
  assert.equal(oriented.triangles, plane.triangles);
});

test("first order finds one path off every wall of a box", () => {
  const paths = computeMeshReflectionPaths(boxMesh(), SOURCE, RECEIVER, [1]);

  assert.equal(paths.length, 6);
  for (const path of paths) {
    assert.equal(path.order, 1);
    assert.equal(path.points.length, 3);
    assert.deepEqual(path.points[0], SOURCE);
    assert.deepEqual(path.points[2], RECEIVER);
  }

  const expected = [
    analyticFirstOrderHit("z", 0),
    analyticFirstOrderHit("z", HEIGHT),
    analyticFirstOrderHit("y", 0),
    analyticFirstOrderHit("y", DEPTH),
    analyticFirstOrderHit("x", 0),
    analyticFirstOrderHit("x", WIDTH),
  ];

  for (const hit of expected) {
    const match = paths.find(
      ({ points }) =>
        Math.abs(points[1].x - hit.x) < 1e-9 &&
        Math.abs(points[1].y - hit.y) < 1e-9 &&
        Math.abs(points[1].z - hit.z) < 1e-9,
    );
    assert.ok(match, `no path reflecting at ${JSON.stringify(hit)}`);
  }
});

test("paths are independent of triangle winding", () => {
  const orders = [1, 2];
  const reference = signature(computeMeshReflectionPaths(boxMesh(), SOURCE, RECEIVER, orders));

  assert.ok(reference.length > 6);
  assert.deepEqual(
    signature(computeMeshReflectionPaths(flipWinding(boxMesh()), SOURCE, RECEIVER, orders)),
    reference,
  );
  assert.deepEqual(
    signature(computeMeshReflectionPaths(mixWinding(boxMesh()), SOURCE, RECEIVER, orders)),
    reference,
  );
});

test("second order paths bounce off two distinct surfaces", () => {
  const paths = computeMeshReflectionPaths(boxMesh(), SOURCE, RECEIVER, [2]);

  // 6 walls × 5 non-repeating partners, minus the sequences whose unfolded
  // segment leaves the wall rectangle.
  assert.ok(paths.length > 0 && paths.length <= 30);

  for (const path of paths) {
    assert.equal(path.order, 2);
    assert.equal(path.points.length, 4);

    const [, first, second] = path.points;
    assert.notDeepEqual(first, second);

    // Every reflection point must sit on the box surface.
    for (const hit of [first, second]) {
      const onSurface =
        Math.abs(hit.x) < 1e-9 ||
        Math.abs(hit.x - WIDTH) < 1e-9 ||
        Math.abs(hit.y) < 1e-9 ||
        Math.abs(hit.y - DEPTH) < 1e-9 ||
        Math.abs(hit.z) < 1e-9 ||
        Math.abs(hit.z - HEIGHT) < 1e-9;
      assert.ok(onSurface, `reflection point off surface: ${JSON.stringify(hit)}`);
    }
  }
});

test("a wall the source sits behind never reflects", () => {
  // Cutting the box down to a single wall leaves the source on one side only;
  // a receiver placed behind that wall has no specular path across it.
  const floorOnly = boxMesh().slice(0, 2);
  const behindFloor = { x: 3, y: 2, z: -1 };

  assert.deepEqual(computeMeshReflectionPaths(floorOnly, SOURCE, behindFloor, [1]), []);
  assert.equal(computeMeshReflectionPaths(floorOnly, SOURCE, RECEIVER, [1]).length, 1);
});

test("degenerate input yields no paths", () => {
  assert.deepEqual(computeMeshReflectionPaths([], SOURCE, RECEIVER, [1, 2]), []);
  assert.deepEqual(computeMeshReflectionPaths(boxMesh(), SOURCE, RECEIVER, []), []);
  assert.deepEqual(computeMeshReflectionPaths(boxMesh(), SOURCE, RECEIVER, [0]), []);
});

test("path count stays within the render budget", () => {
  const paths = computeMeshReflectionPaths(boxMesh(), SOURCE, RECEIVER, [1, 2]);
  assert.ok(paths.length <= MAX_MESH_PATHS);
});
