package scene

import (
	"encoding/json"
	"fmt"

	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
)

const (
	directivityTypeOmni     = "omni"
	directivityTypeCardioid = "cardioid"
)

// Source is an emitting point in the scene.
//
//nolint:recvcheck,tagliatelle // Mutable JSON receiver; camel-case tags preserve the public scene schema.
type Source struct {
	Position    geometry.Vec3       `json:"position"`
	Orientation geometry.Quaternion `json:"orientation"`
	GainDB      float64             `json:"gainDb"`
	Directivity directivity.Model   `json:"-"`
}

//nolint:tagliatelle // This compatibility payload mirrors the public scene schema.
type sourceJSON struct {
	Position    geometry.Vec3       `json:"position"`
	Orientation geometry.Quaternion `json:"orientation"`
	GainDB      float64             `json:"gainDb"`
	Directivity *directivityJSON    `json:"directivity,omitempty"`
}

//nolint:tagliatelle // orderN is part of the established public scene schema.
type directivityJSON struct {
	Type   string        `json:"type"`
	Axis   geometry.Vec3 `json:"axis"`
	OrderN float64       `json:"orderN,omitempty"`
}

func directivityJSONFromModel(model directivity.Model) (*directivityJSON, error) {
	switch typed := model.(type) {
	case directivity.OmniModel:
		return &directivityJSON{Type: directivityTypeOmni}, nil
	case *directivity.OmniModel:
		return &directivityJSON{Type: directivityTypeOmni}, nil
	case directivity.CardioidModel:
		return &directivityJSON{Type: directivityTypeCardioid, Axis: typed.Axis, OrderN: typed.OrderN}, nil
	case *directivity.CardioidModel:
		return &directivityJSON{Type: directivityTypeCardioid, Axis: typed.Axis, OrderN: typed.OrderN}, nil
	default:
		return nil, fmt.Errorf("unsupported directivity model %T", model)
	}
}

func (d directivityJSON) toModel() (directivity.Model, error) {
	switch d.Type {
	case directivityTypeOmni:
		return directivity.OmniModel{}, nil
	case directivityTypeCardioid:
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
			return nil, fmt.Errorf("encode directivity model: %w", err)
		}

		encoded.Directivity = directivityValue
	}

	data, err := json.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("marshal source: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the interface-backed directivity field.
func (s *Source) UnmarshalJSON(data []byte) error {
	var decoded sourceJSON

	err := json.Unmarshal(data, &decoded)
	if err != nil {
		return fmt.Errorf("unmarshal source: %w", err)
	}

	s.Position = decoded.Position
	s.Orientation = decoded.Orientation
	s.GainDB = decoded.GainDB
	s.Directivity = nil

	if decoded.Directivity != nil {
		model, err := decoded.Directivity.toModel()
		if err != nil {
			return fmt.Errorf("decode directivity model: %w", err)
		}

		s.Directivity = model
	}

	return nil
}
