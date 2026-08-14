//go:build js && wasm

package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"syscall/js"
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

	if result.FirstArrivalMs < 0 {
		t.Fatalf("FirstArrivalMs = %v, want >= 0", result.FirstArrivalMs)
	}

	if result.RenderMS < 0 {
		t.Fatalf("RenderMS = %v, want >= 0", result.RenderMS)
	}

	if got, want := len(result.SPLHeatmap.Samples), 6*heatmapColumns*heatmapRows; got != want {
		t.Fatalf("SPL heatmap sample count = %d, want %d", got, want)
	}
	if result.SPLHeatmap.MaximumDB != 0 {
		t.Fatalf("SPL heatmap maximum = %v, want 0 dB relative", result.SPLHeatmap.MaximumDB)
	}
	if result.SPLHeatmap.MinimumDB < heatmapFloorDB || result.SPLHeatmap.MinimumDB > 0 {
		t.Fatalf("SPL heatmap minimum = %v, want within [%v, 0]", result.SPLHeatmap.MinimumDB, heatmapFloorDB)
	}
}

func TestRunDemoPortalRenderReturnsCachedEndpointResponses(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Portal.Enabled = true
	request.Portal.Aperture = 0
	request.Render.Mode = "early"
	request.Render.MaxOrder = 1
	request.Render.DurationSeconds = 0.25

	result, err := runDemoRender(request)
	if err != nil {
		t.Fatalf("runDemoRender(portal) error = %v", err)
	}

	if result.PortalResponses == nil {
		t.Fatal("PortalResponses = nil, want closed/open endpoint WAVs")
	}
	if len(result.PortalResponses.ClosedWAVBytes) <= 44 || len(result.PortalResponses.OpenWAVBytes) <= 44 {
		t.Fatalf("portal endpoint WAV sizes = %d/%d, want encoded samples",
			len(result.PortalResponses.ClosedWAVBytes), len(result.PortalResponses.OpenWAVBytes))
	}
	if !bytes.Equal(result.WAVBytes, result.PortalResponses.ClosedWAVBytes) {
		t.Fatal("zero-aperture preview WAV differs from cached closed response")
	}
	if bytes.Equal(
		result.PortalResponses.ClosedWAVBytes[44:],
		result.PortalResponses.OpenWAVBytes[44:],
	) {
		t.Fatal("closed and open portal WAV payloads are identical")
	}

	for name, wav := range map[string][]byte{
		"preview": result.WAVBytes,
		"closed":  result.PortalResponses.ClosedWAVBytes,
		"open":    result.PortalResponses.OpenWAVBytes,
	} {
		if got := string(wav[:4]); got != "RIFF" {
			t.Errorf("%s WAV signature = %q, want RIFF", name, got)
		}
		if channels := binary.LittleEndian.Uint16(wav[22:24]); channels != 2 {
			t.Errorf("%s WAV channels = %d, want stereo", name, channels)
		}
	}

	jsResult := demoResultToJS(result)
	responses := jsResult.Get("portalResponses")
	if responses.Type() != js.TypeObject {
		t.Fatalf("portalResponses JS type = %s, want object", responses.Type())
	}
	if got := responses.Get("closedWavBytes").Length(); got != len(result.PortalResponses.ClosedWAVBytes) {
		t.Errorf("closedWavBytes JS length = %d, want %d", got, len(result.PortalResponses.ClosedWAVBytes))
	}
	if got := responses.Get("openWavBytes").Length(); got != len(result.PortalResponses.OpenWAVBytes) {
		t.Errorf("openWavBytes JS length = %d, want %d", got, len(result.PortalResponses.OpenWAVBytes))
	}
}

