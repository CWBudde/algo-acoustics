// bvh_kernel.cu — CUDA ray-BVH traversal kernel.
//
// Implements a standard stack-based BVH2 traversal:
//   for each ray (one thread):
//     push root onto per-thread stack
//     while stack not empty:
//       pop node
//       if leaf: test all triangles, update hit
//       if internal: test children AABBs; push intersected children
//
// Stack depth: bounded by BVH depth (~log₂N).  For 10K triangles
// depth ≤ 20; a stack of 32 entries is sufficient.
//
// Memory layout chosen for coalesced access:
//   rays[]  — AoS.  Rays are independent so coalescing is per-thread.
//   nodes[] — depth-first BVH; children stored consecutively.
//   tris[]  — flat array reordered by BVH builder.
//   hits[]  — one entry per ray, written once per kernel.
//
// OptiX note:
//   On Turing/Ampere GPUs with RT cores, OptiX would replace the
//   software AABB tests with hardware BVH traversal, typically 5–20×
//   faster per ray.  OptiX also provides a production-quality BVH
//   builder with spatial splits.  The custom kernel here demonstrates
//   the data flow; switch to optixTrace() in 14.4/14.5 if OptiX SDK
//   becomes available.

#include "bvh.h"
#include <cstdio>

// Per-thread traversal stack depth.
static constexpr int STACK_DEPTH = 32;

// ------------------------------------------------------------------
// Kernel
// ------------------------------------------------------------------
__global__ void trace_rays_kernel(
    const BVHNode* __restrict__ nodes,
    const Triangle* __restrict__ tris,
    const Ray*     __restrict__ rays,
    HitRecord*     __restrict__ hits,
    int num_rays)
{
    const int ray_idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (ray_idx >= num_rays) return;

    const Ray ray = rays[ray_idx];
    const float3 rd_inv = {
        1.0f / ray.dir.x,
        1.0f / ray.dir.y,
        1.0f / ray.dir.z
    };

    float closest_t = ray.tmax;
    int   closest_id = -1;

    // Per-thread traversal stack (lives in local/register memory).
    int stack[STACK_DEPTH];
    int stack_top = 0;
    stack[stack_top++] = 0;  // start at root

    while (stack_top > 0) {
        const int node_idx = stack[--stack_top];
        const BVHNode node = nodes[node_idx];

        // AABB culling.
        float tNear, tFar;
        aabb_intersect(node.box, ray.origin, rd_inv, tNear, tFar);
        if (tNear > tFar || tFar < ray.tmin || tNear > closest_t)
            continue;

        if (node.count > 0) {
            // Leaf: test triangles.
            for (int i = node.left_or_first;
                     i < node.left_or_first + node.count; i++) {
                const float t = tri_intersect(tris[i], ray.origin, ray.dir,
                                              ray.tmin, closest_t);
                if (t > 0.0f && t < closest_t) {
                    closest_t  = t;
                    closest_id = tris[i].id;
                }
            }
        } else {
            // Internal: push both children (closer child last = popped first).
            const int left  = node.left_or_first;
            const int right = node.left_or_first + 1;

            float tNL, tFL, tNR, tFR;
            aabb_intersect(nodes[left].box,  ray.origin, rd_inv, tNL, tFL);
            aabb_intersect(nodes[right].box, ray.origin, rd_inv, tNR, tFR);
            const bool hit_l = (tNL <= tFL && tFL >= ray.tmin && tNL <= closest_t);
            const bool hit_r = (tNR <= tFR && tFR >= ray.tmin && tNR <= closest_t);

            // Push closer child last so it's processed first.
            if (hit_l && hit_r) {
                if (tNL < tNR) {
                    if (stack_top < STACK_DEPTH) stack[stack_top++] = right;
                    if (stack_top < STACK_DEPTH) stack[stack_top++] = left;
                } else {
                    if (stack_top < STACK_DEPTH) stack[stack_top++] = left;
                    if (stack_top < STACK_DEPTH) stack[stack_top++] = right;
                }
            } else if (hit_l) {
                if (stack_top < STACK_DEPTH) stack[stack_top++] = left;
            } else if (hit_r) {
                if (stack_top < STACK_DEPTH) stack[stack_top++] = right;
            }
        }
    }

    hits[ray_idx] = { closest_t, closest_id };
}

// ------------------------------------------------------------------
// Host launcher.
// ------------------------------------------------------------------
void launch_trace_rays(
    const BVHNode* d_nodes,
    const Triangle* d_tris,
    const Ray* d_rays,
    HitRecord* d_hits,
    int num_rays,
    cudaStream_t stream)
{
    constexpr int BLOCK = 128;
    const int grid = (num_rays + BLOCK - 1) / BLOCK;
    trace_rays_kernel<<<grid, BLOCK, 0, stream>>>(
        d_nodes, d_tris, d_rays, d_hits, num_rays);
}
