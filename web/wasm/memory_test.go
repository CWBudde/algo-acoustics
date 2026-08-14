//go:build js && wasm

package main

import (
	"runtime"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
)

// TestMain mirrors main()'s runtime configuration so memory measurements taken
// here reflect what the browser actually runs.
func TestMain(m *testing.M) {
	configureDemoMemory()
	m.Run()
}

const mebibyte = 1 << 20

// TestDemoRenderStaysUnderMemoryBudget renders every corner of the request
// envelope in a single process and asserts the peak never crosses the budget.
//
// One process on purpose: WASM linear memory never shrinks, so a worker's
// footprint is the highest point any render in its lifetime reached. Rendering
// the corners in sequence is what a browsing session looks like.
func TestDemoRenderStaysUnderMemoryBudget(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*demoRequest)
		wantMax int64
	}{
		{
			name:   "default",
			mutate: func(*demoRequest) {},
		},
		{
			name: "mono hybrid at envelope maximum",
			mutate: func(r *demoRequest) {
				r.Render.Mode = "hybrid"
				r.Render.MaxOrder = maxDemoMaxOrder
				r.Render.NumRays = maxDemoNumRays
				r.Render.DurationSeconds = maxDemoDurationSecs
			},
		},
		{
			name: "mono late at envelope maximum",
			mutate: func(r *demoRequest) {
				r.Render.Mode = "late"
				r.Render.MaxOrder = maxDemoMaxOrder
				r.Render.NumRays = maxDemoNumRays
				r.Render.DurationSeconds = maxDemoDurationSecs
			},
		},
		{
			name: "mono early at envelope maximum",
			mutate: func(r *demoRequest) {
				r.Render.Mode = "early"
				r.Render.MaxOrder = maxDemoMaxOrder
				r.Render.NumRays = maxDemoNumRays
				r.Render.DurationSeconds = maxDemoDurationSecs
			},
		},
		{
			name: "largest room at envelope maximum",
			mutate: func(r *demoRequest) {
				r.Room.Width = maxDemoRoomMeters
				r.Room.Depth = maxDemoRoomMeters
				r.Room.Height = maxDemoRoomMeters
				r.Render.Mode = "hybrid"
				r.Render.MaxOrder = maxDemoMaxOrder
				r.Render.NumRays = maxDemoNumRays
				r.Render.DurationSeconds = maxDemoDurationSecs
			},
		},
		{
			name: "portal at envelope maximum",
			mutate: func(r *demoRequest) {
				r.Portal.Enabled = true
				r.Render.Mode = "hybrid"
				r.Render.MaxOrder = maxDemoMaxOrder
				r.Render.NumRays = maxDemoNumRays
				r.Render.DurationSeconds = maxDemoDurationSecs
			},
		},
		{
			name: "mesh room at triangle cap",
			mutate: func(r *demoRequest) {
				r.Room.Kind = "mesh"
				r.Room.Mesh = subdividedDemoMesh(maxDemoMeshTriangles, r.Room)
				r.Render.Mode = "hybrid"
				r.Render.MaxOrder = maxDemoMaxOrder
				r.Render.NumRays = maxDemoNumRays
				r.Render.DurationSeconds = maxDemoDurationSecs
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := defaultDemoRequest()
			test.mutate(&request)

			normalized, warnings, err := normalizeDemoRequest(request)
			if err != nil {
				t.Fatalf("normalizeDemoRequest() error = %v", err)
			}

			result, err := runDemoRender(normalized)
			if err != nil {
				t.Fatalf("runDemoRender() error = %v", err)
			}

			peak := result.Memory.PeakSysBytes
			t.Logf("peak %d MiB (budget %d MiB), order %d, rays %d, %.2f s, %d events, %d warnings",
				peak/mebibyte, demoMemoryBudgetBytes/mebibyte,
				normalized.Render.MaxOrder, normalized.Render.NumRays,
				normalized.Render.DurationSeconds, result.EarlyEventCount, len(warnings))

			if peak > demoMemoryBudgetBytes {
				t.Errorf("peak memory = %d MiB, want at most %d MiB",
					peak/mebibyte, demoMemoryBudgetBytes/mebibyte)
			}
		})
	}
}

// TestRepeatedRendersDoNotRatchetMemory guards the retention fixes: a worker
// that renders over and over must plateau rather than climb, because every
// megabyte it reaches is a megabyte the tab keeps.
func TestRepeatedRendersDoNotRatchetMemory(t *testing.T) {
	request := defaultDemoRequest()
	request.Render.DurationSeconds = 2

	normalized, _, err := normalizeDemoRequest(request)
	if err != nil {
		t.Fatalf("normalizeDemoRequest() error = %v", err)
	}

	var first, last int64
	for index := range 6 {
		if _, err := runDemoRender(normalized); err != nil {
			t.Fatalf("render %d: %v", index, err)
		}

		runtime.GC()

		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if index == 0 {
			first = int64(stats.HeapAlloc)
		}
		last = int64(stats.HeapAlloc)
	}

	t.Logf("live heap after first render %d KiB, after sixth %d KiB", first/1024, last/1024)

	// Retaining one render's samples, WAV, and heatmap would show up here as
	// steady growth across iterations.
	if last > first+16*mebibyte {
		t.Errorf("live heap grew from %d KiB to %d KiB across repeated renders",
			first/1024, last/1024)
	}
}

