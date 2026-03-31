package scene

import (
	"errors"
	"fmt"
	"strings"
)

// ValidationErrors collects all scene validation failures.
type ValidationErrors []error

// Error returns a human-readable summary of the collected validation errors.
func (errs ValidationErrors) Error() string {
	if len(errs) == 0 {
		return ""
	}

	if len(errs) == 1 {
		return errs[0].Error()
	}

	var builder strings.Builder
	builder.WriteString("scene validation failed:")
	for _, err := range errs {
		builder.WriteString("\n- ")
		builder.WriteString(err.Error())
	}

	return builder.String()
}

// Unwrap exposes the underlying errors for errors.Is / errors.As.
func (errs ValidationErrors) Unwrap() []error {
	return []error(errs)
}

// Validate checks that a scene is internally consistent enough for later stages.
func Validate(s *Scene) error {
	if s == nil {
		return errors.New("scene is nil")
	}

	var errs ValidationErrors

	if s.SampleRate <= 0 {
		errs = append(errs, errors.New("sample rate must be greater than zero"))
	}

	bandCount := s.BandSpec.BandCount()
	if err := validateRoom(s.Room); err != nil {
		errs = append(errs, err)
	}

	if s.Room.Kind == RoomKindShoebox && s.Room.Shoebox != nil {
		for index, materialName := range s.Room.Shoebox.WallMaterials {
			if materialName == "" {
				continue
			}

			if _, ok := s.Materials[materialName]; !ok {
				errs = append(errs, fmt.Errorf("shoebox wall material %d references undefined material %q", index, materialName))
			}
		}
	}

	for name, material := range s.Materials {
		if len(material.AbsorptionByBand) != bandCount {
			errs = append(errs, fmt.Errorf("material %q absorption band count = %d, want %d", name, len(material.AbsorptionByBand), bandCount))
		}
	}

	roomBounds, ok := s.Room.Bounds()
	if ok {
		for index, source := range s.Sources {
			if !roomBounds.Contains(source.Position) {
				errs = append(errs, fmt.Errorf("source[%d] position %v is outside the room bounds", index, source.Position))
			}
		}

		for index, receiver := range s.Receivers {
			if !roomBounds.Contains(receiver.Position) {
				errs = append(errs, fmt.Errorf("receiver[%d] position %v is outside the room bounds", index, receiver.Position))
			}
		}
	}

	for index, receiver := range s.Receivers {
		if receiver.Type == ReceiverBinaural && receiver.HRTF == nil {
			errs = append(errs, fmt.Errorf("receiver[%d] binaural receivers require an HRTF dataset", index))
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func validateRoom(room Room) error {
	switch room.Kind {
	case RoomKindShoebox:
		if room.Shoebox == nil {
			return errors.New("shoebox room requires a shoebox definition")
		}

		var errs ValidationErrors
		if room.Shoebox.Width <= 0 {
			errs = append(errs, errors.New("shoebox width must be greater than zero"))
		}
		if room.Shoebox.Depth <= 0 {
			errs = append(errs, errors.New("shoebox depth must be greater than zero"))
		}
		if room.Shoebox.Height <= 0 {
			errs = append(errs, errors.New("shoebox height must be greater than zero"))
		}
		for index, materialName := range room.Shoebox.WallMaterials {
			if materialName == "" {
				errs = append(errs, fmt.Errorf("shoebox wall material %d is empty", index))
			}
		}

		if len(errs) > 0 {
			return errs
		}
	case RoomKindMesh:
		if room.Mesh == nil {
			return errors.New("mesh room requires a mesh definition")
		}
		if len(room.Mesh.Triangles) == 0 {
			return errors.New("mesh room requires at least one triangle")
		}
		if bounds := room.Mesh.BoundingBox(); bounds.Volume() <= 0 {
			return errors.New("mesh room bounds must have positive volume")
		}
	default:
		return errors.New("room kind must be set to shoebox or mesh")
	}

	return nil
}
