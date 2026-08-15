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

const materialConservationTolerance = 1e-12

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
	errs = append(errs, validateSceneRooms(s)...)
	errs = append(errs, validateMaterialReferences(s)...)
	errs = append(errs, validateMaterials(s.Materials, s.BandSpec.BandCount())...)
	errs = append(errs, validatePortals(s)...)
	errs = append(errs, validatePositions(s)...)
	errs = append(errs, validateReceivers(s)...)

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func validateSceneRooms(s *Scene) ValidationErrors {
	var errs ValidationErrors
	if roomIsSet(s.Room) && len(s.Rooms) > 0 {
		errs = append(errs, errors.New("scene must define either room or rooms, not both"))
	}

	if len(s.Rooms) == 1 {
		errs = append(errs, errors.New("rooms must contain at least two rooms; use room for a single-room scene"))
	}

	if len(s.Portals) > 0 && len(s.Rooms) == 0 {
		errs = append(errs, errors.New("portals require the rooms representation"))
	}

	if s.RoomCount() == 0 {
		return append(errs, errors.New("scene must define at least one room"))
	}

	for index := range s.RoomCount() {
		room, ok := s.RoomAt(index)
		if !ok {
			continue
		}

		roomErrs := validationErrors(validateRoom(*room))
		if len(s.Rooms) == 0 {
			errs = append(errs, roomErrs...)
			continue
		}

		for _, err := range roomErrs {
			errs = append(errs, fmt.Errorf("room[%d]: %w", index, err))
		}
	}

	return errs
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

	for roomIndex := range s.RoomCount() {
		room, ok := s.RoomAt(roomIndex)
		if !ok {
			continue
		}

		prefix := ""
		if len(s.Rooms) > 0 {
			prefix = fmt.Sprintf("room[%d] ", roomIndex)
		}

		if room.Kind == RoomKindShoebox && room.Shoebox != nil {
			for wallIndex, materialName := range room.Shoebox.WallMaterials {
				if materialName == "" {
					continue
				}

				if _, ok := s.Materials[materialName]; !ok {
					errs = append(errs, fmt.Errorf("%sshoebox wall material %d references undefined material %q", prefix, wallIndex, materialName))
				}
			}
		}

		if room.Kind != RoomKindMesh {
			continue
		}

		if room.MeshMaterial != "" {
			if _, ok := s.Materials[room.MeshMaterial]; !ok {
				errs = append(errs, fmt.Errorf("%smesh material references undefined material %q", prefix, room.MeshMaterial))
			}
		}

		errs = append(errs, validateTriangleMaterialReferences(s, prefix, *room)...)
	}

	return errs
}

func validateTriangleMaterialReferences(s *Scene, prefix string, room Room) ValidationErrors {
	var errs ValidationErrors

	for triangleIndex, materialName := range room.TriangleMaterials {
		if materialName == "" {
			continue
		}

		if _, ok := s.Materials[materialName]; !ok {
			errs = append(errs, fmt.Errorf("%striangle material %d references undefined material %q", prefix, triangleIndex, materialName))
		}
	}

	return errs
}

func validateMaterialBandCount(name, property string, values []float64, bandCount int) ValidationErrors {
	if len(values) == 0 || len(values) == 1 || len(values) == bandCount {
		return nil
	}

	return ValidationErrors{fmt.Errorf("material %q %s band count = %d, want 1 or %d", name, property, len(values), bandCount)}
}

func validateMaterials(materials map[string]Material, bandCount int) ValidationErrors {
	errs := make(ValidationErrors, 0, len(materials))

	for name, material := range materials {
		errs = append(errs, validateMaterial(name, material, bandCount)...)
	}

	return errs
}

func validateMaterial(name string, material Material, bandCount int) ValidationErrors {
	var errs ValidationErrors

	if len(material.AbsorptionByBand) != 1 && len(material.AbsorptionByBand) != bandCount {
		errs = append(errs, fmt.Errorf("material %q absorption band count = %d, want %d", name, len(material.AbsorptionByBand), bandCount))
	}

	errs = append(errs, validateUnitCoefficients(name, "absorption", material.AbsorptionByBand)...)
	errs = append(errs, validateMaterialScattering(name, material)...)
	errs = append(errs, validateMaterialTransmission(name, material, bandCount)...)
	errs = append(errs, validateMaterialEnergy(name, material, bandCount)...)

	return errs
}

