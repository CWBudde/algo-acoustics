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

type demoRequest struct {
	Room      demoRoom      `json:"room"`
	Materials demoMaterials `json:"materials"`
	Source    demoSource    `json:"source"`
	Receiver  demoReceiver  `json:"receiver"`
	Render    demoRender    `json:"render"`
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
	reportDemoProgress("prepare", 2, "Preparing demo request")

	normalized, err := normalizeDemoRequest(request)
	if err != nil {
		return demoResult{}, err
	}
	if demoCancelled() {
		return demoResult{}, errors.New("render cancelled")
	}

	reportDemoProgress("scene", 10, "Building room scene")
	sc, err := buildDemoScene(normalized)
	if err != nil {
		return demoResult{}, err
	}
	if demoCancelled() {
		return demoResult{}, errors.New("render cancelled")
	}

	renderCfg := ir.RenderConfig{
		SampleRate:      sc.SampleRate,
		DurationSeconds: normalized.Render.DurationSeconds,
		BandSpec:        sc.BandSpec,
	}

	earlyCfg := pipeline.EarlyConfig{MaxOrder: normalized.Render.MaxOrder}
	lateCfg := pipeline.LateConfig{
		NumRays:            normalized.Render.NumRays,
		MaxOrder:           normalized.Render.MaxOrder,
		DurationSeconds:    normalized.Render.DurationSeconds,
		ReceiverRadius:     defaultReceiverRadius,
		BinDurationSeconds: defaultHistogramBinSecs,
	}

	var (
		buffer      *ir.Buffer
		earlyEvents []ir.Event
	)

	switch normalized.Render.Mode {
	case "early":
		reportDemoProgress("early", 25, "Tracing early reflections")
		earlyEvents, err = pipeline.SolveEarly(sc, earlyCfg)
		if err != nil {
			return demoResult{}, err
		}
		if demoCancelled() {
			return demoResult{}, errors.New("render cancelled")
		}

		reportDemoProgress("render", 45, "Rendering early impulse response")
		buffer, err = ir.RenderMono(earlyEvents, renderCfg)
		if err != nil {
			return demoResult{}, fmt.Errorf("render early impulse response: %w", err)
		}
	case "late":
		reportDemoProgress("late", 35, "Tracing late field")
		buffer, err = pipeline.RenderLateBuffer(sc, lateCfg)
		if err != nil {
			return demoResult{}, err
		}
	case "hybrid":
		reportDemoProgress("early", 22, "Tracing early reflections")
		earlyEvents, err = pipeline.SolveEarly(sc, earlyCfg)
		if err != nil {
			return demoResult{}, err
		}
		if demoCancelled() {
			return demoResult{}, errors.New("render cancelled")
		}

		reportDemoProgress("render", 38, "Rendering early impulse response")
		earlyBuffer, renderErr := ir.RenderMono(earlyEvents, renderCfg)
		if renderErr != nil {
			return demoResult{}, fmt.Errorf("render early impulse response: %w", renderErr)
		}

		reportDemoProgress("late", 58, "Tracing late field")
		lateBuffer, renderErr := pipeline.RenderLateBuffer(sc, lateCfg)
		if renderErr != nil {
			return demoResult{}, renderErr
		}
		if demoCancelled() {
			return demoResult{}, errors.New("render cancelled")
		}

		reportDemoProgress("blend", 84, "Blending early and late responses")
		hybridCfg := hybrid.HybridConfig{
			CrossoverTimeSeconds: normalized.Render.CrossoverTimeSeconds,
			CrossoverMode:        hybrid.TimeBased,
			SmoothenCrossover:    true,
			CrossoverWindow: hybrid.FadeWindowConfig{
				Name:  normalized.Render.CrossoverWindow,
				Alpha: normalized.Render.CrossoverWindowAlpha,
			},
		}

		buffer, err = pipeline.RenderHybrid(earlyBuffer, lateBuffer, earlyEvents, hybridCfg)
		if err != nil {
			return demoResult{}, err
		}
	default:
		return demoResult{}, fmt.Errorf("unsupported render mode %q", normalized.Render.Mode)
	}

	if demoCancelled() {
		return demoResult{}, errors.New("render cancelled")
	}
	currentDemoAPIState.storeResult(demoResult{}, buffer)

	reportDemoProgress("heatmap", 91, "Sampling surface SPL")
	splHeatmap, err := buildDemoSPLHeatmap(sc)
	if err != nil {
		return demoResult{}, fmt.Errorf("build surface SPL heatmap: %w", err)
	}

	reportDemoProgress("encode", 96, "Encoding WAV")
	wavBytes, err := export.EncodeMonoWAVBytes(buffer)
	if err != nil {
		return demoResult{}, err
	}
	if demoCancelled() {
		return demoResult{}, errors.New("render cancelled")
	}

	stats := ir.Stats(buffer)
	if demoCancelled() {
		return demoResult{}, errors.New("render cancelled")
	}
	reportDemoProgress("finish", 100, "Done")

	floatSamples := make([]float32, len(buffer.Samples))
	for index, sample := range buffer.Samples {
		floatSamples[index] = float32(sample)
	}

	result := demoResult{
		Mode:            normalized.Render.Mode,
		SampleRate:      buffer.SampleRate,
		DurationSeconds: normalized.Render.DurationSeconds,
		EarlyEventCount: len(earlyEvents),
		NumRays:         normalized.Render.NumRays,
		PeakAmplitude:   stats.PeakAmplitude,
		RMSAmplitude:    stats.RMSAmplitude,
		FirstArrivalMs:  stats.FirstArrivalMs,
		RenderMS:        float64(time.Since(started).Milliseconds()),
		SPLHeatmap:      splHeatmap,
		Samples:         floatSamples,
		WAVBytes:        wavBytes,
	}
	currentDemoAPIState.storeResult(result, buffer)
	currentDemoAPIState.storeRequest(normalized)

	return result, nil
}

