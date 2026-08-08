// Specular reflection paths for triangle-mesh rooms, in acoustic coordinates
// (x = width, y = depth, z = height). Deliberately free of any three.js
// dependency so the geometry can be unit tested on its own; the caller is
// responsible for converting the returned points into scene space.
//
// The enumeration mirrors the backward-unfolding image-source method used by
// the Go solver in ism/mesh_solver.go, with one deliberate difference: it is
// agnostic to triangle winding. Each plane is oriented on demand toward the
// point it has to reflect, so a mesh whose normals point outward (or whose
// winding is inconsistent between faces) yields the same paths as a correctly
// wound one.
//
// Paths are specular-geometric and are not occlusion tested: a leg that clips
// another surface on its way is still returned. This matches the shoebox
// overlay and the analytic shoebox solver in ism/audibility.go, and keeps the
// module free of a ray caster so it stays cheap enough to re-run on every
// source/receiver drag.

const PLANE_EPS = 1e-6;
const SIDE_EPS = 1e-6;
const SEGMENT_EPS = 1e-9;
const TRIANGLE_EPS = 1e-9;

// Upper bound on returned paths. Second-order enumeration is O(planes²), so a
// detailed mesh could otherwise stall the render loop on every state change.
export const MAX_MESH_PATHS = 512;

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

function normalize(v) {
  const length = Math.hypot(v.x, v.y, v.z);
  if (length === 0) {
    return null;
  }

  return { x: v.x / length, y: v.y / length, z: v.z / length };
}

// sideOf returns a positive value when point lies on the side the plane normal
// points toward, matching geometry.Plane.SideOf on the Go side.
export function sideOf(plane, point) {
  return dot(plane.normal, point) - plane.d;
}

function mirrorPoint(plane, point) {
  const distance = 2 * sideOf(plane, point);

  return {
    x: point.x - plane.normal.x * distance,
    y: point.y - plane.normal.y * distance,
    z: point.z - plane.normal.z * distance,
  };
}

// orientPlaneToward flips the plane so its normal faces point. This is what
// makes the enumeration independent of the mesh's triangle winding.
export function orientPlaneToward(plane, point) {
  if (sideOf(plane, point) >= 0) {
    return plane;
  }

  return {
    normal: { x: -plane.normal.x, y: -plane.normal.y, z: -plane.normal.z },
    d: -plane.d,
    triangles: plane.triangles,
  };
}

// meshUniquePlanes collapses coplanar triangles into a single plane, keeping
// every triangle so the on-surface test can span a whole polygonal wall.
// Triangles are matched regardless of winding, so a wall triangulated with
// inconsistent orientation still forms one plane.
export function meshUniquePlanes(triangles) {
  const planes = [];

  for (const triangle of triangles ?? []) {
    const normal = normalize(
      cross(sub(triangle.v1, triangle.v0), sub(triangle.v2, triangle.v0)),
    );
    if (!normal) {
      continue;
    }

    const d = dot(normal, triangle.v0);
    const existing = planes.find((plane) => {
      const alignment = dot(plane.normal, normal);
      if (Math.abs(Math.abs(alignment) - 1) > PLANE_EPS) {
        return false;
      }

      return Math.abs(plane.d - Math.sign(alignment) * d) <= PLANE_EPS;
    });

    if (existing) {
      existing.triangles.push(triangle);
      continue;
    }

    planes.push({ normal, d, triangles: [triangle] });
  }

  return planes;
}

// pointInPlaneTriangles reports whether point lies inside any of the plane's
// triangles, using barycentric coordinates in the triangle's own plane.
export function pointInPlaneTriangles(plane, point) {
  return plane.triangles.some((triangle) => pointInTriangle(triangle, point));
}

