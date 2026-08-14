//go:build js && wasm

package main

import (
	"errors"
	"fmt"
	"time"

	algoacoustics "github.com/cwbudde/algo-acoustics"
	"github.com/cwbudde/algo-acoustics/scene"
)

// Progressive rendering and the render deadline (PLAN.md phase 19.7).
//
// A WASM render is synchronous: it occupies its worker from the first call to
// the last, and nothing the page does can interrupt it. Two consequences shape
// this file.
//
// First, the user sees nothing until the whole render finishes, which at the top
// of the request envelope is many seconds. Phase 18's progressive pipeline is
// the answer: report what is already known, then refine. The demo runs three
// tiers — statistical, preview, and the full render — and pushes each to the
// page as it completes, so a room's decay time appears within milliseconds and
// an audible impulse response within a few hundred.
//
// Second, a render that overruns cannot be aborted from outside. The deadline is
// therefore cooperative: renderDeadline is consulted at the same stage
// boundaries that already check for cancellation, and when it has passed the
// render stops and returns the newest tier that did finish, flagged with a
// warning. A coarse impulse response with an explanation beats a blocked worker
// and no result at all.
//
// The demo deliberately does not call algoacoustics.RenderProgressive itself.
// That function covers a single mono hybrid render; the demo must also serve
// early-only, late-only, and connected-room requests, and its final tier has to
// stay bit-identical to what the non-progressive path produced before this
// pipeline existed. It shares Phase 18's statistical tier through
// ComputeStatisticalMetrics instead.

const (
	// previewTierMaxOrder caps the image-source order of the preview tier. It
	// mirrors ProgressiveConfig.PreviewISMOrder's default.
	previewTierMaxOrder = 2

	// previewTierNumRays is the preview tier's ray budget. It is far below
	// Phase 18's 2,000-ray default because the browser's job is to put a
	// waveform on screen fast; the tier that follows is the accurate one.
	previewTierNumRays = 384

	// previewTierMaxDurationSecs truncates long responses for the preview. Cost
	// scales with duration independently of rays, so a 3 s request would
	// otherwise spend most of its preview budget rendering a tail that the full
	// tier is about to replace.
	previewTierMaxDurationSecs = 1.0
)

// renderDeadline bounds a single render. A zero value means no deadline, which
// is what the Go tests use when they exercise a stage directly.
type renderDeadline struct {
	at time.Time
}

// newRenderDeadline starts the wall-clock budget for a render.
//
// The deadline is checked against time.Now rather than driven by a timer,
// because js/wasm is single-threaded and a Go timer goroutine cannot preempt a
// running solver: the timer would not fire until the very computation it is
// meant to bound had already returned.
func newRenderDeadline(started time.Time) renderDeadline {
	return renderDeadline{at: started.Add(demoRenderTimeout)}
}

// exceeded reports whether the budget has run out.
func (d renderDeadline) exceeded() bool {
	return !d.at.IsZero() && !time.Now().Before(d.at)
}

// errRenderCancelled reports that the page asked for the render to stop.
var errRenderCancelled = errors.New("render cancelled")

// errRenderDeadlineExceeded reports that the wall-clock budget ran out. Callers
// that hold a finished tier answer with it instead of failing.
var errRenderDeadlineExceeded = errors.New("render exceeded the demo time budget")

// checkDemoRenderAborted is the cooperative abort point. It is called where the
// render can cheaply stop: between the solver stages, never inside one.
func checkDemoRenderAborted(deadline renderDeadline) error {
	if demoCancelled() {
		return errRenderCancelled
	}

	if deadline.exceeded() {
		return errRenderDeadlineExceeded
	}

	return nil
}

// demoTier names a stage of the progressive pipeline. The names come from
// Phase 18 so that the demo, the library, and the docs agree.
type demoTier string

const (
	tierStatistical demoTier = "statistical"
	tierPreview     demoTier = "preview"
	tierFinal       demoTier = "final"
)

