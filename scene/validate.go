package scene

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/cwbudde/algo-acoustics/geometry"
)

var validationWarnf = log.Printf

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

	errs = append(errs, validateBandSpec(s)...)
	errs = append(errs, validationErrors(validateRoom(s.Room))...)
	errs = append(errs, validateMaterialReferences(s)...)
	errs = append(errs, validateMaterials(s.Materials, s.BandSpec.BandCount())...)
	errs = append(errs, validatePositions(s)...)
	errs = append(errs, validateReceivers(s)...)

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func validationErrors(err error) ValidationErrors {
	if err == nil {
		return nil
	}

	var errs ValidationErrors
	if errors.As(err, &errs) {
		return errs
	}

	return ValidationErrors{err}
}

func validateMaterialReferences(s *Scene) ValidationErrors {
	var errs ValidationErrors

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

	if s.Room.Kind == RoomKindMesh && s.Room.MeshMaterial != "" {
		if _, ok := s.Materials[s.Room.MeshMaterial]; !ok {
			errs = append(errs, fmt.Errorf("mesh material references undefined material %q", s.Room.MeshMaterial))
		}
	}

	return errs
}

func validateMaterials(materials map[string]Material, bandCount int) ValidationErrors {
	var errs ValidationErrors

	for name, material := range materials {
		if len(material.AbsorptionByBand) != 1 && len(material.AbsorptionByBand) != bandCount {
			errs = append(errs, fmt.Errorf("material %q absorption band count = %d, want %d", name, len(material.AbsorptionByBand), bandCount))
		}

		for index, coeff := range material.AbsorptionByBand {
			if !isFinite(coeff) || coeff < 0 || coeff > 1 {
				errs = append(errs, fmt.Errorf("material %q absorption[%d] = %v, want finite and within [0, 1]", name, index, coeff))
			}
		}

		scattering := material.ScatteringByBand
		if len(scattering) == 0 {
			scattering = material.Scattering[:]
		}

		for index, coeff := range scattering {
			if !isFinite(coeff) || coeff < 0 || coeff > 1 {
				errs = append(errs, fmt.Errorf("material %q scattering[%d] = %v, want finite and within [0, 1]", name, index, coeff))
			}
		}

		for index := 1; index < len(scattering); index++ {
			if scattering[index] < scattering[index-1] {
				validationWarnf("scene validate warning: material %q scattering is not monotonic non-decreasing at band %d", name, index)
				break
			}
		}
	}

	return errs
}

func validatePositions(s *Scene) ValidationErrors {
	var errs ValidationErrors

	roomBounds, ok := s.Room.Bounds()

	for index, source := range s.Sources {
		if !isFiniteVec3(source.Position) {
			errs = append(errs, fmt.Errorf("source[%d] position %v must be finite", index, source.Position))
		} else if ok && !roomBounds.Contains(source.Position) {
			errs = append(errs, fmt.Errorf("source[%d] position %v is outside the room bounds", index, source.Position))
		}
	}

	for index, receiver := range s.Receivers {
		if !isFiniteVec3(receiver.Position) {
			errs = append(errs, fmt.Errorf("receiver[%d] position %v must be finite", index, receiver.Position))
		} else if ok && !roomBounds.Contains(receiver.Position) {
			errs = append(errs, fmt.Errorf("receiver[%d] position %v is outside the room bounds", index, receiver.Position))
		}
	}

	return errs
}

func validateReceivers(s *Scene) ValidationErrors {
	var errs ValidationErrors

	for index, receiver := range s.Receivers {
		switch receiver.Type {
		case "", ReceiverOmni:
			// Preserve the Go API's historical zero-value behavior: an empty
			// receiver type is an implicit omni receiver.
		case ReceiverBinaural:
			if receiver.HRTF == nil {
				errs = append(errs, fmt.Errorf("receiver[%d] binaural receivers require an HRTF dataset", index))
			} else if receiver.HRTF.SampleRate() != s.SampleRate {
				errs = append(errs, fmt.Errorf("receiver[%d] HRTF sample rate = %d, want scene sample rate %d", index, receiver.HRTF.SampleRate(), s.SampleRate))
			}
		default:
			errs = append(errs, fmt.Errorf("receiver[%d] has unsupported receiver type %q", index, receiver.Type))
		}
	}

	return errs
}

