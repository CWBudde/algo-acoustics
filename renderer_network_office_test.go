package algoacoustics

import (
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/scene"
)

// officeFloorRenderScene loads the four-room office fixture and moves the
// receiver into the requested room.
func officeFloorRenderScene(t *testing.T, receiverRoom int) *scene.Scene {
	t.Helper()

	sc, err := scene.LoadSceneFile("examples/scenes/office_floor.json")
	if err != nil {
		t.Fatalf("load office floor fixture: %v", err)
	}

	sc.Receivers[0].Position = geometry.Vec3{X: float64(receiverRoom)*4 + 2, Y: 2, Z: 1.5}

	err = scene.Validate(sc)
	if err != nil {
		t.Fatalf("office floor fixture is invalid: %v", err)
	}

	return sc
}

// bandLevelDB returns the per-band level of an event list.
func bandLevelDB(events []ir.Event, bandCount int) []float64 {
	energy := make([]float64, bandCount)

	for _, event := range events {
		for band := range bandCount {
			gain := 1.0
			if band < len(event.BandGain) {
				gain = event.BandGain[band]
			}

			pressure := event.Amplitude * gain
			energy[band] += pressure * pressure
		}
	}

	levels := make([]float64, bandCount)

	for band, value := range energy {
		if value <= 0 {
			levels[band] = math.Inf(-1)

			continue
		}

		levels[band] = 10 * math.Log10(value)
	}

	return levels
}

// TestNetworkRendererOfficeFloorLevelDropIsMonotonic checks the coarse property
// first: every additional partition must cost level.
func TestNetworkRendererOfficeFloorLevelDropIsMonotonic(t *testing.T) {
	t.Parallel()

	// Three 20 dB partitions total 60 dB, exactly the default floor, so the
	// floor is lowered here to keep the furthest room in play. The default
	// behaviour is pinned separately by
	// TestNetworkRendererOfficeFloorPrunesTheFurthestRoomAtTheDefaultFloor.
	network := NewNetworkRenderer(NetworkRendererConfig{
		ISM:         ism.ISMConfig{MaxOrder: 1},
		BandFloorDB: -90,
	})

	previous := math.Inf(1)

	for room := 1; room <= 3; room++ {
		sc := officeFloorRenderScene(t, room)
		cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 1.0, BandSpec: sc.BandSpec}

		events, err := network.SolveEarly(sc, cfg)
		if err != nil {
			t.Fatalf("room %d: SolveEarly: %v", room, err)
		}

		level := eventPressureLevelDB(events)
		if math.IsInf(level, -1) {
			t.Fatalf("room %d received no energy", room)
		}

		if level >= previous {
			t.Fatalf("room %d level %.2f dB is not below room %d's %.2f dB", room, level, room-1, previous)
		}

		// A 20 dB partition must cost clearly more than a few tenths.
		if drop := previous - level; !math.IsInf(previous, 1) && drop < 3 {
			t.Fatalf("room %d: level dropped only %.2f dB across a partition, want at least 3 dB", room, drop)
		}

		previous = level
	}
}

// TestNetworkRendererOfficeFloorPrunesTheFurthestRoomAtTheDefaultFloor pins
// that source elimination really removes work rather than quietly rendering
// silence. Three 20 dB partitions total 60 dB, which is the default floor, so
// the furthest office is unreachable and the renderer must say so — naming the
// floor, since that is the knob the caller would want.
func TestNetworkRendererOfficeFloorPrunesTheFurthestRoomAtTheDefaultFloor(t *testing.T) {
	t.Parallel()

	sc := officeFloorRenderScene(t, 3)
	cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 1.0, BandSpec: sc.BandSpec}

	network := NewNetworkRenderer(NetworkRendererConfig{ISM: ism.ISMConfig{MaxOrder: 1}})

	_, err := network.SolveEarly(sc, cfg)
	if err == nil {
		t.Fatal("SolveEarly rendered a path that the default floor should have pruned")
	}

	if !strings.Contains(err.Error(), "floor") {
		t.Fatalf("error = %v, want it to name the attenuation floor", err)
	}

	// Lowering the floor must make the same room reachable again.
	deep := NewNetworkRenderer(NetworkRendererConfig{ISM: ism.ISMConfig{MaxOrder: 1}, BandFloorDB: -90})

	events, err := deep.SolveEarly(sc, cfg)
	if err != nil {
		t.Fatalf("SolveEarly with a -90 dB floor: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("a -90 dB floor produced no events")
	}
}