func validateUnitCoefficients(name, property string, values []float64) ValidationErrors {
	var errs ValidationErrors

	for index, coefficient := range values {
		if !isFinite(coefficient) || coefficient < 0 || coefficient > 1 {
			errs = append(errs, fmt.Errorf("material %q %s[%d] = %v, want finite and within [0, 1]", name, property, index, coefficient))
		}
	}

	return errs
}

func validateMaterialScattering(name string, material Material) ValidationErrors {
	scattering := material.ScatteringByBand
	if len(scattering) == 0 {
		scattering = material.Scattering[:]
	}

	errs := validateUnitCoefficients(name, "scattering", scattering)
	for index := 1; index < len(scattering); index++ {
		if scattering[index] < scattering[index-1] {
			validationWarnf("scene validate warning: material %q scattering is not monotonic non-decreasing at band %d", name, index)
			break
		}
	}

	return errs
}

func validateMaterialTransmission(name string, material Material, bandCount int) ValidationErrors {
	var errs ValidationErrors

	errs = append(errs, validateMaterialBandCount(name, "transmission", material.TransmissionByBand, bandCount)...)
	errs = append(errs, validateUnitCoefficients(name, "transmission", material.TransmissionByBand)...)
	errs = append(errs, validateMaterialBandCount(name, "sound reduction index", material.SoundReductionIndex, bandCount)...)

	for index, reduction := range material.SoundReductionIndex {
		if !isFinite(reduction) || reduction < 0 {
			errs = append(errs, fmt.Errorf("material %q sound reduction index[%d] = %v, want finite and non-negative", name, index, reduction))
		}
	}

	if len(material.TransmissionByBand) == 0 || len(material.SoundReductionIndex) == 0 {
		return errs
	}

	for index := range bandCount {
		tau, tauOK := coefficientAt(material.TransmissionByBand, index)

		reduction, reductionOK := coefficientAt(material.SoundReductionIndex, index)
		if tauOK && reductionOK && math.Abs(tau-TransmissionFromSoundReductionIndex(reduction)) > materialConservationTolerance {
			errs = append(errs, fmt.Errorf("material %q transmission and sound reduction index disagree at band %d", name, index))
		}
	}

	return errs
}

func validateMaterialEnergy(name string, material Material, bandCount int) ValidationErrors {
	var errs ValidationErrors

	for index := range bandCount {
		alpha := material.AbsorptionAt(index)

		tau := material.TransmissionAt(index)
		if isFinite(alpha) && isFinite(tau) && alpha+tau > 1+materialConservationTolerance {
			errs = append(errs, fmt.Errorf("material %q absorption + transmission at band %d = %v, want <= 1", name, index, alpha+tau))
		}
	}

	return errs
}

func validatePortals(s *Scene) ValidationErrors {
	errs := make(ValidationErrors, 0, len(s.Portals))

	for index, portal := range s.Portals {
		errs = append(errs, validatePortal(s, index, portal)...)
	}

	return errs
}

func validatePortal(s *Scene, index int, portal Portal) ValidationErrors {
	prefix := fmt.Sprintf("portal[%d]", index)
	roomA, roomAOK := s.RoomAt(portal.RoomIndices[0])
	roomB, roomBOK := s.RoomAt(portal.RoomIndices[1])
	errs := validatePortalReferences(s, prefix, portal, roomAOK, roomBOK)

	plane, polygonErrs, polygonOK := validatePortalPolygon(prefix, portal)

	errs = append(errs, polygonErrs...)
	if !polygonOK {
		return errs
	}

	if roomAOK {
		errs = append(errs, validatePortalBoundary(prefix, portal, *roomA, portal.RoomIndices[0])...)
	}

	if roomBOK {
		errs = append(errs, validatePortalBoundary(prefix, portal, *roomB, portal.RoomIndices[1])...)
	}

	if roomAOK && roomBOK {
		errs = append(errs, validatePortalWinding(prefix, plane, *roomA, *roomB, portal.RoomIndices)...)
	}

	return errs
}

