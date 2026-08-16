//go:build js && wasm

package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Memory budget for the browser demo; see docs/wasm-memory-budget.md.
//
// A Go/WASM linear-memory heap only ever grows: the browser never hands pages
// back, so the highest point a render reaches stays resident for the life of
// the worker. The peak is therefore what matters, not the steady state, and it
// is defended on three fronts:
//
//  1. debug.SetMemoryLimit keeps the collector from letting the heap run far
//     ahead of a small live set. Demo renders churn well over a gigabyte while
//     keeping only a few megabytes live, and without a limit the heap goal
//     ratchets upward on every spike.
//  2. releaseDemoMemory collects at render stage boundaries. js/wasm is
//     single-threaded, so the collector cannot preempt a long allocating call;
//     explicit collection points between stages bound the overshoot.
//  3. estimateDemoMemoryBytes rejects work that would blow the budget outright,
//     downgrading quality knobs before the render starts. Connected-room
//     (portal) renders are the case that needs this: they solve the image-source
//     method in two rooms and pair the results, so the live event set grows with
//     the square of the single-room image count.
const (
	// demoMemoryBudgetBytes is the demo target: peak WASM linear memory
	// stays below this for every request the demo API accepts.
	demoMemoryBudgetBytes int64 = 512 << 20

	// demoMemorySoftLimitBytes is the GC's soft heap ceiling. It is far below
	// the budget on purpose: it is a target for the collector, not a cap on the
	// process, and a tight value is what keeps the heap goal from ratcheting.
	// Measured on the worst-case portal render, this cut peak linear memory
	// from ~1.8 GiB to ~160 MiB at no cost in wall-clock time.
	demoMemorySoftLimitBytes int64 = 64 << 20

	// demoMemoryEstimateCap is the ceiling estimateDemoMemoryBytes downgrades
	// toward. The gap to demoMemoryBudgetBytes absorbs the JS-side Float32Array
	// and Uint8Array copies, the structured clone across postMessage, and the
	// error in the estimate itself.
	demoMemoryEstimateCap int64 = 224 << 20
)

// Connected-room envelope.
//
// A portal render traces two full binaural responses, and each one churns
// roughly 700 MiB while keeping only a few MiB live. Peak linear memory is
// therefore set by how far the heap runs ahead of the collector, which is not a
// smooth function of the request: neighbouring configurations were measured
// four times apart. That rules out predicting the peak from a byte model, so
// the envelope is bounded by measurement instead.
//
// Across this envelope the measured peak is 58-260 MiB. One step beyond it
// (1.5 s, or 8192 rays at 2 s) reaches 456-938 MiB. See
// docs/wasm-memory-budget.md for the full grid.
const (
	maxPortalDurationSecs = 1.2
	maxPortalNumRays      = 4096
	maxPortalMaxOrder     = 5
)

// Calibration constants for estimateDemoMemoryBytes.
//
// These are fitted to measurements taken under go_js_wasm_exec rather than
// derived from struct sizes: what drives peak linear memory is allocation the
// collector has not yet reclaimed, not the live set alone. See
// docs/wasm-memory-budget.md for the measured envelope.
const (
	// demoMemoryBaselineBytes covers the Go runtime, the synthesised HRTF
	// dataset, the scene, and the BVH.
	demoMemoryBaselineBytes int64 = 96 << 20

	// demoMemoryBytesPerEvent is the effective cost of one image-source event,
	// including the path slices and per-band gains allocated while testing it
	// for audibility and the copies made during binaural rendering.
	demoMemoryBytesPerEvent int64 = 2048

	// demoMemoryBytesPerRay covers per-ray state and the per-bounce band slices
	// allocated along a ray's path.
	demoMemoryBytesPerRay int64 = 512

	// demoMemoryBytesPerTriangle covers a mesh room's triangle, its BVH node,
	// and the traversal temporaries it attracts.
	demoMemoryBytesPerTriangle int64 = 768

	// demoMemoryBytesPerHistogramBin is charged per directivity group, per band.
	demoMemoryBytesPerHistogramBin int64 = 64

	// demoDirectivityGroupCount mirrors raytrace.NewDirectivityGroups(12, 6),
	// the binning the binaural late field uses.
	demoDirectivityGroupCount int64 = 72
)

