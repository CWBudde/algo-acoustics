# GPU Acceleration — Requirements & Deployment

## Overview

algo-acoustics optionally offloads FDTD wave simulation and BVH ray tracing to an NVIDIA GPU via a standalone CUDA server binary (`algo-acoustics-gpu`). The Go codebase remains CUDA-free; GPU work is delegated over a Unix socket with bulk data transferred through POSIX shared memory (`/dev/shm`).

GPU acceleration is entirely optional. When the GPU server binary is absent or the GPU is unavailable, the system falls back to CPU-only execution automatically.

## Hardware Requirements

- NVIDIA GPU with compute capability ≥ sm_75 (Turing or newer: RTX 20xx, Quadro T-series, and later)
- Minimum 2 GB VRAM for typical simulations (128³ grid ≈ 24 MB fields)
- 4+ GB VRAM recommended for production (256³ grid ≈ 192 MB fields)

## Software Requirements

- NVIDIA driver ≥ 450 (CUDA 11.0+)
- CUDA Toolkit 12.x (for building; `nvcc` must be on PATH)
- Linux x86_64 (the shared memory transport uses `/dev/shm`)
- C++17 compiler (bundled with CUDA Toolkit)

## Building

```bash
# Build the GPU server binary:
just build-gpu

# Or directly with make:
make -C gpu/server
```

This produces `gpu/server/algo-acoustics-gpu`. Optionally install to `bin/`:

```bash
make -C gpu/server install   # copies to bin/algo-acoustics-gpu
```

## Running

### Integration Tests

```bash
just test-gpu
```

### Benchmarks

```bash
just bench-gpu
```

### Manual Usage

```bash
# Set the server binary path and run GPU-enabled tests:
ALGO_GPU_SERVER=./gpu/server/algo-acoustics-gpu go test -v ./gpu/worker/
```

## Architecture

```
Go process                           CUDA server (subprocess)
│                                    │
├─ worker.StartIfAvailable()         │
│  └─ exec: algo-acoustics-gpu      │
│     --socket /tmp/algo_gpu_N.sock  │
│                                    ├─ cudaGetDeviceCount()
│  ←─── Unix socket connection ───→  ├─ listen + accept
│                                    │
├─ AllocGrid / AllocBVH              │ cudaMalloc
├─ UploadFields (via /dev/shm)       │ cudaMemcpyAsync (pinned → device)
├─ RunFDTD / TraceRays               │ kernel launch on CUDA streams
├─ Results (via /dev/shm)            │ cudaMemcpyAsync (device → pinned)
├─ FreeGrid / FreeBVH                │ cudaFree
└─ Close (MSG_SHUTDOWN)              └─ exit
```

### Key Design Choices

- **Subprocess model**: No cgo dependency; `go build ./...` works without CUDA Toolkit
- **CUDA streams**: Two streams per grid for overlapping compute and transfer
- **Pinned memory**: `cudaMallocHost` staging buffers for 2-3x transfer bandwidth
- **Graceful fallback**: `worker.StartIfAvailable()` returns `ErrGPUUnavailable` when GPU is absent; `worker.IsGPUError()` detects runtime GPU failures (OOM, kernel crash)

## CI/CD

GPU integration tests auto-skip when `ALGO_GPU_SERVER` is not set, so the standard CI pipeline (`go test ./...`) works on machines without a GPU or CUDA Toolkit. GPU-specific testing requires a runner with an NVIDIA GPU and the CUDA Toolkit installed.

To add GPU CI on GitHub Actions, use a self-hosted runner with GPU access:

```yaml
gpu-tests:
  runs-on: [self-hosted, gpu]
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: "1.25.x"
    - run: just build-gpu
    - run: just test-gpu
```

## Troubleshooting

| Symptom                                  | Cause                                                  | Fix                                                |
| ---------------------------------------- | ------------------------------------------------------ | -------------------------------------------------- |
| `ErrGPUUnavailable: binary not found`    | `algo-acoustics-gpu` not in PATH or standard locations | Run `just build-gpu` or set `ALGO_GPU_SERVER`      |
| `ErrGPUUnavailable: server start failed` | No NVIDIA GPU or driver not loaded                     | Check `nvidia-smi`; ensure driver ≥ 450            |
| `GPU server error: GPU out of memory`    | Grid too large for available VRAM                      | Reduce grid resolution or use a GPU with more VRAM |
| `GPU server error: CUDA error`           | Kernel launch failure or driver issue                  | Check server stderr for detailed CUDA error        |
