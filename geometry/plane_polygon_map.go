package geometry

import "math"

const (
	// ppmNormalEps is the tolerance used when comparing plane normals via
	// their dot product: two normals are parallel (and equally oriented)
	// when |n_a·n_b − 1| < ppmNormalEps.
	ppmNormalEps = 1e-6
	// ppmDistEps is the tolerance on the signed plane distance.
	ppmDistEps = 1e-6

	// ppmNormalBucket and ppmDistBucket are the quantisation steps used for
	// hashing. They are deliberately coarser than the matching tolerances so
	// that any two planes that pass the tolerance test land in the same
	// bucket or in an immediately neighbouring one.
	//
	// |n_a·n_b − 1| < 1e-6 implies |n_a − n_b| < ~1.5e-3, so each normal
	// component differs by less than 1.5e-3 < ppmNormalBucket. Likewise the
	// distance differs by less than 1e-6 < ppmDistBucket.
	ppmNormalBucket = 1e-2
	ppmDistBucket   = 1e-5

	// ppmContainsEps is the default barycentric tolerance for point-in-polygon
	// tests on a coplanar triangle set.
	ppmContainsEps = 1e-6
)

// PlanePolygonMap groups the triangles of a mesh by the geometric plane they
// lie on. Mirroring image sources across distinct planes instead of across
// every individual polygon reduces the image-source count substantially for
// meshes with coplanar wall segments (see docs/raven.md section 2.4). The
// polygon that was actually hit is recovered later during the audibility test
// via a point-in-polygon test on the coplanar set.
//
// Plane equivalence is directional: two coplanar triangles whose normals point
// in opposite directions belong to different planes.
type PlanePolygonMap struct {
	// Planes holds the distinct planes in first-seen triangle order.
	Planes []Plane
	// Polygons[i] lists the triangle indices lying on Planes[i].
	Polygons [][]int

	planeOfTri []int // triangle index -> plane index
}

// ppmKey is the quantised hash key of a plane.
type ppmKey struct {
	nx, ny, nz, d int64
}

func quantize(v, bucket float64) int64 {
	return int64(math.Round(v / bucket))
}

// BuildPlanePolygonMap groups the mesh triangles by plane. Returns nil for a
// nil mesh.
func BuildPlanePolygonMap(mesh *Mesh) *PlanePolygonMap {
	if mesh == nil {
		return nil
	}

	ppm := &PlanePolygonMap{
		Planes:     make([]Plane, 0, len(mesh.Triangles)),
		Polygons:   make([][]int, 0, len(mesh.Triangles)),
		planeOfTri: make([]int, len(mesh.Triangles)),
	}

	// Bucketed index: quantised key -> plane indices in that bucket.
	buckets := make(map[ppmKey][]int, len(mesh.Triangles))

	for i, tri := range mesh.Triangles {
		n := tri.Normal()
		d := n.Dot(tri.V0)

		planeIndex := ppm.findPlane(buckets, n, d)
		if planeIndex < 0 {
			planeIndex = len(ppm.Planes)
			ppm.Planes = append(ppm.Planes, Plane{Normal: n, Distance: d})
			ppm.Polygons = append(ppm.Polygons, nil)

			key := planeKey(n, d)
			buckets[key] = append(buckets[key], planeIndex)
		}

		ppm.Polygons[planeIndex] = append(ppm.Polygons[planeIndex], i)
		ppm.planeOfTri[i] = planeIndex
	}

	return ppm
}

func planeKey(n Vec3, d float64) ppmKey {
	return ppmKey{
		nx: quantize(n.X, ppmNormalBucket),
		ny: quantize(n.Y, ppmNormalBucket),
		nz: quantize(n.Z, ppmNormalBucket),
		d:  quantize(d, ppmDistBucket),
	}
}

// PlaneCount returns the number of distinct planes.
func (m *PlanePolygonMap) PlaneCount() int {
	if m == nil {
		return 0
	}

	return len(m.Planes)
}

// TriangleCount returns the number of triangles covered by the map.
func (m *PlanePolygonMap) TriangleCount() int {
	if m == nil {
		return 0
	}

	return len(m.planeOfTri)
}

// PlaneOf returns the plane index of a triangle, or -1 if the triangle index
// is out of range.
func (m *PlanePolygonMap) PlaneOf(triIndex int) int {
	if m == nil || triIndex < 0 || triIndex >= len(m.planeOfTri) {
		return -1
	}

	return m.planeOfTri[triIndex]
}

// SamePlane reports whether two triangles lie on the same plane. Out-of-range
// indices never match.
func (m *PlanePolygonMap) SamePlane(triA, triB int) bool {
	a := m.PlaneOf(triA)
	if a < 0 {
		return false
	}

	return a == m.PlaneOf(triB)
}

// TrianglesOn returns the triangle indices lying on the given plane, or nil if
// the plane index is out of range. The returned slice must not be modified.
func (m *PlanePolygonMap) TrianglesOn(planeIndex int) []int {
	if m == nil || planeIndex < 0 || planeIndex >= len(m.Polygons) {
		return nil
	}

	return m.Polygons[planeIndex]
}

// ContainsPoint reports whether p lies inside any of the coplanar polygons of
// the given plane. This is the point-in-polygon audibility step: after an
// image source has been mirrored across a plane, the hit point must still fall
// on one of the actual polygons that make up that plane.
func (m *PlanePolygonMap) ContainsPoint(mesh *Mesh, planeIndex int, p Vec3) bool {
	if m == nil || mesh == nil {
		return false
	}

	for _, triIndex := range m.TrianglesOn(planeIndex) {
		if triIndex < 0 || triIndex >= len(mesh.Triangles) {
			continue
		}

		if pointInTriangle(mesh.Triangles[triIndex], p, ppmContainsEps) {
			return true
		}
	}

	return false
}

// findPlane returns the index of an existing plane matching (n, d) within the
// tolerances, or -1 if none exists. It probes the plane's own bucket plus all
// immediately neighbouring buckets so that values sitting on a quantisation
// boundary are still matched.
func (m *PlanePolygonMap) findPlane(buckets map[ppmKey][]int, n Vec3, d float64) int {
	base := planeKey(n, d)

	// Fast path: the overwhelmingly common case is an exact bucket match.
	if index := m.matchIn(buckets[base], n, d); index >= 0 {
		return index
	}

	for dx := int64(-1); dx <= 1; dx++ {
		for dy := int64(-1); dy <= 1; dy++ {
			for dz := int64(-1); dz <= 1; dz++ {
				for dd := int64(-1); dd <= 1; dd++ {
					key := ppmKey{
						nx: base.nx + dx,
						ny: base.ny + dy,
						nz: base.nz + dz,
						d:  base.d + dd,
					}

					if index := m.matchIn(buckets[key], n, d); index >= 0 {
						return index
					}
				}
			}
		}
	}

	return -1
}

// matchIn returns the first plane in candidates matching (n, d) within the
// tolerances, or -1.
func (m *PlanePolygonMap) matchIn(candidates []int, n Vec3, d float64) int {
	for _, candidate := range candidates {
		plane := m.Planes[candidate]
		// Parallel normals with the same orientation: dot product close to 1.
		if math.Abs(plane.Normal.Dot(n)-1) < ppmNormalEps &&
			math.Abs(plane.Distance-d) < ppmDistEps {
			return candidate
		}
	}

	return -1
}
