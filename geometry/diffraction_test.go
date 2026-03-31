package geometry_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestExtractDiffractionEdgesCube(t *testing.T) {
	mesh := cubeMesh(geometry.Vec3Zero, geometry.Vec3{X: 1, Y: 1, Z: 1})

	edges := geometry.ExtractDiffractionEdges(mesh)
	if len(edges) != 12 {
		t.Fatalf("len(edges) = %d, want 12", len(edges))
	}

	for index, edge := range edges {
		if math.Abs(edge.WedgeIndex-1.5) > 1e-9 {
			t.Fatalf("edge[%d].WedgeIndex = %v, want 1.5", index, edge.WedgeIndex)
		}

		if math.Abs(edge.Length-1) > 1e-9 {
			t.Fatalf("edge[%d].Length = %v, want 1", index, edge.Length)
		}

		if math.Abs(edge.Direction.Norm()-1) > 1e-9 {
			t.Fatalf("edge[%d].Direction norm = %v, want 1", index, edge.Direction.Norm())
		}
	}
}

func TestExtractDiffractionEdgesLShapedRoomClassification(t *testing.T) {
	mesh := lShapedPrismMesh()

	edges := geometry.ExtractDiffractionEdges(mesh)
	if len(edges) != 17 {
		t.Fatalf("len(edges) = %d, want 17 (18 total minus one concave reentrant edge)", len(edges))
	}

	for _, edge := range edges {
		if isReentrantVerticalEdge(edge) {
			t.Fatalf("concave reentrant edge should not be diffracting: %#v", edge)
		}
	}
}

func isReentrantVerticalEdge(edge geometry.DiffractionEdge) bool {
	return nearlyEqual(edge.Start.X, 1) && nearlyEqual(edge.Start.Y, 1) &&
		nearlyEqual(edge.End.X, 1) && nearlyEqual(edge.End.Y, 1)
}

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func cubeMesh(minCorner, maxCorner geometry.Vec3) *geometry.Mesh {
	v000 := geometry.Vec3{X: minCorner.X, Y: minCorner.Y, Z: minCorner.Z}
	v001 := geometry.Vec3{X: minCorner.X, Y: minCorner.Y, Z: maxCorner.Z}
	v010 := geometry.Vec3{X: minCorner.X, Y: maxCorner.Y, Z: minCorner.Z}
	v011 := geometry.Vec3{X: minCorner.X, Y: maxCorner.Y, Z: maxCorner.Z}
	v100 := geometry.Vec3{X: maxCorner.X, Y: minCorner.Y, Z: minCorner.Z}
	v101 := geometry.Vec3{X: maxCorner.X, Y: minCorner.Y, Z: maxCorner.Z}
	v110 := geometry.Vec3{X: maxCorner.X, Y: maxCorner.Y, Z: minCorner.Z}
	v111 := geometry.Vec3{X: maxCorner.X, Y: maxCorner.Y, Z: maxCorner.Z}

	return &geometry.Mesh{Triangles: []geometry.Triangle{
		{V0: v000, V1: v110, V2: v100},
		{V0: v000, V1: v010, V2: v110},
		{V0: v001, V1: v101, V2: v111},
		{V0: v001, V1: v111, V2: v011},
		{V0: v000, V1: v101, V2: v001},
		{V0: v000, V1: v100, V2: v101},
		{V0: v010, V1: v011, V2: v111},
		{V0: v010, V1: v111, V2: v110},
		{V0: v000, V1: v001, V2: v011},
		{V0: v000, V1: v011, V2: v010},
		{V0: v100, V1: v110, V2: v111},
		{V0: v100, V1: v111, V2: v101},
	}}
}

