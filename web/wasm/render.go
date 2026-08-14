//go:build js && wasm

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/internal/pipeline"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

const (
	defaultDemoSampleRate    = 48000
	defaultDemoMode          = "hybrid"
	defaultDemoMaxOrder      = 4
	defaultDemoNumRays       = 3072
	defaultDemoDurationSecs  = 1.35
	defaultDemoCrossoverSecs = 0.22
	defaultDemoWindowName    = "hann"
	defaultReceiverRadius    = 0.25
	defaultHistogramBinSecs  = 0.01
	positionMarginMeters     = 0.15
)

// The request envelope lives in limits.go: the quality knobs are clamped to it
// and may be reduced further by applyDemoMemoryBudget, while the structural
// limits are hard, because shrinking geometry would change the room rather than
// its fidelity.

var materialLibrary = map[string]scene.Material{
	"defaultMaterial": {
		Name:             "defaultMaterial",
		AbsorptionByBand: []float64{0.12, 0.12, 0.18, 0.24, 0.28, 0.32},
		ScatteringByBand: []float64{0.04, 0.05, 0.06, 0.07, 0.08, 0.09},
	},
	"smoothConcrete": {
		Name:             "smoothConcrete",
		AbsorptionByBand: []float64{0.02, 0.02, 0.03, 0.04, 0.05, 0.07},
		ScatteringByBand: []float64{0.01, 0.01, 0.01, 0.02, 0.02, 0.03},
	},
	"plywoodPanels": {
		Name:             "plywoodPanels",
		AbsorptionByBand: []float64{0.16, 0.14, 0.12, 0.1, 0.09, 0.08},
		ScatteringByBand: []float64{0.06, 0.07, 0.08, 0.09, 0.1, 0.12},
	},
	"glassWindow": {
		Name:             "glassWindow",
		AbsorptionByBand: []float64{0.18, 0.08, 0.05, 0.03, 0.02, 0.02},
		ScatteringByBand: []float64{0.01, 0.01, 0.01, 0.02, 0.03, 0.04},
	},
	"pileCarpet": {
		Name:             "pileCarpet",
		AbsorptionByBand: []float64{0.08, 0.14, 0.32, 0.58, 0.7, 0.72},
		ScatteringByBand: []float64{0.05, 0.06, 0.07, 0.08, 0.08, 0.08},
	},
	"thinCarpet": {
		Name:             "thinCarpet",
		AbsorptionByBand: []float64{0.03, 0.05, 0.14, 0.24, 0.36, 0.39},
		ScatteringByBand: []float64{0.03, 0.04, 0.05, 0.06, 0.06, 0.06},
	},
	"heavyCurtain": {
		Name:             "heavyCurtain",
		AbsorptionByBand: []float64{0.06, 0.1, 0.22, 0.48, 0.66, 0.7},
		ScatteringByBand: []float64{0.08, 0.09, 0.1, 0.11, 0.12, 0.13},
	},
	"perforatedWood": {
		Name:             "perforatedWood",
		AbsorptionByBand: []float64{0.14, 0.22, 0.36, 0.42, 0.39, 0.3},
		ScatteringByBand: []float64{0.08, 0.09, 0.1, 0.12, 0.13, 0.14},
	},
}

var portalMaterialLibrary = map[string]scene.Material{
	"concretePartition": {Name: "concretePartition", AbsorptionByBand: []float64{0.02}, SoundReductionIndex: []float64{50}},
	"plasterboard":      {Name: "plasterboard", AbsorptionByBand: []float64{0.08}, SoundReductionIndex: []float64{35}},
	"woodenDoor":        {Name: "woodenDoor", AbsorptionByBand: []float64{0.08}, SoundReductionIndex: []float64{25}},
	"glassPartition":    {Name: "glassPartition", AbsorptionByBand: []float64{0.03}, SoundReductionIndex: []float64{30}},
	"openDoorway":       {Name: "openDoorway", AbsorptionByBand: []float64{0}, SoundReductionIndex: []float64{0}},
}

type demoRequest struct {
	Room      demoRoom      `json:"room"`
	Materials demoMaterials `json:"materials"`
	Source    demoSource    `json:"source"`
	Receiver  demoReceiver  `json:"receiver"`
	Portal    demoPortal    `json:"portal"`
	Render    demoRender    `json:"render"`
}

type demoPortal struct {
	Enabled      bool        `json:"enabled"`
	Aperture     float64     `json:"aperture"`
	RootOrder    float64     `json:"rootOrder"`
	Material     string      `json:"material"`
	ReceiverRoom demoRoom    `json:"receiverRoom"`
	Opening      demoOpening `json:"opening"`
}

type demoOpening struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Bottom float64 `json:"bottom"`
}

type demoRoom struct {
	Kind   string    `json:"kind"`
	Width  float64   `json:"width"`
	Depth  float64   `json:"depth"`
	Height float64   `json:"height"`
	Mesh   *demoMesh `json:"mesh,omitempty"`
}

type demoMesh struct {
	Triangles []demoTriangle `json:"triangles"`
}

type demoTriangle struct {
	V0 demoPoint `json:"v0"`
	V1 demoPoint `json:"v1"`
	V2 demoPoint `json:"v2"`
}

type demoPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type demoMaterials struct {
	West    string `json:"west"`
	East    string `json:"east"`
	South   string `json:"south"`
	North   string `json:"north"`
	Floor   string `json:"floor"`
	Ceiling string `json:"ceiling"`
}

