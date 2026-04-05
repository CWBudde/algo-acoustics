// server.cu — GPU server: accepts one Unix socket connection and dispatches
// incoming requests to CUDA kernels.
//
// Usage:
//   algo-acoustics-gpu --socket /tmp/algo_gpu_NNNN.sock
//
// The server processes one request at a time (serialised by the Go worker's
// channel).  Multiple connections are not supported; the server exits after
// the connection is closed or a shutdown message is received.
//
// Phases implemented here:
//   AllocGrid / FreeGrid / UploadFields / RunFDTD  → fully implemented (14.4)
//   AllocBVH  / FreeBVH  / TraceRays              → STATUS_NOT_IMPL (14.5)

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <cerrno>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <fcntl.h>

#include "protocol.h"
#include "../fdtd/fdtd_kernel.cuh"
#include "../raytrace/bvh_kernel.h"

// -----------------------------------------------------------------------
// Shared-memory helpers (Linux /dev/shm implementation)
// -----------------------------------------------------------------------

static const char SHM_DIR[] = "/dev/shm/";

static void *shm_open_read(const char *name, size_t size) {
    char path[256];
    snprintf(path, sizeof(path), "%s%s", SHM_DIR, name);
    int fd = open(path, O_RDONLY);
    if (fd < 0) { perror(path); return nullptr; }
    void *p = mmap(nullptr, size, PROT_READ, MAP_SHARED, fd, 0);
    close(fd);
    if (p == MAP_FAILED) { perror("mmap read"); return nullptr; }
    return p;
}

