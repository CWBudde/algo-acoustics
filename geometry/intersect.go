package geometry

import "math"

// RayPlane computes the intersection of a ray with a plane.
// Returns the parameter t (Ray.At(t) is the hit point) and whether the ray
// hits the plane. t < 0 means the plane is behind the ray origin.
func RayPlane(r Ray, p Plane) (t float64, hit bool) {
	denom := r.Direction.Dot(p.Normal)
	if math.Abs(denom) < 1e-12 {
		return 0, false // ray is parallel to the plane
	}

	t = (p.Distance - r.Origin.Dot(p.Normal)) / denom

	return t, true
}

// RayBox computes the entry and exit parameters for a ray through an
// axis-aligned box using the slab method.
// Returns tMin, tMax and whether the ray intersects the box (tMin ≤ tMax and tMax > 0).
func RayBox(r Ray, b Box) (tMin, tMax float64, hit bool) {
	tMin = math.Inf(-1)
	tMax = math.Inf(+1)

	dirs := [3]float64{r.Direction.X, r.Direction.Y, r.Direction.Z}
	origs := [3]float64{r.Origin.X, r.Origin.Y, r.Origin.Z}
	mins := [3]float64{b.Min.X, b.Min.Y, b.Min.Z}
	maxs := [3]float64{b.Max.X, b.Max.Y, b.Max.Z}

	for i := range 3 {
		if math.Abs(dirs[i]) < 1e-12 {
			// Ray is parallel to this slab — check if origin is inside.
			if origs[i] < mins[i] || origs[i] > maxs[i] {
				return 0, 0, false
			}

			continue
		}

		invD := 1 / dirs[i]
		t0 := (mins[i] - origs[i]) * invD
		t1 := (maxs[i] - origs[i]) * invD

		if t0 > t1 {
			t0, t1 = t1, t0
		}

		if t0 > tMin {
			tMin = t0
		}

		if t1 < tMax {
			tMax = t1
		}

		if tMin > tMax {
			return 0, 0, false
		}
	}

	return tMin, tMax, tMax > 0
}
