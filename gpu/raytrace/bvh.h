// bvh.h — shared CPU/GPU BVH and ray data structures.
//
// Uses only scalar arithmetic and CUDA built-in types so this header
// can be included from both host and device code.
//
// BVH layout: flat array of BVHNode, nodes[0] is root.
//   Internal node: left = first child index, right = left+1 (pairs are
//                  stored consecutively). count = 0.
//   Leaf node:     first_tri = index of first triangle in tris[].
//                  count > 0  = number of triangles in this leaf.

#pragma once

#include <cuda_runtime.h>
#include <float.h>

// ------------------------------------------------------------------
// AABB — axis-aligned bounding box.
// ------------------------------------------------------------------
struct AABB {
    float3 lo;  // component-wise minimum
    float3 hi;  // component-wise maximum
};

__host__ __device__ inline AABB aabb_empty()
{
    return { {FLT_MAX, FLT_MAX, FLT_MAX}, {-FLT_MAX, -FLT_MAX, -FLT_MAX} };
}

__host__ __device__ inline AABB aabb_union(AABB a, AABB b)
{
    return {
        { fminf(a.lo.x, b.lo.x), fminf(a.lo.y, b.lo.y), fminf(a.lo.z, b.lo.z) },
        { fmaxf(a.hi.x, b.hi.x), fmaxf(a.hi.y, b.hi.y), fmaxf(a.hi.z, b.hi.z) }
    };
}

// Slab test: returns (tNear, tFar).  Hit if tNear <= tFar.
__host__ __device__ inline void aabb_intersect(
    const AABB& box, float3 ro, float3 rd_inv,
    float& tNear, float& tFar)
{
    const float tx1 = (box.lo.x - ro.x) * rd_inv.x;
    const float tx2 = (box.hi.x - ro.x) * rd_inv.x;
    const float ty1 = (box.lo.y - ro.y) * rd_inv.y;
    const float ty2 = (box.hi.y - ro.y) * rd_inv.y;
    const float tz1 = (box.lo.z - ro.z) * rd_inv.z;
    const float tz2 = (box.hi.z - ro.z) * rd_inv.z;
    tNear = fmaxf(fmaxf(fminf(tx1, tx2), fminf(ty1, ty2)), fminf(tz1, tz2));
    tFar  = fminf(fminf(fmaxf(tx1, tx2), fmaxf(ty1, ty2)), fmaxf(tz1, tz2));
}

// ------------------------------------------------------------------
// Triangle (winding-order independent).
// ------------------------------------------------------------------
struct Triangle {
    float3 v0, v1, v2;
    int    id;   // original triangle index (for material lookup)
};

// Möller–Trumbore ray-triangle intersection.
// Returns t > 0 on hit, -1 on miss.
__host__ __device__ inline float tri_intersect(
    const Triangle& tri, float3 ro, float3 rd,
    float tmin, float tmax)
{
    const float3 e1 = { tri.v1.x - tri.v0.x,
                        tri.v1.y - tri.v0.y,
                        tri.v1.z - tri.v0.z };
    const float3 e2 = { tri.v2.x - tri.v0.x,
                        tri.v2.y - tri.v0.y,
                        tri.v2.z - tri.v0.z };
    // h = rd × e2
    const float3 h = { rd.y*e2.z - rd.z*e2.y,
                       rd.z*e2.x - rd.x*e2.z,
                       rd.x*e2.y - rd.y*e2.x };
    const float a = e1.x*h.x + e1.y*h.y + e1.z*h.z;

    if (fabsf(a) < 1e-8f) return -1.0f;  // parallel

    const float f = 1.0f / a;
    const float3 s = { ro.x - tri.v0.x, ro.y - tri.v0.y, ro.z - tri.v0.z };
    const float  u = f * (s.x*h.x + s.y*h.y + s.z*h.z);
    if (u < 0.0f || u > 1.0f) return -1.0f;

    const float3 q = { s.y*e1.z - s.z*e1.y,
                       s.z*e1.x - s.x*e1.z,
                       s.x*e1.y - s.y*e1.x };
    const float  v = f * (rd.x*q.x + rd.y*q.y + rd.z*q.z);
    if (v < 0.0f || u + v > 1.0f) return -1.0f;

    const float t = f * (e2.x*q.x + e2.y*q.y + e2.z*q.z);
    return (t > tmin && t < tmax) ? t : -1.0f;
}

// ------------------------------------------------------------------
// BVH node (16 + 4 + 4 = 24 bytes).
// count == 0  → internal: left child at left_or_first, right at left_or_first+1.
// count  > 0  → leaf: triangles at tris[left_or_first .. left_or_first+count).
// ------------------------------------------------------------------
struct BVHNode {
    AABB box;
    int  left_or_first;  // child index (internal) or first tri index (leaf)
    int  count;          // 0 = internal, >0 = leaf
};

// ------------------------------------------------------------------
// Ray.
// ------------------------------------------------------------------
struct Ray {
    float3 origin;
    float3 dir;
    float  tmin;
    float  tmax;
};

// ------------------------------------------------------------------
// Hit record written by the traversal kernel.
// ------------------------------------------------------------------
struct HitRecord {
    float t;      // hit distance; FLT_MAX = no hit
    int   tri_id; // triangle id; -1 = no hit
};