// TestNetworkRendererOfficeFloorMatchesTheApparentReductionIndex is the
// analytic check, and it matters more than the monotonic one above.
//
// A plausible 3 dB error would sit in the portal re-emission if a portal
// radiated into 4*pi rather than the half space it actually faces. That error
// is inherited from Phase 21 and is therefore invisible to the equivalence
// test, which agrees with Phase 21 exactly. Only comparing against
// R' = Ls - Lr + 10*log10(S/A) can catch it.
//
// Tolerances follow docs/raven.md section 11.2, which reports agreement within
// about 1.5 dB from 2 to 16 kHz and roughly a 2.5 dB offset below 2 kHz, plus
// margin for Monte Carlo variance. Bands below 125 Hz are excluded there
// because geometrical acoustics does not apply, and this fixture starts at
// 125 Hz for the same reason.
func TestNetworkRendererOfficeFloorMatchesTheApparentReductionIndex(t *testing.T) {
	t.Parallel()

	const (
		partitionArea      = 1.2 * 2.1 // the doorway-sized partition opening
		reductionIndexDB   = 20.0
		lowBandToleranceDB = 5.0
		highBandTolerance  = 3.0
	)

	network := NewNetworkRenderer(NetworkRendererConfig{ISM: ism.ISMConfig{MaxOrder: 1}})

	source := officeFloorRenderScene(t, 0)
	cfg := ir.RenderConfig{SampleRate: source.SampleRate, DurationSeconds: 1.0, BandSpec: source.BandSpec}
	bandCount := source.BandSpec.BandCount()

	sourceEvents, err := network.SolveEarly(officeFloorRenderScene(t, 1), cfg)
	if err != nil {
		t.Fatalf("SolveEarly(room 1): %v", err)
	}

	receiverEvents, err := network.SolveEarly(officeFloorRenderScene(t, 2), cfg)
	if err != nil {
		t.Fatalf("SolveEarly(room 2): %v", err)
	}

	sourceLevels := bandLevelDB(sourceEvents, bandCount)
	receiverLevels := bandLevelDB(receiverEvents, bandCount)

	// The receiving room's equivalent absorption area, from its own surfaces.
	absorptionArea := officeRoomAbsorptionArea(t, source)

	for band := range bandCount {
		if math.IsInf(sourceLevels[band], -1) || math.IsInf(receiverLevels[band], -1) {
			t.Fatalf("band %d carries no energy", band)
		}

		measured := metrics.ApparentSoundReductionIndex(
			sourceLevels[band], receiverLevels[band], partitionArea, absorptionArea[band],
		)

		tolerance := lowBandToleranceDB
		if source.BandSpec.CenterFreqs[band] >= 2000 {
			tolerance = highBandTolerance
		}

		if math.Abs(measured-reductionIndexDB) > tolerance {
			t.Fatalf("band %d (%.0f Hz): apparent reduction index %.2f dB, want %.1f dB within %.1f dB",
				band, source.BandSpec.CenterFreqs[band], measured, reductionIndexDB, tolerance)
		}
	}
}

// officeRoomAbsorptionArea returns the equivalent absorption area of one office
// per band, from its wall, floor, and ceiling materials.
func officeRoomAbsorptionArea(t *testing.T, sc *scene.Scene) []float64 {
	t.Helper()

	room, ok := sc.RoomAt(1)
	if !ok || room.Shoebox == nil {
		t.Fatal("the fixture no longer has a shoebox at index 1")
	}

	box := room.Shoebox
	areas := [6]float64{
		box.Depth * box.Height, box.Depth * box.Height, // -X, +X
		box.Width * box.Height, box.Width * box.Height, // -Y, +Y
		box.Width * box.Depth, box.Width * box.Depth, // -Z, +Z
	}

	total := make([]float64, sc.BandSpec.BandCount())

	for wall, area := range areas {
		material, ok := sc.Materials[box.WallMaterials[wall]]
		if !ok {
			t.Fatalf("wall %d references an undefined material", wall)
		}

		for band := range total {
			total[band] += area * material.AbsorptionAt(band)
		}
	}

	return total
}
