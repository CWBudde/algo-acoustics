// bvh_build.h — simple CPU BVH builder (median-split, binned SAH).
//
// Produces a flat BVHNode array suitable for upload to the GPU.
// This is a prototype-quality builder; for production, consider using
// the OptiX BVH builder or a dedicated LBVH GPU builder.
//
// Interface:
//   build_bvh(tris, n_tris) -> vector<BVHNode>
//     Triangles are reordered in-place; BVH leaf nodes reference the
//     reordered triangle array.

#pragma once

#include "bvh.h"
#include <vector>
#include <algorithm>
#include <numeric>
#include <cassert>

// Maximum triangles per leaf.
static constexpr int BVH_LEAF_MAX = 4;

// Number of SAH binning buckets along each axis.
static constexpr int SAH_BINS = 8;

namespace bvh_build {

inline AABB tri_aabb(const Triangle& t)
{
    return {
        { fminf(fminf(t.v0.x, t.v1.x), t.v2.x),
          fminf(fminf(t.v0.y, t.v1.y), t.v2.y),
          fminf(fminf(t.v0.z, t.v1.z), t.v2.z) },
        { fmaxf(fmaxf(t.v0.x, t.v1.x), t.v2.x),
          fmaxf(fmaxf(t.v0.y, t.v1.y), t.v2.y),
          fmaxf(fmaxf(t.v0.z, t.v1.z), t.v2.z) }
    };
}

inline float3 tri_centroid(const Triangle& t)
{
    return { (t.v0.x + t.v1.x + t.v2.x) / 3.0f,
             (t.v0.y + t.v1.y + t.v2.y) / 3.0f,
             (t.v0.z + t.v1.z + t.v2.z) / 3.0f };
}

inline float aabb_surface(const AABB& b)
{
    const float dx = b.hi.x - b.lo.x;
    const float dy = b.hi.y - b.lo.y;
    const float dz = b.hi.z - b.lo.z;
    return 2.0f * (dx*dy + dy*dz + dz*dx);
}

// Recursive builder; appends to `nodes`.
static void build_recursive(
    std::vector<BVHNode>& nodes,
    std::vector<Triangle>& tris,
    int first, int count)
{
    // Compute bounding box of this range.
    AABB box = aabb_empty();
    for (int i = first; i < first + count; i++)
        box = aabb_union(box, tri_aabb(tris[i]));

    int node_idx = (int)nodes.size();
    nodes.push_back({ box, first, count });  // placeholder (may become internal)

    if (count <= BVH_LEAF_MAX) return;  // leaf

    // Binned SAH split: find axis and position minimising split cost.
    float best_cost = FLT_MAX;
    int   best_axis = 0;
    int   best_bin  = SAH_BINS / 2;

    for (int axis = 0; axis < 3; axis++) {
        // Centroid bounding box along this axis.
        float cmin = FLT_MAX, cmax = -FLT_MAX;
        for (int i = first; i < first + count; i++) {
            float3 c = tri_centroid(tris[i]);
            float cv = (axis == 0) ? c.x : (axis == 1) ? c.y : c.z;
            cmin = fminf(cmin, cv);
            cmax = fmaxf(cmax, cv);
        }
        if (cmax - cmin < 1e-8f) continue;

        // Build bins.
        struct Bin { AABB box; int count; };
        Bin bins[SAH_BINS] = {};
        for (int b = 0; b < SAH_BINS; b++) bins[b].box = aabb_empty();

        for (int i = first; i < first + count; i++) {
            float3 c = tri_centroid(tris[i]);
            float cv = (axis == 0) ? c.x : (axis == 1) ? c.y : c.z;
            int b = (int)((cv - cmin) / (cmax - cmin) * SAH_BINS);
            b = std::min(b, SAH_BINS - 1);
            bins[b].box = aabb_union(bins[b].box, tri_aabb(tris[i]));
            bins[b].count++;
        }

        // Sweep left→right and right→left to compute split cost.
        AABB left_box = aabb_empty();
        int  left_cnt = 0;
        float left_area[SAH_BINS - 1], right_area[SAH_BINS - 1];
        int   left_count[SAH_BINS - 1], right_count[SAH_BINS - 1];

        for (int b = 0; b < SAH_BINS - 1; b++) {
            left_box = aabb_union(left_box, bins[b].box);
            left_cnt += bins[b].count;
            left_area[b]  = aabb_surface(left_box);
            left_count[b] = left_cnt;
        }

        AABB right_box = aabb_empty();
        int  right_cnt = 0;
        for (int b = SAH_BINS - 2; b >= 0; b--) {
            right_box = aabb_union(right_box, bins[b + 1].box);
            right_cnt += bins[b + 1].count;
            right_area[b]  = aabb_surface(right_box);
            right_count[b] = right_cnt;
        }

        const float parent_area = aabb_surface(box);
        for (int b = 0; b < SAH_BINS - 1; b++) {
            const float cost = (left_area[b]  * left_count[b] +
                                right_area[b] * right_count[b]) / parent_area;
            if (cost < best_cost) {
                best_cost = cost;
                best_axis = axis;
                best_bin  = b;
            }
        }
    }

    // Partition triangles around the split.
    float3 cmin3 = { FLT_MAX, FLT_MAX, FLT_MAX };
    float3 cmax3 = { -FLT_MAX, -FLT_MAX, -FLT_MAX };
    for (int i = first; i < first + count; i++) {
        float3 c = tri_centroid(tris[i]);
        cmin3.x = fminf(cmin3.x, c.x); cmax3.x = fmaxf(cmax3.x, c.x);
        cmin3.y = fminf(cmin3.y, c.y); cmax3.y = fmaxf(cmax3.y, c.y);
        cmin3.z = fminf(cmin3.z, c.z); cmax3.z = fmaxf(cmax3.z, c.z);
    }
    const float cmin_a = (best_axis == 0) ? cmin3.x : (best_axis == 1) ? cmin3.y : cmin3.z;
    const float cmax_a = (best_axis == 0) ? cmax3.x : (best_axis == 1) ? cmax3.y : cmax3.z;
    const float split  = cmin_a + (cmax_a - cmin_a) * (best_bin + 1) / (float)SAH_BINS;

    auto pivot = std::partition(
        tris.begin() + first, tris.begin() + first + count,
        [&](const Triangle& t) {
            float3 c = tri_centroid(t);
            float cv = (best_axis == 0) ? c.x : (best_axis == 1) ? c.y : c.z;
            return cv < split;
        });

    int left_count_actual = (int)(pivot - (tris.begin() + first));
    if (left_count_actual == 0 || left_count_actual == count) {
        // Degenerate split → force median.
        left_count_actual = count / 2;
    }

    // Mark node as internal; children will be appended recursively.
    int left_child = (int)nodes.size();
    nodes[node_idx].left_or_first = left_child;
    nodes[node_idx].count = 0;  // internal

    build_recursive(nodes, tris, first, left_count_actual);
    build_recursive(nodes, tris, first + left_count_actual, count - left_count_actual);
}

// Public entry point.
inline std::vector<BVHNode> build_bvh(std::vector<Triangle>& tris)
{
    std::vector<BVHNode> nodes;
    nodes.reserve(2 * tris.size());
    build_recursive(nodes, tris, 0, (int)tris.size());
    return nodes;
}

} // namespace bvh_build
