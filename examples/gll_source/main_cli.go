//go:build !js || !wasm

package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(outputFilename); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
