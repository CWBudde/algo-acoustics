package geometry

// Mesh is a triangulated surface. Full implementation (BVH, OBJ loading,
// validation) is added in Phase 10.
type Mesh struct {
	Triangles []Triangle
}

// BoundingBox returns the axis-aligned bounding box of all vertices in the mesh.
// Returns a zero Box for an empty mesh.
func (m *Mesh) BoundingBox() Box {
	if len(m.Triangles) == 0 {
		return Box{}
	}

	b := Box{
		Min: m.Triangles[0].V0,
		Max: m.Triangles[0].V0,
	}

	for _, tri := range m.Triangles {
		for _, v := range []Vec3{tri.V0, tri.V1, tri.V2} {
			b.Min = Vec3{
				X: min64(b.Min.X, v.X),
				Y: min64(b.Min.Y, v.Y),
				Z: min64(b.Min.Z, v.Z),
			}
			b.Max = Vec3{
				X: max64(b.Max.X, v.X),
				Y: max64(b.Max.Y, v.Y),
				Z: max64(b.Max.Z, v.Z),
			}
		}
	}

	return b
}
