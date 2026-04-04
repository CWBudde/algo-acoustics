// fdtd_bench.cu — benchmark harness for the CUDA FDTD stencil kernels.
//
// Measures throughput in Gcells/s and effective memory bandwidth for three
// grid sizes that bracket the target problem range (see PLAN.md §14.1):
//
//   128³ ~2M active cells, ~24 MB fields  — preview / regression scale
//   256³ ~16M active cells, ~192 MB fields — production audio scale
//   512³ ~134M active cells, ~1.5 GB fields — high-resolution / stress test
//
// GPU: T550 Laptop (4 GB, sm_75); 512³ at float32 fits with 0.5 GB margin.
// Note: 1024³ would require ~12 GB; not suitable for this GPU.
//
// Outputs a Markdown table of:
//   Grid | Kernel  | Steps | Time(ms) | Gcells/s | EffBW(GB/s) | vs CPU
//
// CPU baseline from 14.1 profiling (single-core, i7-1255U):
//   43³ (59K active cells): 412 µs/step  → ~0.144 Gcells/s

#include "fdtd_kernel.cuh"
#include <cstdio>
#include <cstdlib>
#include <cmath>
#include <cuda_runtime.h>

// Physical constants matching the Go codebase.
static constexpr float SPEED_OF_SOUND = 343.0f; // m/s

// CFL-stable time step: Δt = CFL_FACTOR * h / (c * sqrt(3))
static constexpr float CFL_FACTOR = 0.95f;

// Number of warm-up and timed steps per benchmark run.
// Large grids use fewer steps so the benchmark completes in reasonable time.
static constexpr int WARMUP_STEPS      = 5;
static constexpr int BENCH_STEPS_SMALL = 200;   // 128³, 256³
static constexpr int BENCH_STEPS_LARGE = 20;    // 512³

// ------------------------------------------------------------------
// GPU kernel: fill a field with a constant non-zero value.
// Used to avoid a slow O(N³) host-side init loop for large grids.
// The exact initial values do not affect stencil throughput benchmarks.
// ------------------------------------------------------------------
__global__ void fill_field(float* field, long long n, float val)
{
    const long long i = (long long)blockIdx.x * blockDim.x + threadIdx.x;
    if (i < n) field[i] = val;
}

// ------------------------------------------------------------------
// Run one benchmark configuration.
// kernel_name: "naive" or "tiled"
// Returns wall-clock milliseconds for bench_steps steps.
// ------------------------------------------------------------------
static double bench_kernel(
    const char* kernel_name, int nx, int ny, int nz,
    float* d_next, float* d_cur, float* d_prev,
    int bench_steps)
{
    const float h   = 0.025f; // target grid spacing (m)
    const float dt  = CFL_FACTOR * h / (SPEED_OF_SOUND * sqrtf(3.0f));
    const float lam = (SPEED_OF_SOUND * dt / h) * (SPEED_OF_SOUND * dt / h);

    auto launch = [&](float* n, const float* c, const float* p) {
        if (kernel_name[0] == 'n')
            launch_fdtd_naive(n, c, p, nx, ny, nz, lam);
        else
            launch_fdtd_tiled(n, c, p, nx, ny, nz, lam);
    };

    // Warm-up.
    for (int s = 0; s < WARMUP_STEPS; s++) {
        launch(d_next, d_cur, d_prev);
        float* tmp = d_prev; d_prev = d_cur; d_cur = d_next; d_next = tmp;
    }
    CUDA_CHECK(cudaDeviceSynchronize());

    // Timed run.
    cudaEvent_t t0, t1;
    CUDA_CHECK(cudaEventCreate(&t0));
    CUDA_CHECK(cudaEventCreate(&t1));
    CUDA_CHECK(cudaEventRecord(t0));

    for (int s = 0; s < bench_steps; s++) {
        launch(d_next, d_cur, d_prev);
        float* tmp = d_prev; d_prev = d_cur; d_cur = d_next; d_next = tmp;
    }

    CUDA_CHECK(cudaEventRecord(t1));
    CUDA_CHECK(cudaEventSynchronize(t1));

    float ms = 0.0f;
    CUDA_CHECK(cudaEventElapsedTime(&ms, t0, t1));
    CUDA_CHECK(cudaEventDestroy(t0));
    CUDA_CHECK(cudaEventDestroy(t1));

    return static_cast<double>(ms);
}

