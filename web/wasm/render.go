package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/cwbudde/wav"
	"github.com/go-audio/audio"
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
	pcmBitDepth              = 16
	pcmMonoChannels          = 1
	pcmAudioFormat           = 1
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
	Width  float64 `json:"width"`
	Depth  float64 `json:"depth"`
	Height float64 `json:"height"`
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
	SampleRate      int
	DurationSeconds float64
	EarlyEventCount int
	NumRays         int
	PeakAmplitude   float64
	RMSAmplitude    float64
	FirstArrivalMs  float64
	RenderMS        float64
	Samples         []float32
	WAVBytes        []byte
}

func defaultDemoRequest() demoRequest {
	return demoRequest{
		Room: demoRoom{Width: 6.4, Depth: 4.8, Height: 2.9},
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
	request := defaultDemoRequest()
	if strings.TrimSpace(payload) != "" {
		if err := json.Unmarshal([]byte(payload), &request); err != nil {
			return demoResult{}, fmt.Errorf("decode demo request: %w", err)
		}
	}

	return runDemoRender(request)
}

func runDemoRender(request demoRequest) (demoResult, error) {
	started := time.Now()

	normalized, err := normalizeDemoRequest(request)
	if err != nil {
		return demoResult{}, err
	}

	sc, err := buildDemoScene(normalized)
	if err != nil {
		return demoResult{}, err
	}

	renderCfg := ir.RenderConfig{
		SampleRate:      sc.SampleRate,
		DurationSeconds: normalized.Render.DurationSeconds,
		BandSpec:        sc.BandSpec,
	}

	var (
		buffer      *ir.Buffer
		earlyEvents []ir.Event
	)

	switch normalized.Render.Mode {
	case "early":
		earlyEvents, err = solveEarly(sc, normalized.Render.MaxOrder)
		if err != nil {
			return demoResult{}, err
		}

		buffer, err = ir.RenderMono(earlyEvents, renderCfg)
		if err != nil {
			return demoResult{}, fmt.Errorf("render early impulse response: %w", err)
		}
	case "late":
		buffer, err = renderLateBuffer(sc, normalized.Render.DurationSeconds, normalized.Render.NumRays, normalized.Render.MaxOrder)
		if err != nil {
			return demoResult{}, err
		}
	case "hybrid":
		earlyEvents, err = solveEarly(sc, normalized.Render.MaxOrder)
		if err != nil {
			return demoResult{}, err
		}

		earlyBuffer, renderErr := ir.RenderMono(earlyEvents, renderCfg)
		if renderErr != nil {
			return demoResult{}, fmt.Errorf("render early impulse response: %w", renderErr)
		}

		lateBuffer, renderErr := renderLateBuffer(sc, normalized.Render.DurationSeconds, normalized.Render.NumRays, normalized.Render.MaxOrder)
		if renderErr != nil {
			return demoResult{}, renderErr
		}

		buffer = hybrid.CombineBuffers(earlyBuffer, lateBuffer, hybrid.HybridConfig{
			CrossoverTimeSeconds: normalized.Render.CrossoverTimeSeconds,
			CrossoverMode:        hybrid.TimeBased,
			SmoothenCrossover:    true,
			CrossoverWindow: hybrid.FadeWindowConfig{
				Name:  normalized.Render.CrossoverWindow,
				Alpha: normalized.Render.CrossoverWindowAlpha,
			},
		})
		if buffer == nil {
			return demoResult{}, errors.New("combine hybrid buffers")
		}
	default:
		return demoResult{}, fmt.Errorf("unsupported render mode %q", normalized.Render.Mode)
	}

	wavBytes, err := encodeMonoWAVBytes(buffer)
	if err != nil {
		return demoResult{}, err
	}

	peak, rms, firstArrivalMs := analyzeSamples(buffer.Samples, buffer.SampleRate)
	floatSamples := make([]float32, len(buffer.Samples))
	for index, sample := range buffer.Samples {
		floatSamples[index] = float32(sample)
	}

	return demoResult{
		SampleRate:      buffer.SampleRate,
		DurationSeconds: normalized.Render.DurationSeconds,
		EarlyEventCount: len(earlyEvents),
		NumRays:         normalized.Render.NumRays,
		PeakAmplitude:   peak,
		RMSAmplitude:    rms,
		FirstArrivalMs:  firstArrivalMs,
		RenderMS:        float64(time.Since(started).Milliseconds()),
		Samples:         floatSamples,
		WAVBytes:        wavBytes,
	}, nil
}

func normalizeDemoRequest(request demoRequest) (demoRequest, error) {
	defaults := defaultDemoRequest()

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
	if err := hybrid.ValidateFadeWindowConfig(hybrid.FadeWindowConfig{
		Name:  request.Render.CrossoverWindow,
		Alpha: request.Render.CrossoverWindowAlpha,
	}); err != nil {
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
	for name, material := range materialLibrary {
		materials[name] = material
	}

	var directivityModel directivity.Model = directivity.OmniModel{}
	if request.Source.Directivity == "cardioid" {
		azimuthRadians := request.Source.AzimuthDegrees * math.Pi / 180
		axis := geometry.Vec3{X: math.Cos(azimuthRadians), Y: math.Sin(azimuthRadians)}.Normalize()
		if axis == geometry.Vec3Zero {
			axis = geometry.Vec3{X: 1}
		}

		directivityModel = directivity.CardioidModel{Axis: axis, OrderN: request.Source.CardioidOrder}
	}

	sc := &scene.Scene{
		Room: scene.Room{
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
		},
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

	if err := scene.Validate(sc); err != nil {
		return nil, fmt.Errorf("validate demo scene: %w", err)
	}

	return sc, nil
}

func solveEarly(sc *scene.Scene, maxOrder int) ([]ir.Event, error) {
	solver := ism.ISMSolver{}
	events, err := solver.Solve(sc, ism.ISMConfig{
		MaxOrder:     maxOrder,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     sc.BandSpec,
	})
	if err != nil {
		return nil, fmt.Errorf("solve early reflections: %w", err)
	}

	return events, nil
}

func renderLateBuffer(sc *scene.Scene, durationSeconds float64, numRays, maxOrder int) (*ir.Buffer, error) {
	maxBounces := maxOrder * 2
	if maxBounces < 1 {
		maxBounces = 1
	}

	tracer := raytrace.RayTracer{
		Config: raytrace.LaunchConfig{
			NumRays:        numRays,
			MaxBounces:     maxBounces,
			MaxTimeSeconds: durationSeconds,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:              sc,
		ReceiverRadius:     defaultReceiverRadius,
		BinDurationSeconds: defaultHistogramBinSecs,
	}
	histogram, err := tracer.Trace()
	if err != nil {
		return nil, fmt.Errorf("trace late field: %w", err)
	}

	return hybrid.HistogramToBuffer(histogram, sc.SampleRate), nil
}

func encodeMonoWAVBytes(buf *ir.Buffer) ([]byte, error) {
	if buf == nil {
		return nil, errors.New("buffer must not be nil")
	}
	if buf.SampleRate <= 0 {
		return nil, errors.New("buffer sample rate must be positive")
	}

	var output memoryWAVWriter
	encoder := wav.NewEncoder(&output, buf.SampleRate, pcmBitDepth, pcmMonoChannels, pcmAudioFormat)
	samples := make([]float32, len(buf.Samples))
	for index, sample := range buf.Samples {
		samples[index] = float32(sample)
	}

	if err := encoder.Write(&audio.Float32Buffer{
		Format: &audio.Format{NumChannels: pcmMonoChannels, SampleRate: buf.SampleRate},
		Data:   samples,
	}); err != nil {
		return nil, fmt.Errorf("write wav data: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close wav encoder: %w", err)
	}

	return output.Bytes(), nil
}

func analyzeSamples(samples []float64, sampleRate int) (peak, rms, firstArrivalMs float64) {
	if len(samples) == 0 || sampleRate <= 0 {
		return 0, 0, 0
	}

	firstArrivalIndex := -1
	var energySum float64
	for index, sample := range samples {
		magnitude := math.Abs(sample)
		if magnitude > peak {
			peak = magnitude
		}
		energySum += sample * sample
		if firstArrivalIndex < 0 && magnitude > 1e-9 {
			firstArrivalIndex = index
		}
	}

	rms = math.Sqrt(energySum / float64(len(samples)))
	if firstArrivalIndex >= 0 {
		firstArrivalMs = float64(firstArrivalIndex) * 1000 / float64(sampleRate)
	}

	return peak, rms, firstArrivalMs
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

func clamp(value, minValue, maxValue float64) float64 {
	if maxValue < minValue {
		maxValue = minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}

	return value
}

func clampInt(value, minValue, maxValue int) int {
	if maxValue < minValue {
		maxValue = minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}

	return value
}

type memoryWAVWriter struct {
	data []byte
	pos  int64
}

func (w *memoryWAVWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	end := w.pos + int64(len(p))
	if end > int64(len(w.data)) {
		grown := make([]byte, end)
		copy(grown, w.data)
		w.data = grown
	}

	copy(w.data[w.pos:end], p)
	w.pos = end
	return len(p), nil
}

func (w *memoryWAVWriter) Seek(offset int64, whence int) (int64, error) {
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = w.pos + offset
	case io.SeekEnd:
		next = int64(len(w.data)) + offset
	default:
		return 0, fmt.Errorf("unsupported seek whence %d", whence)
	}

	if next < 0 {
		return 0, fmt.Errorf("negative seek position %d", next)
	}

	w.pos = next
	return next, nil
}

func (w *memoryWAVWriter) Bytes() []byte {
	return append([]byte(nil), w.data...)
}
