//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"syscall/js"
	"time"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// Demo limits; see docs/web-demo-limits.md.
//
// The demo accepts requests from two places with very different trust: the page
// itself, whose sliders are already bounded, and window.algoAcousticsDemo, which
// any script on the page can call with arbitrary JSON. These constants are the
// envelope for both, and they are the single place the envelope is written down:
// normalizeDemoRequest enforces them, demoLimitsToJS publishes them so the page
// can size its own controls from them, and docs/web-demo-limits.md explains
// them.
//
// Three separate resources bound a browser render, and they bind at different
// points:
//
//  1. Memory. A Go/WASM heap only grows, so a request that peaks above the tab's
//     budget costs the user the tab. That is the memory budget's concern (see
//     docs/wasm-memory-budget.md) and is handled separately by
//     applyDemoMemoryBudget, which reduces quality knobs rather than rejecting
//     the request.
//  2. Wall-clock. A synchronous WASM render blocks its worker for its whole
//     duration, and the measured envelope reaches well past what anyone will
//     wait for. This is what demoRenderTimeout and the tier pipeline address.
//  3. Structure. Geometry and material counts drive both of the above, but
//     silently shrinking them would change which room is being simulated, so
//     they are rejected with an error instead of reduced.
const (
	// demoSampleRate is fixed. The plan's "3 s at 48 kHz" is a single limit:
	// 144,000 samples per channel, which is what the buffer estimates in
	// estimateDemoMemoryBytes and the WAV encoder are sized against.
	demoSampleRate = defaultDemoSampleRate

	// maxDemoDurationSecs caps the impulse response length. At demoSampleRate
	// this is the plan's 3 s / 48 kHz ceiling.
	maxDemoDurationSecs = 3.0

	// minDemoDurationSecs is short enough to hear the early field alone and long
	// enough that a crossover still has somewhere to sit.
	minDemoDurationSecs = 0.25

	// maxDemoNumRays caps the Monte Carlo ray budget.
	//
	// The original roadmap proposed 50,000. Memory allows it comfortably — 50,000 rays cost
	// about 25 MiB against a 512 MiB budget — but wall-clock does not. Measured
	// under go_js_wasm_exec, ray cost is close to linear: 3,072 rays render in
	// 1.5 s, 16,384 in 7.0 s, so 50,000 rays would take roughly 21 s at a 1.35 s
	// response and about a minute at 3 s. That is not a demo, and it is not
	// something a 10 s timeout can rescue into a good result either. The cap is
	// therefore set from the measured time envelope rather than the memory one.
	// See docs/web-demo-limits.md for the measurements.
	maxDemoNumRays = 16384

	// minDemoNumRays is the floor applyDemoMemoryBudget may reduce toward. Below
	// this the late field is too sparse to sound like a room.
	minDemoNumRays = 128

	// maxDemoMaxOrder caps the image-source reflection order. A six-sided room
	// generates (4n³ + 6n² + 8n + 3) / 3 image sources, so order 12 is already
	// 2,925 candidates per source.
	maxDemoMaxOrder = 12
	minDemoMaxOrder = 1

	// maxDemoSurfaces caps the number of distinct reflecting planes in a room.
	//
	// This is the plan's 50-surface limit, and it is counted in planes rather
	// than triangles on purpose: subdividing a wall multiplies triangles without
	// adding a single reflecting surface, and it is the plane count that drives
	// image-source growth. A shoebox has six.
	maxDemoSurfaces = 50

	// maxDemoMeshTriangles bounds an uploaded room mesh independently of the
	// surface count, because triangles drive BVH size and heatmap probe cost
	// even when they all lie in a handful of planes.
	maxDemoMeshTriangles = 20000

	// maxDemoMaterials bounds the material library, which setMaterial would
	// otherwise grow without limit across a session.
	maxDemoMaterials = 128

	// minDemoRoomMeters and maxDemoRoomMeters bound room dimensions. The UI
	// sliders are far tighter (index.html caps width and depth at 16 m); this
	// bounds the raw API, which the page does not go through.
	minDemoRoomMeters = 2.0
	maxDemoRoomMeters = 50.0

	// demoRenderTimeout is the wall-clock budget for one render. On expiry the
	// demo returns the best tier it has finished instead of an error, so the
	// user gets an audible, if coarser, impulse response. See tiers.go.
	demoRenderTimeout = 10 * time.Second
)

