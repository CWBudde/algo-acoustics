package scene_test

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestSoundReductionTransmissionRoundTrip(t *testing.T) {
	t.Parallel()

	for _, reduction := range []float64{0, 12.5, 25, 35, 50} {
		transmission := scene.TransmissionFromSoundReductionIndex(reduction)

		got := scene.SoundReductionIndexFromTransmission(transmission)
		if math.Abs(got-reduction) > 1e-12 {
			t.Fatalf("round trip for %v dB = %v, want %v", reduction, got, reduction)
		}
	}
}

func TestMaterialTransmissionJSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := scene.Material{
		Name:                "partition",
		AbsorptionByBand:    []float64{0.1},
		SoundReductionIndex: []float64{35},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded scene.Material

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("round-trip mismatch:\noriginal: %#v\ndecoded: %#v", original, decoded)
	}

	want := math.Pow(10, -3.5)
	if got := decoded.TransmissionAt(5); math.Abs(got-want) > 1e-15 {
		t.Fatalf("TransmissionAt(5) = %v, want %v", got, want)
	}
}

func TestValidateMaterialTransmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		transmission []float64
		reduction    []float64
		absorption   []float64
		want         string
	}{
		{name: "valid transmission", transmission: []float64{0.2}, absorption: []float64{0.3}},
		{name: "valid reduction", reduction: []float64{30}, absorption: []float64{0.3}},
		{name: "transmission range", transmission: []float64{1.1}, absorption: []float64{0}, want: "transmission[0]"},
		{name: "negative reduction", reduction: []float64{-1}, absorption: []float64{0}, want: "sound reduction index[0]"},
		{name: "energy conservation", transmission: []float64{0.8}, absorption: []float64{0.3}, want: "absorption + transmission"},
		{name: "representations disagree", transmission: []float64{0.2}, reduction: []float64{20}, absorption: []float64{0.1}, want: "disagree"},
		{name: "band count", transmission: []float64{0.1, 0.2}, absorption: []float64{0.1}, want: "transmission band count"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sc := validScene()
			material := sc.Materials["plaster"]
			material.AbsorptionByBand = test.absorption
			material.TransmissionByBand = test.transmission
			material.SoundReductionIndex = test.reduction
			sc.Materials["plaster"] = material

			err := scene.Validate(&sc)
			if test.want == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}

				return
			}

			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestMaterialLibraryTransmissionEntries(t *testing.T) {
	t.Parallel()

	wants := map[string]float64{
		"painted_concrete": 50,
		"plasterboard":     35,
		"wooden_door":      25,
		"glass":            30,
		"open_doorway":     0,
	}

	for name, reduction := range wants {
		material := scene.Material{}

		got, ok := material.FromLibrary(name)
		if !ok {
			t.Fatalf("FromLibrary(%q) returned false", name)
		}

		if len(got.SoundReductionIndex) != scene.NumBands || got.SoundReductionIndex[0] != reduction {
			t.Fatalf("FromLibrary(%q).SoundReductionIndex = %#v, want six bands at %v dB", name, got.SoundReductionIndex, reduction)
		}

		if got.AbsorptionAt(0)+got.TransmissionAt(0) > 1 {
			t.Fatalf("FromLibrary(%q) violates energy conservation", name)
		}
	}
}

func TestMaterialLibraryTransmissionIsDeepCopied(t *testing.T) {
	t.Parallel()

	material := scene.Material{}

	first, ok := material.FromLibrary("plasterboard")
	if !ok {
		t.Fatal("FromLibrary(plasterboard) returned false")
	}

	first.SoundReductionIndex[0] = 0

	second, _ := material.FromLibrary("plasterboard")
	if second.SoundReductionIndex[0] != 35 {
		t.Fatal("mutating returned sound reduction data changed the material library")
	}
}

func TestMultiRoomSceneJSONRoundTripAndValidation(t *testing.T) {
	t.Parallel()

	original := validMultiRoomScene()

	err := scene.Validate(&original)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if strings.Contains(string(data), `"room":`) || !strings.Contains(string(data), `"rooms":`) {
		t.Fatalf("multi-room JSON uses wrong room representation: %s", data)
	}

	var decoded scene.Scene

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("round-trip mismatch:\noriginal: %#v\ndecoded: %#v", original, decoded)
	}

	if got, ok := decoded.RoomIndexAt(decoded.Sources[0].Position); !ok || got != 0 {
		t.Fatalf("source room = (%d, %v), want (0, true)", got, ok)
	}

	if got, ok := decoded.RoomIndexAt(decoded.Receivers[0].Position); !ok || got != 1 {
		t.Fatalf("receiver room = (%d, %v), want (1, true)", got, ok)
	}
}

