package main

import (
	"strings"
	"testing"
)

func TestRunDemoRenderProducesWaveformAndMetrics(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Render.NumRays = 256
	request.Render.DurationSeconds = 0.75
	request.Render.CrossoverTimeSeconds = 0.16

	result, err := runDemoRender(request)
	if err != nil {
		t.Fatalf("runDemoRender() error = %v", err)
	}

	if got, want := result.SampleRate, defaultDemoSampleRate; got != want {
		t.Fatalf("SampleRate = %d, want %d", got, want)
	}
	if len(result.Samples) == 0 {
		t.Fatal("Samples is empty, want rendered waveform")
	}
	if len(result.WAVBytes) == 0 {
		t.Fatal("WAVBytes is empty, want encoded audio")
	}
	if result.EarlyEventCount <= 0 {
		t.Fatalf("EarlyEventCount = %d, want > 0", result.EarlyEventCount)
	}
	if result.PeakAmplitude <= 0 {
		t.Fatalf("PeakAmplitude = %v, want > 0", result.PeakAmplitude)
	}
	if result.FirstArrivalMs <= 0 {
		t.Fatalf("FirstArrivalMs = %v, want > 0", result.FirstArrivalMs)
	}
	if result.RenderMS < 0 {
		t.Fatalf("RenderMS = %v, want >= 0", result.RenderMS)
	}
}

func TestRunDemoRenderRejectsUnsupportedMode(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Render.Mode = "invalid"

	_, err := runDemoRender(request)
	if err == nil {
		t.Fatal("runDemoRender() error = nil, want unsupported mode error")
	}
	if !strings.Contains(err.Error(), "unsupported render mode") {
		t.Fatalf("runDemoRender() error = %q, want unsupported mode message", err)
	}
}