func TestNormalizeDemoPortalRequestUsesBrowserMaterialKeysAndDefaults(t *testing.T) {
	t.Parallel()

	for _, material := range []string{"glassPartition", "openDoorway"} {
		request := defaultDemoRequest()
		request.Portal.Enabled = true
		request.Portal.Material = material
		request.Portal.Opening = demoOpening{}

		normalized, _, err := normalizeDemoRequest(request)
		if err != nil {
			t.Fatalf("normalizeDemoRequest(%s) error = %v", material, err)
		}
		if normalized.Portal.Material != material {
			t.Errorf("normalized portal material = %q, want %q", normalized.Portal.Material, material)
		}
		if normalized.Portal.Opening.Width != defaultDemoRequest().Portal.Opening.Width ||
			normalized.Portal.Opening.Height != defaultDemoRequest().Portal.Opening.Height {
			t.Errorf("normalized opening = %#v, want defaults %#v",
				normalized.Portal.Opening, defaultDemoRequest().Portal.Opening)
		}
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

// TestRunDemoRenderMeshRoomSupportsAllModes verifies that mesh rooms support
// early, late, and hybrid rendering modes (not forced to late).
func TestRunDemoRenderMeshRoomSupportsAllModes(t *testing.T) {
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

		if result.Mode != requested {
			t.Errorf("requested=%q: result.Mode = %q, want %q",
				requested, result.Mode, requested)
		}

		if result.PeakAmplitude <= 0 {
			t.Errorf("requested=%q: PeakAmplitude = %v, want > 0", requested, result.PeakAmplitude)
		}

		// Early and hybrid modes should produce ISM events for mesh rooms.
		if requested == "early" || requested == "hybrid" {
			if result.EarlyEventCount == 0 {
				t.Errorf("requested=%q: EarlyEventCount = 0, want > 0 for mesh ISM",
					requested)
			}
		}
	}
}

// TestRunDemoRenderGainDbScalesPeakAmplitude verifies that the GainDB source
// parameter is no longer absorbed by peak normalization. A 12 dB drop must
// reduce the peak amplitude by approximately 4× (10^(−12/20) ≈ 0.25 in
// pressure).
func TestRunDemoRenderGainDbScalesPeakAmplitude(t *testing.T) {
	t.Parallel()

	base := defaultDemoRequest()
	base.Render.Mode = "late"
	base.Render.NumRays = 512
	base.Render.DurationSeconds = 0.5

	base.Source.GainDB = 0
	result0, err := runDemoRender(base)
	if err != nil {
		t.Fatalf("gainDb=0: error = %v", err)
	}

	base.Source.GainDB = -12
	result12, err := runDemoRender(base)
	if err != nil {
		t.Fatalf("gainDb=-12: error = %v", err)
	}

	if result0.PeakAmplitude <= 0 {
		t.Fatalf("gainDb=0: PeakAmplitude = %v, want > 0", result0.PeakAmplitude)
	}

	if result12.PeakAmplitude <= 0 {
		t.Fatalf("gainDb=-12: PeakAmplitude = %v, want > 0", result12.PeakAmplitude)
	}

	// Energy ∝ 10^(GainDB/10), pressure ∝ 10^(GainDB/20).
	// Expected ratio ≈ 0.25; allow generous tolerance of 0.10–0.50.
	ratio := result12.PeakAmplitude / result0.PeakAmplitude
	if ratio >= 0.50 {
		t.Errorf("peak ratio (gainDb=-12)/(gainDb=0) = %.3f, want < 0.50 (expected ≈0.25): gain has no effect",
			ratio)
	}

	if ratio < 0.10 {
		t.Errorf("peak ratio (gainDb=-12)/(gainDb=0) = %.3f, want ≥ 0.10: drop is too extreme",
			ratio)
	}
}

// TestRunDemoRenderCardioidDirectionAffectsPeak verifies that pointing a
// cardioid source toward the receiver produces a substantially higher peak than
// pointing it away. The default source→receiver azimuth is ≈16°; −164° is the
// opposite direction. Uses early-reflections mode for deterministic results.
func TestRunDemoRenderCardioidDirectionAffectsPeak(t *testing.T) {
	t.Parallel()

	base := defaultDemoRequest()
	base.Render.Mode = "early"
	base.Render.MaxOrder = 3
	base.Render.DurationSeconds = 0.4
	base.Source.Directivity = "cardioid"
	base.Source.CardioidOrder = 1.5

	// ≈16°: on-axis toward receiver.
	base.Source.AzimuthDegrees = 16
	toward, err := runDemoRender(base)
	if err != nil {
		t.Fatalf("toward: error = %v", err)
	}

	// −164°: 180° off-axis, away from receiver.
	base.Source.AzimuthDegrees = -164
	away, err := runDemoRender(base)
	if err != nil {
		t.Fatalf("away: error = %v", err)
	}

	if toward.PeakAmplitude <= 0 {
		t.Fatalf("toward: PeakAmplitude = %v, want > 0", toward.PeakAmplitude)
	}

	// Direct path gain ≈ 1 toward, ≈ 0 away. Wall reflections from perpendicular
	// surfaces blur the difference, so 1.5× is the reliable threshold.
	if toward.PeakAmplitude <= away.PeakAmplitude*1.5 {
		t.Errorf("toward peak %v ≤ 1.5× away peak %v: cardioid azimuth has no effect",
			toward.PeakAmplitude, away.PeakAmplitude)
	}
}

// TestRunDemoRenderCardioidOrderAffectsPeak verifies that a higher cardioid
// order (sharper focus) attenuates off-axis energy more strongly. The source is
// aimed 90° away from the receiver (broadside), so a high-order cardioid
// suppresses the receiver far more than a low-order one.
func TestRunDemoRenderCardioidOrderAffectsPeak(t *testing.T) {
	t.Parallel()

	base := defaultDemoRequest()
	base.Render.Mode = "early"
	base.Render.MaxOrder = 3
	base.Render.DurationSeconds = 0.4
	base.Source.Directivity = "cardioid"
	// 106° = 16° (toward receiver) + 90°: receiver sits 90° off the cardioid axis.
	base.Source.AzimuthDegrees = 106

	base.Source.CardioidOrder = 0.25 // nearly omni
	broad, err := runDemoRender(base)
	if err != nil {
		t.Fatalf("order=0.25: error = %v", err)
	}

	base.Source.CardioidOrder = 2.5 // sharply focused
	focused, err := runDemoRender(base)
	if err != nil {
		t.Fatalf("order=2.5: error = %v", err)
	}

	if broad.PeakAmplitude <= 0 {
		t.Fatalf("order=0.25: PeakAmplitude = %v, want > 0", broad.PeakAmplitude)
	}

	// At 90° off-axis: gain(0.25) = 0.5^0.25 ≈ 0.84; gain(2.5) = 0.5^2.5 ≈ 0.18.
	// Wall reflections blur the theoretical ratio; 1.5× is the reliable threshold.
	if broad.PeakAmplitude <= focused.PeakAmplitude*1.5 {
		t.Errorf("order=0.25 peak %v ≤ 1.5× order=2.5 peak %v: cardioid order has no effect",
			broad.PeakAmplitude, focused.PeakAmplitude)
	}
}

// TestRunDemoRenderEarlyIRContainsNegativeSamples verifies that the early
// impulse response for the default shoebox demo contains at least some negative
// samples.
//
// Physically, wall reflections can invert polarity: the Wayverb pressure
// reflectance model produces negative values at grazing angles, especially for
// high-absorption surfaces. The rendered IR must preserve these sign changes,
// not collapse everything to positive values.
//
// This test uses the default demo preset and varies source/receiver positions
// and materials. If none of the configurations produce negative samples, the
// broadband band-gain averaging is washing out the per-band phase inversions.
func TestRunDemoRenderEarlyIRContainsNegativeSamples(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		modify func(*demoRequest)
	}{
		{
			name: "default preset",
			modify: func(req *demoRequest) {
				req.Render.MaxOrder = 6
			},
		},
		{
			name: "high-absorption walls, grazing geometry",
			modify: func(req *demoRequest) {
				req.Materials = demoMaterials{
					West:    "pileCarpet",
					East:    "pileCarpet",
					South:   "pileCarpet",
					North:   "pileCarpet",
					Floor:   "pileCarpet",
					Ceiling: "pileCarpet",
				}
				req.Source.X = 0.5
				req.Source.Y = 2.4
				req.Source.Z = 0.3
				req.Receiver.X = 5.9
				req.Receiver.Y = 2.4
				req.Receiver.Z = 0.3
				req.Render.MaxOrder = 6
			},
		},
		{
			name: "heavy curtain walls, opposite corners",
			modify: func(req *demoRequest) {
				req.Materials = demoMaterials{
					West:    "heavyCurtain",
					East:    "heavyCurtain",
					South:   "heavyCurtain",
					North:   "heavyCurtain",
					Floor:   "heavyCurtain",
					Ceiling: "heavyCurtain",
				}
				req.Source.X = 0.4
				req.Source.Y = 0.4
				req.Source.Z = 0.3
				req.Receiver.X = 6.0
				req.Receiver.Y = 4.4
				req.Receiver.Z = 0.3
				req.Render.MaxOrder = 8
			},
		},
	}

	for _, tc := range cases {
		req := defaultDemoRequest()
		req.Render.Mode = "early"
		req.Render.DurationSeconds = 0.5
		tc.modify(&req)

		result, err := runDemoRender(req)
		if err != nil {
			t.Fatalf("%s: runDemoRender() error = %v", tc.name, err)
		}

		if len(result.Samples) == 0 {
			t.Fatalf("%s: Samples is empty", tc.name)
		}

		var negativeCount int
		for _, sample := range result.Samples {
			if sample < 0 {
				negativeCount++
			}
		}

		if negativeCount == 0 {
			t.Errorf("%s: early IR has zero negative samples out of %d total: "+
				"pressure reflectance sign changes are lost (all samples >= 0)",
				tc.name, len(result.Samples))
		} else {
			t.Logf("%s: %d negative samples out of %d (%.1f%%)",
				tc.name, negativeCount, len(result.Samples),
				100*float64(negativeCount)/float64(len(result.Samples)))
		}
	}
}

