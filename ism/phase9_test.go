package ism

import (
	"math"
	"math/bits"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestISMSolverValidateAllFirstOrderWallReflectionsForSymmetricShoebox(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	sc := symmetricPhase9Scene(t)

	events, err := solver.Solve(&sc, ISMConfig{MaxOrder: 1, SpeedOfSound: acoustics.SpeedOfSound})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	images := GenerateImageSources(sc.Sources[0].Position, sc.Room.Shoebox, 1)
	for _, image := range images {
		if image.Order != 1 {
			continue
		}

		event := findSpecularEventForImage(t, events, sc.Receivers[0].Position, image.Position)

		wantDistance := sc.Receivers[0].Position.Distance(image.Position)
		if math.Abs(event.DistanceMeters-wantDistance) > 1e-12 {
			t.Fatalf("reflection distance = %v, want %v", event.DistanceMeters, wantDistance)
		}

		if math.Abs(event.TimeSeconds-wantDistance/acoustics.SpeedOfSound) > 1e-12 {
			t.Fatalf("reflection time = %v, want %v", event.TimeSeconds, wantDistance/acoustics.SpeedOfSound)
		}

		if math.Abs(event.Amplitude-1/wantDistance) > 1e-12 {
			t.Fatalf("reflection amplitude = %v, want %v", event.Amplitude, 1/wantDistance)
		}

		for bandIndex, gain := range event.BandGain {
			if math.Abs(gain-1) > 1e-12 {
				t.Fatalf("reflection BandGain[%d] = %v, want 1", bandIndex, gain)
			}
		}
	}
}

func TestISMSolverValidateSecondOrderReflectionEdgeCases(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	sc := symmetricPhase9Scene(t)

	events, err := solver.Solve(&sc, ISMConfig{MaxOrder: 2, SpeedOfSound: acoustics.SpeedOfSound})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	edgeCases := [][3]int{{1, 1, 0}, {1, 0, 1}, {0, 1, 1}}
	for _, edgeCase := range edgeCases {
		image := findImageSourceByOrderTriple(t, sc.Sources[0].Position, sc.Room.Shoebox, 2, edgeCase[0], edgeCase[1], edgeCase[2])
		if bits.OnesCount8(image.WallMask) != 2 {
			t.Fatalf("image %+v wall mask = %06b, want two walls", image.Position, image.WallMask)
		}

		event := findSpecularEventForImage(t, events, sc.Receivers[0].Position, image.Position)

		wantDistance := sc.Receivers[0].Position.Distance(image.Position)
		if math.Abs(event.DistanceMeters-wantDistance) > 1e-12 {
			t.Fatalf("second-order distance = %v, want %v", event.DistanceMeters, wantDistance)
		}
	}
}

func symmetricPhase9Scene(t *testing.T) scene.Scene {
	t.Helper()

	return scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:         10,
				Depth:         10,
				Height:        10,
				WallMaterials: [6]string{"hard", "hard", "hard", "hard", "hard", "hard"},
			},
		},
		Materials: map[string]scene.Material{
			"hard": {
				Name:             "hard",
				AbsorptionByBand: []float64{0, 0, 0, 0, 0, 0},
				ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 2, Y: 3, Z: 4},
			Orientation: geometry.QuatIdentity(),
			GainDB:      0,
			Directivity: directivity.OmniModel{},
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 8, Y: 7, Z: 6},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}

func findImageSourceByOrderTriple(t *testing.T, src geometry.Vec3, room *scene.Shoebox, maxOrder, orderX, orderY, orderZ int) ImageSource {
	t.Helper()

	for _, image := range GenerateImageSources(src, room, maxOrder) {
		if image.orderX == orderX && image.orderY == orderY && image.orderZ == orderZ {
			return image
		}
	}

	t.Fatalf("did not find image source for order triple (%d,%d,%d)", orderX, orderY, orderZ)

	return ImageSource{}
}

func findSpecularEventForImage(t *testing.T, events []ir.Event, receiver geometry.Vec3, image geometry.Vec3) *ir.Event {
	t.Helper()

	wantDirection := image.Sub(receiver).Normalize()
	wantDistance := receiver.Distance(image)

	for index := range events {
		event := &events[index]
		if event.Kind != ir.EventSpecular {
			continue
		}

		if math.Abs(event.DistanceMeters-wantDistance) > 1e-12 {
			continue
		}

		if !directionMatches(event.Direction, wantDirection) {
			continue
		}

		return event
	}

	t.Fatalf("did not find specular event for image %+v", image)

	return nil
}