type demoSource struct {
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Z              float64 `json:"z"`
	GainDB         float64 `json:"gainDb"`
	Directivity    string  `json:"directivity"`
	AzimuthDegrees float64 `json:"azimuthDegrees"`
	CardioidOrder  float64 `json:"cardioidOrder"`
}

type demoReceiver struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type demoRender struct {
	Mode                 string  `json:"mode"`
	MaxOrder             int     `json:"maxOrder"`
	NumRays              int     `json:"numRays"`
	DurationSeconds      float64 `json:"durationSeconds"`
	CrossoverTimeSeconds float64 `json:"crossoverTimeSeconds"`
	CrossoverWindow      string  `json:"crossoverWindow"`
	CrossoverWindowAlpha float64 `json:"crossoverWindowAlpha"`
}

type demoResult struct {
	Mode            string
	SampleRate      int
	DurationSeconds float64
	EarlyEventCount int
	NumRays         int
	PeakAmplitude   float64
	RMSAmplitude    float64
	FirstArrivalMs  float64
	RenderMS        float64
	SPLHeatmap      demoSPLHeatmap
	Samples         []float32
	WAVBytes        []byte
	PortalResponses *demoPortalResponses
	Warnings        []string
	Memory          demoMemoryStats
}

type demoPortalResponses struct {
	ClosedWAVBytes []byte
	OpenWAVBytes   []byte
}

func defaultDemoRequest() demoRequest {
	return demoRequest{
		Room: demoRoom{Kind: "shoebox", Width: 6.4, Depth: 4.8, Height: 2.9},
		Materials: demoMaterials{
			West:    "perforatedWood",
			East:    "glassWindow",
			South:   "smoothConcrete",
			North:   "defaultMaterial",
			Floor:   "pileCarpet",
			Ceiling: "heavyCurtain",
		},
		Source: demoSource{
			X:              1.4,
			Y:              1.9,
			Z:              1.25,
			GainDB:         0,
			Directivity:    "omni",
			AzimuthDegrees: 18,
			CardioidOrder:  1.15,
		},
		Receiver: demoReceiver{X: 4.85, Y: 2.9, Z: 1.2},
		Portal: demoPortal{
			RootOrder:    2,
			Material:     "woodenDoor",
			ReceiverRoom: demoRoom{Kind: "shoebox", Width: 6.4, Depth: 4.8, Height: 2.9},
			Opening:      demoOpening{Width: 1.2, Height: 2.1},
		},
		Render: demoRender{
			Mode:                 defaultDemoMode,
			MaxOrder:             defaultDemoMaxOrder,
			NumRays:              defaultDemoNumRays,
			DurationSeconds:      defaultDemoDurationSecs,
			CrossoverTimeSeconds: defaultDemoCrossoverSecs,
			CrossoverWindow:      defaultDemoWindowName,
			CrossoverWindowAlpha: 0,
		},
	}
}

func runDemoRenderJSON(payload string) (demoResult, error) {
	request := currentDemoAPIState.snapshotRequest()
	if strings.TrimSpace(payload) != "" {
		err := json.Unmarshal([]byte(payload), &request)
		if err != nil {
			return demoResult{}, fmt.Errorf("decode demo request: %w", err)
		}
	}

	return runDemoRender(request)
}

func runDemoRender(request demoRequest) (demoResult, error) {
	started := time.Now()

	return runDemoRenderWithDeadline(request, started, newRenderDeadline(started))
}

// runDemoRenderWithDeadline is runDemoRender with the wall-clock budget passed
// in, so tests can drive the timeout path without waiting out the real budget.
func runDemoRenderWithDeadline(
	request demoRequest, started time.Time, deadline renderDeadline,
) (demoResult, error) {
	reportDemoProgress("prepare", 2, "Preparing demo request")

	normalized, warnings, err := normalizeDemoRequest(request)
	if err != nil {
		return demoResult{}, err
	}
	if demoCancelled() {
		return demoResult{}, errRenderCancelled
	}
	if normalized.Portal.Enabled {
		return runDemoPortalRender(normalized, warnings, started, deadline)
	}

	reportDemoProgress("scene", 6, "Building room scene")
	sc, err := buildDemoScene(normalized)
	if err != nil {
		return demoResult{}, err
	}
	if demoCancelled() {
		return demoResult{}, errRenderCancelled
	}

	reportDemoTierStatistics(sc, started)

	preview := renderDemoPreviewTier(sc, normalized, started)

	// The deadline may already have passed while the preview ran. Starting the
	// full render anyway would blow the budget by its entire duration, which at
	// the top of the envelope is far longer than the budget itself.
	if deadline.exceeded() && preview != nil {
		return finishDemoResult(sc, previewTierRequest(normalized), preview, warnings, started,
			timeoutWarning(tierPreview, time.Since(started)))
	}

	buffer, earlyEvents, err := renderDemoMono(sc, normalized, deadline)
	if err != nil {
		if errors.Is(err, errRenderDeadlineExceeded) && preview != nil {
			return finishDemoResult(sc, previewTierRequest(normalized), preview, warnings, started,
				timeoutWarning(tierPreview, time.Since(started)))
		}

		return demoResult{}, err
	}

	return finishDemoResult(sc, normalized, &demoTierBuffer{
		buffer:          buffer,
		earlyEventCount: len(earlyEvents),
	}, warnings, started, "")
}

// demoTierBuffer is a rendered tier: the impulse response plus the one figure
// that cannot be recovered from it afterwards.
type demoTierBuffer struct {
	buffer          *ir.Buffer
	earlyEventCount int
}