static void *shm_open_write(const char *name, size_t size) {
    char path[256];
    snprintf(path, sizeof(path), "%s%s", SHM_DIR, name);
    int fd = open(path, O_RDWR);
    if (fd < 0) { perror(path); return nullptr; }
    void *p = mmap(nullptr, size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
    close(fd);
    if (p == MAP_FAILED) { perror("mmap write"); return nullptr; }
    return p;
}

static void shm_close(void *p, size_t size) {
    if (p && p != MAP_FAILED) munmap(p, size);
}

// -----------------------------------------------------------------------
// Socket I/O helpers
// -----------------------------------------------------------------------

static bool read_exact(int fd, void *buf, size_t n) {
    uint8_t *p = (uint8_t *)buf;
    size_t remaining = n;
    while (remaining > 0) {
        ssize_t r = read(fd, p, remaining);
        if (r <= 0) return false;
        p += r;
        remaining -= r;
    }
    return true;
}

static bool write_exact(int fd, const void *buf, size_t n) {
    const uint8_t *p = (const uint8_t *)buf;
    size_t remaining = n;
    while (remaining > 0) {
        ssize_t w = write(fd, p, remaining);
        if (w <= 0) return false;
        p += w;
        remaining -= w;
    }
    return true;
}

static bool send_response(int fd, uint32_t status,
                           const void *data = nullptr, uint32_t data_len = 0) {
    resp_hdr_t h = { status, data_len };
    if (!write_exact(fd, &h, sizeof(h))) return false;
    if (data && data_len > 0)
        if (!write_exact(fd, data, data_len)) return false;
    return true;
}

// -----------------------------------------------------------------------
// GridState — persistent FDTD pressure fields in GPU VRAM.
//
// Three device float arrays of length nx·ny·nz:
//   pCur  — pressure at time t
//   pPrev — pressure at time t-Δt
//   pNext — scratch buffer (written by the kernel, then rotated)
//
// The handle returned to the Go client is the pointer cast to uint64_t.
// The server is single-threaded so no lock is needed.
// -----------------------------------------------------------------------

struct GridState {
    float *pCur;
    float *pPrev;
    float *pNext;
    int    nx, ny, nz;
};

// -----------------------------------------------------------------------
// extract_sample — CUDA kernel: copy one element from a device array into
// a pre-allocated device result buffer at position [step].
// Launched as <<<1,1>>> in the default stream after each fdtd_step kernel.
// -----------------------------------------------------------------------

__global__ void extract_sample(const float *field, float *results,
                                int rcv_idx, int step) {
    results[step] = field[rcv_idx];
}

// -----------------------------------------------------------------------
// FDTD handlers (Phase 14.4)
// -----------------------------------------------------------------------

static void handle_alloc_grid(int conn, const uint8_t *payload, uint32_t len) {
    if (len < sizeof(alloc_grid_req_t)) {
        send_response(conn, STATUS_BAD_MSG);
        return;
    }
    alloc_grid_req_t req;
    memcpy(&req, payload, sizeof(req));

    const long long N  = (long long)req.nx * req.ny * req.nz;
    const size_t    sz = (size_t)N * sizeof(float);

    auto *g = new GridState{};
    g->nx = (int)req.nx;
    g->ny = (int)req.ny;
    g->nz = (int)req.nz;

    if (cudaMalloc(&g->pCur,  sz) != cudaSuccess ||
        cudaMalloc(&g->pPrev, sz) != cudaSuccess ||
        cudaMalloc(&g->pNext, sz) != cudaSuccess) {
        cudaFree(g->pCur);
        cudaFree(g->pPrev);
        cudaFree(g->pNext);
        delete g;
        send_response(conn, STATUS_OOM);
        return;
    }
    cudaMemset(g->pCur,  0, sz);
    cudaMemset(g->pPrev, 0, sz);
    cudaMemset(g->pNext, 0, sz);

    fprintf(stderr, "[gpu-server] AllocGrid %dx%dx%d  handle=0x%llx\n",
            req.nx, req.ny, req.nz, (unsigned long long)(uintptr_t)g);

    alloc_grid_resp_t resp = { (uint64_t)(uintptr_t)g };
    send_response(conn, STATUS_OK, &resp, sizeof(resp));
}

static void handle_free_grid(int conn, const uint8_t *payload, uint32_t len) {
    if (len < sizeof(free_grid_req_t)) {
        send_response(conn, STATUS_BAD_MSG);
        return;
    }
    free_grid_req_t req;
    memcpy(&req, payload, sizeof(req));
    auto *g = reinterpret_cast<GridState *>((uintptr_t)req.handle);
    cudaFree(g->pCur);
    cudaFree(g->pPrev);
    cudaFree(g->pNext);
    delete g;
    send_response(conn, STATUS_OK);
}

static void handle_upload_fields(int conn, const uint8_t *payload, uint32_t len) {
    if (len < sizeof(upload_fields_req_t)) {
        send_response(conn, STATUS_BAD_MSG);
        return;
    }
    upload_fields_req_t req;
    memcpy(&req, payload, sizeof(req));
    auto *g = reinterpret_cast<GridState *>((uintptr_t)req.handle);

    const long long N  = (long long)g->nx * g->ny * g->nz;
    const size_t    sz = (size_t)N * sizeof(float);

    // Shared memory layout: [pCur (N floats) | pPrev (N floats)]
    void *p = shm_open_read(req.shm_name.s, 2 * sz);
    if (!p) {
        send_response(conn, STATUS_BAD_MSG);
        return;
    }

    cudaError_t e1 = cudaMemcpy(g->pCur,  p,                     sz, cudaMemcpyHostToDevice);
    cudaError_t e2 = cudaMemcpy(g->pPrev, (const char *)p + sz,  sz, cudaMemcpyHostToDevice);
    shm_close(p, 2 * sz);

    if (e1 != cudaSuccess || e2 != cudaSuccess) {
        fprintf(stderr, "[gpu-server] UploadFields cudaMemcpy: %s / %s\n",
                cudaGetErrorString(e1), cudaGetErrorString(e2));
        send_response(conn, STATUS_CUDA);
        return;
    }
    send_response(conn, STATUS_OK);
}

static void handle_run_fdtd(int conn, const uint8_t *payload, uint32_t len) {
    if (len < sizeof(run_fdtd_req_t)) {
        send_response(conn, STATUS_BAD_MSG);
        return;
    }
    run_fdtd_req_t req;
    memcpy(&req, payload, sizeof(req));
    auto *g = reinterpret_cast<GridState *>((uintptr_t)req.handle);

    // λ = (c · Δt / h)²  (Courant number squared)
    const float cfl    = req.speed_of_sound * req.dt / req.ds;
    const float lambda = cfl * cfl;

    fprintf(stderr, "[gpu-server] RunFDTD %u steps  λ=%.4f  src=%u  rcv=%u\n",
            req.steps, lambda, req.src_idx, req.rcv_idx);

    // Allocate device buffer for the receiver time series.
    float *d_results = nullptr;
    if (cudaMalloc(&d_results, req.steps * sizeof(float)) != cudaSuccess) {
        send_response(conn, STATUS_OOM);
        return;
    }
    cudaMemset(d_results, 0, req.steps * sizeof(float));

    // Time-march loop.  All kernels run in the default stream (stream 0)
    // and therefore execute in issue order — no explicit sync between steps.
    for (uint32_t step = 0; step < req.steps; step++) {
        launch_fdtd_naive(g->pNext, g->pCur, g->pPrev,
                          g->nx, g->ny, g->nz, lambda, /*stream=*/0);

        // Record receiver value: pNext[rcv_idx] → d_results[step].
        extract_sample<<<1, 1>>>(g->pNext, d_results, (int)req.rcv_idx, (int)step);

        // Rotate buffers (pointer swap, no device-side copy):
        //   pPrev ← pCur,  pCur ← pNext,  pNext ← old pPrev
        float *tmp = g->pPrev;
        g->pPrev   = g->pCur;
        g->pCur    = g->pNext;
        g->pNext   = tmp;
    }
    cudaDeviceSynchronize();

    // Write results to the pre-created shared memory segment.
    const size_t result_sz = req.steps * sizeof(float);
    void *p = shm_open_write(req.result_shm.s, result_sz);
    if (!p) {
        cudaFree(d_results);
        send_response(conn, STATUS_BAD_MSG);
        return;
    }
    cudaMemcpy(p, d_results, result_sz, cudaMemcpyDeviceToHost);
    shm_close(p, result_sz);
    cudaFree(d_results);

    send_response(conn, STATUS_OK);
}

// -----------------------------------------------------------------------
// BVHState — persistent BVH + triangle mesh in GPU VRAM (Phase 14.5).
//
// AllocBVH uploads nodes[] and tris[] once; they stay in VRAM across
// multiple TraceRays calls.  Handle = BVHState* cast to uint64_t.
// -----------------------------------------------------------------------

struct BVHState {
    BVHNode  *d_nodes;
    Triangle *d_tris;
    uint32_t  node_count;
    uint32_t  tri_count;
};

// -----------------------------------------------------------------------
// BVH / ray-tracing handlers (Phase 14.5)
// -----------------------------------------------------------------------

static void handle_alloc_bvh(int conn, const uint8_t *payload, uint32_t len) {
    if (len < sizeof(alloc_bvh_req_t)) {
        send_response(conn, STATUS_BAD_MSG);
        return;
    }
    alloc_bvh_req_t req;
    memcpy(&req, payload, sizeof(req));

    const size_t node_sz = (size_t)req.node_count * sizeof(BVHNode);
    const size_t tri_sz  = (size_t)req.tri_count  * sizeof(Triangle);

    // Shared memory layout: [nodes (node_count × BVHNode) | tris (tri_count × Triangle)]
    void *p = shm_open_read(req.shm_name.s, node_sz + tri_sz);
    if (!p) {
        send_response(conn, STATUS_BAD_MSG);
        return;
    }

    auto *b = new BVHState{};
    b->node_count = req.node_count;
    b->tri_count  = req.tri_count;

    bool ok = (cudaMalloc(&b->d_nodes, node_sz) == cudaSuccess &&
               cudaMalloc(&b->d_tris,  tri_sz)  == cudaSuccess);
    if (ok) {
        ok = (cudaMemcpy(b->d_nodes, p,                      node_sz, cudaMemcpyHostToDevice) == cudaSuccess &&
              cudaMemcpy(b->d_tris,  (const char *)p + node_sz, tri_sz,  cudaMemcpyHostToDevice) == cudaSuccess);
    }
    shm_close(p, node_sz + tri_sz);

    if (!ok) {
        cudaFree(b->d_nodes);
        cudaFree(b->d_tris);
        delete b;
        send_response(conn, STATUS_OOM);
        return;
    }

    fprintf(stderr, "[gpu-server] AllocBVH %u nodes %u tris  handle=0x%llx\n",
            req.node_count, req.tri_count, (unsigned long long)(uintptr_t)b);

    alloc_bvh_resp_t resp = { (uint64_t)(uintptr_t)b };
    send_response(conn, STATUS_OK, &resp, sizeof(resp));
}

static void handle_free_bvh(int conn, const uint8_t *payload, uint32_t len) {
    if (len < sizeof(free_bvh_req_t)) {
        send_response(conn, STATUS_BAD_MSG);
        return;
    }
    free_bvh_req_t req;
    memcpy(&req, payload, sizeof(req));
    auto *b = reinterpret_cast<BVHState *>((uintptr_t)req.handle);
    cudaFree(b->d_nodes);
    cudaFree(b->d_tris);
    delete b;
    send_response(conn, STATUS_OK);
}

static void handle_trace_rays(int conn, const uint8_t *payload, uint32_t len) {
    if (len < sizeof(trace_rays_req_t)) {
        send_response(conn, STATUS_BAD_MSG);
        return;
    }
    trace_rays_req_t req;
    memcpy(&req, payload, sizeof(req));
    auto *b = reinterpret_cast<BVHState *>((uintptr_t)req.handle);

    const size_t ray_sz = (size_t)req.ray_count * sizeof(Ray);
    const size_t hit_sz = (size_t)req.ray_count * sizeof(HitRecord);

    // Upload rays from shared memory.
    void *rays_p = shm_open_read(req.rays_shm.s, ray_sz);
    if (!rays_p) {
        send_response(conn, STATUS_BAD_MSG);
        return;
    }
    Ray *d_rays = nullptr;
    cudaMalloc(&d_rays, ray_sz);
    cudaMemcpy(d_rays, rays_p, ray_sz, cudaMemcpyHostToDevice);
    shm_close(rays_p, ray_sz);

    // Allocate hit buffer on device.
    HitRecord *d_hits = nullptr;
    cudaMalloc(&d_hits, hit_sz);

    // Launch traversal.
    launch_trace_rays(b->d_nodes, b->d_tris, d_rays, d_hits,
                      (int)req.ray_count, /*stream=*/0);
    cudaDeviceSynchronize();

    // Write hits to pre-created shared memory.
    void *hits_p = shm_open_write(req.hits_shm.s, hit_sz);
    if (!hits_p) {
        cudaFree(d_rays);
        cudaFree(d_hits);
        send_response(conn, STATUS_BAD_MSG);
        return;
    }
    cudaMemcpy(hits_p, d_hits, hit_sz, cudaMemcpyDeviceToHost);
    shm_close(hits_p, hit_sz);

    cudaFree(d_rays);
    cudaFree(d_hits);

    fprintf(stderr, "[gpu-server] TraceRays %u rays\n", req.ray_count);
    send_response(conn, STATUS_OK);
}

// -----------------------------------------------------------------------
// Dispatch loop
// -----------------------------------------------------------------------

// Returns false if the connection should be closed (Shutdown or I/O error).
static bool handle_one(int conn) {
    req_hdr_t hdr;
    if (!read_exact(conn, &hdr, sizeof(hdr))) return false;

    // Read payload.
    uint8_t *payload = nullptr;
    if (hdr.payload_len > 0) {
        payload = new uint8_t[hdr.payload_len];
        if (!read_exact(conn, payload, hdr.payload_len)) {
            delete[] payload;
            return false;
        }
    }

    bool keep_running = true;

    switch (hdr.type) {
    case MSG_PING:
        send_response(conn, STATUS_OK);
        break;

    case MSG_SHUTDOWN:
        send_response(conn, STATUS_OK);
        keep_running = false;
        break;

    case MSG_ALLOC_GRID:
        handle_alloc_grid(conn, payload, hdr.payload_len);
        break;
    case MSG_FREE_GRID:
        handle_free_grid(conn, payload, hdr.payload_len);
        break;
    case MSG_UPLOAD_FIELDS:
        handle_upload_fields(conn, payload, hdr.payload_len);
        break;
    case MSG_RUN_FDTD:
        handle_run_fdtd(conn, payload, hdr.payload_len);
        break;

    case MSG_ALLOC_BVH:
        handle_alloc_bvh(conn, payload, hdr.payload_len);
        break;
    case MSG_FREE_BVH:
        handle_free_bvh(conn, payload, hdr.payload_len);
        break;
    case MSG_TRACE_RAYS:
        handle_trace_rays(conn, payload, hdr.payload_len);
        break;

    default:
        fprintf(stderr, "[gpu-server] unknown message type 0x%04x\n", hdr.type);
        send_response(conn, STATUS_BAD_MSG);
        break;
    }

    delete[] payload;
    return keep_running;
}

// -----------------------------------------------------------------------
// main
// -----------------------------------------------------------------------

int main(int argc, char **argv) {
    // Parse --socket <path>.
    const char *sock_path = nullptr;
    for (int i = 1; i < argc - 1; i++) {
        if (strcmp(argv[i], "--socket") == 0) {
            sock_path = argv[i + 1];
            break;
        }
    }
    if (!sock_path) {
        fprintf(stderr, "usage: %s --socket <path>\n", argv[0]);
        return 1;
    }

    // Print CUDA device info.
    int dev_count = 0;
    cudaGetDeviceCount(&dev_count);
    if (dev_count == 0) {
        fprintf(stderr, "[gpu-server] no CUDA device found\n");
        return 1;
    }
    cudaDeviceProp prop;
    cudaGetDeviceProperties(&prop, 0);
    fprintf(stderr, "[gpu-server] device: %s (sm_%d%d, %.1f GB)\n",
            prop.name, prop.major, prop.minor,
            prop.totalGlobalMem / 1e9);

    // Create Unix domain socket.
    int server_fd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (server_fd < 0) { perror("socket"); return 1; }

    struct sockaddr_un addr = {};
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, sock_path, sizeof(addr.sun_path) - 1);
    unlink(sock_path);

    if (bind(server_fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        perror("bind"); return 1;
    }
    if (listen(server_fd, 1) < 0) {
        perror("listen"); return 1;
    }

    fprintf(stderr, "[gpu-server] listening on %s\n", sock_path);

    // Accept exactly one connection.
    int conn = accept(server_fd, nullptr, nullptr);
    if (conn < 0) { perror("accept"); return 1; }

    fprintf(stderr, "[gpu-server] client connected\n");

    while (handle_one(conn)) {}

    close(conn);
    close(server_fd);
    unlink(sock_path);
    fprintf(stderr, "[gpu-server] shutdown\n");
    return 0;
}
