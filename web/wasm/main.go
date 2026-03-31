//go:build js && wasm

package main

import (
	"errors"
	"syscall/js"
)

func main() {
	api := js.Global().Get("Object").New()
	api.Set("renderScene", js.FuncOf(renderSceneJS))
	js.Global().Set("algoAcousticsDemo", api)
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

	output.Set("sampleRate", result.SampleRate)
	output.Set("durationSeconds", result.DurationSeconds)
	output.Set("earlyEventCount", result.EarlyEventCount)
	output.Set("numRays", result.NumRays)
	output.Set("peakAmplitude", result.PeakAmplitude)
	output.Set("rmsAmplitude", result.RMSAmplitude)
	output.Set("firstArrivalMs", result.FirstArrivalMs)
	output.Set("renderMs", result.RenderMS)
	output.Set("samples", samples)
	output.Set("wavBytes", wavBytes)

	return output
}