// renderDemoMono runs the mono render for one set of quality knobs.
//
// Both the preview tier and the full render go through here, so the only thing
// separating them is request.Render — the scene, and therefore the room being
// simulated, is shared.
func renderDemoMono(sc *scene.Scene, request demoRequest, deadline renderDeadline) (*ir.Buffer, []ir.Event, error) {
	// Checked up front as well as between stages: "late" runs a single solver
	// call with no interior boundary, so without this an already-spent budget
	// would only be noticed once the whole trace had finished.
	err := checkDemoRenderAborted(deadline)
	if err != nil {
		return nil, nil, err
	}

	renderCfg := ir.RenderConfig{
		SampleRate:      sc.SampleRate,
		DurationSeconds: request.Render.DurationSeconds,
		BandSpec:        sc.BandSpec,
	}

	earlyCfg := pipeline.EarlyConfig{MaxOrder: request.Render.MaxOrder}
	lateCfg := pipeline.LateConfig{
		NumRays:            request.Render.NumRays,
		MaxOrder:           request.Render.MaxOrder,
		DurationSeconds:    request.Render.DurationSeconds,
		ReceiverRadius:     defaultReceiverRadius,
		BinDurationSeconds: defaultHistogramBinSecs,
	}

	var (
		buffer      *ir.Buffer
		earlyEvents []ir.Event
	)

	switch request.Render.Mode {
	case "early":
		reportDemoProgress("early", 25, "Tracing early reflections")
		earlyEvents, err = pipeline.SolveEarly(sc, earlyCfg)
		if err != nil {
			return nil, nil, err
		}
		if err = checkDemoRenderAborted(deadline); err != nil {
			return nil, nil, err
		}

		reportDemoProgress("render", 45, "Rendering early impulse response")
		buffer, err = ir.RenderMono(earlyEvents, renderCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("render early impulse response: %w", err)
		}
	case "late":
		reportDemoProgress("late", 35, "Tracing late field")
		buffer, err = pipeline.RenderLateBuffer(sc, lateCfg)
		if err != nil {
			return nil, nil, err
		}
	case "hybrid":
		reportDemoProgress("early", 22, "Tracing early reflections")
		earlyEvents, err = pipeline.SolveEarly(sc, earlyCfg)
		if err != nil {
			return nil, nil, err
		}
		if err = checkDemoRenderAborted(deadline); err != nil {
			return nil, nil, err
		}

		reportDemoProgress("render", 38, "Rendering early impulse response")
		earlyBuffer, renderErr := ir.RenderMono(earlyEvents, renderCfg)
		if renderErr != nil {
			return nil, nil, fmt.Errorf("render early impulse response: %w", renderErr)
		}

		reportDemoProgress("late", 58, "Tracing late field")
		lateBuffer, renderErr := pipeline.RenderLateBuffer(sc, lateCfg)
		if renderErr != nil {
			return nil, nil, renderErr
		}
		if err = checkDemoRenderAborted(deadline); err != nil {
			return nil, nil, err
		}

		reportDemoProgress("blend", 84, "Blending early and late responses")
		hybridCfg := hybrid.HybridConfig{
			CrossoverTimeSeconds: request.Render.CrossoverTimeSeconds,
			CrossoverMode:        hybrid.TimeBased,
			SmoothenCrossover:    true,
			CrossoverWindow: hybrid.FadeWindowConfig{
				Name:  request.Render.CrossoverWindow,
				Alpha: request.Render.CrossoverWindowAlpha,
			},
		}

		buffer, err = pipeline.RenderHybrid(earlyBuffer, lateBuffer, earlyEvents, hybridCfg)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("unsupported render mode %q", request.Render.Mode)
	}

	if err = checkDemoRenderAborted(deadline); err != nil {
		return nil, nil, err
	}

	return buffer, earlyEvents, nil
}

// renderDemoPreviewTier renders the coarse tier and pushes it to the page.
//
// It returns nil when a preview would not pay for itself or when it fails. A
// failed preview is deliberately not fatal: it is an optimisation of what the
// user sees, and the accurate render is still ahead.
//
// The preview is not subject to the render deadline. Its cost is bounded by
// construction — at most previewTierNumRays rays, previewTierMaxOrder
// reflections, and previewTierMaxDurationSecs of response — and it is the very
// result the deadline falls back to, so aborting it would leave the timeout with
// nothing to return. A user cancel still stops it, because cancellation and the
// deadline are separate conditions.
func renderDemoPreviewTier(sc *scene.Scene, request demoRequest, started time.Time) *demoTierBuffer {
	if !worthPreviewing(request) {
		return nil
	}

	previewRequest := previewTierRequest(request)

	reportDemoProgress("preview", 12, "Rendering preview")
	buffer, earlyEvents, err := renderDemoMono(sc, previewRequest, renderDeadline{})
	if err != nil || buffer == nil {
		return nil
	}

	tier := &demoTierBuffer{buffer: buffer, earlyEventCount: len(earlyEvents)}
	reportDemoTier(demoTierPayload{
		Tier:            tierPreview,
		ElapsedMS:       float64(time.Since(started).Milliseconds()),
		SampleRate:      buffer.SampleRate,
		DurationSeconds: previewRequest.Render.DurationSeconds,
		NumRays:         previewRequest.Render.NumRays,
		MaxOrder:        previewRequest.Render.MaxOrder,
		EarlyEventCount: tier.earlyEventCount,
		Samples:         bufferToFloat32(buffer),
	})

	// The preview's churn is charged to the same heap the full render is about
	// to use, so collect before handing it over.
	releaseDemoMemory()

	return tier
}