// TestRunDemoRenderLateIRHasBalancedPolarity verifies that the late field
// (ray-traced, noise-shaped) contains a healthy mix of positive and negative
// samples — unlike the early ISM output which is almost entirely positive.
//
// ToLateMono shapes energy into uniform noise: scale * (2*rand - 1), so
// roughly 50 % of samples should be negative. If this test fails, the noise
// shaping or energy conversion has been broken.
func TestRunDemoRenderLateIRHasBalancedPolarity(t *testing.T) {
	t.Parallel()

	req := defaultDemoRequest()
	req.Render.Mode = "late"
	req.Render.NumRays = 512
	req.Render.DurationSeconds = 0.5

	result, err := runDemoRender(req)
	if err != nil {
		t.Fatalf("runDemoRender() error = %v", err)
	}

	if len(result.Samples) == 0 {
		t.Fatal("Samples is empty")
	}

	var negativeCount int
	for _, sample := range result.Samples {
		if sample < 0 {
			negativeCount++
		}
	}

	negativeFraction := float64(negativeCount) / float64(len(result.Samples))

	// Noise shaping produces ~50% negative samples within active bins. The
	// overall fraction is lower because leading silence (before the first ray
	// arrival) contributes zero-valued samples. Allow >= 10%.
	if negativeFraction < 0.10 {
		t.Errorf("late IR has only %.1f%% negative samples (%d/%d): "+
			"expected roughly balanced polarity from noise shaping",
			negativeFraction*100, negativeCount, len(result.Samples))
	}

	t.Logf("late IR: %d negative samples out of %d (%.1f%%)",
		negativeCount, len(result.Samples), negativeFraction*100)
}

