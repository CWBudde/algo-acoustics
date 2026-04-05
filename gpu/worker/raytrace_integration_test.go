package worker

// Integration test for the full ray-tracing path:
//   AllocBVH → TraceRays → FreeBVH
//
// Set ALGO_GPU_SERVER to the compiled binary path to run:
//
//	ALGO_GPU_SERVER=./gpu/server/algo-acoustics-gpu go test ./gpu/worker/ -run TestRayTraceIntegration -v
//
// Automatically skipped when the env var is absent.

import (
	"context"
	"math"
	"os"
	"testing"
)

// TestRayTraceIntegration traces rays against a single triangle in the xz-plane.
//
// Scene: one triangle (0,0,0)–(10,0,0)–(0,0,10), id=42.
// BVH: single leaf node covering the triangle's AABB.
//
// Rays:
//   - Ray 0: (1, 5, 1) dir (0,−1, 0)  → hits at t=5.0, tri_id=42
//   - Ray 1: (1, 5, 1) dir (0, +1, 0) → misses (pointing away)  t=FLT_MAX, tri_id=−1
func TestRayTraceIntegration(t *testing.T) {
	bin := os.Getenv("ALGO_GPU_SERVER")
	if bin == "" {
		t.Skip("ALGO_GPU_SERVER not set — skipping GPU integration test")
	}

	ctx := context.Background()
	w, err := Start(ctx, bin)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Close() //nolint:errcheck

	if err := w.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	// ------------------------------------------------------------------
	// Build a minimal BVH: one leaf node, one triangle.
	// ------------------------------------------------------------------
	tris := []Triangle{
		{V0X: 0, V0Y: 0, V0Z: 0,
			V1X: 10, V1Y: 0, V1Z: 0,
			V2X: 0, V2Y: 0, V2Z: 10,
			ID: 42},
	}
	nodes := []BVHNode{
		{
			LoX: 0, LoY: 0, LoZ: 0,
			HiX: 10, HiY: 0, HiZ: 10,
			LeftOrFirst: 0,
			Count:       1,
		},
	}

	bvh, err := w.AllocBVH(ctx, nodes, tris)
	if err != nil {
		t.Fatalf("AllocBVH: %v", err)
	}
	defer func() {
		if err := w.FreeBVH(ctx, bvh); err != nil {
			t.Logf("FreeBVH: %v", err)
		}
	}()

	// ------------------------------------------------------------------
	// Trace two rays.
	// ------------------------------------------------------------------
	rays := []Ray{
		// Ray 0: hits the triangle
		{OriginX: 1, OriginY: 5, OriginZ: 1,
			DirX: 0, DirY: -1, DirZ: 0,
			Tmin: 0, Tmax: 100},
		// Ray 1: misses (pointing up)
		{OriginX: 1, OriginY: 5, OriginZ: 1,
			DirX: 0, DirY: 1, DirZ: 0,
			Tmin: 0, Tmax: 100},
	}

	hits, err := w.TraceRays(ctx, bvh, rays)
	if err != nil {
		t.Fatalf("TraceRays: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("got %d hit records, want 2", len(hits))
	}

	// Ray 0 — expect hit at t ≈ 5.0, tri_id = 42.
	h0 := hits[0]
	if math.IsNaN(float64(h0.T)) || math.IsInf(float64(h0.T), 0) {
		t.Errorf("ray 0: T = %v (NaN or Inf)", h0.T)
	}
	if math.Abs(float64(h0.T)-5.0) > 1e-4 {
		t.Errorf("ray 0: T = %v, want ~5.0", h0.T)
	}
	if h0.TriID != 42 {
		t.Errorf("ray 0: TriID = %d, want 42", h0.TriID)
	}
	t.Logf("ray 0: t=%.6f  tri_id=%d", h0.T, h0.TriID)

	// Ray 1 — expect miss: tri_id = -1.
	h1 := hits[1]
	if h1.TriID != -1 {
		t.Errorf("ray 1: TriID = %d, want -1 (miss)", h1.TriID)
	}
	t.Logf("ray 1: t=%.6f  tri_id=%d", h1.T, h1.TriID)
}