// reportDemoTierStatistics pushes Phase 18's Tier 1 estimates to the page. They
// need no simulation, so the user sees a decay time within milliseconds of
// pressing render.
func reportDemoTierStatistics(sc *scene.Scene, started time.Time) {
	statistics, ok := computeDemoStatistics(sc)
	if !ok {
		return
	}

	reportDemoTier(demoTierPayload{
		Tier:       tierStatistical,
		ElapsedMS:  float64(time.Since(started).Milliseconds()),
		Statistics: &statistics,
	})
}

// finishDemoResult turns a rendered buffer into the result the page consumes:
// surface heatmap, encoded WAV, waveform samples, and statistics.
//
// timeoutNotice is empty for a complete render and carries the explanation when
// the deadline forced a fall back to the preview tier. The stages here run past
// an expired deadline on purpose — abandoning a finished impulse response
// because encoding it would overshoot by a few hundred milliseconds would throw
// away the very result the fallback exists to deliver.
func finishDemoResult(
	sc *scene.Scene,
	request demoRequest,
	tier *demoTierBuffer,
	warnings []string,
	started time.Time,
	timeoutNotice string,
) (demoResult, error) {
	// The tracing stages are the churn-heavy ones; collect before the heatmap
	// adds its own so the heap goal does not carry their peak forward.
	releaseDemoMemory()

	reportDemoProgress("heatmap", 91, "Sampling surface SPL")
	splHeatmap, err := buildDemoSPLHeatmap(sc)
	if err != nil {
		return demoResult{}, fmt.Errorf("build surface SPL heatmap: %w", err)
	}

	reportDemoProgress("encode", 96, "Encoding WAV")
	wavBytes, err := export.EncodeMonoWAVBytes(tier.buffer)
	if err != nil {
		return demoResult{}, err
	}
	if demoCancelled() {
		return demoResult{}, errRenderCancelled
	}

	stats := ir.Stats(tier.buffer)
	reportDemoProgress("finish", 100, "Done")

	if timeoutNotice != "" {
		warnings = append(warnings, timeoutNotice)
	}

	result := demoResult{
		Mode:            request.Render.Mode,
		SampleRate:      tier.buffer.SampleRate,
		DurationSeconds: request.Render.DurationSeconds,
		EarlyEventCount: tier.earlyEventCount,
		NumRays:         request.Render.NumRays,
		PeakAmplitude:   stats.PeakAmplitude,
		RMSAmplitude:    stats.RMSAmplitude,
		FirstArrivalMs:  stats.FirstArrivalMs,
		RenderMS:        float64(time.Since(started).Milliseconds()),
		SPLHeatmap:      splHeatmap,
		Samples:         bufferToFloat32(tier.buffer),
		WAVBytes:        wavBytes,
		Warnings:        warnings,
	}
	currentDemoAPIState.storeResult(result, tier.buffer)
	currentDemoAPIState.storeRequest(request)

	releaseDemoMemory()
	result.Memory = demoMemorySnapshot(estimateDemoMemoryBytes(request))

	return result, nil
}

// bufferToFloat32 narrows an impulse response for the browser, which draws and
// plays float32.
func bufferToFloat32(buffer *ir.Buffer) []float32 {
	if buffer == nil {
		return nil
	}

	samples := make([]float32, len(buffer.Samples))
	for index, sample := range buffer.Samples {
		samples[index] = float32(sample)
	}

	return samples
}

