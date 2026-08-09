package geometry

import "math"

const rayCylinderEpsilon = 1e-12

// RayCylinderHit describes the closest approach between a ray and the open
// axis of a finite cylinder. RayDistance is measured along the normalized ray;
// EdgeFraction is in (0, 1). The cylinder end caps are deliberately excluded.
type RayCylinderHit struct {
	RayPoint      Vec3
	EdgePoint     Vec3
	RayDistance   float64
	EdgeFraction  float64
	FlyByDistance float64
}

// RayOpenFiniteCylinder reports whether ray passes within radius of the open
// finite cylinder around edgeStart--edgeEnd between ray distances tMin and
// tMax. Tangential passages count as hits. Rays parallel to the cylinder axis
// and closest approaches at either open endpoint do not.
func RayOpenFiniteCylinder(
	ray Ray,
	edgeStart, edgeEnd Vec3,
	radius, tMin, tMax float64,
) (RayCylinderHit, bool) {
	if radius < 0 || tMax < tMin || ray.Direction == Vec3Zero {
		return RayCylinderHit{}, false
	}

	edge := edgeEnd.Sub(edgeStart)
	edgeLengthSquared := edge.Dot(edge)
	if edgeLengthSquared <= rayCylinderEpsilon {
		return RayCylinderHit{}, false
	}

	dir := ray.Direction.Normalize()
	w := ray.Origin.Sub(edgeStart)
	b := dir.Dot(edge)
	d := dir.Dot(w)
	e := edge.Dot(w)
	denominator := edgeLengthSquared - b*b
	if math.Abs(denominator) <= rayCylinderEpsilon*edgeLengthSquared {
		return RayCylinderHit{}, false
	}

	rayDistance := (b*e - edgeLengthSquared*d) / denominator
	edgeFraction := (e - b*d) / denominator
	if rayDistance < tMin || rayDistance > tMax || edgeFraction <= 0 || edgeFraction >= 1 {
		return RayCylinderHit{}, false
	}

	rayPoint := ray.Origin.Add(dir.Scale(rayDistance))
	edgePoint := edgeStart.Add(edge.Scale(edgeFraction))
	flyByDistance := rayPoint.Distance(edgePoint)
	if flyByDistance > radius+rayCylinderEpsilon {
		return RayCylinderHit{}, false
	}

	return RayCylinderHit{
		RayPoint:      rayPoint,
		EdgePoint:     edgePoint,
		RayDistance:   rayDistance,
		EdgeFraction:  edgeFraction,
		FlyByDistance: flyByDistance,
	}, true
}
