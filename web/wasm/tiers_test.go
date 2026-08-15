//go:build js && wasm

package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// expiredDeadline is a budget that ran out before the render started.
func expiredDeadline() renderDeadline {
	return renderDeadline{at: time.Now().Add(-time.Second)}
}

func TestRenderDeadlineExceeded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		deadline renderDeadline
		want     bool
	}{
		{name: "zero value never expires", deadline: renderDeadline{}, want: false},
		{name: "budget still open", deadline: renderDeadline{at: time.Now().Add(time.Hour)}, want: false},
		{name: "budget spent", deadline: expiredDeadline(), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.deadline.exceeded(); got != tt.want {
				t.Errorf("exceeded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewRenderDeadlineUsesTheDemoBudget(t *testing.T) {
	t.Parallel()

	started := time.Now()
	deadline := newRenderDeadline(started)

	if got := deadline.at.Sub(started); got != demoRenderTimeout {
		t.Errorf("budget = %v, want %v", got, demoRenderTimeout)
	}

	if deadline.exceeded() {
		t.Error("a fresh deadline reports as exceeded")
	}
}

func TestCheckDemoRenderAbortedReportsTheDeadline(t *testing.T) {
	t.Parallel()

	err := checkDemoRenderAborted(expiredDeadline())
	if !errors.Is(err, errRenderDeadlineExceeded) {
		t.Fatalf("error = %v, want errRenderDeadlineExceeded", err)
	}

	err = checkDemoRenderAborted(renderDeadline{})
	if err != nil {
		t.Errorf("error = %v, want nil for an unbounded render", err)
	}
}

func TestPreviewTierRequestOnlyReducesQuality(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Render.NumRays = maxDemoNumRays
	request.Render.MaxOrder = maxDemoMaxOrder
	request.Render.DurationSeconds = maxDemoDurationSecs

	preview := previewTierRequest(request)

	if preview.Render.NumRays != previewTierNumRays {
		t.Errorf("NumRays = %d, want %d", preview.Render.NumRays, previewTierNumRays)
	}

	if preview.Render.MaxOrder != previewTierMaxOrder {
		t.Errorf("MaxOrder = %d, want %d", preview.Render.MaxOrder, previewTierMaxOrder)
	}

	if preview.Render.DurationSeconds != previewTierMaxDurationSecs {
		t.Errorf("DurationSeconds = %v, want %v", preview.Render.DurationSeconds, previewTierMaxDurationSecs)
	}

	// The preview must simulate the same room, not a different one.
	if preview.Room != request.Room || preview.Materials != request.Materials {
		t.Error("preview tier changed the room or its materials")
	}

	if preview.Source != request.Source || preview.Receiver != request.Receiver {
		t.Error("preview tier moved the source or receiver")
	}
}

func TestPreviewTierRequestNeverRaisesQuality(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Render.NumRays = minDemoNumRays
	request.Render.MaxOrder = minDemoMaxOrder
	request.Render.DurationSeconds = minDemoDurationSecs

	preview := previewTierRequest(request)

	if preview.Render.NumRays != minDemoNumRays {
		t.Errorf("NumRays = %d, want %d", preview.Render.NumRays, minDemoNumRays)
	}

	if preview.Render.MaxOrder != minDemoMaxOrder {
		t.Errorf("MaxOrder = %d, want %d", preview.Render.MaxOrder, minDemoMaxOrder)
	}

	if preview.Render.DurationSeconds != minDemoDurationSecs {
		t.Errorf("DurationSeconds = %v, want %v", preview.Render.DurationSeconds, minDemoDurationSecs)
	}

	if preview.Render.CrossoverTimeSeconds > preview.Render.DurationSeconds {
		t.Errorf(
			"crossover %v sits past the preview duration %v",
			preview.Render.CrossoverTimeSeconds, preview.Render.DurationSeconds,
		)
	}
}

func TestWorthPreviewing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*demoRequest)
		want   bool
	}{
		{
			name:   "default request is worth previewing",
			mutate: func(*demoRequest) {},
			want:   true,
		},
		{
			name: "a request already below the preview settings is not",
			mutate: func(request *demoRequest) {
				request.Render.NumRays = minDemoNumRays
				request.Render.MaxOrder = minDemoMaxOrder
				request.Render.DurationSeconds = minDemoDurationSecs
			},
			want: false,
		},
		{
			name: "connected rooms have no cheap preview",
			mutate: func(request *demoRequest) {
				request.Portal.Enabled = true
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request := defaultDemoRequest()
			tt.mutate(&request)

			if got := worthPreviewing(request); got != tt.want {
				t.Errorf("worthPreviewing() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeDemoStatisticsEstimatesDecayWithoutSimulating(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()

	normalized, _, err := normalizeDemoRequest(request)
	if err != nil {
		t.Fatalf("normalizeDemoRequest() error = %v", err)
	}

	sc, err := buildDemoScene(normalized)
	if err != nil {
		t.Fatalf("buildDemoScene() error = %v", err)
	}

	statistics, ok := computeDemoStatistics(sc)
	if !ok {
		t.Fatal("computeDemoStatistics() reported no estimates for a shoebox room")
	}

	if statistics.SabineRT60Secs <= 0 {
		t.Errorf("SabineRT60Secs = %v, want > 0", statistics.SabineRT60Secs)
	}

	// Eyring is derived from a logarithmic absorption term and always predicts a
	// shorter decay than Sabine for the same room.
	if statistics.EyringRT60Secs <= 0 || statistics.EyringRT60Secs > statistics.SabineRT60Secs {
		t.Errorf(
			"EyringRT60Secs = %v, want a positive value at or below Sabine's %v",
			statistics.EyringRT60Secs, statistics.SabineRT60Secs,
		)
	}
}

func TestComputeDemoStatisticsSkipsRoomsWithoutAShoeboxModel(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Room.Kind = "mesh"

	normalized, _, err := normalizeDemoRequest(request)
	if err != nil {
		t.Fatalf("normalizeDemoRequest() error = %v", err)
	}

	sc, err := buildDemoScene(normalized)
	if err != nil {
		t.Fatalf("buildDemoScene() error = %v", err)
	}

	if _, ok := computeDemoStatistics(sc); ok {
		t.Error("computeDemoStatistics() reported estimates for a mesh room")
	}
}

func TestMidBandAverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		byBand []float64
		want   float64
	}{
		{name: "empty", byBand: nil, want: 0},
		{name: "short arrays average whole", byBand: []float64{1, 3}, want: 2},
		{name: "six bands average the middle pair", byBand: []float64{1, 2, 3, 5, 8, 13}, want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := midBandAverage(tt.byBand); got != tt.want {
				t.Errorf("midBandAverage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProjectFullRenderCostScalesWithRaysAndDuration(t *testing.T) {
	t.Parallel()

	preview := defaultDemoRequest()
	preview.Render.NumRays = 384
	preview.Render.DurationSeconds = 1

	tests := []struct {
		name        string
		rays        int
		duration    float64
		previewCost time.Duration
		want        time.Duration
	}{
		{
			name:        "same settings project the same cost",
			rays:        384,
			duration:    1,
			previewCost: 200 * time.Millisecond,
			want:        200 * time.Millisecond,
		},
		{
			name:        "rays scale linearly",
			rays:        3840,
			duration:    1,
			previewCost: 200 * time.Millisecond,
			want:        2 * time.Second,
		},
		{
			name:        "duration scales linearly",
			rays:        384,
			duration:    3,
			previewCost: 200 * time.Millisecond,
			want:        600 * time.Millisecond,
		},
		{
			name:        "the worst case in the envelope is projected as hopeless",
			rays:        maxDemoNumRays,
			duration:    maxDemoDurationSecs,
			previewCost: 200 * time.Millisecond,
			want:        25600 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			full := defaultDemoRequest()
			full.Render.NumRays = tt.rays
			full.Render.DurationSeconds = tt.duration

			if got := projectFullRenderCost(tt.previewCost, preview, full); got != tt.want {
				t.Errorf("projectFullRenderCost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProjectFullRenderCostWithoutAMeasurement(t *testing.T) {
	t.Parallel()

	preview := defaultDemoRequest()
	full := defaultDemoRequest()

	if got := projectFullRenderCost(0, preview, full); got != 0 {
		t.Errorf("projectFullRenderCost() = %v, want 0 when the preview was not timed", got)
	}
}

func TestRenderDeadlineAffords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cost time.Duration
		want bool
	}{
		{name: "well inside the budget", cost: 2 * time.Second, want: true},
		{name: "inside the overrun tolerance", cost: 13 * time.Second, want: true},
		{name: "hopeless", cost: 30 * time.Second, want: false},
		{name: "unmeasured cost is always allowed", cost: 0, want: true},
	}

	for _, tt := range tests {
		// Not parallel, and the deadline is built next to the call: affords()
		// reads the clock, so a subtest parked waiting for a parallel slot
		// behind the package's multi-second render tests would measure a budget
		// that had already drained.
		t.Run(tt.name, func(t *testing.T) {
			deadline := renderDeadline{at: time.Now().Add(10 * time.Second)}

			if got := deadline.affords(tt.cost); got != tt.want {
				t.Errorf("affords(%v) = %v, want %v", tt.cost, got, tt.want)
			}
		})
	}

	if !(renderDeadline{}).affords(time.Hour) {
		t.Error("an unbounded render refused an expensive stage")
	}
}

func TestPreviewFallbackNoticeStopsAHopelessRender(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Render.NumRays = maxDemoNumRays
	request.Render.DurationSeconds = maxDemoDurationSecs

	started := time.Now()
	deadline := renderDeadline{at: started.Add(demoRenderTimeout)}
	// A preview that took a second projects the full render at well over a
	// minute, which no tolerance can accommodate.
	preview := &demoTierBuffer{cost: time.Second}

	notice := previewFallbackNotice(request, preview, started, deadline)
	if notice == "" {
		t.Fatal("previewFallbackNotice() = \"\", want the render to be abandoned")
	}

	if !strings.Contains(notice, "projects to") {
		t.Errorf("notice = %q, want it to explain the projection", notice)
	}
}

func TestPreviewFallbackNoticeAllowsAnAffordableRender(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Render.NumRays = 768
	request.Render.DurationSeconds = 1

	started := time.Now()
	deadline := renderDeadline{at: started.Add(demoRenderTimeout)}
	// Twice the preview's rays at the same duration: about 40 ms projected.
	preview := &demoTierBuffer{cost: 20 * time.Millisecond}

	if notice := previewFallbackNotice(request, preview, started, deadline); notice != "" {
		t.Errorf("previewFallbackNotice() = %q, want the full render to go ahead", notice)
	}
}

func TestPreviewFallbackNoticeReportsAnAlreadySpentBudget(t *testing.T) {
	t.Parallel()

	notice := previewFallbackNotice(
		defaultDemoRequest(),
		&demoTierBuffer{cost: time.Millisecond},
		time.Now(),
		expiredDeadline(),
	)

	if !strings.Contains(notice, "exceeded the") {
		t.Errorf("notice = %q, want it to report the spent budget", notice)
	}
}

// A spent budget must not cost the user their result: the preview tier is
// bounded by construction, runs regardless, and becomes the answer.
func TestExpiredDeadlineReturnsThePreviewTierWithAWarning(t *testing.T) {
	request := defaultDemoRequest()
	request.Render.DurationSeconds = 0.75

	started := time.Now()

	result, err := runDemoRenderWithDeadline(request, started, expiredDeadline())
	if err != nil {
		t.Fatalf("runDemoRenderWithDeadline() error = %v, want a partial result", err)
	}

	if len(result.Samples) == 0 {
		t.Fatal("Samples is empty, want the preview waveform")
	}

	if len(result.WAVBytes) == 0 {
		t.Fatal("WAVBytes is empty, want playable audio for the partial result")
	}

	if len(result.SPLHeatmap.Samples) == 0 {
		t.Error("SPLHeatmap is empty, want the partial result to be complete enough to display")
	}

	var timeoutWarnings int

	for _, warning := range result.Warnings {
		if strings.Contains(warning, "render timeout") {
			timeoutWarnings++
		}
	}

	if timeoutWarnings != 1 {
		t.Fatalf("timeout warnings = %d, want exactly 1 in %q", timeoutWarnings, result.Warnings)
	}

	// The returned result must describe the preview that was actually rendered,
	// not the request that was asked for, or the page would report settings the
	// samples do not correspond to.
	preview := previewTierRequest(request)
	if result.NumRays != preview.Render.NumRays {
		t.Errorf("NumRays = %d, want the preview's %d", result.NumRays, preview.Render.NumRays)
	}

	if want := int(preview.Render.DurationSeconds * demoSampleRate); len(result.Samples) != want {
		t.Errorf("sample count = %d, want the preview's %d", len(result.Samples), want)
	}
}

// Without a preview to fall back on there is nothing to return, and the caller
// must be told why rather than handed a silent empty response.
func TestExpiredDeadlineWithoutAPreviewFails(t *testing.T) {
	request := defaultDemoRequest()
	request.Render.NumRays = minDemoNumRays
	request.Render.MaxOrder = minDemoMaxOrder
	request.Render.DurationSeconds = minDemoDurationSecs

	_, err := runDemoRenderWithDeadline(request, time.Now(), expiredDeadline())
	if !errors.Is(err, errRenderDeadlineExceeded) {
		t.Fatalf("error = %v, want errRenderDeadlineExceeded", err)
	}
}

func TestGenerousDeadlineLeavesTheFullRenderIntact(t *testing.T) {
	request := defaultDemoRequest()
	request.Render.NumRays = 256
	request.Render.DurationSeconds = 0.5

	started := time.Now()

	result, err := runDemoRenderWithDeadline(request, started, renderDeadline{at: started.Add(time.Hour)})
	if err != nil {
		t.Fatalf("runDemoRenderWithDeadline() error = %v", err)
	}

	for _, warning := range result.Warnings {
		if strings.Contains(warning, "render timeout") {
			t.Errorf("unexpected timeout warning %q on a render that fit its budget", warning)
		}
	}

	if result.NumRays != request.Render.NumRays {
		t.Errorf("NumRays = %d, want the requested %d", result.NumRays, request.Render.NumRays)
	}

	if want := int(request.Render.DurationSeconds * demoSampleRate); len(result.Samples) != want {
		t.Errorf("sample count = %d, want the requested %d", len(result.Samples), want)
	}
}

// The preview must not change what a render that fits its budget produces.
func TestPreviewTierDoesNotAlterTheFinalResult(t *testing.T) {
	request := defaultDemoRequest()
	request.Render.NumRays = 512
	request.Render.DurationSeconds = 0.5

	first, err := runDemoRender(request)
	if err != nil {
		t.Fatalf("runDemoRender() error = %v", err)
	}

	second, err := runDemoRender(request)
	if err != nil {
		t.Fatalf("runDemoRender() error = %v", err)
	}

	if first.PeakAmplitude != second.PeakAmplitude || first.RMSAmplitude != second.RMSAmplitude {
		t.Fatalf(
			"repeated renders diverged: peak %v vs %v, rms %v vs %v",
			first.PeakAmplitude, second.PeakAmplitude, first.RMSAmplitude, second.RMSAmplitude,
		)
	}

	if len(first.Samples) != len(second.Samples) {
		t.Fatalf("sample counts differ: %d vs %d", len(first.Samples), len(second.Samples))
	}

	for index := range first.Samples {
		if first.Samples[index] != second.Samples[index] {
			t.Fatalf("samples differ at index %d: %v vs %v", index, first.Samples[index], second.Samples[index])
		}
	}
}