func runDemoPortalRender(
	request demoRequest, warnings []string, started time.Time, deadline renderDeadline,
) (demoResult, error) {
	reportDemoProgress("scene", 8, "Building connected-room scene")
	closedScene, err := buildDemoScene(request)
	if err != nil {
		return demoResult{}, err
	}

	reportDemoTierStatistics(closedScene, started)

	reportDemoProgress("closed", 16, "Rendering closed-portal response")
	closed, closedEvents, err := renderDemoPortalBRIR(closedScene, request)
	if err != nil {
		return demoResult{}, fmt.Errorf("render closed portal: %w", err)
	}
	if demoCancelled() {
		return demoResult{}, errRenderCancelled
	}

	// Each portal response is roughly 700 MiB of churn against a live set of a
	// few MiB. Collecting between them keeps the second response from inheriting
	// the heap goal the first one reached.
	releaseDemoMemory()

	// A connected-room render has no cheap preview tier — both endpoints are
	// full binaural renders — so the timeout falls back to the one endpoint that
	// did finish, used for both. The aperture crossfade then interpolates
	// between identical responses and becomes inert, which the warning says
	// plainly; a usable stereo response with an inert control still beats a
	// blocked worker and nothing to listen to.
	open, openEvents := closed, closedEvents
	timeoutNotice := ""

	if deadline.exceeded() {
		timeoutNotice = fmt.Sprintf(
			"render timeout: exceeded the %.0f s demo budget after %.1f s; "+
				"returning the closed-portal response for both endpoints, so the aperture control has no effect",
			demoRenderTimeout.Seconds(), time.Since(started).Seconds(),
		)
		warnings = append(warnings, timeoutNotice)
	} else {
		openScene := *closedScene
		openScene.Portals = append([]scene.Portal(nil), closedScene.Portals...)
		openScene.Portals[0].State = scene.PortalOpen

		reportDemoProgress("open", 56, "Rendering open-portal response")
		open, openEvents, err = renderDemoPortalBRIR(&openScene, request)
		if err != nil {
			return demoResult{}, fmt.Errorf("render open portal: %w", err)
		}
		if demoCancelled() {
			return demoResult{}, errRenderCancelled
		}
	}

	releaseDemoMemory()

	reportDemoProgress("blend", 88, "Applying aperture crossfade")
	cache, err := hybrid.NewPortalBRIRCache(closed, open)
	if err != nil {
		return demoResult{}, fmt.Errorf("cache portal responses: %w", err)
	}
	preview, err := cache.AtAperture(request.Portal.Aperture, request.Portal.RootOrder)
	if err != nil {
		return demoResult{}, fmt.Errorf("interpolate portal response: %w", err)
	}

	reportDemoProgress("encode", 96, "Encoding binaural WAV responses")
	closedWAV, err := export.EncodeStereoWAVBytes(closed.Left, closed.Right)
	if err != nil {
		return demoResult{}, fmt.Errorf("encode closed portal WAV: %w", err)
	}
	openWAV, err := export.EncodeStereoWAVBytes(open.Left, open.Right)
	if err != nil {
		return demoResult{}, fmt.Errorf("encode open portal WAV: %w", err)
	}
	previewWAV, err := export.EncodeStereoWAVBytes(preview.Left, preview.Right)
	if err != nil {
		return demoResult{}, fmt.Errorf("encode portal preview WAV: %w", err)
	}

	stats := ir.Stats(preview.Left)
	floatSamples := make([]float32, len(preview.Left.Samples))
	for index, sample := range preview.Left.Samples {
		floatSamples[index] = float32(sample)
	}

	result := demoResult{
		Mode:            request.Render.Mode,
		SampleRate:      preview.Left.SampleRate,
		DurationSeconds: request.Render.DurationSeconds,
		EarlyEventCount: max(closedEvents, openEvents),
		NumRays:         request.Render.NumRays,
		PeakAmplitude:   stats.PeakAmplitude,
		RMSAmplitude:    stats.RMSAmplitude,
		FirstArrivalMs:  stats.FirstArrivalMs,
		RenderMS:        float64(time.Since(started).Milliseconds()),
		Samples:         floatSamples,
		WAVBytes:        previewWAV,
		PortalResponses: &demoPortalResponses{
			ClosedWAVBytes: closedWAV,
			OpenWAVBytes:   openWAV,
		},
		Warnings: warnings,
	}
	currentDemoAPIState.storeResult(result, preview.Left)
	currentDemoAPIState.storeRequest(request)
	reportDemoProgress("finish", 100, "Done")

	releaseDemoMemory()
	result.Memory = demoMemorySnapshot(estimateDemoMemoryBytes(request))

	return result, nil
}

func renderDemoPortalBRIR(sc *scene.Scene, request demoRequest) (hybrid.BRIR, int, error) {
	renderCfg := ir.RenderConfig{
		SampleRate:      sc.SampleRate,
		DurationSeconds: request.Render.DurationSeconds,
		BandSpec:        sc.BandSpec,
	}
	earlyCfg := pipeline.EarlyConfig{MaxOrder: request.Render.MaxOrder}
	lateCfg := pipeline.LateConfig{
		NumRays:            request.Render.NumRays,
		MaxOrder:           request.Render.MaxOrder,
		DurationSeconds:    request.Render.DurationSeconds,
		ReceiverRadius:     defaultReceiverRadius,
		BinDurationSeconds: defaultHistogramBinSecs,
	}
	receiver := sc.Receivers[0]

	var earlyEvents []ir.Event
	if request.Render.Mode != "late" {
		var err error
		earlyEvents, err = pipeline.SolveEarly(sc, earlyCfg)
		if err != nil {
			return hybrid.BRIR{}, 0, err
		}
	}

	var earlyLeft, earlyRight *ir.Buffer
	if request.Render.Mode != "late" {
		headEvents := append([]ir.Event(nil), earlyEvents...)
		for index := range headEvents {
			headEvents[index].Direction = receiver.WorldToHeadDir(headEvents[index].Direction)
		}
		var err error
		earlyLeft, earlyRight, err = ir.RenderBinaural(headEvents, receiver.HRTF, renderCfg)
		if err != nil {
			return hybrid.BRIR{}, 0, fmt.Errorf("render early binaural response: %w", err)
		}
	}

	if request.Render.Mode == "early" {
		return hybrid.BRIR{Left: earlyLeft, Right: earlyRight}, len(earlyEvents), nil
	}

	lateLeft, lateRight, err := pipeline.RenderLateBinaural(sc, receiver, lateCfg)
	if err != nil {
		return hybrid.BRIR{}, 0, err
	}
	if request.Render.Mode == "late" {
		return hybrid.BRIR{Left: lateLeft, Right: lateRight}, 0, nil
	}

	hybridCfg := hybrid.HybridConfig{
		CrossoverTimeSeconds: request.Render.CrossoverTimeSeconds,
		CrossoverMode:        hybrid.TimeBased,
		SmoothenCrossover:    true,
		CrossoverWindow: hybrid.FadeWindowConfig{
			Name:  request.Render.CrossoverWindow,
			Alpha: request.Render.CrossoverWindowAlpha,
		},
	}
	left, err := pipeline.RenderHybrid(earlyLeft, lateLeft, earlyEvents, hybridCfg)
	if err != nil {
		return hybrid.BRIR{}, 0, fmt.Errorf("blend left portal response: %w", err)
	}
	right, err := pipeline.RenderHybrid(earlyRight, lateRight, earlyEvents, hybridCfg)
	if err != nil {
		return hybrid.BRIR{}, 0, fmt.Errorf("blend right portal response: %w", err)
	}

	return hybrid.BRIR{Left: left, Right: right}, len(earlyEvents), nil
}

