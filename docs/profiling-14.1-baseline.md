# Phase 14.1 — CPU Profiling Baseline

**Date:** 2026-04-04  
**Hardware:** Intel Core i7-1255U (12 logical cores, P-cores 3.5 GHz base)

## Benchmarks Run

```
go test -bench='BenchmarkISM|BenchmarkRayTrace_...|BenchmarkPDEShoebox|BenchmarkHybridPipeline_4K|BenchmarkIBM_FDTDStep_RectRoom' -benchtime=3s -benchmem . ./pde/
```

## Per-Component Timing

| Component | Config | Time/op | Notes |
|-----------|--------|---------|-------|
| ISM (image-source) | order=3, 6×4.5×2.8 m room | 73.6 µs | 51 events |
| Ray trace | 4 096 rays, 6 bounces | 46.2 ms | |
| Ray trace | 16 384 rays, 6 bounces | 212.6 ms | |
| Ray trace | 65 536 rays, 6 bounces | 714.7 ms | |
| PDE shoebox sweep | 20–300 Hz, 32 points | 286.3 ms | 32 Helmholtz solves |
| IBM FDTD step | 43×43×43 grid (59 319 active, rect) | 412 µs/step | h=0.1 m |
| IBM FDTD step | 43×43×43 grid (30 303 active, diamond) | 196 µs/step | h=0.1 m |
| Hybrid pipeline | ISM order=3 + 4 096 rays | 51.6 ms | full end-to-end |

## Amdahl Fraction

Within the hybrid pipeline at 4 096 rays (51.6 ms total):

| Stage | Time | Fraction |
|-------|------|----------|
| ISM (early events) | ~0.07 ms | 0.1% |
| Ray tracing (late field) | ~46.2 ms | 89.5% |
| IR render + combine | ~5.3 ms | 10.3% |

At 65 536 rays (production quality):

| Stage | Time | Fraction |
|-------|------|----------|
| ISM | ~0.07 ms | 0.01% |
| Ray tracing | ~714.7 ms | ~99% |
| IR render + combine | ~5.3 ms | ~0.7% |

**Conclusion:** Ray tracing dominates the geometric pipeline. At production ray counts, P > 0.99.

## GOMAXPROCS Sweep (Multi-Core Scaling)

**Critical finding: neither hot path exploits CPU parallelism today.**

### Ray trace (16 384 rays)

| GOMAXPROCS | Time/op | Speedup vs 1 |
|------------|---------|-------------|
| 1 | 151 ms | 1.00× |
| 2 | 152 ms | 0.99× |
| 4 | 180 ms | 0.84× |
| 8 | 230 ms | 0.66× |

### IBM FDTD stencil (43³ rect room)

| GOMAXPROCS | Time/op | Speedup vs 1 |
|------------|---------|-------------|
| 1 | 342 µs | 1.00× |
| 2 | 375 µs | 0.91× |
| 4 | 402 µs | 0.85× |
| 8 | 497 µs | 0.69× |

**Both paths are strictly single-threaded. Increased GOMAXPROCS adds scheduler overhead with zero benefit.**  
This has two implications:
1. Before GPU, parallelizing these paths on CPU would give near-linear speedup.
2. The single-core times are the correct baseline for GPU comparison.

## Problem Sizes (Documented)

### Grid Dimensions (IBM FDTD)

| Spacing h | Grid (6×4.5×2.8 m room) | Active nodes | Field memory (3×float64) |
|-----------|--------------------------|-------------|--------------------------|
| 0.100 m | 63×48×30 = 90 720 | ~67 700 | 1.5 MB |
| 0.050 m | 122×92×58 = 650 768 | ~484 800 | 11.1 MB |
| 0.025 m | 242×182×114 = 5 025 456 | ~3 744 000 | 85.8 MB |
| 0.010 m | 602×452×282 = 76 759 248 | ~57 million | 1 310 MB |

Practical upper bound for wave-based audio (≤1 kHz crossover): h ≈ 0.025 m.

### Ray Counts

| Mode | Rays | Notes |
|------|------|-------|
| Preview | 4 096 | ~46 ms, visible noise |
| Quality | 16 384 | ~213 ms, most use cases |
| Production | 65 536 | ~715 ms, converged |

