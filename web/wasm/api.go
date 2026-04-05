//go:build js && wasm

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/scene"
)

type demoAPIState struct {
	roomID     int
	request    demoRequest
	lastBuffer *ir.Buffer
	lastResult *demoResult
}

type demoParameters struct {
	T60            float64 `json:"t60"`
	EDT            float64 `json:"edt"`
	T20            float64 `json:"t20"`
	T30            float64 `json:"t30"`
	C50            float64 `json:"c50"`
	C80            float64 `json:"c80"`
	D50            float64 `json:"d50"`
	PeakAmplitude  float64 `json:"peakAmplitude"`
	RMSAmplitude   float64 `json:"rmsAmplitude"`
	FirstArrivalMs float64 `json:"firstArrivalMs"`
}

var currentDemoAPIState = newDemoAPIState()

func newDemoAPIState() *demoAPIState {
	request := defaultDemoRequest()
	return &demoAPIState{request: request}
}

func (s *demoAPIState) nextRoomID() string {
	s.roomID++
	return fmt.Sprintf("room-%d", s.roomID)
}

func (s *demoAPIState) snapshotRequest() demoRequest {
	if s == nil {
		return defaultDemoRequest()
	}

	return cloneDemoRequest(s.request)
}

func (s *demoAPIState) storeRequest(request demoRequest) {
	if s == nil {
		return
	}

	s.request = cloneDemoRequest(request)
}

func (s *demoAPIState) storeResult(result demoResult, buffer *ir.Buffer) {
	if s == nil {
		return
	}

	s.lastResult = &result
	s.lastBuffer = cloneIRBuffer(buffer)
}

func cloneDemoRequest(request demoRequest) demoRequest {
	cloned := request
	if request.Room.Mesh != nil {
		cloned.Room.Mesh = cloneDemoMesh(request.Room.Mesh)
	}

	return cloned
}

func cloneDemoMesh(mesh *demoMesh) *demoMesh {
	if mesh == nil {
		return nil
	}

	cloned := &demoMesh{Triangles: make([]demoTriangle, len(mesh.Triangles))}
	copy(cloned.Triangles, mesh.Triangles)
	return cloned
}

func cloneIRBuffer(buffer *ir.Buffer) *ir.Buffer {
	if buffer == nil {
		return nil
	}

	cloned := &ir.Buffer{
		SampleRate: buffer.SampleRate,
		Samples:    make([]float64, len(buffer.Samples)),
	}
	copy(cloned.Samples, buffer.Samples)
	return cloned
}

func registerDemoAPI() {
	api := js.Global().Get("Object").New()
	api.Set("renderScene", js.FuncOf(renderSceneJS))
	api.Set("createRoom", js.FuncOf(createRoomJS))
	api.Set("setMaterial", js.FuncOf(setMaterialJS))
	api.Set("setSource", js.FuncOf(setSourceJS))
	api.Set("setReceiver", js.FuncOf(setReceiverJS))
	api.Set("simulate", js.FuncOf(simulateJS))
	api.Set("getParameters", js.FuncOf(getParametersJS))
	js.Global().Set("algoAcousticsDemo", api)
}

func createRoomJS(this js.Value, args []js.Value) any {
	payload, err := extractPayload(args)
	if err != nil {
		return js.ValueOf(map[string]any{"error": err.Error()})
	}

	room := demoRoom{Kind: "shoebox"}
	if strings.TrimSpace(payload) != "" {
		if err := json.Unmarshal([]byte(payload), &room); err != nil {
			var request demoRequest
			if err := json.Unmarshal([]byte(payload), &request); err != nil {
				return js.ValueOf(map[string]any{"error": fmt.Sprintf("decode room: %v", err)})
			}

			room = request.Room
		}
	}

	if strings.TrimSpace(room.Kind) == "" {
		room.Kind = "shoebox"
	}

	currentDemoAPIState.request.Room = room
	roomID := currentDemoAPIState.nextRoomID()

	return js.ValueOf(roomID)
}

func setMaterialJS(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return js.ValueOf(map[string]any{"error": "surfaceID must be a string"})
	}

	surfaceID := strings.TrimSpace(args[0].String())
	if surfaceID == "" {
		return js.ValueOf(map[string]any{"error": "surfaceID must not be empty"})
	}

	absorption := parseFloatArrayArg(args, 1)
	if len(absorption) == 0 {
		absorption = []float64{0}
	}

	scattering := parseFloatArrayArg(args, 2)
	if len(scattering) == 0 {
		scattering = []float64{0}
	}

	bandCount := acoustics.Octave6.BandCount()
	absorption = normalizeBandCoefficients(absorption, bandCount)
	scattering = normalizeBandCoefficients(scattering, bandCount)

	materialLibrary[surfaceID] = scene.Material{
		Name:             surfaceID,
		AbsorptionByBand: append([]float64(nil), absorption...),
		ScatteringByBand: append([]float64(nil), scattering...),
	}

	switch surfaceID {
	case "west":
		currentDemoAPIState.request.Materials.West = surfaceID
	case "east":
		currentDemoAPIState.request.Materials.East = surfaceID
	case "south":
		currentDemoAPIState.request.Materials.South = surfaceID
	case "north":
		currentDemoAPIState.request.Materials.North = surfaceID
	case "floor":
		currentDemoAPIState.request.Materials.Floor = surfaceID
	case "ceiling":
		currentDemoAPIState.request.Materials.Ceiling = surfaceID
	}

	return js.ValueOf(true)
}

