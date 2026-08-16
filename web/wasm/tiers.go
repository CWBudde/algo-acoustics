//go:build js && wasm

package main

import (
	"errors"
	"fmt"
	"math"
	"time"

	algoacoustics "github.com/cwbudde/algo-acoustics"
	"github.com/cwbudde/algo-acoustics/scene"
)

// Progressive rendering and the render deadline; see docs/web-demo-limits.md.
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

// remaining reports how much of the budget is left. An unbounded render always
// has time left, reported as the largest representable duration.
func (d renderDeadline) remaining() time.Duration {
	if d.at.IsZero() {
		return time.Duration(math.MaxInt64)
	}

	return max(time.Until(d.at), 0)
}

// affords reports whether a stage projected to take cost should be started.
func (d renderDeadline) affords(cost time.Duration) bool {
	if d.at.IsZero() || cost <= 0 {
		return true
	}

	return float64(cost) <= float64(d.remaining())*overrunTolerance
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

// Only these two are pushed as tier messages. The full render arrives as the
// result itself, so announcing it a second time as a tier would have the page
// draw the same waveform twice.
const (
	tierStatistical demoTier = "statistical"
	tierPreview     demoTier = "preview"
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
	SabineRT60Secs float64
	EyringRT60Secs float64
	C80DB          float64
	D50            float64
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

// projectedTimeoutWarning explains a render that was not started because it
// could not have finished in time.
func projectedTimeoutWarning(projected, remaining time.Duration) string {
	return fmt.Sprintf(
		"render timeout: the full render projects to %.0f s against %.1f s left of the %.0f s demo budget; "+
			"returning the preview result",
		projected.Seconds(), remaining.Seconds(), demoRenderTimeout.Seconds(),
	)
}

// overrunTolerance is how far a projection may exceed the remaining budget
// before the full render is abandoned unstarted.
//
// The projection below is crude, so the tolerance leans towards running the
// render: being wrong high costs the user detail they asked for, while being
// wrong low costs them only the overshoot that the stage checkpoints would catch
// anyway.
const overrunTolerance = 1.5

// projectFullRenderCost estimates what the full render will cost from what the
// preview actually cost.
//
// This exists because the deadline is only cooperative. A late-field trace is a
// single uninterruptible call, so a checkpoint placed after it cannot stop a
// render that has already overrun — measured, the worst case inside the envelope
// blows a 10 s budget by another 8 s before anything notices. Predicting from a
// measurement taken on this device, in this browser, on this room is the only
// way to keep that promise, and it is a far better basis than a constant fitted
// on some other machine.
//
// The model is deliberately simple: ray-tracing cost is close to linear in both
// ray count and response duration, and those two knobs dominate. Image-source
// order is not modelled, which is one reason for the tolerance above.
func projectFullRenderCost(previewCost time.Duration, preview, full demoRequest) time.Duration {
	if previewCost <= 0 || preview.Render.NumRays <= 0 || preview.Render.DurationSeconds <= 0 {
		return 0
	}

	rayRatio := float64(full.Render.NumRays) / float64(preview.Render.NumRays)
	durationRatio := full.Render.DurationSeconds / preview.Render.DurationSeconds

	return time.Duration(float64(previewCost) * rayRatio * durationRatio)
}