func validateBandSpec(s *Scene) ValidationErrors {
	var errs ValidationErrors
	centers := s.BandSpec.CenterFreqs
	lowers := s.BandSpec.LowerEdges
	uppers := s.BandSpec.UpperEdges

	if len(centers) == 0 {
		errs = append(errs, errors.New("band spec must contain at least one band"))
	}

	if len(lowers) != len(centers) || len(uppers) != len(centers) {
		errs = append(errs, fmt.Errorf("band spec lengths must match: centers=%d lower_edges=%d upper_edges=%d", len(centers), len(lowers), len(uppers)))
		return errs
	}

	for index := range centers {
		errs = append(errs, validateBand(index, centers, lowers, uppers, s.SampleRate)...)
	}

	return errs
}

func validateBand(index int, centers, lowers, uppers []float64, sampleRate int) ValidationErrors {
	center, lower, upper := centers[index], lowers[index], uppers[index]
	if !isFinite(center) || !isFinite(lower) || !isFinite(upper) || lower <= 0 || center <= 0 || upper <= 0 {
		return ValidationErrors{fmt.Errorf("band spec band[%d] frequencies must be finite and greater than zero", index)}
	}

	var errs ValidationErrors
	if lower >= center || center >= upper {
		errs = append(errs, fmt.Errorf("band spec band[%d] must satisfy lower edge < center frequency < upper edge", index))
	}

	if index > 0 && (centers[index-1] >= center || lowers[index-1] >= lower || uppers[index-1] >= upper) {
		errs = append(errs, fmt.Errorf("band spec frequencies must be strictly increasing at band[%d]", index))
	}

	if sampleRate > 0 && upper > float64(sampleRate)/2 {
		errs = append(errs, fmt.Errorf("band spec upper edge[%d] = %v exceeds Nyquist frequency %v", index, upper, float64(sampleRate)/2))
	}

	return errs
}

func validateRoom(room Room) error {
	switch room.Kind {
	case RoomKindShoebox:
		return validateShoeboxRoom(room)
	case RoomKindMesh:
		return validateMeshRoom(room)
	default:
		return errors.New("room kind must be set to shoebox or mesh")
	}
}

func validateShoeboxRoom(room Room) error {
	if !room.IsValid() {
		return errors.New("shoebox room requires a shoebox definition")
	}

	var errs ValidationErrors
	if !isFinite(room.Shoebox.Width) || room.Shoebox.Width <= 0 {
		errs = append(errs, errors.New("shoebox width must be finite and greater than zero"))
	}

	if !isFinite(room.Shoebox.Depth) || room.Shoebox.Depth <= 0 {
		errs = append(errs, errors.New("shoebox depth must be finite and greater than zero"))
	}

	if !isFinite(room.Shoebox.Height) || room.Shoebox.Height <= 0 {
		errs = append(errs, errors.New("shoebox height must be finite and greater than zero"))
	}

	for index, materialName := range room.Shoebox.WallMaterials {
		if materialName == "" {
			errs = append(errs, fmt.Errorf("shoebox wall material %d is empty", index))
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func validateMeshRoom(room Room) error {
	if !room.IsValid() {
		return errors.New("mesh room requires a mesh definition")
	}

	err := room.Mesh.Validate()
	if err != nil {
		var issues *geometry.MeshValidationIssues
		if !errors.As(err, &issues) || issues.HasProblems() {
			return fmt.Errorf("mesh room is invalid: %w", err)
		}
	}

	if bounds := room.Mesh.BoundingBox(); bounds.Volume() <= 0 {
		return errors.New("mesh room bounds must have positive volume")
	}

	return nil
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func isFiniteVec3(v geometry.Vec3) bool {
	return isFinite(v.X) && isFinite(v.Y) && isFinite(v.Z)
}