function pointInTriangle(triangle, point) {
  const e0 = sub(triangle.v1, triangle.v0);
  const e1 = sub(triangle.v2, triangle.v0);
  const vp = sub(point, triangle.v0);

  const d00 = dot(e0, e0);
  const d01 = dot(e0, e1);
  const d11 = dot(e1, e1);
  const denom = d00 * d11 - d01 * d01;
  if (Math.abs(denom) <= TRIANGLE_EPS) {
    return false;
  }

  const d20 = dot(vp, e0);
  const d21 = dot(vp, e1);
  const u = (d11 * d20 - d01 * d21) / denom;
  const v = (d00 * d21 - d01 * d20) / denom;

  return u >= -TRIANGLE_EPS && v >= -TRIANGLE_EPS && u + v <= 1 + TRIANGLE_EPS;
}

// intersectSegmentWithPlane returns the crossing point strictly between start
// and end, or null when the segment runs parallel to the plane or crosses it
// outside the segment.
function intersectSegmentWithPlane(plane, start, end) {
  const direction = sub(end, start);
  const denom = dot(plane.normal, direction);
  if (Math.abs(denom) < SEGMENT_EPS) {
    return null;
  }

  const t = (plane.d - dot(plane.normal, start)) / denom;
  if (t <= SEGMENT_EPS || t >= 1 - SEGMENT_EPS) {
    return null;
  }

  return {
    x: start.x + direction.x * t,
    y: start.y + direction.y * t,
    z: start.z + direction.z * t,
  };
}

// buildPath unfolds a sequence of planes (ordered source-first) into the list
// of reflection points, or returns null when the sequence carries no valid
// specular path between source and receiver.
function buildPath(sequence, source, receiver) {
  const images = [source];
  for (const plane of sequence) {
    images.push(mirrorPoint(plane, images[images.length - 1]));
  }

  const hitsReverse = [];
  let target = receiver;

  for (let index = sequence.length - 1; index >= 0; index -= 1) {
    const plane = sequence[index];
    const hit = intersectSegmentWithPlane(plane, target, images[index + 1]);
    if (!hit || !pointInPlaneTriangles(plane, hit)) {
      return null;
    }

    hitsReverse.push(hit);
    target = mirrorPoint(plane, target);
  }

  const hits = hitsReverse.reverse();

  // Specular validity: at every bounce the incoming and outgoing legs must both
  // lie on the reflective side of the surface. Orienting the plane toward the
  // incoming point first keeps this test independent of triangle winding.
  for (let index = 0; index < sequence.length; index += 1) {
    const previous = index === 0 ? source : hits[index - 1];
    const next = index === hits.length - 1 ? receiver : hits[index + 1];
    const plane = orientPlaneToward(sequence[index], previous);

    if (sideOf(plane, previous) <= SIDE_EPS || sideOf(plane, next) <= SIDE_EPS) {
      return null;
    }
  }

  return [source, ...hits, receiver];
}

// computeMeshReflectionPaths enumerates every valid specular path of the
// requested orders between source and receiver. Points are returned in
// acoustic coordinates, source first and receiver last.
export function computeMeshReflectionPaths(triangles, source, receiver, orders) {
  const requested = (orders ?? []).filter((order) => order >= 1);
  if (!requested.length) {
    return [];
  }

  const planes = meshUniquePlanes(triangles);
  if (!planes.length) {
    return [];
  }

  const paths = [];

  for (const order of [...new Set(requested)].sort((a, b) => a - b)) {
    for (const sequence of planeSequences(planes, order)) {
      if (paths.length >= MAX_MESH_PATHS) {
        return paths;
      }

      const points = buildPath(sequence, source, receiver);
      if (points) {
        paths.push({ order, points });
      }
    }
  }

  return paths;
}

// planeSequences yields every ordered plane sequence of the given length,
// skipping consecutive repeats since a ray cannot reflect off the same surface
// twice in a row.
function* planeSequences(planes, length) {
  if (length <= 0) {
    return;
  }

  if (length === 1) {
    for (const plane of planes) {
      yield [plane];
    }

    return;
  }

  for (const head of planeSequences(planes, length - 1)) {
    for (const plane of planes) {
      // Plane objects come from meshUniquePlanes, so identity is exact.
      if (head[head.length - 1] === plane) {
        continue;
      }

      yield [...head, plane];
    }
  }
}