func TestApplyDemoMemoryBudgetClampsPortalEnvelope(t *testing.T) {
	request := defaultDemoRequest()
	request.Portal.Enabled = true
	request.Render.MaxOrder = maxDemoMaxOrder
	request.Render.NumRays = maxDemoNumRays
	request.Render.DurationSeconds = maxDemoDurationSecs

	adjusted, warnings := applyDemoMemoryBudget(request)

	if adjusted.Render.NumRays != maxPortalNumRays {
		t.Errorf("rays = %d, want %d", adjusted.Render.NumRays, maxPortalNumRays)
	}
	if adjusted.Render.MaxOrder != maxPortalMaxOrder {
		t.Errorf("maxOrder = %d, want %d", adjusted.Render.MaxOrder, maxPortalMaxOrder)
	}
	if adjusted.Render.DurationSeconds != maxPortalDurationSecs {
		t.Errorf("duration = %v, want %v", adjusted.Render.DurationSeconds, maxPortalDurationSecs)
	}

	if len(warnings) != 3 {
		t.Fatalf("warnings = %d, want 3: %v", len(warnings), warnings)
	}
	for _, warning := range warnings {
		if !strings.Contains(warning, "memory budget") {
			t.Errorf("warning %q does not name the memory budget", warning)
		}
	}

	// A shortened duration must not leave the crossover stranded past the end.
	if adjusted.Render.CrossoverTimeSeconds > adjusted.Render.DurationSeconds {
		t.Errorf("crossover %v exceeds duration %v",
			adjusted.Render.CrossoverTimeSeconds, adjusted.Render.DurationSeconds)
	}
}

func TestApplyDemoMemoryBudgetLeavesAffordableRequestsAlone(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*demoRequest)
	}{
		{"default", func(*demoRequest) {}},
		{"mono at maximum", func(r *demoRequest) {
			r.Render.MaxOrder = maxDemoMaxOrder
			r.Render.NumRays = maxDemoNumRays
			r.Render.DurationSeconds = maxDemoDurationSecs
		}},
		{"portal inside envelope", func(r *demoRequest) {
			r.Portal.Enabled = true
			r.Render.MaxOrder = maxPortalMaxOrder
			r.Render.NumRays = maxPortalNumRays
			r.Render.DurationSeconds = maxPortalDurationSecs
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := defaultDemoRequest()
			test.mutate(&request)

			adjusted, warnings := applyDemoMemoryBudget(request)

			if len(warnings) != 0 {
				t.Errorf("warnings = %v, want none", warnings)
			}
			if adjusted.Render != request.Render {
				t.Errorf("render settings changed from %+v to %+v", request.Render, adjusted.Render)
			}
		})
	}
}

func TestEstimateDemoMemoryRisesWithCost(t *testing.T) {
	base := defaultDemoRequest()

	tests := []struct {
		name   string
		mutate func(*demoRequest)
	}{
		{"more rays", func(r *demoRequest) { r.Render.NumRays *= 2 }},
		{"higher order", func(r *demoRequest) { r.Render.MaxOrder++ }},
		{"longer duration", func(r *demoRequest) { r.Render.DurationSeconds *= 2 }},
		{"portal enabled", func(r *demoRequest) { r.Portal.Enabled = true }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			costlier := base
			test.mutate(&costlier)

			if estimateDemoMemoryBytes(costlier) <= estimateDemoMemoryBytes(base) {
				t.Errorf("estimate %d did not rise above baseline %d",
					estimateDemoMemoryBytes(costlier), estimateDemoMemoryBytes(base))
			}
		})
	}
}

func TestIsmImageCount(t *testing.T) {
	// (4n³ + 6n² + 8n + 3) / 3 for a six-sided room.
	tests := []struct {
		order int
		want  int64
	}{
		{0, 1},
		{1, 7},
		{2, 25},
		{3, 63},
		{4, 129},
		{5, 231},
	}

	for _, test := range tests {
		if got := ismImageCount(test.order); got != test.want {
			t.Errorf("ismImageCount(%d) = %d, want %d", test.order, got, test.want)
		}
	}
}

func TestNormalizeDemoRequestRejectsOversizedMesh(t *testing.T) {
	request := defaultDemoRequest()
	request.Room.Kind = "mesh"
	request.Room.Mesh = &demoMesh{Triangles: make([]demoTriangle, maxDemoMeshTriangles+1)}

	if _, _, err := normalizeDemoRequest(request); err == nil {
		t.Fatal("normalizeDemoRequest() error = nil, want an error naming the triangle cap")
	} else if !strings.Contains(err.Error(), "triangles") {
		t.Errorf("error %q does not mention the triangle count", err)
	}
}