func lShapedPrismMesh() *geometry.Mesh {
	type voxel struct{ X, Y, Z int }
	filled := map[voxel]struct{}{
		{X: 0, Y: 0, Z: 0}: {},
		{X: 1, Y: 0, Z: 0}: {},
		{X: 0, Y: 1, Z: 0}: {},
	}

	type faceDef struct {
		dx, dy, dz int
		normal     geometry.Vec3
		quad       func(x, y, z int) [4]geometry.Vec3
	}

	faces := []faceDef{
		{
			dx:     1,
			normal: geometry.Vec3{X: 1},
			quad: func(x, y, z int) [4]geometry.Vec3 {
				xf := float64(x + 1)
				yf := float64(y)
				zf := float64(z)

				return [4]geometry.Vec3{{X: xf, Y: yf, Z: zf}, {X: xf, Y: yf + 1, Z: zf}, {X: xf, Y: yf + 1, Z: zf + 1}, {X: xf, Y: yf, Z: zf + 1}}
			},
		},
		{
			dx:     -1,
			normal: geometry.Vec3{X: -1},
			quad: func(x, y, z int) [4]geometry.Vec3 {
				xf := float64(x)
				yf := float64(y)
				zf := float64(z)

				return [4]geometry.Vec3{{X: xf, Y: yf, Z: zf}, {X: xf, Y: yf, Z: zf + 1}, {X: xf, Y: yf + 1, Z: zf + 1}, {X: xf, Y: yf + 1, Z: zf}}
			},
		},
		{
			dy:     1,
			normal: geometry.Vec3{Y: 1},
			quad: func(x, y, z int) [4]geometry.Vec3 {
				xf := float64(x)
				yf := float64(y + 1)
				zf := float64(z)

				return [4]geometry.Vec3{{X: xf, Y: yf, Z: zf}, {X: xf, Y: yf, Z: zf + 1}, {X: xf + 1, Y: yf, Z: zf + 1}, {X: xf + 1, Y: yf, Z: zf}}
			},
		},
		{
			dy:     -1,
			normal: geometry.Vec3{Y: -1},
			quad: func(x, y, z int) [4]geometry.Vec3 {
				xf := float64(x)
				yf := float64(y)
				zf := float64(z)

				return [4]geometry.Vec3{{X: xf, Y: yf, Z: zf}, {X: xf + 1, Y: yf, Z: zf}, {X: xf + 1, Y: yf, Z: zf + 1}, {X: xf, Y: yf, Z: zf + 1}}
			},
		},
		{
			dz:     1,
			normal: geometry.Vec3{Z: 1},
			quad: func(x, y, z int) [4]geometry.Vec3 {
				xf := float64(x)
				yf := float64(y)
				zf := float64(z + 1)

				return [4]geometry.Vec3{{X: xf, Y: yf, Z: zf}, {X: xf + 1, Y: yf, Z: zf}, {X: xf + 1, Y: yf + 1, Z: zf}, {X: xf, Y: yf + 1, Z: zf}}
			},
		},
		{
			dz:     -1,
			normal: geometry.Vec3{Z: -1},
			quad: func(x, y, z int) [4]geometry.Vec3 {
				xf := float64(x)
				yf := float64(y)
				zf := float64(z)

				return [4]geometry.Vec3{{X: xf, Y: yf, Z: zf}, {X: xf, Y: yf + 1, Z: zf}, {X: xf + 1, Y: yf + 1, Z: zf}, {X: xf + 1, Y: yf, Z: zf}}
			},
		},
	}

	triangles := make([]geometry.Triangle, 0)

	makeTri := func(a, b, c, outward geometry.Vec3) geometry.Triangle {
		tri := geometry.Triangle{V0: a, V1: b, V2: c}
		if tri.Normal().Dot(outward) < 0 {
			tri = geometry.Triangle{V0: a, V1: c, V2: b}
		}

		return tri
	}

	for cell := range filled {
		for _, face := range faces {
			neighbor := voxel{X: cell.X + face.dx, Y: cell.Y + face.dy, Z: cell.Z + face.dz}
			if _, ok := filled[neighbor]; ok {
				continue
			}

			quad := face.quad(cell.X, cell.Y, cell.Z)
			triangles = append(triangles,
				makeTri(quad[0], quad[1], quad[2], face.normal),
				makeTri(quad[0], quad[2], quad[3], face.normal),
			)
		}
	}

	return &geometry.Mesh{Triangles: triangles}
}