func normalizeDemoRequest(request demoRequest) (demoRequest, error) {
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

	mode, err := normalizeMode(request.Render.Mode, defaults.Render.Mode)
	if err != nil {
		return demoRequest{}, err
	}

	request.Render.Mode = mode

	if request.Render.MaxOrder <= 0 {
		request.Render.MaxOrder = defaults.Render.MaxOrder
	}

	request.Render.MaxOrder = clampInt(request.Render.MaxOrder, 1, 12)
	if request.Render.NumRays <= 0 {
		request.Render.NumRays = defaults.Render.NumRays
	}

	request.Render.NumRays = clampInt(request.Render.NumRays, 128, 16384)
	if request.Render.DurationSeconds <= 0 {
		request.Render.DurationSeconds = defaults.Render.DurationSeconds
	}

	request.Render.DurationSeconds = clamp(request.Render.DurationSeconds, 0.25, 3)
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
		return demoRequest{}, fmt.Errorf("invalid crossover window: %w", err)
	}

	request.Source.X = clamp(request.Source.X, positionMarginMeters, request.Room.Width-positionMarginMeters)
	request.Source.Y = clamp(request.Source.Y, positionMarginMeters, request.Room.Depth-positionMarginMeters)
	request.Source.Z = clamp(request.Source.Z, positionMarginMeters, request.Room.Height-positionMarginMeters)
	request.Receiver.X = clamp(request.Receiver.X, positionMarginMeters, request.Room.Width-positionMarginMeters)
	request.Receiver.Y = clamp(request.Receiver.Y, positionMarginMeters, request.Room.Depth-positionMarginMeters)
	request.Receiver.Z = clamp(request.Receiver.Z, positionMarginMeters, request.Room.Height-positionMarginMeters)

	return request, nil
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

	err := scene.Validate(sc)
	if err != nil {
		return nil, fmt.Errorf("validate demo scene: %w", err)
	}

	return sc, nil
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