func TestNormalizeDemoRequestClampsRoomDimensions(t *testing.T) {
	request := defaultDemoRequest()
	request.Room.Width = 5000
	request.Room.Depth = 0.01
	request.Room.Height = 5000

	normalized, _, err := normalizeDemoRequest(request)
	if err != nil {
		t.Fatalf("normalizeDemoRequest() error = %v", err)
	}

	if normalized.Room.Width != maxDemoRoomMeters {
		t.Errorf("width = %v, want %v", normalized.Room.Width, maxDemoRoomMeters)
	}
	if normalized.Room.Height != maxDemoRoomMeters {
		t.Errorf("height = %v, want %v", normalized.Room.Height, maxDemoRoomMeters)
	}
	// A depth of 0.01 is positive, so it is clamped up rather than defaulted.
	if normalized.Room.Depth != minDemoRoomMeters {
		t.Errorf("depth = %v, want %v", normalized.Room.Depth, minDemoRoomMeters)
	}
}

func TestStoreResultDropsHeavyPayloads(t *testing.T) {
	state := newDemoAPIState()
	buffer := &ir.Buffer{SampleRate: defaultDemoSampleRate, Samples: make([]float64, 128)}

	state.storeResult(demoResult{
		PeakAmplitude: 0.5,
		Samples:       make([]float32, 4096),
		WAVBytes:      make([]byte, 4096),
		SPLHeatmap:    demoSPLHeatmap{Samples: make([]demoSPLSample, 210)},
		PortalResponses: &demoPortalResponses{
			ClosedWAVBytes: make([]byte, 4096),
			OpenWAVBytes:   make([]byte, 4096),
		},
	}, buffer)

	if state.lastResult.Samples != nil {
		t.Error("retained samples, want them dropped")
	}
	if state.lastResult.WAVBytes != nil {
		t.Error("retained WAV bytes, want them dropped")
	}
	if state.lastResult.SPLHeatmap.Samples != nil {
		t.Error("retained heatmap samples, want them dropped")
	}
	if state.lastResult.PortalResponses != nil {
		t.Error("retained portal responses, want them dropped")
	}

	// The scalars getParameters reads, and the buffer itself, must survive.
	if state.lastResult.PeakAmplitude != 0.5 {
		t.Errorf("peak amplitude = %v, want 0.5", state.lastResult.PeakAmplitude)
	}
	if state.lastBuffer != buffer {
		t.Error("lastBuffer was not retained")
	}
}

func TestMeshHeatmapProbesAreCapped(t *testing.T) {
	room := demoRoom{Width: 6.4, Depth: 4.8, Height: 2.9}
	mesh := buildDemoGeometryMesh(subdividedDemoMesh(maxDemoMeshTriangles, room))

	probes := meshHeatmapProbes(mesh, geometry.Vec3{X: 1, Y: 1, Z: 1})

	if len(probes) == 0 {
		t.Fatal("meshHeatmapProbes() returned no probes")
	}
	if len(probes) > maxMeshHeatmapProbes {
		t.Errorf("probes = %d, want at most %d", len(probes), maxMeshHeatmapProbes)
	}
}

// subdividedDemoMesh returns a closed shoebox mesh subdivided until it has at
// least the requested triangle count, then trimmed to exactly that count.
func subdividedDemoMesh(triangles int, room demoRoom) *demoMesh {
	base := buildDemoLoftMesh(room.Width, room.Depth, room.Height)

	mesh := &demoMesh{Triangles: make([]demoTriangle, 0, len(base.Triangles))}
	for _, triangle := range base.Triangles {
		mesh.Triangles = append(mesh.Triangles, demoTriangle{
			V0: demoPoint{X: triangle.V0.X, Y: triangle.V0.Y, Z: triangle.V0.Z},
			V1: demoPoint{X: triangle.V1.X, Y: triangle.V1.Y, Z: triangle.V1.Z},
			V2: demoPoint{X: triangle.V2.X, Y: triangle.V2.Y, Z: triangle.V2.Z},
		})
	}

	for len(mesh.Triangles) < triangles {
		split := make([]demoTriangle, 0, len(mesh.Triangles)*4)
		for _, tri := range mesh.Triangles {
			m01 := midpointDemoPoint(tri.V0, tri.V1)
			m12 := midpointDemoPoint(tri.V1, tri.V2)
			m20 := midpointDemoPoint(tri.V2, tri.V0)
			split = append(
				split,
				demoTriangle{V0: tri.V0, V1: m01, V2: m20},
				demoTriangle{V0: m01, V1: tri.V1, V2: m12},
				demoTriangle{V0: m20, V1: m12, V2: tri.V2},
				demoTriangle{V0: m01, V1: m12, V2: m20},
			)
		}
		mesh.Triangles = split
	}

	mesh.Triangles = mesh.Triangles[:triangles]

	return mesh
}

func midpointDemoPoint(a, b demoPoint) demoPoint {
	return demoPoint{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2, Z: (a.Z + b.Z) / 2}
}
