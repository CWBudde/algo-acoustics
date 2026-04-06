//go:build js && wasm

package main

import (
	"math"
	"strings"
	"testing"
)

func TestRunDemoRenderProducesWaveformAndMetrics(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Render.NumRays = 256
	request.Render.DurationSeconds = 0.75
	request.Render.CrossoverTimeSeconds = 0.16

	result, err := runDemoRender(request)
	if err != nil {
		t.Fatalf("runDemoRender() error = %v", err)
	}

	if got, want := result.SampleRate, defaultDemoSampleRate; got != want {
		t.Fatalf("SampleRate = %d, want %d", got, want)
	}

	if len(result.Samples) == 0 {
		t.Fatal("Samples is empty, want rendered waveform")
	}

	if len(result.WAVBytes) == 0 {
		t.Fatal("WAVBytes is empty, want encoded audio")
	}

	if result.EarlyEventCount <= 0 {
		t.Fatalf("EarlyEventCount = %d, want > 0", result.EarlyEventCount)
	}

	if result.PeakAmplitude <= 0 {
		t.Fatalf("PeakAmplitude = %v, want > 0", result.PeakAmplitude)
	}

	if result.FirstArrivalMs <= 0 {
		t.Fatalf("FirstArrivalMs = %v, want > 0", result.FirstArrivalMs)
	}

	if result.RenderMS < 0 {
		t.Fatalf("RenderMS = %v, want >= 0", result.RenderMS)
	}
}

func TestRunDemoRenderRejectsUnsupportedMode(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Render.Mode = "invalid"

	_, err := runDemoRender(request)
	if err == nil {
		t.Fatal("runDemoRender() error = nil, want unsupported mode error")
	}

	if !strings.Contains(err.Error(), "unsupported render mode") {
		t.Fatalf("runDemoRender() error = %q, want unsupported mode message", err)
	}
}

// TestRunDemoRenderModesProduceDifferentResults verifies that "early", "late",
// and "hybrid" modes produce meaningfully different IRs for the same shoebox
// scene. If the mode switch were broken (e.g., all paths executing the same
// code), peak/RMS values would be identical across modes.
func TestRunDemoRenderModesProduceDifferentResults(t *testing.T) {
	t.Parallel()

	base := defaultDemoRequest()
	base.Render.NumRays = 512
	base.Render.MaxOrder = 3
	base.Render.DurationSeconds = 0.5
	base.Render.CrossoverTimeSeconds = 0.12

	modes := []string{"early", "late", "hybrid"}
	peaks := make(map[string]float64, len(modes))
	counts := make(map[string]int, len(modes))

	for _, mode := range modes {
		req := base
		req.Render.Mode = mode

		result, err := runDemoRender(req)
		if err != nil {
			t.Fatalf("mode=%q runDemoRender() error = %v", mode, err)
		}

		if result.Mode != mode {
			t.Errorf("mode=%q: result.Mode = %q, want %q", mode, result.Mode, mode)
		}

		if result.PeakAmplitude <= 0 {
			t.Errorf("mode=%q: PeakAmplitude = %v, want > 0", mode, result.PeakAmplitude)
		}

		peaks[mode] = result.PeakAmplitude
		counts[mode] = result.EarlyEventCount
	}

	// "early" mode must have ISM events; "late" must have none.
	if counts["early"] == 0 {
		t.Errorf("early mode: EarlyEventCount = 0, want > 0")
	}

	if counts["late"] != 0 {
		t.Errorf("late mode: EarlyEventCount = %d, want 0", counts["late"])
	}

	if counts["hybrid"] == 0 {
		t.Errorf("hybrid mode: EarlyEventCount = 0, want > 0 (ISM events expected)")
	}

	// "early" (sparse ISM spikes) and "late" (diffuse noise) must be detectably
	// different — their peaks must differ by at least 10 %.
	relDiff := math.Abs(peaks["early"]-peaks["late"]) / math.Max(peaks["early"], peaks["late"])
	if relDiff < 0.10 {
		t.Errorf("early peak %v and late peak %v are too similar (rel diff %.3f < 0.10): mode switch may be broken",
			peaks["early"], peaks["late"], relDiff)
	}
}

// TestRunDemoRenderMeshRoomForcesLateMode verifies that a mesh room always runs
// in "late" mode regardless of the requested mode, and that result.Mode
// reflects the actual mode used (not the user's request).
func TestRunDemoRenderMeshRoomForcesLateMode(t *testing.T) {
	t.Parallel()

	mesh := smallCubeMeshRequest(4, 3, 2.5)

	for _, requested := range []string{"early", "hybrid", "late"} {
		req := defaultDemoRequest()
		req.Room = demoRoom{Kind: "mesh", Mesh: mesh}
		req.Source.X = 1
		req.Source.Y = 1
		req.Source.Z = 1
		req.Receiver.X = 3
		req.Receiver.Y = 2
		req.Receiver.Z = 2
		req.Render.Mode = requested
		req.Render.NumRays = 256
		req.Render.DurationSeconds = 0.4

		result, err := runDemoRender(req)
		if err != nil {
			t.Fatalf("requested=%q runDemoRender() error = %v", requested, err)
		}

		if result.Mode != "late" {
			t.Errorf("requested=%q: result.Mode = %q, want %q (mesh rooms must run as late)",
				requested, result.Mode, "late")
		}

		// Mesh room in late mode must never return ISM events.
		if result.EarlyEventCount != 0 {
			t.Errorf("requested=%q: EarlyEventCount = %d, want 0 for mesh room in late mode",
				requested, result.EarlyEventCount)
		}

		if result.PeakAmplitude <= 0 {
			t.Errorf("requested=%q: PeakAmplitude = %v, want > 0", requested, result.PeakAmplitude)
		}
	}
}

// smallCubeMeshRequest builds the 12-triangle closed-box mesh for a demoRequest.
func smallCubeMeshRequest(w, d, h float64) *demoMesh {
	p := func(x, y, z float64) demoPoint { return demoPoint{X: x, Y: y, Z: z} }
	tri := func(v0, v1, v2 demoPoint) demoTriangle { return demoTriangle{V0: v0, V1: v1, V2: v2} }

	v000 := p(0, 0, 0)
	v001 := p(0, 0, h)
	v010 := p(0, d, 0)
	v011 := p(0, d, h)
	v100 := p(w, 0, 0)
	v101 := p(w, 0, h)
	v110 := p(w, d, 0)
	v111 := p(w, d, h)

	return &demoMesh{Triangles: []demoTriangle{
		tri(v000, v010, v001), tri(v001, v010, v011), // -X face
		tri(v100, v101, v110), tri(v101, v111, v110), // +X face
		tri(v000, v001, v100), tri(v001, v101, v100), // -Y face
		tri(v010, v110, v011), tri(v011, v110, v111), // +Y face
		tri(v000, v100, v010), tri(v010, v100, v110), // -Z face
		tri(v001, v011, v101), tri(v011, v111, v101), // +Z face
	}}
}