func normalizeDemoRequest(request demoRequest) (demoRequest, []string, error) {
	defaults := defaultDemoRequest()

	switch strings.TrimSpace(request.Room.Kind) {
	case "", "shoebox":
		request.Room.Kind = "shoebox"
	case "mesh":
		request.Room.Kind = "mesh"
	default:
		request.Room.Kind = defaults.Room.Kind
	}

	if request.Room.Width <= 0 {
		request.Room.Width = defaults.Room.Width
	}

	if request.Room.Depth <= 0 {
		request.Room.Depth = defaults.Room.Depth
	}

	if request.Room.Height <= 0 {
		request.Room.Height = defaults.Room.Height
	}

	request.Room.Width = clamp(request.Room.Width, minDemoRoomMeters, maxDemoRoomMeters)
	request.Room.Depth = clamp(request.Room.Depth, minDemoRoomMeters, maxDemoRoomMeters)
	request.Room.Height = clamp(request.Room.Height, minDemoRoomMeters, maxDemoRoomMeters)

	err := validateDemoStructure(request)
	if err != nil {
		return demoRequest{}, nil, err
	}

	request.Materials.West = normalizeMaterialName(request.Materials.West, defaults.Materials.West)
	request.Materials.East = normalizeMaterialName(request.Materials.East, defaults.Materials.East)
	request.Materials.South = normalizeMaterialName(request.Materials.South, defaults.Materials.South)
	request.Materials.North = normalizeMaterialName(request.Materials.North, defaults.Materials.North)
	request.Materials.Floor = normalizeMaterialName(request.Materials.Floor, defaults.Materials.Floor)
	request.Materials.Ceiling = normalizeMaterialName(request.Materials.Ceiling, defaults.Materials.Ceiling)

	request.Source.Directivity = normalizeDirectivity(request.Source.Directivity, defaults.Source.Directivity)
	request.Source.GainDB = clamp(request.Source.GainDB, -24, 12)

	request.Source.AzimuthDegrees = clamp(request.Source.AzimuthDegrees, -180, 180)
	if request.Source.CardioidOrder <= 0 {
		request.Source.CardioidOrder = defaults.Source.CardioidOrder
	}

	request.Source.CardioidOrder = clamp(request.Source.CardioidOrder, 0.25, 2.5)

	if request.Portal.RootOrder <= 0 || math.IsNaN(request.Portal.RootOrder) || math.IsInf(request.Portal.RootOrder, 0) {
		request.Portal.RootOrder = defaults.Portal.RootOrder
	}
	request.Portal.RootOrder = clamp(request.Portal.RootOrder, 0.25, 8)
	if math.IsNaN(request.Portal.Aperture) || math.IsInf(request.Portal.Aperture, 0) {
		request.Portal.Aperture = defaults.Portal.Aperture
	}
	request.Portal.Aperture = clamp(request.Portal.Aperture, 0, 1)
	if _, ok := portalMaterialLibrary[request.Portal.Material]; !ok {
		request.Portal.Material = defaults.Portal.Material
	}
	if request.Portal.ReceiverRoom.Width <= 0 || math.IsNaN(request.Portal.ReceiverRoom.Width) || math.IsInf(request.Portal.ReceiverRoom.Width, 0) {
		request.Portal.ReceiverRoom.Width = request.Room.Width
	}
	if request.Portal.ReceiverRoom.Depth <= 0 || math.IsNaN(request.Portal.ReceiverRoom.Depth) || math.IsInf(request.Portal.ReceiverRoom.Depth, 0) {
		request.Portal.ReceiverRoom.Depth = request.Room.Depth
	}
	if request.Portal.ReceiverRoom.Height <= 0 || math.IsNaN(request.Portal.ReceiverRoom.Height) || math.IsInf(request.Portal.ReceiverRoom.Height, 0) {
		request.Portal.ReceiverRoom.Height = request.Room.Height
	}
	request.Portal.ReceiverRoom.Kind = "shoebox"
	sharedDepth := min(request.Room.Depth, request.Portal.ReceiverRoom.Depth)
	sharedHeight := min(request.Room.Height, request.Portal.ReceiverRoom.Height)
	if request.Portal.Opening.Width <= 0 || math.IsNaN(request.Portal.Opening.Width) || math.IsInf(request.Portal.Opening.Width, 0) {
		request.Portal.Opening.Width = defaults.Portal.Opening.Width
	}
	if request.Portal.Opening.Height <= 0 || math.IsNaN(request.Portal.Opening.Height) || math.IsInf(request.Portal.Opening.Height, 0) {
		request.Portal.Opening.Height = defaults.Portal.Opening.Height
	}
	if math.IsNaN(request.Portal.Opening.Bottom) || math.IsInf(request.Portal.Opening.Bottom, 0) {
		request.Portal.Opening.Bottom = defaults.Portal.Opening.Bottom
	}
	request.Portal.Opening.Width = clamp(request.Portal.Opening.Width, 0.25, sharedDepth)
	request.Portal.Opening.Height = clamp(request.Portal.Opening.Height, 0.25, sharedHeight)
	request.Portal.Opening.Bottom = clamp(request.Portal.Opening.Bottom, 0, sharedHeight-request.Portal.Opening.Height)
	if request.Portal.Enabled && request.Room.Kind != "shoebox" {
		return demoRequest{}, nil, errors.New("portal demo supports shoebox rooms only")
	}

	mode, err := normalizeMode(request.Render.Mode, defaults.Render.Mode)
	if err != nil {
		return demoRequest{}, nil, err
	}

	request.Render.Mode = mode

	if request.Render.MaxOrder <= 0 {
		request.Render.MaxOrder = defaults.Render.MaxOrder
	}

	request.Render.MaxOrder = clampInt(request.Render.MaxOrder, minDemoMaxOrder, maxDemoMaxOrder)
	if request.Render.NumRays <= 0 {
		request.Render.NumRays = defaults.Render.NumRays
	}

	request.Render.NumRays = clampInt(request.Render.NumRays, minDemoNumRays, maxDemoNumRays)
	if request.Render.DurationSeconds <= 0 {
		request.Render.DurationSeconds = defaults.Render.DurationSeconds
	}

	request.Render.DurationSeconds = clamp(request.Render.DurationSeconds, minDemoDurationSecs, maxDemoDurationSecs)
	if request.Render.CrossoverTimeSeconds <= 0 {
		request.Render.CrossoverTimeSeconds = defaults.Render.CrossoverTimeSeconds
	}

	request.Render.CrossoverTimeSeconds = clamp(request.Render.CrossoverTimeSeconds, 0.03, request.Render.DurationSeconds*0.85)
	if strings.TrimSpace(request.Render.CrossoverWindow) == "" {
		request.Render.CrossoverWindow = defaults.Render.CrossoverWindow
	}

	err = hybrid.ValidateFadeWindowConfig(hybrid.FadeWindowConfig{
		Name:  request.Render.CrossoverWindow,
		Alpha: request.Render.CrossoverWindowAlpha,
	})
	if err != nil {
		return demoRequest{}, nil, fmt.Errorf("invalid crossover window: %w", err)
	}

	request.Source.X = clamp(request.Source.X, positionMarginMeters, request.Room.Width-positionMarginMeters)
	request.Source.Y = clamp(request.Source.Y, positionMarginMeters, request.Room.Depth-positionMarginMeters)
	request.Source.Z = clamp(request.Source.Z, positionMarginMeters, request.Room.Height-positionMarginMeters)
	receiverRoom := request.Room
	if request.Portal.Enabled {
		receiverRoom = request.Portal.ReceiverRoom
	}
	request.Receiver.X = clamp(request.Receiver.X, positionMarginMeters, receiverRoom.Width-positionMarginMeters)
	request.Receiver.Y = clamp(request.Receiver.Y, positionMarginMeters, receiverRoom.Depth-positionMarginMeters)
	request.Receiver.Z = clamp(request.Receiver.Z, positionMarginMeters, receiverRoom.Height-positionMarginMeters)

	request, warnings := applyDemoMemoryBudget(request)

	return request, warnings, nil
}