// configureDemoMemory installs the GC soft limit. Called once from main.
func configureDemoMemory() {
	debug.SetMemoryLimit(demoMemorySoftLimitBytes)
}

// releaseDemoMemory collects garbage at a render stage boundary. Renders are a
// long sequence of allocating calls on a single thread; without these points
// the heap goal ratchets up on the first spike and, because WASM linear memory
// never shrinks, stays there.
func releaseDemoMemory() {
	runtime.GC()
}

// demoMemoryStats reports the demo's memory position to the browser.
type demoMemoryStats struct {
	HeapBytes     int64 `json:"heapBytes"`
	SysBytes      int64 `json:"sysBytes"`
	PeakSysBytes  int64 `json:"peakSysBytes"`
	BudgetBytes   int64 `json:"budgetBytes"`
	EstimateBytes int64 `json:"estimateBytes"`
}

// peakDemoSysBytes is the high-water mark of runtime.MemStats.Sys. It is sticky
// by design: linear memory never shrinks, so the highest value ever observed is
// the footprint the tab keeps.
var peakDemoSysBytes int64

// demoMemorySnapshot samples the runtime and updates the sticky peak.
func demoMemorySnapshot(estimate int64) demoMemoryStats {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	sys := int64(stats.Sys)
	if sys > peakDemoSysBytes {
		peakDemoSysBytes = sys
	}

	return demoMemoryStats{
		HeapBytes:     int64(stats.HeapAlloc),
		SysBytes:      sys,
		PeakSysBytes:  peakDemoSysBytes,
		BudgetBytes:   demoMemoryBudgetBytes,
		EstimateBytes: estimate,
	}
}

// ismImageCount returns the number of image sources a six-sided room generates
// up to the given reflection order: (4n³ + 6n² + 8n + 3) / 3.
func ismImageCount(order int) int64 {
	n := int64(order)
	if n < 0 {
		return 1
	}

	return (4*n*n*n + 6*n*n + 8*n + 3) / 3
}

// demoEventCount estimates how many image-source events a request produces.
//
// A connected-room render solves the image-source method in the source room and
// in the receiving room and pairs the results, so its event count is the square
// of the single-room count. That squaring is what makes portal renders the only
// configuration able to exhaust the budget.
func demoEventCount(request demoRequest) int64 {
	if request.Render.Mode == "late" {
		return 0
	}

	images := ismImageCount(request.Render.MaxOrder)
	if request.Portal.Enabled {
		return images * images
	}

	// A mesh room is not scaled by its triangle count. Subdividing a surface
	// multiplies triangles without adding reflecting planes, and measurement
	// bears this out: a 20,000-triangle room at the maximum order yields a
	// single audible image source. Its cost is geometry, charged separately.
	return images
}

// estimateDemoMemoryBytes projects the peak linear memory a request needs.
//
// It is deliberately a conservative over-estimate: being wrong high costs a
// quality downgrade, being wrong low costs the user a dead tab.
func estimateDemoMemoryBytes(request demoRequest) int64 {
	samples := int64(request.Render.DurationSeconds * float64(defaultDemoSampleRate))

	// Dense buffers. A mono hybrid render holds the early, late, and blended
	// buffers at once; a portal render holds a stereo pair for the closed, open,
	// and interpolated responses, plus three encoded WAVs.
	bufferBytesPerSample := int64(8*3 + 4 + 2 + 4)
	if request.Portal.Enabled {
		bufferBytesPerSample = 8*6*2 + 2*2*3 + 4 + 4
	}

	total := demoMemoryBaselineBytes
	total += samples * bufferBytesPerSample
	total += demoEventCount(request) * demoMemoryBytesPerEvent

	if request.Room.Kind == "mesh" && request.Room.Mesh != nil {
		total += int64(len(request.Room.Mesh.Triangles)) * demoMemoryBytesPerTriangle
	}

	rayPasses := int64(1)
	if request.Portal.Enabled {
		rayPasses = 2
	}
	total += int64(request.Render.NumRays) * demoMemoryBytesPerRay * rayPasses

	bins := int64(request.Render.DurationSeconds / defaultHistogramBinSecs)
	if request.Portal.Enabled {
		// Binaural synthesis keeps one histogram per directivity group.
		total += bins * demoDirectivityGroupCount * demoMemoryBytesPerHistogramBin * rayPasses
	} else {
		total += bins * demoMemoryBytesPerHistogramBin
	}

	return total
}

