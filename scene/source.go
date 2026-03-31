package scene

import (
	"encoding/json"
	"fmt"

	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
)

// Source is an emitting point in the scene.
type Source struct {
	Position    geometry.Vec3       `json:"position"`
	Orientation geometry.Quaternion `json:"orientation"`
	GainDB      float64             `json:"gainDb"`
	Directivity directivity.Model   `json:"-"`
}

type sourceJSON struct {
	Position    geometry.Vec3       `json:"position"`
	Orientation geometry.Quaternion `json:"orientation"`
	GainDB      float64             `json:"gainDb"`
	Directivity *directivityJSON    `json:"directivity,omitempty"`
}

type directivityJSON struct {
	Type   string        `json:"type"`
	Axis   geometry.Vec3 `json:"axis,omitempty"`
	OrderN float64       `json:"orderN,omitempty"`
}

func directivityJSONFromModel(model directivity.Model) (*directivityJSON, error) {
	switch typed := model.(type) {
	case directivity.OmniModel:
		return &directivityJSON{Type: "omni"}, nil
	case *directivity.OmniModel:
		return &directivityJSON{Type: "omni"}, nil
	case directivity.CardioidModel:
		return &directivityJSON{Type: "cardioid", Axis: typed.Axis, OrderN: typed.OrderN}, nil
	case *directivity.CardioidModel:
		return &directivityJSON{Type: "cardioid", Axis: typed.Axis, OrderN: typed.OrderN}, nil
	default:
		return nil, fmt.Errorf("unsupported directivity model %T", model)
	}
}

func (d directivityJSON) toModel() (directivity.Model, error) {
	switch d.Type {
	case "omni":
		return directivity.OmniModel{}, nil
	case "cardioid":
		return directivity.CardioidModel{Axis: d.Axis, OrderN: d.OrderN}, nil
	default:
		return nil, fmt.Errorf("unsupported directivity type %q", d.Type)
	}
}

// DirectionTo returns the direction to target in the source-local frame.
func (s Source) DirectionTo(target geometry.Vec3) geometry.Vec3 {
	relative := target.Sub(s.Position)
	return effectiveOrientation(s.Orientation).Conj().Rotate(relative).Normalize()
}

// MarshalJSON preserves the interface-backed directivity field.
func (s Source) MarshalJSON() ([]byte, error) {
	encoded := sourceJSON{
		Position:    s.Position,
		Orientation: s.Orientation,
		GainDB:      s.GainDB,
	}

	if s.Directivity != nil {
		directivityValue, err := directivityJSONFromModel(s.Directivity)
		if err != nil {
			return nil, err
		}

		encoded.Directivity = directivityValue
	}

	return json.Marshal(encoded)
}

// UnmarshalJSON restores the interface-backed directivity field.
func (s *Source) UnmarshalJSON(data []byte) error {
	var decoded sourceJSON

	err := json.Unmarshal(data, &decoded)
	if err != nil {
		return err
	}

	s.Position = decoded.Position
	s.Orientation = decoded.Orientation
	s.GainDB = decoded.GainDB
	s.Directivity = nil

	if decoded.Directivity != nil {
		model, err := decoded.Directivity.toModel()
		if err != nil {
			return err
		}

		s.Directivity = model
	}

	return nil
}