func setSourceJS(this js.Value, args []js.Value) any {
	x, y, z, err := parseXYZArgs(args)
	if err != nil {
		return js.ValueOf(map[string]any{"error": err.Error()})
	}

	currentDemoAPIState.request.Source.X = x
	currentDemoAPIState.request.Source.Y = y
	currentDemoAPIState.request.Source.Z = z

	return js.ValueOf(true)
}

func setReceiverJS(this js.Value, args []js.Value) any {
	x, y, z, err := parseXYZArgs(args)
	if err != nil {
		return js.ValueOf(map[string]any{"error": err.Error()})
	}

	currentDemoAPIState.request.Receiver.X = x
	currentDemoAPIState.request.Receiver.Y = y
	currentDemoAPIState.request.Receiver.Z = z

	return js.ValueOf(true)
}

func simulateJS(this js.Value, args []js.Value) any {
	payload, err := extractPayload(args)
	if err != nil {
		return js.ValueOf(map[string]any{"error": err.Error()})
	}

	result, err := runDemoRenderJSON(payload)
	if err != nil {
		return js.ValueOf(map[string]any{"error": err.Error()})
	}

	samples := js.Global().Get("Float32Array").New(len(result.Samples))
	for index, sample := range result.Samples {
		samples.SetIndex(index, sample)
	}

	return samples
}

func getParametersJS(this js.Value, _ []js.Value) any {
	buffer := currentDemoAPIState.lastBuffer
	if buffer == nil {
		return js.ValueOf(map[string]any{"error": "no rendered impulse response is available"})
	}

	parameters := demoParameters{}

	if value, err := metrics.T60FromDecaySlope(buffer); err == nil {
		parameters.T60 = value
	}
	if value, err := metrics.EDT(buffer); err == nil {
		parameters.EDT = value
	}
	if value, err := metrics.T20(buffer); err == nil {
		parameters.T20 = value
	}
	if value, err := metrics.T30(buffer); err == nil {
		parameters.T30 = value
	}
	if value, err := metrics.C50(buffer); err == nil {
		parameters.C50 = value
	}
	if value, err := metrics.C80(buffer); err == nil {
		parameters.C80 = value
	}
	if value, err := metrics.D50(buffer); err == nil {
		parameters.D50 = value
	}

	if currentDemoAPIState.lastResult != nil {
		parameters.PeakAmplitude = currentDemoAPIState.lastResult.PeakAmplitude
		parameters.RMSAmplitude = currentDemoAPIState.lastResult.RMSAmplitude
		parameters.FirstArrivalMs = currentDemoAPIState.lastResult.FirstArrivalMs
	}

	return js.ValueOf(map[string]any{
		"t60":            parameters.T60,
		"edt":            parameters.EDT,
		"t20":            parameters.T20,
		"t30":            parameters.T30,
		"c50":            parameters.C50,
		"c80":            parameters.C80,
		"d50":            parameters.D50,
		"peakAmplitude":  parameters.PeakAmplitude,
		"rmsAmplitude":   parameters.RMSAmplitude,
		"firstArrivalMs": parameters.FirstArrivalMs,
	})
}

func parseFloatArrayArg(args []js.Value, index int) []float64 {
	if len(args) <= index {
		return nil
	}

	value := args[index]
	if !value.Truthy() || value.IsNull() || value.IsUndefined() {
		return nil
	}

	switch value.Type() {
	case js.TypeNumber:
		return []float64{value.Float()}
	case js.TypeString:
		var out []float64
		if err := json.Unmarshal([]byte(value.String()), &out); err == nil {
			return out
		}
	case js.TypeObject:
		if value.Length() > 0 {
			out := make([]float64, 0, value.Length())
			for i := 0; i < value.Length(); i++ {
				out = append(out, value.Index(i).Float())
			}
			return out
		}

		jsonObject := js.Global().Get("JSON")
		if jsonObject.Truthy() {
			encoded := jsonObject.Call("stringify", value).String()
			var out []float64
			if err := json.Unmarshal([]byte(encoded), &out); err == nil {
				return out
			}
		}
	}

	return nil
}

func parseXYZArgs(args []js.Value) (float64, float64, float64, error) {
	if len(args) >= 3 && args[0].Type() == js.TypeNumber && args[1].Type() == js.TypeNumber && args[2].Type() == js.TypeNumber {
		return args[0].Float(), args[1].Float(), args[2].Float(), nil
	}

	if len(args) > 0 {
		payload, err := extractPayload(args)
		if err != nil {
			return 0, 0, 0, err
		}

		var point struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
			Z float64 `json:"z"`
		}

		if err := json.Unmarshal([]byte(payload), &point); err == nil {
			return point.X, point.Y, point.Z, nil
		}
	}

	return 0, 0, 0, errors.New("expected x, y, z arguments")
}

func normalizeBandCoefficients(values []float64, bandCount int) []float64 {
	if bandCount <= 0 {
		return append([]float64(nil), values...)
	}

	out := make([]float64, bandCount)
	if len(values) == 0 {
		return out
	}

	copied := copy(out, values)
	fill := out[copied-1]
	for index := copied; index < len(out); index++ {
		out[index] = fill
	}

	return out
}
