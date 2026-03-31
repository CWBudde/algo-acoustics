package main

import (
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestBenchmarkCorpusSmoke(t *testing.T) {
	t.Parallel()

	corpus := []string{
		filepath.Join("..", "..", "testdata", "rooms", "tiny_room.json"),
		filepath.Join("..", "..", "testdata", "rooms", "control_room.json"),
		filepath.Join("..", "..", "testdata", "rooms", "lecture_room.json"),
		filepath.Join("..", "..", "testdata", "rooms", "pa_room.json"),
	}

	for _, path := range corpus {
		t.Run(filepath.Base(path), func(t *testing.T) {
			sc, err := scene.LoadSceneFile(path)
			if err != nil {
				t.Fatalf("LoadSceneFile() error = %v", err)
			}

			if err := scene.Validate(sc); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			events, err := solveFixture(sc, 2)
			if err != nil {
				t.Fatalf("solveFixture() error = %v", err)
			}

			buf, err := ir.RenderMono(events, ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 2.5, BandSpec: sc.BandSpec})
			if err != nil {
				t.Fatalf("RenderMono() error = %v", err)
			}

			if _, err := metrics.T60FromDecaySlope(buf); err != nil {
				t.Fatalf("T60FromDecaySlope() error = %v", err)
			}

			if _, err := metrics.EDT(buf); err != nil {
				t.Fatalf("EDT() error = %v", err)
			}

			if _, err := metrics.C80(buf); err != nil {
				t.Fatalf("C80() error = %v", err)
			}
		})
	}
}
