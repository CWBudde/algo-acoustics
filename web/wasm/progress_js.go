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

func demoCancelled() bool {
	value := js.Global().Get("algoAcousticsDemoCancelRequested")
	return value.Truthy() && value.Bool()
}
