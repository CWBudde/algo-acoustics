package raytrace

import (
	"errors"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// MeshTracer accelerates ray-mesh intersection queries with a BVH.
type MeshTracer struct {
	Mesh      *geometry.Mesh
	BVH       *geometry.BVHNode
	Materials []*scene.Material
}

// NewMeshTracer constructs a mesh tracer from a validated triangle mesh.
func NewMeshTracer(mesh *geometry.Mesh, materials []*scene.Material) (MeshTracer, error) {
	if mesh == nil {
		return MeshTracer{}, errors.New("mesh is nil")
	}

	if err := mesh.Validate(); err != nil {
		var issues *geometry.MeshValidationIssues
		if !errors.As(err, &issues) || issues.HasProblems() {
			return MeshTracer{}, err
		}
	}

	bvh := geometry.BuildBVH(mesh)
	if bvh == nil {
		return MeshTracer{}, errors.New("mesh tracer requires at least one triangle")
	}

	tracer := MeshTracer{
		Mesh: mesh,
		BVH:  bvh,
	}
	if len(materials) > 0 {
		tracer.Materials = append([]*scene.Material(nil), materials...)
	}

	return tracer, nil
}

// NextHit returns the closest triangle hit along r.
func (t MeshTracer) NextHit(r geometry.Ray) (geometry.Vec3, geometry.Vec3, int, bool) {
	if t.Mesh == nil || t.BVH == nil {
		return geometry.Vec3{}, geometry.Vec3{}, 0, false
	}

	tHit, triIdx, hit := t.BVH.Intersect(r)
	if !hit || triIdx < 0 || triIdx >= len(t.Mesh.Triangles) {
		return geometry.Vec3{}, geometry.Vec3{}, 0, false
	}

	tri := t.Mesh.Triangles[triIdx]
	normal := tri.Normal()
	if normal == geometry.Vec3Zero {
		return geometry.Vec3{}, geometry.Vec3{}, 0, false
	}

	return r.At(tHit), normal, triIdx, true
}