func TestPortalHelpers(t *testing.T) {
	t.Parallel()

	portal := validMultiRoomScene().Portals[0]
	if got, want := portal.Area(), 4.0; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Area() = %v, want %v", got, want)
	}

	if got, want := portal.Center(), (geometry.Vec3{X: 6, Y: 2, Z: 1.5}); got.Distance(want) > 1e-12 {
		t.Fatalf("Center() = %v, want %v", got, want)
	}

	if got := portal.Normal(); got.Distance(geometry.Vec3{X: 1}) > 1e-12 {
		t.Fatalf("Normal() = %v, want +X", got)
	}

	portal.State = scene.PortalOpen
	if got := portal.TransmissionAt(validMultiRoomScene().Materials, 0); got != 1 {
		t.Fatalf("open TransmissionAt() = %v, want 1", got)
	}
}

func TestValidateRejectsInvalidPortals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*scene.Portal)
		want   string
	}{
		{name: "room index", mutate: func(p *scene.Portal) { p.RoomIndices[1] = 2 }, want: "out of range"},
		{name: "same room", mutate: func(p *scene.Portal) { p.RoomIndices[1] = 0 }, want: "distinct rooms"},
		{name: "state", mutate: func(p *scene.Portal) { p.State = "ajar" }, want: "unsupported state"},
		{name: "material", mutate: func(p *scene.Portal) { p.Material = "missing" }, want: "undefined material"},
		{name: "nonplanar", mutate: func(p *scene.Portal) { p.Polygon[2].X += 0.1 }, want: "coplanar"},
		{name: "off wall", mutate: func(p *scene.Portal) {
			for i := range p.Polygon {
				p.Polygon[i].X = 5.9
			}
		}, want: "boundary wall"},
		{name: "wrong winding", mutate: func(p *scene.Portal) { p.Polygon[1], p.Polygon[3] = p.Polygon[3], p.Polygon[1] }, want: "winding"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sc := validMultiRoomScene()
			test.mutate(&sc.Portals[0])

			err := scene.Validate(&sc)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestValidateRejectsBothRoomRepresentations(t *testing.T) {
	t.Parallel()

	sc := validMultiRoomScene()
	sc.Room = sc.Rooms[0]

	err := scene.Validate(&sc)
	if err == nil || !strings.Contains(err.Error(), "either room or rooms") {
		t.Fatalf("Validate() error = %v, want conflicting room representations", err)
	}
}

func TestValidateRejectsSingleEntryRoomsAndLegacyPortals(t *testing.T) {
	t.Parallel()

	t.Run("single entry rooms", func(t *testing.T) {
		t.Parallel()

		sc := validMultiRoomScene()
		sc.Rooms = sc.Rooms[:1]
		sc.Portals = nil
		sc.Receivers[0].Position = geometry.Vec3{X: 3, Y: 2, Z: 1.5}

		err := scene.Validate(&sc)
		if err == nil || !strings.Contains(err.Error(), "use room for a single-room scene") {
			t.Fatalf("Validate() error = %v, want single-room representation error", err)
		}
	})

	t.Run("legacy room with portals", func(t *testing.T) {
		t.Parallel()

		sc := validScene()
		sc.Portals = validMultiRoomScene().Portals

		err := scene.Validate(&sc)
		if err == nil || !strings.Contains(err.Error(), "portals require the rooms representation") {
			t.Fatalf("Validate() error = %v, want portals representation error", err)
		}
	})
}

func TestMultiRoomSummary(t *testing.T) {
	t.Parallel()

	sc := validMultiRoomScene()

	summary := scene.Summary(&sc)
	for _, want := range []string{"rooms: 2", "room[1]: shoebox", "at (6.000, 0.000, 0.000)", "portals: 1"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("Summary() = %q, want substring %q", summary, want)
		}
	}
}

func validMultiRoomScene() scene.Scene {
	walls := [6]string{"wall", "wall", "wall", "wall", "wall", "wall"}

	return scene.Scene{
		Rooms: []scene.Room{
			{
				Kind: scene.RoomKindShoebox,
				Shoebox: &scene.Shoebox{
					Width: 6, Depth: 4, Height: 3, WallMaterials: walls,
				},
			},
			{
				Kind: scene.RoomKindShoebox,
				Shoebox: &scene.Shoebox{
					Width: 6, Depth: 4, Height: 3, WallMaterials: walls,
					Origin: geometry.Vec3{X: 6},
				},
			},
		},
		Portals: []scene.Portal{{
			RoomIndices: [2]int{0, 1},
			Polygon: []geometry.Vec3{
				{X: 6, Y: 1, Z: 0.5},
				{X: 6, Y: 3, Z: 0.5},
				{X: 6, Y: 3, Z: 2.5},
				{X: 6, Y: 1, Z: 2.5},
			},
			Material: "door",
			State:    scene.PortalClosed,
		}},
		Materials: map[string]scene.Material{
			"wall": {
				Name:             "wall",
				AbsorptionByBand: []float64{0.1},
			},
			"door": {
				Name:                "door",
				AbsorptionByBand:    []float64{0.1},
				SoundReductionIndex: []float64{25},
			},
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 2, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 10, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}
