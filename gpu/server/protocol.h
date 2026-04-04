// protocol.h — wire protocol shared between the Go worker and the CUDA server.
//
// All integer fields are little-endian.
// Struct sizes MUST match the Go definitions in gpu/worker/protocol.go.
// The test TestProtocolStructSizes in worker_test.go validates Go's sizes;
// the static_assert lines below validate the C sizes at compile time.

#pragma once
#include <stdint.h>
#include <assert.h>

// -----------------------------------------------------------------------
// Shared memory name (64-byte null-padded, matching Go ShmName).
// -----------------------------------------------------------------------
typedef struct { char s[64]; } shm_name_t;

// -----------------------------------------------------------------------
// Request header (8 bytes)
// -----------------------------------------------------------------------
typedef struct __attribute__((packed)) {
    uint16_t type;
    uint16_t flags;
    uint32_t payload_len;
} req_hdr_t;
static_assert(sizeof(req_hdr_t) == 8, "req_hdr_t size mismatch");

// -----------------------------------------------------------------------
// Response header (8 bytes)
// -----------------------------------------------------------------------
typedef struct __attribute__((packed)) {
    uint32_t status;
    uint32_t response_len;
} resp_hdr_t;
static_assert(sizeof(resp_hdr_t) == 8, "resp_hdr_t size mismatch");

// -----------------------------------------------------------------------
// Status codes (must match Go StatusXxx constants)
// -----------------------------------------------------------------------
#define STATUS_OK          0u
#define STATUS_NOT_IMPL    1u
#define STATUS_BAD_HANDLE  2u
#define STATUS_OOM         3u
#define STATUS_CUDA        4u
#define STATUS_BAD_MSG     5u

// -----------------------------------------------------------------------
// Message type constants (must match Go MsgXxx constants)
// -----------------------------------------------------------------------
#define MSG_PING           0x0001u
#define MSG_SHUTDOWN       0x0002u
#define MSG_ALLOC_GRID     0x1001u
#define MSG_FREE_GRID      0x1002u
#define MSG_UPLOAD_FIELDS  0x1003u
#define MSG_RUN_FDTD       0x1004u
#define MSG_ALLOC_BVH      0x2001u
#define MSG_FREE_BVH       0x2002u
#define MSG_TRACE_RAYS     0x2003u

// -----------------------------------------------------------------------
// Payload structs
// -----------------------------------------------------------------------

typedef struct __attribute__((packed)) {
    uint32_t nx, ny, nz;          // 12 bytes
} alloc_grid_req_t;
static_assert(sizeof(alloc_grid_req_t) == 12, "alloc_grid_req size mismatch");

typedef struct __attribute__((packed)) {
    uint64_t handle;               // 8 bytes
} alloc_grid_resp_t;

typedef struct __attribute__((packed)) {
    uint64_t handle;               // 8 bytes
} free_grid_req_t;

typedef struct __attribute__((packed)) {
    uint64_t  handle;              // 8
    shm_name_t shm_name;           // 64
} upload_fields_req_t;             // 72 bytes
static_assert(sizeof(upload_fields_req_t) == 72, "upload_fields_req size mismatch");

typedef struct __attribute__((packed)) {
    uint64_t   handle;             // 8
    uint32_t   steps;              // 4
    uint32_t   src_idx;            // 4
    uint32_t   rcv_idx;            // 4
    float      speed_of_sound;     // 4
    float      dt;                 // 4
    shm_name_t result_shm;         // 64
} run_fdtd_req_t;                  // 92 bytes
static_assert(sizeof(run_fdtd_req_t) == 92, "run_fdtd_req size mismatch");

typedef struct __attribute__((packed)) {
    uint32_t   node_count;         // 4
    uint32_t   tri_count;          // 4
    shm_name_t shm_name;           // 64
} alloc_bvh_req_t;                 // 72 bytes
static_assert(sizeof(alloc_bvh_req_t) == 72, "alloc_bvh_req size mismatch");

typedef struct __attribute__((packed)) {
    uint64_t handle;               // 8 bytes
} alloc_bvh_resp_t;

typedef struct __attribute__((packed)) {
    uint64_t handle;               // 8 bytes
} free_bvh_req_t;

typedef struct __attribute__((packed)) {
    uint64_t   handle;             // 8
    uint32_t   ray_count;          // 4
    shm_name_t rays_shm;           // 64
    shm_name_t hits_shm;           // 64
} trace_rays_req_t;                // 140 bytes
static_assert(sizeof(trace_rays_req_t) == 140, "trace_rays_req size mismatch");
