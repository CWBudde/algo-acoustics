//go:build js && wasm

package main

import "syscall/js"

func reportDemoProgress(stage string, percent float64, message string) {
	fn := js.Global().Get("algoAcousticsDemoProgress")
	if fn.Type() != js.TypeFunction {
		return
	}

	fn.Invoke(stage, percent, message)
}

// reportDemoTier hands a progressive tier to the page mid-render.
//
// The worker installs the callback for the duration of a render and forwards
// each tier over postMessage. Posting from inside a synchronous WASM call works
// because postMessage only queues the message on the receiving thread; it does
// not need the worker's own event loop, which the render is occupying.
//
// A missing callback is not an error. The Go tests call the render directly, and
// the raw window.algoAcousticsDemo API has no worker to install one.
func reportDemoTier(payload demoTierPayload) {
	fn := js.Global().Get("algoAcousticsDemoTier")
	if fn.Type() != js.TypeFunction {
		return
	}

	fn.Invoke(string(payload.Tier), demoTierPayloadToJS(payload))
}

func demoTierPayloadToJS(payload demoTierPayload) js.Value {
	output := js.Global().Get("Object").New()
	output.Set("tier", string(payload.Tier))
	output.Set("elapsedMs", payload.ElapsedMS)

	if payload.Statistics != nil {
		statistics := js.Global().Get("Object").New()
		statistics.Set("sabineRt60Secs", payload.Statistics.SabineRT60Secs)
		statistics.Set("eyringRt60Secs", payload.Statistics.EyringRT60Secs)
		statistics.Set("c80Db", payload.Statistics.C80DB)
		statistics.Set("d50", payload.Statistics.D50)
		output.Set("statistics", statistics)
	}

	if payload.Samples != nil {
		output.Set("samples", float32SliceToJS(payload.Samples))
		output.Set("sampleRate", payload.SampleRate)
		output.Set("durationSeconds", payload.DurationSeconds)
		output.Set("numRays", payload.NumRays)
		output.Set("maxOrder", payload.MaxOrder)
		output.Set("earlyEventCount", payload.EarlyEventCount)
	}

	return output
}

func demoCancelled() bool {
	value := js.Global().Get("algoAcousticsDemoCancelRequested")
	return value.Truthy() && value.Bool()
}
