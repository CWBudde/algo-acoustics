// fdtd_kernel.cuh — 3-D acoustic FDTD stencil kernels
//
// Discretisation of the scalar wave equation:
//
//   ∂²p/∂t² = c² ∇²p
//
// Leapfrog time integration (2nd-order accurate in time and space):
//
//   p_new = 2·p_cur - p_old + λ · L₇(p_cur)
//
// where  λ = (c·Δt/h)²  (Courant number squared)
// and    L₇ is the 7-point Laplacian:
//
//   L₇(p)[i,j,k] = p[i+1,j,k] + p[i-1,j,k]
//                + p[i,j+1,k] + p[i,j-1,k]
//                + p[i,j,k+1] + p[i,j,k-1]
//                - 6·p[i,j,k]
//
// Array layout: row-major, C order → flat index = ix·Ny·Nz + iy·Nz + iz
//
// Two kernel variants are provided for comparison:
//   fdtd_step_naive — no shared memory; relies on L1/L2 cache
//   fdtd_step_tiled — XY-plane slab loaded into shared memory per thread-block

#pragma once

#include <cuda_runtime.h>

// ------------------------------------------------------------------
// Thread-block tile dimensions for the tiled kernel.
// TILE_IZ = warp size (32) → coalesced reads along the fastest axis (IZ).
// TILE_IY tuned for occupancy on sm_75 (256 threads/block).
// ------------------------------------------------------------------
constexpr int TILE_IZ = 32;
constexpr int TILE_IY = 8;

// ------------------------------------------------------------------
// Error-checking helper.
// ------------------------------------------------------------------
#define CUDA_CHECK(call)                                                  \
    do {                                                                  \
        cudaError_t _e = (call);                                          \
        if (_e != cudaSuccess) {                                          \
            fprintf(stderr, "CUDA error %s:%d  %s\n",                    \
                    __FILE__, __LINE__, cudaGetErrorString(_e));          \
            exit(EXIT_FAILURE);                                           \
        }                                                                 \
    } while (0)

// ------------------------------------------------------------------
// Kernel declarations
// ------------------------------------------------------------------

// Naive kernel: one thread per grid point, all accesses from global memory.
// Baseline to measure the effect of shared-memory tiling.
__global__ void fdtd_step_naive(
    float* __restrict__ pNext,
    const float* __restrict__ pCur,
    const float* __restrict__ pPrev,
    int nx, int ny, int nz,
    float lambda);   // λ = (c·Δt/h)²

// Tiled kernel: thread-block loads a (TILE_X+2)×(TILE_Y+2) XY slab into
// shared memory to eliminate redundant X and Y neighbour loads.
// Z neighbours are fetched from global memory (typically still in L1).
__global__ void fdtd_step_tiled(
    float* __restrict__ pNext,
    const float* __restrict__ pCur,
    const float* __restrict__ pPrev,
    int nx, int ny, int nz,
    float lambda);

// ------------------------------------------------------------------
// Host-side launcher that selects the appropriate grid/block geometry.
// ------------------------------------------------------------------
void launch_fdtd_naive(
    float* pNext, const float* pCur, const float* pPrev,
    int nx, int ny, int nz, float lambda, cudaStream_t stream = 0);

void launch_fdtd_tiled(
    float* pNext, const float* pCur, const float* pPrev,
    int nx, int ny, int nz, float lambda, cudaStream_t stream = 0);