func validatePortalReferences(s *Scene, prefix string, portal Portal, roomAOK, roomBOK bool) ValidationErrors {
	var errs ValidationErrors

	if !roomAOK {
		errs = append(errs, fmt.Errorf("%s room index %d is out of range", prefix, portal.RoomIndices[0]))
	}

	if !roomBOK {
		errs = append(errs, fmt.Errorf("%s room index %d is out of range", prefix, portal.RoomIndices[1]))
	}

	if portal.RoomIndices[0] == portal.RoomIndices[1] {
		errs = append(errs, fmt.Errorf("%s must connect two distinct rooms", prefix))
	}

	if portal.State != PortalOpen && portal.State != PortalClosed {
		errs = append(errs, fmt.Errorf("%s has unsupported state %q", prefix, portal.State))
	}

	if _, ok := s.Materials[portal.Material]; !ok {
		errs = append(errs, fmt.Errorf("%s references undefined material %q", prefix, portal.Material))
	}

	return errs
}

func validatePortalPolygon(prefix string, portal Portal) (geometry.Plane, ValidationErrors, bool) {
	if len(portal.Polygon) < 3 {
		return geometry.Plane{}, ValidationErrors{fmt.Errorf("%s polygon requires at least three vertices", prefix)}, false
	}

	var errs ValidationErrors

	for vertexIndex, vertex := range portal.Polygon {
		if !isFiniteVec3(vertex) {
			errs = append(errs, fmt.Errorf("%s polygon vertex[%d] must be finite", prefix, vertexIndex))
		}
	}

	if len(errs) > 0 {
		return geometry.Plane{}, errs, false
	}

	normal := portal.Normal()
	if normal == geometry.Vec3Zero || portal.Area() <= portalGeometryTolerance {
		return geometry.Plane{}, ValidationErrors{fmt.Errorf("%s polygon must have non-zero area", prefix)}, false
	}

	plane := geometry.NewPlaneFromPointNormal(portal.Polygon[0], normal)
	if !portalPolygonIsPlanar(portal.Polygon, plane) {
		return geometry.Plane{}, ValidationErrors{fmt.Errorf("%s polygon vertices must be coplanar", prefix)}, false
	}

	return plane, nil, true
}

func validatePortalBoundary(prefix string, portal Portal, room Room, roomIndex int) ValidationErrors {
	if portalOnRoomBoundary(portal, room) {
		return nil
	}

	return ValidationErrors{fmt.Errorf("%s polygon is not on a boundary wall of room %d", prefix, roomIndex)}
}

func validatePortalWinding(prefix string, plane geometry.Plane, roomA, roomB Room, roomIndices [2]int) ValidationErrors {
	boundsA, boundsAOK := roomA.Bounds()

	boundsB, boundsBOK := roomB.Bounds()
	if !boundsAOK || !boundsBOK {
		return nil
	}

	sideA := plane.SideOf(boundsA.Center())

	sideB := plane.SideOf(boundsB.Center())
	if sideA < -portalGeometryTolerance && sideB > portalGeometryTolerance {
		return nil
	}

	return ValidationErrors{fmt.Errorf("%s polygon winding must point from room %d to room %d", prefix, roomIndices[0], roomIndices[1])}
}

func portalPolygonIsPlanar(vertices []geometry.Vec3, plane geometry.Plane) bool {
	for _, vertex := range vertices[1:] {
		if math.Abs(plane.SideOf(vertex)) > portalGeometryTolerance {
			return false
		}
	}

	return true
}

func portalOnRoomBoundary(portal Portal, room Room) bool {
	switch room.Kind {
	case RoomKindShoebox:
		if room.Shoebox == nil {
			return false
		}

		return portalOnShoeboxBoundary(portal.Polygon, room.Shoebox.Bounds())
	case RoomKindMesh:
		if room.Mesh == nil {
			return false
		}

		portalNormal := portal.Normal()
		portalPlane := geometry.NewPlaneFromPointNormal(portal.Polygon[0], portalNormal)

		for _, triangle := range room.Mesh.Triangles {
			triangleNormal := triangle.Normal()
			if math.Abs(math.Abs(triangleNormal.Dot(portalNormal))-1) > portalGeometryTolerance {
				continue
			}

			if math.Abs(portalPlane.SideOf(triangle.V0)) <= portalGeometryTolerance &&
				math.Abs(portalPlane.SideOf(triangle.V1)) <= portalGeometryTolerance &&
				math.Abs(portalPlane.SideOf(triangle.V2)) <= portalGeometryTolerance {
				return true
			}
		}
	}

	return false
}

