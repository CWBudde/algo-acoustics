//go:build js && wasm

package main

import (
	"syscall/js"
)

func main() {
	js.Global().Set("runGLLSourceExample", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return js.ValueOf(map[string]any{"error": "missing GLL bytes argument"})
		}

		gllBytes := make([]byte, args[0].Get("byteLength").Int())
		js.CopyBytesToGo(gllBytes, args[0])

		result, err := runWASMWithOptions(gllBytes, wasmExampleOptions(args))
		if err != nil {
			return js.ValueOf(map[string]any{"error": err.Error()})
		}

		wavBytes := js.Global().Get("Uint8Array").New(len(result.WAVBytes))
		js.CopyBytesToJS(wavBytes, result.WAVBytes)

		output := js.Global().Get("Object").New()
		output.Set("frontEnergyRatio", result.FrontEnergyRatio)
		output.Set("rearEnergyRatio", result.RearEnergyRatio)
		output.Set("wavBytes", wavBytes)
		return output
	}))

	select {}
}

func wasmExampleOptions(args []js.Value) exampleOptions {
	opts := defaultExampleOptions()
	if len(args) < 2 || args[1].Type() != js.TypeObject {
		return opts
	}

	if name := args[1].Get("crossoverWindow"); name.Type() == js.TypeString {
		opts.CrossoverWindow.Name = name.String()
	}
	if alpha := args[1].Get("crossoverWindowAlpha"); alpha.Type() == js.TypeNumber {
		opts.CrossoverWindow.Alpha = alpha.Float()
	}

	return opts
}
