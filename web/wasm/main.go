//go:build js && wasm

package main

import (
	"errors"
	"syscall/js"
)

func main() {
	registerDemoAPI()
	js.Global().Set("algoAcousticsDemoReady", true)
	js.Global().Call("dispatchEvent", js.Global().Get("Event").New("algo-acoustics-demo-ready"))

	select {}
}

func renderSceneJS(this js.Value, args []js.Value) any {
	payload, err := extractPayload(args)
	if err != nil {
		return js.ValueOf(map[string]any{"error": err.Error()})
	}

	result, err := runDemoRenderJSON(payload)
	if err != nil {
		return js.ValueOf(map[string]any{"error": err.Error()})
	}

	return demoResultToJS(result)
}

func extractPayload(args []js.Value) (string, error) {
	if len(args) == 0 || args[0].IsUndefined() || args[0].IsNull() {
		return "{}", nil
	}

	firstArg := args[0]
	if firstArg.Type() == js.TypeString {
		return firstArg.String(), nil
	}

	jsonObject := js.Global().Get("JSON")
	if !jsonObject.Truthy() {
		return "", errors.New("browser JSON API is unavailable")
	}

	return jsonObject.Call("stringify", firstArg).String(), nil
}

func demoResultToJS(result demoResult) js.Value {
	output := js.Global().Get("Object").New()
	samples := js.Global().Get("Float32Array").New(len(result.Samples))
	for index, sample := range result.Samples {
		samples.SetIndex(index, sample)
	}

	wavBytes := js.Global().Get("Uint8Array").New(len(result.WAVBytes))
	js.CopyBytesToJS(wavBytes, result.WAVBytes)

	output.Set("mode", result.Mode)
	output.Set("sampleRate", result.SampleRate)
	output.Set("durationSeconds", result.DurationSeconds)
	output.Set("earlyEventCount", result.EarlyEventCount)
	output.Set("numRays", result.NumRays)
	output.Set("peakAmplitude", result.PeakAmplitude)
	output.Set("rmsAmplitude", result.RMSAmplitude)
	output.Set("firstArrivalMs", result.FirstArrivalMs)
	output.Set("renderMs", result.RenderMS)
	output.Set("splHeatmap", demoSPLHeatmapToJS(result.SPLHeatmap))
	output.Set("samples", samples)
	output.Set("wavBytes", wavBytes)
	if result.PortalResponses != nil {
		responses := js.Global().Get("Object").New()
		responses.Set("closedWavBytes", byteSliceToJS(result.PortalResponses.ClosedWAVBytes))
		responses.Set("openWavBytes", byteSliceToJS(result.PortalResponses.OpenWAVBytes))
		output.Set("portalResponses", responses)
	}

	return output
}

func byteSliceToJS(data []byte) js.Value {
	output := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(output, data)

	return output
}

func demoSPLHeatmapToJS(heatmap demoSPLHeatmap) js.Value {
	output := js.Global().Get("Object").New()
	output.Set("minimumDb", heatmap.MinimumDB)
	output.Set("maximumDb", heatmap.MaximumDB)

	samples := js.Global().Get("Array").New(len(heatmap.Samples))
	for index, sample := range heatmap.Samples {
		item := js.Global().Get("Object").New()
		item.Set("surfaceId", sample.SurfaceID)
		item.Set("x", sample.X)
		item.Set("y", sample.Y)
		item.Set("z", sample.Z)
		item.Set("levelDb", sample.LevelDB)
		samples.SetIndex(index, item)
	}
	output.Set("samples", samples)

	return output
}