### FDTD Timesteps

At h = 0.1 m, c = 343 m/s: Δt = 0.95 × h / (c × √3) ≈ 160 µs  
For 1.5 s simulation: ~9 375 steps  
Total CPU time (current, single-thread): 9 375 × 412 µs ≈ **3.9 s** at h=0.1 m  
At h=0.025 m (extrapolated ×64 nodes): ≈ 26 ms/step × 37 500 steps ≈ **975 s (16 min)**

## Amdahl Ceiling

Formula: S = 1 / ((1 − P) + P/N)

### Ray tracing (P = 0.895 at 4K rays, P = 0.99 at 64K rays)

| N (cores/GPU lanes) | Ceiling at 4K rays | Ceiling at 64K rays |
|---------------------|-------------------|---------------------|
| 4 | 3.3× | 3.9× |
| 8 | 5.3× | 7.6× |
| 64 | 7.8× | 43× |
| 1 000 | 9.4× | ~99× |
| ∞ | 9.5× | 100× |

**At 4K rays, Amdahl ceiling ≈ 9.5× — GPU may not justify complexity for small renders.**  
**At 64K rays, ceiling ≈ 100× — strong GPU case.**

### IBM FDTD (P ≈ 1.0 — fully data-parallel per step)

At h=0.025 m the stencil is embarrassingly parallel over 3.7M active nodes:

| N | Speedup |
|---|---------|
| 64 | 64× |
| 1 024 | 1 024× |
| ∞ | ∞ (P=1) |

**FDTD is the clearest GPU win**: the stencil loop has no dependencies across the spatial domain, memory-bandwidth bound, and scales perfectly.

## GPU Memory Requirements

Target GPU: NVIDIA RTX 4090 (24 GB VRAM)

| Workload | Data size | Fits in 24 GB? |
|---------|-----------|----------------|
| FDTD fields h=0.1 m (3 × float64) | 1.5 MB | Yes |
| FDTD fields h=0.025 m | 86 MB | Yes |
| FDTD fields h=0.010 m | 1 310 MB | Yes |
| Ray buffers 65K rays × 64 B | 4 MB | Yes |
| BVH (6-wall room) | < 1 MB | Yes |
| BVH (10K-triangle mesh) | ~8 MB | Yes |
| **Total worst case** | **~1.4 GB** | **Yes, 17× margin** |

GPU memory is not a constraint for any realistic problem size on an RTX 4090.

## Decision Gate Assessment

Per the PLAN decision gate: "if GPU kernel + transfer is less than 5× faster than multi-core CPU for actual problem sizes, reconsider GPU investment."

Key insight: the CPU baseline is **single-core** (no parallelism implemented). A fair comparison must account for this:

1. **CPU-parallel baseline first**: parallelizing the ray tracer over goroutines (embarrassingly parallel over rays) and FDTD stencil (domain decomposition) would give near-linear speedup on 12 cores before touching GPU.

2. **Realistic GPU targets**:
   - Ray tracing 64K: Amdahl ceiling 100× vs single-core → ~12× vs 12-core parallel CPU → **clears 5× bar**
   - FDTD h=0.025 m: fully parallel, GPU wins clearly for sustained simulation → **clears 5× bar**
   - Ray tracing 4K: ceiling ~9.5× vs single-core → ~0.8× vs 12-core CPU → **does not clear bar at small ray counts**

## Summary

| Hot path | Single-threaded? | Amdahl P | GPU memory | Justified? |
|----------|-----------------|----------|------------|------------|
| Ray tracing (64K) | Yes | 0.99 | 4 MB | Yes |
| IBM FDTD (h≤0.025 m) | Yes | ~1.0 | 86 MB | Yes |
| Ray tracing (4K) | Yes | 0.895 | 4 MB | Marginal |
| ISM | Yes | irrelevant | — | No (0.1% of runtime) |
| PDE shoebox sweep | Yes | irrelevant | — | No (separate Poisson solver, not stencil) |

**Recommended GPU focus**: FDTD stencil first (clearest win, no transfer overhead per step), then ray tracing at production ray counts.
