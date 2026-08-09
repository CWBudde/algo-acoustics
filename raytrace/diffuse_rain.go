package raytrace

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// SurfaceReceiver models a planar detector area (e.g. a portal surface)
// for capturing diffuse rain energy from ray tracing. It will serve as
// the portal detector in multi-room sound transmission (Phase 21).
type SurfaceReceiver struct {
	Center  geometry.Vec3   // Center of the detector surface
	Normal  geometry.Vec3   // Outward-facing unit normal of the surface
	Area    float64         // Area in m^2
	Polygon []geometry.Vec3 // Ordered, planar polygon vertices
}

// NewSurfaceReceiver constructs a planar polygon detector. Vertices must be
// ordered around a convex polygon; the winding defines the detector normal.
func NewSurfaceReceiver(polygon []geometry.Vec3) (SurfaceReceiver, error) {
	vertices := append([]geometry.Vec3(nil), polygon...)
	if len(vertices) > 1 && vertices[0] == vertices[len(vertices)-1] {
		vertices = vertices[:len(vertices)-1]
	}

	if len(vertices) < 3 {
		return SurfaceReceiver{}, errors.New("surface receiver requires at least three vertices")
	}

	var normal geometry.Vec3

	for index, vertex := range vertices {
		next := vertices[(index+1)%len(vertices)]
		normal.X += (vertex.Y - next.Y) * (vertex.Z + next.Z)
		normal.Y += (vertex.Z - next.Z) * (vertex.X + next.X)
		normal.Z += (vertex.X - next.X) * (vertex.Y + next.Y)
	}

	normal = normal.Normalize()
	if normal == geometry.Vec3Zero {
		return SurfaceReceiver{}, errors.New("surface receiver polygon is degenerate")
	}

	const planarTolerance = 1e-7

	plane := geometry.NewPlaneFromPointNormal(vertices[0], normal)
	for _, vertex := range vertices[1:] {
		if math.Abs(plane.SideOf(vertex)) > planarTolerance {
			return SurfaceReceiver{}, errors.New("surface receiver polygon is not planar")
		}
	}

	var area float64
	var weightedCenter geometry.Vec3

	for index := 1; index+1 < len(vertices); index++ {
		triangle := geometry.Triangle{V0: vertices[0], V1: vertices[index], V2: vertices[index+1]}
		triangleArea := triangle.Area()
		area += triangleArea
		weightedCenter = weightedCenter.Add(triangle.Centroid().Scale(triangleArea))
	}

	if area <= 0 || math.IsNaN(area) || math.IsInf(area, 0) {
		return SurfaceReceiver{}, errors.New("surface receiver polygon has invalid area")
	}

	return SurfaceReceiver{
		Center:  weightedCenter.Scale(1 / area),
		Normal:  normal,
		Area:    area,
		Polygon: vertices,
	}, nil
}

// Intersects reports the nearest intersection with the detector polygon in
// [tMin, tMax]. The polygon is double-sided; Normal controls rain orientation.
func (s SurfaceReceiver) Intersects(ray geometry.Ray, tMin, tMax float64) (float64, bool) {
	if len(s.Polygon) < 3 || tMax < tMin {
		return 0, false
	}

	nearest := math.Inf(1)

	for index := 1; index+1 < len(s.Polygon); index++ {
		triangle := geometry.Triangle{V0: s.Polygon[0], V1: s.Polygon[index], V2: s.Polygon[index+1]}

		distance, hit := geometry.RayTriangle(ray, triangle)
		if hit && distance >= tMin && distance <= tMax && distance < nearest {
			nearest = distance
		}
	}

	if math.IsInf(nearest, 1) {
		return 0, false
	}

	return nearest, true
}

// DiffuseRainContribution holds the energy and arrival time of a single
// diffuse rain deposit into the energy histogram.
type DiffuseRainContribution struct {
	BandEnergy  []float64
	ArrivalTime float64
}