// demoTierPayload is one progressive update pushed to the page mid-render.
//
// It is deliberately not a demoResult: a tier carries only what the page can
// use immediately — a waveform to draw and figures to show — and never the
// encoded WAV, surface heatmap, or download payload, which belong to the render
// that finishes. Keeping tiers light also keeps them cheap to produce and to
// structured-clone across postMessage.
type demoTierPayload struct {
	Tier            demoTier
	ElapsedMS       float64
	Statistics      *demoStatistics
	SampleRate      int
	DurationSeconds float64
	NumRays         int
	MaxOrder        int
	EarlyEventCount int
	Samples         []float32
}

// demoStatistics carries the Tier 1 estimates to the page. The per-band arrays
// are collapsed to broadband values because the demo shows a single number; the
// mid-frequency bands are the ones a listener judges a room by.
type demoStatistics struct {
	SabineRT60Secs float64 `json:"sabineRt60Secs"`
	EyringRT60Secs float64 `json:"eyringRt60Secs"`
	C80DB          float64 `json:"c80Db"`
	D50            float64 `json:"d50"`
}

// computeDemoStatistics runs Phase 18's statistical tier and reduces it to the
// broadband figures the demo displays. The second result reports whether the
// estimators produced anything: a mesh room has no shoebox volume to work from.
func computeDemoStatistics(sc *scene.Scene) (demoStatistics, bool) {
	metrics := algoacoustics.ComputeStatisticalMetrics(sc)
	if metrics == nil || len(metrics.SabineRT60ByBand) == 0 {
		return demoStatistics{}, false
	}

	return demoStatistics{
		SabineRT60Secs: midBandAverage(metrics.SabineRT60ByBand),
		EyringRT60Secs: midBandAverage(metrics.EyringRT60ByBand),
		C80DB:          midBandAverage(metrics.C80ByBand),
		D50:            midBandAverage(metrics.D50ByBand),
	}, true
}

// midBandAverage averages the middle of a per-band array, which for the demo's
// six octave bands is the 500 Hz and 1 kHz pair that single-number room criteria
// are conventionally quoted at.
func midBandAverage(byBand []float64) float64 {
	if len(byBand) == 0 {
		return 0
	}

	if len(byBand) < 4 {
		var sum float64
		for _, value := range byBand {
			sum += value
		}

		return sum / float64(len(byBand))
	}

	middle := len(byBand) / 2

	return (byBand[middle-1] + byBand[middle]) / 2
}

// previewTierRequest derives the preview settings from a normalized request.
//
// Every knob only ever moves downward, so the preview cannot cost more than the
// render it is previewing, and it inherits the room, materials, positions, and
// crossover so that what the user hears is the same room rendered coarsely
// rather than a different one.
func previewTierRequest(request demoRequest) demoRequest {
	preview := cloneDemoRequest(request)
	preview.Render.MaxOrder = min(request.Render.MaxOrder, previewTierMaxOrder)
	preview.Render.NumRays = min(request.Render.NumRays, previewTierNumRays)
	preview.Render.DurationSeconds = min(request.Render.DurationSeconds, previewTierMaxDurationSecs)
	preview.Render.CrossoverTimeSeconds = clamp(
		request.Render.CrossoverTimeSeconds, 0.03, preview.Render.DurationSeconds*0.85,
	)

	return preview
}

// worthPreviewing reports whether a preview tier would earn its cost.
//
// A request that is already at or below the preview settings would be rendered
// twice for no gain, and a connected-room request cannot be previewed cheaply at
// all: it traces two full binaural responses whatever the quality knobs say.
func worthPreviewing(request demoRequest) bool {
	if request.Portal.Enabled {
		return false
	}

	preview := previewTierRequest(request)

	return preview.Render.NumRays < request.Render.NumRays ||
		preview.Render.MaxOrder < request.Render.MaxOrder ||
		preview.Render.DurationSeconds < request.Render.DurationSeconds
}

// timeoutWarning explains a truncated render in the same voice as the memory
// budget's warnings, which the page already surfaces in its render log.
func timeoutWarning(tier demoTier, elapsed time.Duration) string {
	return fmt.Sprintf(
		"render timeout: exceeded the %.0f s demo budget after %.1f s; returning the %s result",
		demoRenderTimeout.Seconds(), elapsed.Seconds(), tier,
	)
}
