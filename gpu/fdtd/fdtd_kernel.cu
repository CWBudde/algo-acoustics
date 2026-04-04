// fdtd_kernel.cu — CUDA implementations of the 3-D acoustic FDTD stencil.
// See fdtd_kernel.cuh for the physics and interface documentation.
//
// Memory layout: row-major [NX][NY][NZ], flat index = ix*ny*nz + iy*nz + iz.
// The fastest-varying axis in memory is IZ.
//
// Coalescing rule: consecutive threads in a warp must access consecutive
// flat indices → threadIdx.x must map to IZ (not IX as one might naively
// write).  Mapping threadIdx.x → IX gives stride ny*nz between adjacent
// threads — completely uncoalesced and ~30× slower.

#include "fdtd_kernel.cuh"
#include <cstdio>

// ======================================================================
// Naive kernel — 1-D flat decomposition, fully coalesced along IZ.
//
// Each thread is assigned one grid point by flat index.
// Consecutive threads differ by 1 in flat index → differ by 1 in IZ
// → adjacent-thread accesses to pCur/pPrev/pNext are coalesced.
//
// ±Y neighbours (stride nz) and ±X neighbours (stride ny·nz) rely on
// L1 cache; for grids where one IY slice fits in L1 (≤32 KB → nz ≤ 8K
// for float32) these are typically cache hits.
// ======================================================================
__global__ void fdtd_step_naive(
    float* __restrict__ pNext,
    const float* __restrict__ pCur,
    const float* __restrict__ pPrev,
    int nx, int ny, int nz,
    float lambda)
{
    const long long flat = (long long)blockIdx.x * blockDim.x + threadIdx.x;
    const long long total = (long long)nx * ny * nz;
    if (flat >= total) return;

    // Decompose flat index back to (ix, iy, iz).
    const int iz = (int)(flat % nz);
    const int iy = (int)(flat / nz % ny);
    const int ix = (int)(flat / ((long long)ny * nz));

    // Skip boundary nodes (hardwall Dirichlet: p=0 at faces).
    if (ix == 0 || ix >= nx - 1 ||
        iy == 0 || iy >= ny - 1 ||
        iz == 0 || iz >= nz - 1)
        return;

    const long long stride_x = (long long)ny * nz;
    const long long stride_y = nz;

    const float cur = pCur[flat];
    const float lap =
        pCur[flat + stride_x] + pCur[flat - stride_x] +   // ±X
        pCur[flat + stride_y] + pCur[flat - stride_y] +   // ±Y
        pCur[flat + 1]        + pCur[flat - 1]        -   // ±Z (coalesced)
        6.0f * cur;

    pNext[flat] = 2.0f * cur - pPrev[flat] + lambda * lap;
}

