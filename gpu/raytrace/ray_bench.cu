// ray_bench.cu — benchmark harness for the CUDA ray-BVH traversal kernel.
//
// Generates a random 10K-triangle scene (triangles distributed on the
// surface of a unit sphere), builds a BVH on the CPU, uploads to the GPU,
// then benchmarks ray throughput for three batch sizes:
//
//   100 000 rays  — small batch (interactive preview)
//   1 000 000 rays — mid-size batch
//  10 000 000 rays — large batch (production quality)
//
// CPU baseline (from PLAN.md §14.1, i7-1255U single-core):
//   65 536 rays → 714 ms  →  ~0.092 Mrays/s
//
// The Go raytrace package uses a sphere receiver and full diffuse
// scattering; the benchmark here measures pure intersection throughput
// (finding the nearest hit) which is the dominant cost.

#include "bvh.h"
#include "bvh_build.h"
#include <cstdio>
#include <cstdlib>
#include <cmath>
#include <vector>
#include <random>
#include <cuda_runtime.h>

#define CUDA_CHECK(call)                                                  \
    do {                                                                  \
        cudaError_t _e = (call);                                          \
        if (_e != cudaSuccess) {                                          \
            fprintf(stderr, "CUDA error %s:%d  %s\n",                    \
                    __FILE__, __LINE__, cudaGetErrorString(_e));          \
            exit(EXIT_FAILURE);                                           \
        }                                                                 \
    } while (0)

// Declared in bvh_kernel.cu.
void launch_trace_rays(
    const BVHNode*, const Triangle*, const Ray*, HitRecord*,
    int, cudaStream_t);

static constexpr int   N_TRIS  = 10000;
static constexpr int   WARMUP  = 3;
static constexpr int   REPEATS = 10;

// ------------------------------------------------------------------
// Generate random triangles distributed on a unit sphere surface.
// Returns a vector of N_TRIS triangles.
// ------------------------------------------------------------------
static std::vector<Triangle> make_scene(uint32_t seed = 42)
{
    std::mt19937 rng(seed);
    std::uniform_real_distribution<float> uni(-1.0f, 1.0f);
    std::normal_distribution<float>       norm(0.0f, 1.0f);

    auto rand_sphere_pt = [&]() -> float3 {
        float x, y, z, len;
        do {
            x = norm(rng); y = norm(rng); z = norm(rng);
            len = sqrtf(x*x + y*y + z*z);
        } while (len < 1e-5f);
        return { x/len, y/len, z/len };
    };

    std::vector<Triangle> tris;
    tris.reserve(N_TRIS);
    for (int i = 0; i < N_TRIS; i++) {
        float3 c = rand_sphere_pt();
        // Small triangle centred at c on the sphere surface.
        const float size = 0.015f + 0.01f * fabsf(uni(rng));
        const float3 u = { c.y, -c.x, 0.0f };
        float ulen = sqrtf(u.x*u.x + u.y*u.y + u.z*u.z);
        if (ulen < 1e-5f) ulen = 1.0f;
        const float3 un = { u.x/ulen, u.y/ulen, u.z/ulen };
        const float3 v  = {
            c.y*un.z - c.z*un.y,
            c.z*un.x - c.x*un.z,
            c.x*un.y - c.y*un.x
        };
        tris.push_back({
            { c.x + size*un.x,  c.y + size*un.y,  c.z + size*un.z },
            { c.x - size*un.x + size*v.x,
              c.y - size*un.y + size*v.y,
              c.z - size*un.z + size*v.z },
            { c.x - size*un.x - size*v.x,
              c.y - size*un.y - size*v.y,
              c.z - size*un.z - size*v.z },
            i
        });
    }
    return tris;
}

// ------------------------------------------------------------------
// Generate random rays from the interior of the unit sphere.
// ------------------------------------------------------------------
static std::vector<Ray> make_rays(int n, uint32_t seed = 123)
{
    std::mt19937 rng(seed);
    std::normal_distribution<float> norm(0.0f, 1.0f);
    std::uniform_real_distribution<float> uni(-0.5f, 0.5f);

    std::vector<Ray> rays;
    rays.reserve(n);
    for (int i = 0; i < n; i++) {
        float3 dir = { norm(rng), norm(rng), norm(rng) };
        float  len = sqrtf(dir.x*dir.x + dir.y*dir.y + dir.z*dir.z);
        if (len < 1e-5f) len = 1.0f;
        dir = { dir.x/len, dir.y/len, dir.z/len };
        rays.push_back({ { uni(rng), uni(rng), uni(rng) }, dir, 1e-4f, 1e4f });
    }
    return rays;
}