// ------------------------------------------------------------------
// Benchmark one grid size with both kernels; print results.
// ------------------------------------------------------------------
static void bench_grid(int nx, int ny, int nz)
{
    const long long ncells = (long long)nx * ny * nz;
    const long long nbytes = ncells * sizeof(float);
    const float h   = 0.025f;
    const float dt  = CFL_FACTOR * h / (SPEED_OF_SOUND * sqrtf(3.0f));

    // Check device has enough memory for 3 fields + small overhead.
    size_t free_mem, total_mem;
    CUDA_CHECK(cudaMemGetInfo(&free_mem, &total_mem));
    if (3 * nbytes > (long long)free_mem * 9 / 10) {
        printf("| %dx%dx%d | %-5s | — | — | — | — | skipped: not enough VRAM (%lld MB needed, %lld MB free) |\n",
            nx, ny, nz, "—",
            3 * nbytes / (1024*1024),
            (long long)free_mem / (1024*1024));
        return;
    }

    // Allocate three device fields and initialise on the GPU
    // (avoids a slow O(N³) host loop for large grids).
    float *d_next, *d_cur, *d_prev;
    CUDA_CHECK(cudaMalloc(&d_next, nbytes));
    CUDA_CHECK(cudaMalloc(&d_cur,  nbytes));
    CUDA_CHECK(cudaMalloc(&d_prev, nbytes));

    {
        const int blk = 256;
        const int grd = (int)((ncells + blk - 1) / blk);
        fill_field<<<grd, blk>>>(d_cur,  ncells, 0.5f);
        fill_field<<<grd, blk>>>(d_prev, ncells, 0.5f);
        fill_field<<<grd, blk>>>(d_next, ncells, 0.0f);
        CUDA_CHECK(cudaDeviceSynchronize());
    }

    // CPU single-core reference: scaled from 14.1 baseline.
    // Baseline: 43³ grid (79507 total nodes) → 412 µs/step single-core.
    // FDTD ops are O(N) so we scale linearly with cell count.
    const double cpu_us_per_step = 412.0 * ncells / 79507.0;

    const int bench_steps = (ncells > 256LL*256*256) ? BENCH_STEPS_LARGE : BENCH_STEPS_SMALL;

    const int fill_blk = 256;
    const int fill_grd = (int)((ncells + fill_blk - 1) / fill_blk);

    const char* kernels[] = {"naive", "tiled"};
    for (const char* kname : kernels) {
        // Re-initialise on GPU so both kernels start from the same state.
        fill_field<<<fill_grd, fill_blk>>>(d_cur,  ncells, 0.5f);
        fill_field<<<fill_grd, fill_blk>>>(d_prev, ncells, 0.5f);
        fill_field<<<fill_grd, fill_blk>>>(d_next, ncells, 0.0f);
        CUDA_CHECK(cudaDeviceSynchronize());

        const double total_ms = bench_kernel(kname, nx, ny, nz, d_next, d_cur, d_prev, bench_steps);
        const double ms_per_step = total_ms / bench_steps;
        const double gcells_per_s = ncells / (ms_per_step * 1e-3) / 1e9;

        // Effective memory bandwidth: 7 reads + 1 write per cell per step.
        // (naive) or ~4.1 reads + 1 write for tiled (accounting for halo amortisation).
        const double rw_bytes = (double)ncells *
            (kname[0] == 'n' ? 8.0 : 5.1) * sizeof(float);
        const double eff_bw_gbs = rw_bytes / (ms_per_step * 1e-3) / 1e9;

        const double speedup_vs_cpu = cpu_us_per_step / (ms_per_step * 1e3);

        printf("| %3dx%3dx%3d | %-5s | %d | %8.2f | %7.3f | %7.2f | %6.1fx |\n",
            nx, ny, nz,
            kname,
            bench_steps,
            ms_per_step,
            gcells_per_s,
            eff_bw_gbs,
            speedup_vs_cpu);
    }

    CUDA_CHECK(cudaFree(d_next));
    CUDA_CHECK(cudaFree(d_cur));
    CUDA_CHECK(cudaFree(d_prev));

    // Also print Δt and CFL for documentation.
    printf("|   (h=%.4fm, Δt=%.2eµs, λ=%.4f, %lld MB fields)  |\n",
        h, dt*1e6, (SPEED_OF_SOUND*dt/h)*(SPEED_OF_SOUND*dt/h),
        3*nbytes/(1024*1024));
}

// ------------------------------------------------------------------
// main
// ------------------------------------------------------------------
int main()
{
    // Print device info.
    cudaDeviceProp prop;
    CUDA_CHECK(cudaGetDeviceProperties(&prop, 0));
    printf("Device: %s  (sm_%d%d, %.1f GB, %d SMs)\n",
        prop.name,
        prop.major, prop.minor,
        prop.totalGlobalMem / 1e9,
        prop.multiProcessorCount);
    printf("Peak bandwidth: %.1f GB/s\n",
        2.0 * prop.memoryClockRate * 1e3 * (prop.memoryBusWidth / 8) / 1e9);
    printf("L2 cache: %d MB\n\n", prop.l2CacheSize / (1024*1024));

    printf("CPU single-core baseline (i7-1255U, from 14.1):\n");
    printf("  43x43x43 grid (59K active nodes): 412 µs/step  →  0.144 Gcells/s\n\n");

    printf("| Grid        | Kernel | Steps | ms/step  | Gcells/s | EffBW(GB/s) | vs CPU |\n");
    printf("|-------------|--------|-------|----------|----------|-------------|--------|\n");

    bench_grid(128, 128, 128);
    bench_grid(256, 256, 256);
    bench_grid(512, 512, 512);

    printf("\nNote: 1024³ requires ~12 GB VRAM (exceeds T550's 4 GB — omitted).\n");
    printf("EffBW for naive assumes 7R+1W per cell; for tiled assumes ~4.1R+1W (halo amortised).\n");
    printf("CPU comparison uses single-core 43³ baseline scaled linearly by cell count.\n");

    return 0;
}