// ======================================================================
// Tiled kernel — IY-IZ slab in shared memory, coalesced along IZ.
//
// Thread block: (TILE_IZ, TILE_IY, 1).  threadIdx.x → IZ, threadIdx.y → IY.
// Each block covers a TILE_IZ × TILE_IY patch of a single IX slice.
// The ±IY neighbours are loaded once into shared memory so warps do not
// repeatedly fetch them from global memory.
// ±IX neighbours are fetched from global memory (stride ny·nz); they
// benefit from L1 reuse across threadblocks that process adjacent IX slices.
//
// Shared memory per block: (TILE_IZ+2) × (TILE_IY+2) × 4 B ≈ 1.4 KB.
// ======================================================================
__global__ void fdtd_step_tiled(
    float* __restrict__ pNext,
    const float* __restrict__ pCur,
    const float* __restrict__ pPrev,
    int nx, int ny, int nz,
    float lambda)
{
    // +2 for 1-cell halo on each side.
    __shared__ float s_cur[TILE_IY + 2][TILE_IZ + 2];

    const int tx = threadIdx.x;  // IZ local  (0 .. TILE_IZ-1)
    const int ty = threadIdx.y;  // IY local  (0 .. TILE_IY-1)

    const int iz = blockIdx.x * TILE_IZ + tx;
    const int iy = blockIdx.y * TILE_IY + ty;
    const int ix = blockIdx.z;          // one block per IX slice

    const int sx = tx + 1;  // shared mem coords (offset by 1 for halo)
    const int sy = ty + 1;

    // ------------------------------------------------------------------
    // Load tile interior.
    // ------------------------------------------------------------------
    const bool valid = (ix < nx && iy < ny && iz < nz);
    s_cur[sy][sx] = valid ? pCur[ix * (ny * nz) + iy * nz + iz] : 0.0f;

    // ------------------------------------------------------------------
    // Load IY halos (top and bottom rows of the slab).
    // ------------------------------------------------------------------
    if (ty == 0) {
        const int iy_b = iy - 1;
        s_cur[0][sx] = (iy_b >= 0 && ix < nx && iz < nz)
                       ? pCur[ix * (ny * nz) + iy_b * nz + iz] : 0.0f;
    }
    if (ty == TILE_IY - 1) {
        const int iy_t = iy + 1;
        s_cur[TILE_IY + 1][sx] = (iy_t < ny && ix < nx && iz < nz)
                                  ? pCur[ix * (ny * nz) + iy_t * nz + iz] : 0.0f;
    }

    // ------------------------------------------------------------------
    // Load IZ halos (left and right columns of the slab).
    // ------------------------------------------------------------------
    if (tx == 0) {
        const int iz_l = iz - 1;
        s_cur[sy][0] = (iz_l >= 0 && ix < nx && iy < ny)
                       ? pCur[ix * (ny * nz) + iy * nz + iz_l] : 0.0f;
    }
    if (tx == TILE_IZ - 1) {
        const int iz_r = iz + 1;
        s_cur[sy][TILE_IZ + 1] = (iz_r < nz && ix < nx && iy < ny)
                                   ? pCur[ix * (ny * nz) + iy * nz + iz_r] : 0.0f;
    }

    __syncthreads();

    // ------------------------------------------------------------------
    // Compute stencil for interior points only.
    // ------------------------------------------------------------------
    if (!valid || ix == 0 || ix >= nx - 1 ||
        iy == 0 || iy >= ny - 1 ||
        iz == 0 || iz >= nz - 1)
        return;

    const long long flat = (long long)ix * ny * nz + iy * nz + iz;

    // ±IY and ±IZ from shared memory; ±IX from global (L1-cached).
    const float cur = s_cur[sy][sx];
    const float lap =
        pCur[flat + (long long)ny * nz] + pCur[flat - (long long)ny * nz] +  // ±X global
        s_cur[sy + 1][sx] + s_cur[sy - 1][sx] +   // ±Y shared
        s_cur[sy][sx + 1] + s_cur[sy][sx - 1] -   // ±Z shared (coalesced)
        6.0f * cur;

    pNext[flat] = 2.0f * cur - pPrev[flat] + lambda * lap;
}

// ======================================================================
// Host launchers
// ======================================================================
void launch_fdtd_naive(
    float* pNext, const float* pCur, const float* pPrev,
    int nx, int ny, int nz, float lambda, cudaStream_t stream)
{
    // 1-D flat decomposition; threads cover flat index space.
    const long long total = (long long)nx * ny * nz;
    const int block = 256;
    const int grid  = (int)((total + block - 1) / block);
    fdtd_step_naive<<<grid, block, 0, stream>>>(pNext, pCur, pPrev, nx, ny, nz, lambda);
}

void launch_fdtd_tiled(
    float* pNext, const float* pCur, const float* pPrev,
    int nx, int ny, int nz, float lambda, cudaStream_t stream)
{
    // blockIdx.x → IZ tile, blockIdx.y → IY tile, blockIdx.z → IX slice.
    dim3 block(TILE_IZ, TILE_IY, 1);
    dim3 grid(
        (nz + TILE_IZ - 1) / TILE_IZ,
        (ny + TILE_IY - 1) / TILE_IY,
        nx);
    fdtd_step_tiled<<<grid, block, 0, stream>>>(pNext, pCur, pPrev, nx, ny, nz, lambda);
}