// applyDemoMemoryBudget reduces quality knobs until the request's projected
// peak fits the budget, returning the adjusted request and a warning for every
// reduction it made.
//
// Structural inputs (mesh triangle count, room size, material count) are capped
// with errors in normalizeDemoRequest instead: silently decimating geometry
// would change which room is being simulated, which is worse than refusing.
func applyDemoMemoryBudget(request demoRequest) (demoRequest, []string) {
	var warnings []string

	if request.Portal.Enabled {
		if request.Render.NumRays > maxPortalNumRays {
			warnings = append(warnings, fmt.Sprintf(
				"memory budget: connected-room render reduced rays from %d to %d",
				request.Render.NumRays, maxPortalNumRays,
			))
			request.Render.NumRays = maxPortalNumRays
		}

		if request.Render.MaxOrder > maxPortalMaxOrder {
			warnings = append(warnings, fmt.Sprintf(
				"memory budget: connected-room render reduced reflection order from %d to %d",
				request.Render.MaxOrder, maxPortalMaxOrder,
			))
			request.Render.MaxOrder = maxPortalMaxOrder
		}

		if request.Render.DurationSeconds > maxPortalDurationSecs {
			warnings = append(warnings, fmt.Sprintf(
				"memory budget: connected-room render reduced duration from %.2f s to %.2f s",
				request.Render.DurationSeconds, maxPortalDurationSecs,
			))
			request.Render.DurationSeconds = maxPortalDurationSecs
		}
	}

	for estimateDemoMemoryBytes(request) > demoMemoryEstimateCap {
		switch {
		case request.Render.NumRays > minDemoNumRays:
			previous := request.Render.NumRays
			request.Render.NumRays = max(minDemoNumRays, previous/2)
			warnings = append(warnings, fmt.Sprintf(
				"memory budget: reduced rays from %d to %d", previous, request.Render.NumRays,
			))
		case request.Render.MaxOrder > minDemoMaxOrder:
			previous := request.Render.MaxOrder
			request.Render.MaxOrder = previous - 1
			warnings = append(warnings, fmt.Sprintf(
				"memory budget: reduced reflection order from %d to %d", previous, request.Render.MaxOrder,
			))
		case request.Render.DurationSeconds > minDemoDurationSecs:
			previous := request.Render.DurationSeconds
			request.Render.DurationSeconds = max(minDemoDurationSecs, previous*0.75)
			warnings = append(warnings, fmt.Sprintf(
				"memory budget: reduced duration from %.2f s to %.2f s", previous, request.Render.DurationSeconds,
			))
		default:
			warnings = append(warnings, fmt.Sprintf(
				"memory budget: minimum settings still project %d MiB against a %d MiB budget",
				estimateDemoMemoryBytes(request)>>20, demoMemoryBudgetBytes>>20,
			))

			return request, warnings
		}
	}

	// The crossover must stay inside a duration that may have just shrunk.
	request.Render.CrossoverTimeSeconds = clamp(
		request.Render.CrossoverTimeSeconds, 0.03, request.Render.DurationSeconds*0.85,
	)

	return request, warnings
}