// computeDiffuseRain calculates the secondary radiation energy from a diffuse
// reflection point toward the receiver sphere. This implements the RAVEN
// "diffuse rain" variance-reduction technique (Schroeder 2011, Eq. 5.20):
//
//	E_s = E_P * s * (1 - cos(gamma/2)) * 2 * cos(Theta) * exp(-m*r)
//
// where E_P is the per-band particle energy (already after wall absorption),
// s is the per-band scattering coefficient, gamma is the receiver opening
// angle, Theta is the angle between surface normal and receiver direction,
// and r is the distance from reflection point to receiver center.
//
// If the receiver is not visible from the reflection point (occluded by
// geometry), nil is returned.
func computeDiffuseRain(
	reflectionPoint, surfaceNormal geometry.Vec3,
	energy, scattering []float64,
	receiver SphereReceiver,
	tracer Tracer,
	centerFreqs []float64,
	pathLength float64,
	speedOfSound float64,
) *DiffuseRainContribution {
	if len(energy) == 0 || receiver.Radius <= 0 || speedOfSound <= 0 {
		return nil
	}

	// Direction and distance from reflection point to receiver center.
	toReceiver := receiver.Center.Sub(reflectionPoint)
	dist := toReceiver.Norm()

	if dist <= 0 {
		return nil
	}

	dir := toReceiver.Scale(1 / dist)

	// Lambert cosine factor: cos(Theta) between surface normal and
	// the direction toward the receiver. Negative means the receiver
	// is behind the surface — no rain contribution.
	cosTheta := surfaceNormal.Dot(dir)
	if cosTheta <= 0 {
		return nil
	}

	// Visibility check: cast a ray from the reflection point toward
	// the receiver. If geometry is hit before reaching the receiver
	// sphere, the path is occluded.
	if tracer != nil {
		origin := reflectionPoint.Add(dir.Scale(wallEpsilon))
		ray := geometry.NewRay(origin, dir)

		hitPoint, _, _, hit := tracer.NextHit(ray)
		if hit {
			hitDist := origin.Distance(hitPoint)
			receiverEntry := dist - receiver.Radius

			if hitDist < receiverEntry-wallEpsilon {
				return nil
			}
		}
	}

	// Detector solid angle: the full opening angle gamma = 2*asin(R/r).
	// cos(gamma/2) = cos(asin(R/r)) = sqrt(1 - (R/r)^2).
	ratio := receiver.Radius / dist
	if ratio > 1 {
		ratio = 1
	}

	cosHalfGamma := math.Sqrt(1 - ratio*ratio)
	solidAngleFactor := (1 - cosHalfGamma) * 2

	// Compute per-band rain energy.
	bandEnergy := make([]float64, len(energy))

	for i := range energy {
		s := 1.0
		if i < len(scattering) {
			s = clamp01(scattering[i])
		}

		if s <= 0 || energy[i] <= 0 {
			continue
		}

		// Air absorption over the rain path.
		freqHz := float64(i)
		if i < len(centerFreqs) {
			freqHz = centerFreqs[i]
		}

		alpha := AlphaAirISO9613_1(freqHz, defaultAirTemperatureC, defaultRelativeHumidity)
		airAtten := math.Pow(10, -alpha*dist/10)

		bandEnergy[i] = energy[i] * s * solidAngleFactor * cosTheta * airAtten
	}

	arrivalTime := (pathLength + dist) / speedOfSound

	return &DiffuseRainContribution{
		BandEnergy:  bandEnergy,
		ArrivalTime: arrivalTime,
	}
}

// computeSurfaceRain calculates the secondary radiation energy from a diffuse
// reflection point toward a planar surface detector. This implements the RAVEN
// surface-detector diffuse rain formula (Schroeder 2011, Eq. 5.24):
//
//	E_s = E_P * s * A / (2*pi*r^2) * cos(Psi) * cos(Theta) * exp(-m*r)
//
// where A is the detector area, Psi is the angle between the connection vector
// and the detector normal, and Theta is the angle between the wall normal and
// the connection vector.
//
// If the detector is not visible (occluded or backfacing), nil is returned.
func computeSurfaceRain(
	reflectionPoint, surfaceNormal geometry.Vec3,
	energy, scattering []float64,
	detector SurfaceReceiver,
	tracer Tracer,
	centerFreqs []float64,
	pathLength float64,
	speedOfSound float64,
) *DiffuseRainContribution {
	if len(energy) == 0 || detector.Area <= 0 || speedOfSound <= 0 {
		return nil
	}

	toDetector := detector.Center.Sub(reflectionPoint)
	dist := toDetector.Norm()

	if dist <= 0 {
		return nil
	}

	dir := toDetector.Scale(1 / dist)

	// Lambert cosine factor: cos(Theta) between wall normal and detector direction.
	cosTheta := surfaceNormal.Dot(dir)
	if cosTheta <= 0 {
		return nil
	}

	// Detector orientation factor: cos(Psi) between the connection vector
	// and the detector's inward normal. The detector normal points outward
	// from the receiving side, so the inward direction is -Normal.
	cosPsi := -detector.Normal.Dot(dir)
	if cosPsi <= 0 {
		return nil
	}

	// Visibility check.
	if tracer != nil {
		origin := reflectionPoint.Add(dir.Scale(wallEpsilon))
		ray := geometry.NewRay(origin, dir)

		hitPoint, _, _, hit := tracer.NextHit(ray)
		if hit {
			hitDist := origin.Distance(hitPoint)

			if hitDist < dist-wallEpsilon {
				return nil
			}
		}
	}

	// Solid angle approximation: A / (2*pi*r^2).
	solidAngleFactor := detector.Area / (2 * math.Pi * dist * dist)

	bandEnergy := make([]float64, len(energy))

	for i := range energy {
		s := 1.0
		if i < len(scattering) {
			s = clamp01(scattering[i])
		}

		if s <= 0 || energy[i] <= 0 {
			continue
		}

		freqHz := float64(i)
		if i < len(centerFreqs) {
			freqHz = centerFreqs[i]
		}

		alpha := AlphaAirISO9613_1(freqHz, defaultAirTemperatureC, defaultRelativeHumidity)
		airAtten := math.Pow(10, -alpha*dist/10)

		bandEnergy[i] = energy[i] * s * solidAngleFactor * cosPsi * cosTheta * airAtten
	}

	arrivalTime := (pathLength + dist) / speedOfSound

	return &DiffuseRainContribution{
		BandEnergy:  bandEnergy,
		ArrivalTime: arrivalTime,
	}
}