func portalOnShoeboxBoundary(vertices []geometry.Vec3, bounds geometry.Box) bool {
	for face := range 6 {
		matches := true

		for _, vertex := range vertices {
			if !portalVertexOnShoeboxFace(vertex, bounds, face) {
				matches = false
				break
			}
		}

		if matches {
			return true
		}
	}

	return false
}

func portalVertexOnShoeboxFace(vertex geometry.Vec3, bounds geometry.Box, face int) bool {
	switch face {
	case 0:
		return math.Abs(vertex.X-bounds.Min.X) <= portalGeometryTolerance && portalWithinYZ(vertex, bounds)
	case 1:
		return math.Abs(vertex.X-bounds.Max.X) <= portalGeometryTolerance && portalWithinYZ(vertex, bounds)
	case 2:
		return math.Abs(vertex.Y-bounds.Min.Y) <= portalGeometryTolerance && portalWithinXZ(vertex, bounds)
	case 3:
		return math.Abs(vertex.Y-bounds.Max.Y) <= portalGeometryTolerance && portalWithinXZ(vertex, bounds)
	case 4:
		return math.Abs(vertex.Z-bounds.Min.Z) <= portalGeometryTolerance && portalWithinXY(vertex, bounds)
	case 5:
		return math.Abs(vertex.Z-bounds.Max.Z) <= portalGeometryTolerance && portalWithinXY(vertex, bounds)
	default:
		return false
	}
}

func portalWithinYZ(vertex geometry.Vec3, bounds geometry.Box) bool {
	return withinPortalBounds(vertex.Y, bounds.Min.Y, bounds.Max.Y) && withinPortalBounds(vertex.Z, bounds.Min.Z, bounds.Max.Z)
}

func portalWithinXZ(vertex geometry.Vec3, bounds geometry.Box) bool {
	return withinPortalBounds(vertex.X, bounds.Min.X, bounds.Max.X) && withinPortalBounds(vertex.Z, bounds.Min.Z, bounds.Max.Z)
}

func portalWithinXY(vertex geometry.Vec3, bounds geometry.Box) bool {
	return withinPortalBounds(vertex.X, bounds.Min.X, bounds.Max.X) && withinPortalBounds(vertex.Y, bounds.Min.Y, bounds.Max.Y)
}

func withinPortalBounds(value, lower, upper float64) bool {
	return value >= lower-portalGeometryTolerance && value <= upper+portalGeometryTolerance
}

func validatePositions(s *Scene) ValidationErrors {
	var errs ValidationErrors

	for index, source := range s.Sources {
		if !isFiniteVec3(source.Position) {
			errs = append(errs, fmt.Errorf("source[%d] position %v must be finite", index, source.Position))
		} else if _, ok := s.RoomIndexAt(source.Position); !ok {
			if s.RoomCount() == 1 {
				errs = append(errs, fmt.Errorf("source[%d] position %v is outside the room bounds", index, source.Position))
			} else {
				errs = append(errs, fmt.Errorf("source[%d] position %v must be inside exactly one room", index, source.Position))
			}
		}
	}

	for index, receiver := range s.Receivers {
		if !isFiniteVec3(receiver.Position) {
			errs = append(errs, fmt.Errorf("receiver[%d] position %v must be finite", index, receiver.Position))
		} else if _, ok := s.RoomIndexAt(receiver.Position); !ok {
			if s.RoomCount() == 1 {
				errs = append(errs, fmt.Errorf("receiver[%d] position %v is outside the room bounds", index, receiver.Position))
			} else {
				errs = append(errs, fmt.Errorf("receiver[%d] position %v must be inside exactly one room", index, receiver.Position))
			}
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
	if len(room.TriangleMaterials) > 0 {
		errs = append(errs, errors.New("shoebox room must not set triangle materials; use wallMaterials"))
	}

	if !isFiniteVec3(room.Shoebox.Origin) {
		errs = append(errs, errors.New("shoebox origin must be finite"))
	}

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

	// An explicitly present but empty list is a mismatch, not an omission: the
	// documented contract is absent or one entry per triangle.
	if count := len(room.TriangleMaterials); room.TriangleMaterials != nil && count != len(room.Mesh.Triangles) {
		return fmt.Errorf("mesh room triangle material count = %d, want %d", count, len(room.Mesh.Triangles))
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
