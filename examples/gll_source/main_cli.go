//go:build !js || !wasm

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	opts := defaultExampleOptions()
	outputPath := flag.String("output", outputFilename, "output WAV path")

	flag.StringVar(&opts.CrossoverWindow.Name, "crossover-window", opts.CrossoverWindow.Name, "hybrid crossover window")
	flag.Float64Var(&opts.CrossoverWindow.Alpha, "crossover-window-alpha", 0, "shape parameter for parametric hybrid crossover windows")
	flag.Parse()

	err := runWithOptions(*outputPath, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