// surfacePlaneEpsilon quantizes plane normals and offsets when counting distinct
// surfaces. Meshes exported from modelling tools carry small per-triangle normal
// jitter, and without a tolerance a subdivided flat wall would count as many
// surfaces rather than one.
const surfacePlaneEpsilon = 1e-4

// countDemoSurfaces returns the number of distinct reflecting planes in a room.
//
// A shoebox always has six. A mesh room is counted by grouping triangles onto
// the plane they lie in, so subdivision does not inflate the count.
func countDemoSurfaces(room demoRoom) int {
	if room.Kind != "mesh" || room.Mesh == nil || len(room.Mesh.Triangles) == 0 {
		return 6
	}

	planes := make(map[[4]int64]struct{}, len(room.Mesh.Triangles))

	for _, tri := range room.Mesh.Triangles {
		triangle := geometry.Triangle{
			V0: geometry.Vec3{X: tri.V0.X, Y: tri.V0.Y, Z: tri.V0.Z},
			V1: geometry.Vec3{X: tri.V1.X, Y: tri.V1.Y, Z: tri.V1.Z},
			V2: geometry.Vec3{X: tri.V2.X, Y: tri.V2.Y, Z: tri.V2.Z},
		}

		normal := triangle.Normal()
		if normal == geometry.Vec3Zero {
			// A degenerate triangle reflects nothing; it is not a surface.
			continue
		}

		normal = normal.Normalize()

		// A plane and its back face are one surface, so the normal is flipped
		// into a canonical hemisphere before quantizing.
		if normal.X < 0 || (normal.X == 0 && (normal.Y < 0 || (normal.Y == 0 && normal.Z < 0))) {
			normal = normal.Scale(-1)
		}

		offset := normal.Dot(triangle.V0)
		planes[[4]int64{
			quantize(normal.X),
			quantize(normal.Y),
			quantize(normal.Z),
			quantize(offset),
		}] = struct{}{}
	}

	if len(planes) == 0 {
		return 6
	}

	return len(planes)
}

// quantize snaps a coordinate to the surface-matching tolerance.
func quantize(value float64) int64 {
	return int64(math.Round(value / surfacePlaneEpsilon))
}

// validateDemoStructure rejects requests whose geometry or material count
// exceeds the demo envelope. These are errors rather than reductions: decimating
// a mesh or dropping materials would answer a question the caller did not ask.
func validateDemoStructure(request demoRequest) error {
	if mesh := request.Room.Mesh; mesh != nil && len(mesh.Triangles) > maxDemoMeshTriangles {
		return fmt.Errorf(
			"room mesh has %d triangles, which exceeds the demo limit of %d",
			len(mesh.Triangles), maxDemoMeshTriangles,
		)
	}

	if surfaces := countDemoSurfaces(request.Room); surfaces > maxDemoSurfaces {
		return fmt.Errorf(
			"room has %d distinct surfaces, which exceeds the demo limit of %d",
			surfaces, maxDemoSurfaces,
		)
	}

	return nil
}

// demoLimitsToJS publishes the envelope so the page can size its own controls
// from it rather than repeating the numbers in HTML that can drift out of sync.
func demoLimitsToJS() js.Value {
	limits := js.Global().Get("Object").New()
	limits.Set("sampleRate", demoSampleRate)
	limits.Set("maxSurfaces", maxDemoSurfaces)
	limits.Set("maxMeshTriangles", maxDemoMeshTriangles)
	limits.Set("maxMaterials", maxDemoMaterials)
	limits.Set("minNumRays", minDemoNumRays)
	limits.Set("maxNumRays", maxDemoNumRays)
	limits.Set("minMaxOrder", minDemoMaxOrder)
	limits.Set("maxMaxOrder", maxDemoMaxOrder)
	limits.Set("minDurationSeconds", minDemoDurationSecs)
	limits.Set("maxDurationSeconds", maxDemoDurationSecs)
	limits.Set("minRoomMeters", minDemoRoomMeters)
	limits.Set("maxRoomMeters", maxDemoRoomMeters)
	limits.Set("renderTimeoutSeconds", demoRenderTimeout.Seconds())

	return limits
}
