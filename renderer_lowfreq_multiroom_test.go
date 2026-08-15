package algoacoustics

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// TestLowFreqSceneForMultiRoomLocalizesTheGroup pins that the extracted room
// group is translated to the origin.
//
// The Helmholtz solver indexes cells from a zero origin and ignores
// Shoebox.Origin, so a receiving room away from the origin would have its
// source and receiver clamped to the wrong boundary cells — frequently the same
// cell, which collapses the transfer function entirely.
func TestLowFreqSceneForMultiRoomLocalizesTheGroup(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)

	lowFreq, err := LowFreqSceneForMultiRoom(sc)
	if err != nil {
		t.Fatalf("LowFreqSceneForMultiRoom: %v", err)
	}

	shoebox := lowFreq.Scene.Room.Shoebox
	if shoebox.Origin != geometry.Vec3Zero {
		t.Fatalf("shoebox origin = %+v, want the group localized to zero", shoebox.Origin)
	}

	inside := func(name string, position geometry.Vec3) {
		t.Helper()

		if position.X < 0 || position.X > shoebox.Width ||
			position.Y < 0 || position.Y > shoebox.Depth ||
			position.Z < 0 || position.Z > shoebox.Height {
			t.Fatalf("%s at %+v lies outside the localized box %vx%vx%v",
				name, position, shoebox.Width, shoebox.Depth, shoebox.Height)
		}
	}

	inside("substitute source", lowFreq.Scene.Sources[0].Position)
	inside("receiver", lowFreq.Scene.Receivers[0].Position)

	// The two must land in different cells, or the solve degenerates.
	if lowFreq.Scene.Sources[0].Position.Distance(lowFreq.Scene.Receivers[0].Position) < 0.5 {
		t.Fatal("the substitute source and the receiver collapsed onto the same point")
	}
}

// TestLowFreqSceneForMultiRoomCarriesPathTransmission pins that the modal field
// is attenuated by the partitions upstream of it. Without this a wall costing
// the geometric field tens of dB would leave the low-frequency blend untouched.
func TestLowFreqSceneForMultiRoomCarriesPathTransmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tau  float64
	}{
		{name: "open partition", tau: 1},
		{name: "quarter transmission", tau: 0.25},
		{name: "heavy partition", tau: 0.001},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			lowFreq, err := LowFreqSceneForMultiRoom(transmissionTestScene(test.tau))
			if err != nil {
				t.Fatalf("LowFreqSceneForMultiRoom: %v", err)
			}

			want := math.Sqrt(test.tau)
			if diff := math.Abs(lowFreq.PressureGain - want); diff > 1e-9 {
				t.Fatalf("PressureGain = %v, want sqrt(tau) = %v", lowFreq.PressureGain, want)
			}
		})
	}
}

// TestLowFreqSceneForMultiRoomExcitesThePropagatingPortal pins that the
// substitute source sits on the portal the rendered field actually arrives
// through, not on whichever portal happens to have the lowest index.
func TestLowFreqSceneForMultiRoomExcitesThePropagatingPortal(t *testing.T) {
	t.Parallel()

	sc := deadEndPortalScene()

	lowFreq, err := LowFreqSceneForMultiRoom(sc)
	if err != nil {
		t.Fatalf("LowFreqSceneForMultiRoom: %v", err)
	}

	// Portal 0 is the dead end and has the lower index; portal 1 carries the
	// path from the source. Both bound the receiver's room, and the excitation
	// must sit at the second.
	origin := sc.Rooms[1].Shoebox.Origin
	world := lowFreq.Scene.Sources[0].Position.Add(origin)

	if math.Abs(world.X-8) > 0.01 {
		t.Fatalf("substitute source at x=%v, want the propagating portal at x=8", world.X)
	}
}

// deadEndPortalScene puts the receiver's room between a dead-end room and the
// source's room, with the dead-end portal carrying the lower index.
func deadEndPortalScene() *scene.Scene {
	walls := [6]string{"wall", "wall", "wall", "wall", "wall", "wall"}

	room := func(originX float64) scene.Room {
		return scene.Room{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{
			Origin: geometry.Vec3{X: originX}, Width: 4, Depth: 3, Height: 2.5, WallMaterials: walls,
		}}
	}

	portal := func(x float64, rooms [2]int) scene.Portal {
		return scene.Portal{
			RoomIndices: rooms,
			Polygon: []geometry.Vec3{
				{X: x, Y: 0.5, Z: 0.2},
				{X: x, Y: 2.5, Z: 0.2},
				{X: x, Y: 2.5, Z: 2.2},
				{X: x, Y: 0.5, Z: 2.2},
			},
			Material: "portal",
			State:    scene.PortalClosed,
		}
	}

	return &scene.Scene{
		// Room 0 is the dead end, room 1 holds the receiver, room 2 the source.
		Rooms: []scene.Room{room(0), room(4), room(8)},
		Portals: []scene.Portal{
			portal(4, [2]int{0, 1}),
			portal(8, [2]int{1, 2}),
		},
		Materials: map[string]scene.Material{
			"wall":   {Name: "wall", AbsorptionByBand: []float64{0.1}},
			"portal": {Name: "portal", AbsorptionByBand: []float64{0}, TransmissionByBand: []float64{0.25}},
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 10, Y: 1.5, Z: 1.25}}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 6, Y: 1.5, Z: 1.25}, Type: scene.ReceiverOmni}},
		BandSpec:   transmissionTestScene(0.25).BandSpec,
		SampleRate: 8000,
	}
}
