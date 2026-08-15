package ism

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
)

func loadGLLTestModel(t *testing.T) *directivity.GLLModel {
	t.Helper()

	model, err := directivity.LoadGLL(filepath.Join("..", "testdata", "gll", "synthetic_ls.gll"), "")
	if err != nil {
		t.Fatalf("LoadGLL() error = %v", err)
	}

	return model
}

// TestISMGLLSourceProducesDirectionalEvents pins the ISM behaviour of a GLL
// source: the geometry it produces, and the fact that its band gains actually
// carry the measured pattern. The second half is the regression that matters —
// a GLL balloon whose measurements fail to load yields unity gain everywhere,
// which is indistinguishable from an omnidirectional source.
func TestISMGLLSourceProducesDirectionalEvents(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	sc := testScene(t)
	sc.Sources[0].Directivity = loadGLLTestModel(t)

	events, err := solver.Solve(&sc, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	// Directivity scales event amplitude; it must not change which paths exist.
	omni := testScene(t)

	omniEvents, err := solver.Solve(&omni, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve() omni error = %v", err)
	}

	if len(events) != len(omniEvents) {
		t.Fatalf("GLL source produced %d events, want the %d of an omni source", len(events), len(omniEvents))
	}

	for index, event := range events {
		want := omniEvents[index]

		if event.Kind != want.Kind {
			t.Fatalf("event %d kind = %v, want %v", index, event.Kind, want.Kind)
		}

		if math.Abs(event.TimeSeconds-want.TimeSeconds) > 1e-12 {
			t.Fatalf("event %d time = %v, want %v", index, event.TimeSeconds, want.TimeSeconds)
		}

		if math.Abs(event.DistanceMeters-want.DistanceMeters) > 1e-9 {
			t.Fatalf("event %d distance = %v, want %v", index, event.DistanceMeters, want.DistanceMeters)
		}
	}

	direct := firstDirectEvent(events)
	if direct == nil {
		t.Fatal("expected a direct event")
	}

	// The receiver sits on the source axis, so the direct path is normalised
	// to unity by the balloon's own on-axis reference.
	for bandIndex, gain := range direct.BandGain {
		if math.Abs(gain-1) > 1e-9 {
			t.Fatalf("direct BandGain[%d] = %v, want the on-axis reference 1", bandIndex, gain)
		}
	}

	// Somewhere off-axis the measured pattern must depart from unity.
	spread := 0.0

	for _, event := range events {
		for _, gain := range event.BandGain {
			spread = math.Max(spread, math.Abs(gain-1))
		}
	}

	if spread < 1e-3 {
		t.Fatalf("every GLL band gain is within %v of unity; the balloon measurements did not reach the solver", spread)
	}
}

// TestISMGLLBalloonExtractionMatchesGLLSource checks that a balloon extracted
// from a GLL source is a faithful stand-in for it inside the solver, which is
// the point of extraction: the table can replace the file without moving the
// result.
func TestISMGLLBalloonExtractionMatchesGLLSource(t *testing.T) {
	t.Parallel()

	model := loadGLLTestModel(t)

	sc := testScene(t)

	// A 5-degree grid matches the fixture's own angular resolution.
	balloon, err := model.ExtractBalloon(
		sc.BandSpec.CenterFreqs,
		directivity.SphericalGrid{AzimuthCount: 72, ElevationCount: 37},
		directivity.Bilinear,
	)
	if err != nil {
		t.Fatalf("ExtractBalloon() error = %v", err)
	}

	solver := ISMSolver{}

	sc.Sources[0].Directivity = model

	reference, err := solver.Solve(&sc, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve() with the GLL model error = %v", err)
	}

	sc.Sources[0].Directivity = balloon

	extracted, err := solver.Solve(&sc, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve() with the extracted balloon error = %v", err)
	}

	if len(extracted) != len(reference) {
		t.Fatalf("extracted balloon produced %d events, want %d", len(extracted), len(reference))
	}

	for index, event := range extracted {
		want := reference[index]

		if event.Kind != want.Kind || math.Abs(event.TimeSeconds-want.TimeSeconds) > 1e-12 {
			t.Fatalf("event %d = (%v, %v), want (%v, %v)",
				index, event.Kind, event.TimeSeconds, want.Kind, want.TimeSeconds)
		}

		// The direct path leaves along +X, which is a tabulated grid
		// direction, so there it has to agree exactly rather than closely.
		tolerance := 1.0
		if event.Kind == ir.EventDirect {
			tolerance = 1e-6
		}

		for bandIndex := range event.BandGain {
			if !gainsCloseInDB(event.BandGain[bandIndex], want.BandGain[bandIndex], tolerance) {
				t.Fatalf("event %d band %d gain = %v, want %v within %v dB",
					index, bandIndex, event.BandGain[bandIndex], want.BandGain[bandIndex], tolerance)
			}
		}
	}
}

func TestISMGLLSourceRespectsOrientation(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}

	// Turning the source through 180 degrees must present a different part of
	// the balloon to the receiver, confirming the model is evaluated in the
	// source-local frame rather than world space.
	forward := testScene(t)
	forward.Sources[0].Directivity = loadGLLTestModel(t)

	forwardEvents, err := solver.Solve(&forward, ISMConfig{MaxOrder: 0})
	if err != nil {
		t.Fatalf("Solve() forward error = %v", err)
	}

	turned := testScene(t)
	turned.Sources[0].Directivity = loadGLLTestModel(t)
	turned.Sources[0].Orientation = geometry.QuatFromAxisAngle(geometry.Vec3{Z: 1}, math.Pi)

	turnedEvents, err := solver.Solve(&turned, ISMConfig{MaxOrder: 0})
	if err != nil {
		t.Fatalf("Solve() turned error = %v", err)
	}

	forwardDirect := firstDirectEvent(forwardEvents)
	turnedDirect := firstDirectEvent(turnedEvents)

	if forwardDirect == nil || turnedDirect == nil {
		t.Fatal("expected a direct event in both orientations")
	}

	changed := false

	for bandIndex := range forwardDirect.BandGain {
		if !gainsCloseInDB(forwardDirect.BandGain[bandIndex], turnedDirect.BandGain[bandIndex], 1e-6) {
			changed = true

			break
		}
	}

	if !changed {
		t.Fatal("rotating the source by 180 degrees did not change the direct band gains")
	}
}

// gainsCloseInDB compares two linear gains with a tolerance expressed in dB,
// which is the scale directivity data is specified on.
func gainsCloseInDB(got, want, toleranceDB float64) bool {
	if got == want {
		return true
	}

	if got <= 0 || want <= 0 {
		return math.Abs(got-want) <= 1e-12
	}

	return math.Abs(20*math.Log10(got/want)) <= toleranceDB
}
