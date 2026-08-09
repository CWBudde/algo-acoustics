package scene

import (
	"encoding/json"
	"fmt"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
)

// Scene ties together room geometry, portals, material palette, emitters, and
// receivers. Room preserves the original single-room Go and JSON API; new
// multi-room scenes use Rooms instead.
//

type Scene struct {
	Room       Room                `json:"-"`
	Rooms      []Room              `json:"-"`
	Portals    []Portal            `json:"-"`
	Materials  map[string]Material `json:"-"`
	Sources    []Source            `json:"-"`
	Receivers  []Receiver          `json:"-"`
	BandSpec   acoustics.BandSpec  `json:"-"`
	SampleRate int                 `json:"-"`
}

//nolint:tagliatelle // This compatibility payload mirrors the public scene schema.
type sceneJSON struct {
	Room       *Room               `json:"room,omitempty"`
	Rooms      []Room              `json:"rooms,omitempty"`
	Portals    []Portal            `json:"portals,omitempty"`
	Materials  map[string]Material `json:"materials,omitempty"`
	Sources    []Source            `json:"sources,omitempty"`
	Receivers  []Receiver          `json:"receivers,omitempty"`
	BandSpec   acoustics.BandSpec  `json:"bandSpec"`
	SampleRate int                 `json:"sampleRate"`
}

// MarshalJSON emits the established room field for legacy scenes and the new
// rooms field for multi-room scenes.
func (s Scene) MarshalJSON() ([]byte, error) {
	payload := sceneJSON{
		Rooms:      s.Rooms,
		Portals:    s.Portals,
		Materials:  s.Materials,
		Sources:    s.Sources,
		Receivers:  s.Receivers,
		BandSpec:   s.BandSpec,
		SampleRate: s.SampleRate,
	}

	if len(s.Rooms) == 0 && roomIsSet(s.Room) {
		room := s.Room
		payload.Room = &room
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal scene: %w", err)
	}

	return data, nil
}

// UnmarshalJSON accepts both the legacy room field and the new rooms field.
// Validation rejects documents that supply both representations.
func (s *Scene) UnmarshalJSON(data []byte) error {
	var payload sceneJSON

	err := json.Unmarshal(data, &payload)
	if err != nil {
		return fmt.Errorf("unmarshal scene: %w", err)
	}

	s.Room = Room{}
	if payload.Room != nil {
		s.Room = *payload.Room
	}

	s.Rooms = append([]Room(nil), payload.Rooms...)
	s.Portals = append([]Portal(nil), payload.Portals...)
	s.Materials = payload.Materials
	s.Sources = payload.Sources
	s.Receivers = payload.Receivers
	s.BandSpec = payload.BandSpec
	s.SampleRate = payload.SampleRate

	return nil
}

// RoomCount returns the number of authored rooms, treating the legacy Room
// field as room zero.
func (s *Scene) RoomCount() int {
	if s == nil {
		return 0
	}

	if len(s.Rooms) > 0 {
		return len(s.Rooms)
	}

	if roomIsSet(s.Room) {
		return 1
	}

	return 0
}

// RoomAt returns the room at index, treating the legacy Room field as room
// zero.
func (s *Scene) RoomAt(index int) (*Room, bool) {
	if s == nil || index < 0 {
		return nil, false
	}

	if len(s.Rooms) > 0 {
		if index >= len(s.Rooms) {
			return nil, false
		}

		return &s.Rooms[index], true
	}

	if index == 0 && roomIsSet(s.Room) {
		return &s.Room, true
	}

	return nil, false
}

// RoomIndexAt returns the unique room whose bounds contain position. Points
// shared by overlapping rooms or a common boundary are intentionally
// considered ambiguous.
func (s *Scene) RoomIndexAt(position geometry.Vec3) (int, bool) {
	match := -1

	for index := range s.RoomCount() {
		room, ok := s.RoomAt(index)
		if !ok {
			continue
		}

		bounds, ok := room.Bounds()
		if !ok || !bounds.Contains(position) {
			continue
		}

		if match >= 0 {
			return 0, false
		}

		match = index
	}

	return match, match >= 0
}

func roomIsSet(room Room) bool {
	return room.Kind != "" || room.Shoebox != nil || room.Mesh != nil || room.MeshPath != "" || room.MeshMaterial != ""
}