// ------------------------------------------------------------------
// Run one benchmark configuration; return Mrays/s.
// ------------------------------------------------------------------
static double bench_rays(
    int num_rays,
    const BVHNode* d_nodes,
    const Triangle* d_tris)
{
    // Generate and upload rays.
    auto h_rays = make_rays(num_rays);
    Ray* d_rays;
    CUDA_CHECK(cudaMalloc(&d_rays, num_rays * sizeof(Ray)));
    CUDA_CHECK(cudaMemcpy(d_rays, h_rays.data(),
                          num_rays * sizeof(Ray), cudaMemcpyHostToDevice));

    HitRecord* d_hits;
    CUDA_CHECK(cudaMalloc(&d_hits, num_rays * sizeof(HitRecord)));

    // Warm-up.
    for (int r = 0; r < WARMUP; r++)
        launch_trace_rays(d_nodes, d_tris, d_rays, d_hits, num_rays, 0);
    CUDA_CHECK(cudaDeviceSynchronize());

    // Timed run.
    cudaEvent_t t0, t1;
    CUDA_CHECK(cudaEventCreate(&t0));
    CUDA_CHECK(cudaEventCreate(&t1));
    CUDA_CHECK(cudaEventRecord(t0));

    for (int r = 0; r < REPEATS; r++)
        launch_trace_rays(d_nodes, d_tris, d_rays, d_hits, num_rays, 0);

    CUDA_CHECK(cudaEventRecord(t1));
    CUDA_CHECK(cudaEventSynchronize(t1));

    float ms;
    CUDA_CHECK(cudaEventElapsedTime(&ms, t0, t1));
    CUDA_CHECK(cudaEventDestroy(t0));
    CUDA_CHECK(cudaEventDestroy(t1));

    CUDA_CHECK(cudaFree(d_rays));
    CUDA_CHECK(cudaFree(d_hits));

    return (double)num_rays * REPEATS / (ms * 1e-3) / 1e6;  // Mrays/s
}

// ------------------------------------------------------------------
// main
// ------------------------------------------------------------------
int main()
{
    cudaDeviceProp prop;
    CUDA_CHECK(cudaGetDeviceProperties(&prop, 0));
    printf("Device: %s  (sm_%d%d, %.1f GB)\n",
           prop.name, prop.major, prop.minor,
           prop.totalGlobalMem / 1e9);

    printf("\nBuilding %d-triangle scene and BVH (CPU)...\n", N_TRIS);
    auto tris = make_scene();
    auto nodes = bvh_build::build_bvh(tris);
    printf("  BVH: %d nodes, max leaf=%d tris\n",
           (int)nodes.size(), BVH_LEAF_MAX);

    // Upload scene to GPU.
    BVHNode*  d_nodes;
    Triangle* d_tris;
    const size_t node_bytes = nodes.size() * sizeof(BVHNode);
    const size_t tri_bytes  = tris.size()  * sizeof(Triangle);
    CUDA_CHECK(cudaMalloc(&d_nodes, node_bytes));
    CUDA_CHECK(cudaMalloc(&d_tris,  tri_bytes));
    CUDA_CHECK(cudaMemcpy(d_nodes, nodes.data(), node_bytes, cudaMemcpyHostToDevice));
    CUDA_CHECK(cudaMemcpy(d_tris,  tris.data(),  tri_bytes,  cudaMemcpyHostToDevice));
    printf("  Scene uploaded: %.2f MB nodes + %.2f MB tris\n",
           node_bytes / 1e6, tri_bytes / 1e6);

    printf("\nCPU single-core baseline (i7-1255U, from 14.1):\n");
    printf("  65 536 rays: 714 ms  →  0.092 Mrays/s\n\n");

    printf("| Rays      | Mrays/s  | vs CPU single-core | Approx ms/batch |\n");
    printf("|-----------|----------|--------------------|------------------|\n");

    const double cpu_mrays = 65536.0 / 714.0 / 1000.0;  // Mrays/s

    int batch_sizes[] = { 100000, 1000000, 10000000 };
    for (int n : batch_sizes) {
        // Check VRAM: each ray = sizeof(Ray) + sizeof(HitRecord) bytes.
        size_t free_mem, total_mem;
        CUDA_CHECK(cudaMemGetInfo(&free_mem, &total_mem));
        const size_t needed = (size_t)n * (sizeof(Ray) + sizeof(HitRecord));
        if (needed > free_mem * 9 / 10) {
            printf("| %9d | skipped  | — | not enough VRAM (%zu MB) |\n",
                   n, needed / (1024*1024));
            continue;
        }

        const double mrays = bench_rays(n, d_nodes, d_tris);
        const double speedup = mrays / cpu_mrays;
        const double ms_batch = (double)n / (mrays * 1e6) * 1000.0;
        printf("| %9d | %8.2f | %18.1fx | %16.2f |\n",
               n, mrays, speedup, ms_batch);
    }

    CUDA_CHECK(cudaFree(d_nodes));
    CUDA_CHECK(cudaFree(d_tris));

    printf("\nNote: OptiX hardware ray tracing (RT cores on Turing/T550) would\n");
    printf("typically add 5-20x over this software BVH traversal.\n");
    printf("See PLAN.md §14.2 and docs/profiling-14.1-baseline.md for context.\n");

    return 0;
}