// TestRunDemoRenderEarlyAndLateBothHaveBalancedPolarity verifies that both
// early (ISM with per-band filtering) and late (ray-traced noise shaping)
// produce a healthy mix of positive and negative samples. Per-band rendering
// preserves frequency-dependent phase inversions from the pressure reflectance
// model, so both paths should have >= 10% negative samples.
func TestRunDemoRenderEarlyAndLateBothHaveBalancedPolarity(t *testing.T) {
	t.Parallel()

	base := defaultDemoRequest()
	base.Render.DurationSeconds = 0.5
	base.Render.NumRays = 512
	base.Render.MaxOrder = 4

	negativeFraction := func(samples []float32) float64 {
		neg := 0
		for _, s := range samples {
			if s < 0 {
				neg++
			}
		}
		return float64(neg) / float64(len(samples))
	}

	for _, mode := range []string{"early", "late"} {
		req := base
		req.Render.Mode = mode

		result, err := runDemoRender(req)
		if err != nil {
			t.Fatalf("%s: error = %v", mode, err)
		}

		frac := negativeFraction(result.Samples)
		t.Logf("%s: %.2f%% negative samples", mode, frac*100)

		if frac < 0.10 {
			t.Errorf("%s: only %.2f%% negative samples, want >= 10%%", mode, frac*100)
		}
	}
}

