// bvh_kernel.h — host-callable ray-BVH traversal launcher.
// Implemented in bvh_kernel.cu.
#pragma once
#include "bvh.h"

// Launch the BVH traversal kernel.  All device pointers must already be
// allocated and filled.  The kernel writes one HitRecord per ray into d_hits.
void launch_trace_rays(
    const BVHNode*  d_nodes,
    const Triangle* d_tris,
    const Ray*      d_rays,
    HitRecord*      d_hits,
    int             num_rays,
    cudaStream_t    stream = 0);