func buildDemoScene(request demoRequest) (*scene.Scene, error) {
	if _, ok := materialLibrary[request.Materials.West]; !ok {
		return nil, fmt.Errorf("unknown west wall material %q", request.Materials.West)
	}

	materials := map[string]scene.Material{}
	maps.Copy(materials, materialLibrary)

	var directivityModel directivity.Model = directivity.OmniModel{}

	if request.Source.Directivity == "cardioid" {
		azimuthRadians := request.Source.AzimuthDegrees * math.Pi / 180

		axis := geometry.Vec3{X: math.Cos(azimuthRadians), Y: math.Sin(azimuthRadians)}.Normalize()
		if axis == geometry.Vec3Zero {
			axis = geometry.Vec3{X: 1}
		}

		directivityModel = directivity.CardioidModel{Axis: axis, OrderN: request.Source.CardioidOrder}
	}

	room := scene.Room{
		Kind: scene.RoomKindShoebox,
		Shoebox: &scene.Shoebox{
			Width:  request.Room.Width,
			Depth:  request.Room.Depth,
			Height: request.Room.Height,
			WallMaterials: [6]string{
				request.Materials.West,
				request.Materials.East,
				request.Materials.South,
				request.Materials.North,
				request.Materials.Floor,
				request.Materials.Ceiling,
			},
		},
	}

	if request.Room.Kind == "mesh" {
		mesh := buildDemoGeometryMesh(request.Room.Mesh)
		if mesh == nil {
			mesh = buildDemoLoftMesh(request.Room.Width, request.Room.Depth, request.Room.Height)
		}
		room = scene.Room{
			Kind:         scene.RoomKindMesh,
			Mesh:         mesh,
			MeshMaterial: request.Materials.West,
		}
	}

	sc := &scene.Scene{
		Room:       room,
		Materials:  materials,
		BandSpec:   acoustics.Octave6,
		SampleRate: defaultDemoSampleRate,
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: request.Source.X, Y: request.Source.Y, Z: request.Source.Z},
			GainDB:      request.Source.GainDB,
			Directivity: directivityModel,
		}},
		Receivers: []scene.Receiver{{
			Position: geometry.Vec3{X: request.Receiver.X, Y: request.Receiver.Y, Z: request.Receiver.Z},
			Type:     scene.ReceiverOmni,
		}},
	}

	if request.Portal.Enabled {
		portalMaterial := portalMaterialLibrary[request.Portal.Material]
		materials[request.Portal.Material] = portalMaterial
		receiverRoom := scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Origin: geometry.Vec3{X: request.Room.Width},
				Width:  request.Portal.ReceiverRoom.Width,
				Depth:  request.Portal.ReceiverRoom.Depth,
				Height: request.Portal.ReceiverRoom.Height,
				WallMaterials: [6]string{
					request.Materials.West,
					request.Materials.East,
					request.Materials.South,
					request.Materials.North,
					request.Materials.Floor,
					request.Materials.Ceiling,
				},
			},
		}
		y0 := (min(request.Room.Depth, request.Portal.ReceiverRoom.Depth) - request.Portal.Opening.Width) / 2
		z0 := request.Portal.Opening.Bottom
		x := request.Room.Width
		sc.Room = scene.Room{}
		sc.Rooms = []scene.Room{room, receiverRoom}
		sc.Portals = []scene.Portal{{
			RoomIndices: [2]int{0, 1},
			Polygon: []geometry.Vec3{
				{X: x, Y: y0, Z: z0},
				{X: x, Y: y0 + request.Portal.Opening.Width, Z: z0},
				{X: x, Y: y0 + request.Portal.Opening.Width, Z: z0 + request.Portal.Opening.Height},
				{X: x, Y: y0, Z: z0 + request.Portal.Opening.Height},
			},
			Material: request.Portal.Material,
			State:    scene.PortalClosed,
		}}
		sc.Receivers[0] = scene.Receiver{
			Position: geometry.Vec3{
				X: request.Room.Width + request.Receiver.X,
				Y: request.Receiver.Y,
				Z: request.Receiver.Z,
			},
			Type: scene.ReceiverBinaural,
			HRTF: demoHRTF(defaultDemoSampleRate),
		}
	}

	err := scene.Validate(sc)
	if err != nil {
		return nil, fmt.Errorf("validate demo scene: %w", err)
	}

	return sc, nil
}

