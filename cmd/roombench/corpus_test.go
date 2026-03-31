package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestBenchmarkCorpusSmoke(t *testing.T) {
	t.Parallel()

	corpus := []string{
		roomFixturePath("tiny_room.json"),
		roomFixturePath("control_room.json"),
		roomFixturePath("lecture_room.json"),
		roomFixturePath("pa_room.json"),
	}

	for _, path := range corpus {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			sc, err := scene.LoadSceneFile(path)
			if err != nil {
				t.Fatalf("LoadSceneFile() error = %v", err)
			}

			err = scene.Validate(sc)
			if err != nil {
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

			_, err = metrics.T60FromDecaySlope(buf)
			if err != nil {
				t.Fatalf("T60FromDecaySlope() error = %v", err)
			}

			_, err = metrics.EDT(buf)
			if err != nil {
				t.Fatalf("EDT() error = %v", err)
			}

			_, err = metrics.C80(buf)
			if err != nil {
				t.Fatalf("C80() error = %v", err)
			}
		})
	}
}

func roomFixturePath(name string) string {
	candidates := []string{
		filepath.Join("testdata", "rooms", name),
		filepath.Join("..", "..", "testdata", "rooms", name),
	}

	for _, candidate := range candidates {
		absCandidate, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}

		_, err = os.Stat(absCandidate)
		if err == nil {
			return absCandidate
		}
	}

	return candidates[0]
}