// TestRunDemoRenderShoeboxVsNearShoeboxMeshPolarity compares the polarity
// distribution of two acoustically near-identical rooms:
//
//   - A perfect shoebox (6.4 × 4.8 × 2.9 m) rendered via the ISM solver.
//   - A mesh box with one dimension 1 mm larger (6.401 × 4.8 × 2.9 m)
//     rendered via the ray tracer.
//
// Both paths should produce balanced polarity (>= 10% negative samples):
// the ISM path via per-band filtering that preserves phase inversions, and
// the ray-trace path via noise shaping.
func TestRunDemoRenderShoeboxVsNearShoeboxMeshPolarity(t *testing.T) {
	t.Parallel()

	negativeFraction := func(samples []float32) float64 {
		neg := 0
		for _, s := range samples {
			if s < 0 {
				neg++
			}
		}
		return float64(neg) / float64(len(samples))
	}

	// Perfect shoebox — uses ISM solver (early mode).
	shoeboxReq := defaultDemoRequest()
	shoeboxReq.Render.Mode = "early"
	shoeboxReq.Render.MaxOrder = 4
	shoeboxReq.Render.DurationSeconds = 0.5

	shoeboxResult, err := runDemoRender(shoeboxReq)
	if err != nil {
		t.Fatalf("shoebox: error = %v", err)
	}

	// Near-shoebox mesh — one wall offset by 1 mm. Forces the ray-trace path.
	meshReq := defaultDemoRequest()
	meshReq.Room = demoRoom{
		Kind:   "mesh",
		Width:  6.4,
		Depth:  4.8,
		Height: 2.9,
		Mesh:   smallCubeMeshRequest(6.401, 4.8, 2.9),
	}
	meshReq.Render.Mode = "late" // mesh rooms use ray tracing
	meshReq.Render.NumRays = 2048
	meshReq.Render.DurationSeconds = 0.5

	meshResult, err := runDemoRender(meshReq)
	if err != nil {
		t.Fatalf("mesh: error = %v", err)
	}

	shoeboxNeg := negativeFraction(shoeboxResult.Samples)
	meshNeg := negativeFraction(meshResult.Samples)

	t.Logf("shoebox (ISM):        %.2f%% negative samples", shoeboxNeg*100)
	t.Logf("near-shoebox (mesh):  %.2f%% negative samples", meshNeg*100)

	// Both paths must produce balanced polarity.
	if shoeboxNeg < 0.10 {
		t.Errorf("shoebox IR has only %.2f%% negative samples: "+
			"per-band filtering should preserve phase inversions", shoeboxNeg*100)
	}

	if meshNeg < 0.10 {
		t.Errorf("near-shoebox mesh IR has only %.2f%% negative samples: "+
			"expected >= 10%% from noise shaping", meshNeg*100)
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
