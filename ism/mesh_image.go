package ism

import (
	"math"
	"sort"

	"github.com/cwbudde/algo-acoustics/geometry"
)

const defaultMaxCandidates = 500_000

// MeshISMConfig configures mesh image source generation.
type MeshISMConfig struct {
	MaxOrder      int
	MaxDistance    float64
	MaxCandidates int // 0 = default (500_000)
}

// MeshImageSource describes one image source for a mesh room.
type MeshImageSource struct {
	Position     geometry.Vec3
	Order        int
	TriangleHits []int // triangle indices hit at each reflection (length == Order)
}

// meshUniquePlane groups a plane with the triangle indices that lie on it.
type meshUniquePlane struct {
	plane    geometry.Plane
	triIndex int // representative triangle index
}

// meshUniquePlanes deduplicates triangles that share the same geometric plane.
// Two triangles share a plane when their normals are parallel and they have the
// same signed distance from the origin.
func meshUniquePlanes(mesh *geometry.Mesh) []meshUniquePlane {
	const normalEps = 1e-6
	const distEps = 1e-6

	planes := make([]meshUniquePlane, 0, len(mesh.Triangles))

	for i, tri := range mesh.Triangles {
		n := tri.Normal()
		d := n.Dot(tri.V0)
		p := geometry.Plane{Normal: n, Distance: d}

		duplicate := false

		for _, existing := range planes {
			// Parallel normals: dot product close to 1 (same direction).
			if math.Abs(existing.plane.Normal.Dot(n)-1) < normalEps &&
				math.Abs(existing.plane.Distance-d) < distEps {
				duplicate = true

				break
			}
		}

		if !duplicate {
			planes = append(planes, meshUniquePlane{plane: p, triIndex: i})
		}
	}

	return planes
}

// GenerateMeshImageSources enumerates mesh image sources up to the configured limits.
func GenerateMeshImageSources(src geometry.Vec3, mesh *geometry.Mesh, bvh *geometry.BVHNode, cfg MeshISMConfig) []MeshImageSource {
	if mesh == nil || cfg.MaxOrder < 0 {
		return nil
	}

	maxCandidates := cfg.MaxCandidates
	if maxCandidates <= 0 {
		maxCandidates = defaultMaxCandidates
	}

	planes := meshUniquePlanes(mesh)

	// Order-0 source is the original position.
	result := []MeshImageSource{{
		Position:     src,
		Order:        0,
		TriangleHits: nil,
	}}

	if cfg.MaxOrder == 0 {
		return result
	}

	// BFS queue: process sources order by order.
	type candidate struct {
		pos          geometry.Vec3
		order        int
		triangleHits []int
	}

	queue := []candidate{{pos: src, order: 0, triangleHits: nil}}

	for len(queue) > 0 && len(result) < maxCandidates {
		current := queue[0]
		queue = queue[1:]

		if current.order >= cfg.MaxOrder {
			continue
		}

		nextOrder := current.order + 1

		for _, up := range planes {
			// Only mirror if the source is on the normal side of the plane.
			if up.plane.SideOf(current.pos) <= pathEpsilon {
				continue
			}

			mirrored := up.plane.ReflectPoint(current.pos)

			// Prune by distance from original source.
			if cfg.MaxDistance > 0 && mirrored.Distance(src) > cfg.MaxDistance {
				continue
			}

			if len(result) >= maxCandidates {
				break
			}

			hits := make([]int, len(current.triangleHits)+1)
			copy(hits, current.triangleHits)
			hits[len(hits)-1] = up.triIndex

			imgSrc := MeshImageSource{
				Position:     mirrored,
				Order:        nextOrder,
				TriangleHits: hits,
			}

			result = append(result, imgSrc)

			if nextOrder < cfg.MaxOrder {
				queue = append(queue, candidate{
					pos:          mirrored,
					order:        nextOrder,
					triangleHits: hits,
				})
			}
		}
	}

	// Sort by order, then by distance from original source.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Order != result[j].Order {
			return result[i].Order < result[j].Order
		}

		return result[i].Position.Distance(src) < result[j].Position.Distance(src)
	})

	return result
}