func demoHRTF(sampleRate int) hrtf.Dataset {
	leftNear := []float64{1}
	rightFar := make([]float64, 9)
	rightFar[8] = 0.65
	rightNear := []float64{1}
	leftFar := make([]float64, 9)
	leftFar[8] = 0.65

	return hrtf.NearestNeighborDataset{
		SampleRateHz: sampleRate,
		Grid: &hrtf.MeasurementGrid{
			Directions: []geometry.Vec3{{X: -1}, {X: 1}, {Y: -1}, {Y: 1}},
			LeftHRIRs:  [][]float64{leftNear, leftFar, {1}, {1}},
			RightHRIRs: [][]float64{rightFar, rightNear, {1}, {1}},
		},
	}
}

func buildDemoLoftMesh(width, depth, height float64) *geometry.Mesh {
	if width <= 0 || depth <= 0 || height <= 0 {
		return &geometry.Mesh{}
	}

	v000 := geometry.Vec3{X: 0, Y: 0, Z: 0}
	v100 := geometry.Vec3{X: width, Y: 0, Z: 0}
	v010 := geometry.Vec3{X: width / 2, Y: depth, Z: 0}
	v001 := geometry.Vec3{X: 0, Y: 0, Z: height}
	v101 := geometry.Vec3{X: width, Y: 0, Z: height}
	v011 := geometry.Vec3{X: width / 2, Y: depth, Z: height}

	return &geometry.Mesh{Triangles: []geometry.Triangle{
		{V0: v000, V1: v010, V2: v100},
		{V0: v001, V1: v101, V2: v011},
		{V0: v000, V1: v100, V2: v101},
		{V0: v000, V1: v101, V2: v001},
		{V0: v100, V1: v010, V2: v011},
		{V0: v100, V1: v011, V2: v101},
		{V0: v010, V1: v000, V2: v001},
		{V0: v010, V1: v001, V2: v011},
	}}
}

func buildDemoGeometryMesh(mesh *demoMesh) *geometry.Mesh {
	if mesh == nil || len(mesh.Triangles) == 0 {
		return nil
	}

	out := &geometry.Mesh{Triangles: make([]geometry.Triangle, 0, len(mesh.Triangles))}
	for _, tri := range mesh.Triangles {
		out.Triangles = append(out.Triangles, geometry.Triangle{
			V0: geometry.Vec3{X: tri.V0.X, Y: tri.V0.Y, Z: tri.V0.Z},
			V1: geometry.Vec3{X: tri.V1.X, Y: tri.V1.Y, Z: tri.V1.Z},
			V2: geometry.Vec3{X: tri.V2.X, Y: tri.V2.Y, Z: tri.V2.Z},
		})
	}

	return out
}

func normalizeMaterialName(name, fallback string) string {
	trimmed := strings.TrimSpace(name)
	if _, ok := materialLibrary[trimmed]; ok {
		return trimmed
	}

	return fallback
}

func normalizeDirectivity(name, fallback string) string {
	switch strings.TrimSpace(name) {
	case "omni", "cardioid":
		return strings.TrimSpace(name)
	default:
		return fallback
	}
}

func normalizeMode(name, fallback string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fallback, nil
	}

	switch trimmed {
	case "early", "late", "hybrid":
		return trimmed, nil
	default:
		return "", fmt.Errorf("unsupported render mode %q", trimmed)
	}
}

func clamp(value, lo, hi float64) float64 {
	return max(lo, min(value, max(lo, hi)))
}

func clampInt(value, lo, hi int) int {
	return max(lo, min(value, max(lo, hi)))
}
