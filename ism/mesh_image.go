package ism

import (
	"sort"

	"github.com/cwbudde/algo-acoustics/geometry"
)

const defaultMaxCandidates = 500_000

// MeshISMConfig configures mesh image source generation.
type MeshISMConfig struct {
	MaxOrder      int
	MaxDistance   float64
	MaxCandidates int // 0 = default (500_000)

	// PPM is an optional pre-built plane-polygon map for the mesh. Callers
	// that render several sources in the same room can build it once and
	// reuse it. When nil it is built internally.
	PPM *geometry.PlanePolygonMap
}

// MeshImageSource describes one image source for a mesh room.
type MeshImageSource struct {
	Position geometry.Vec3
	Order    int
	// TriangleHits holds a representative triangle index of the plane hit at
	// each reflection (length == Order).
	TriangleHits []int
	// PlaneHits holds the plane index (into the PlanePolygonMap) hit at each
	// reflection (length == Order).
	PlaneHits []int
}

// GenerateMeshImageSources enumerates mesh image sources up to the configured limits.
func GenerateMeshImageSources(src geometry.Vec3, mesh *geometry.Mesh, cfg MeshISMConfig) []MeshImageSource {
	if mesh == nil || cfg.MaxOrder < 0 {
		return nil
	}

	maxCandidates := cfg.MaxCandidates
	if maxCandidates <= 0 {
		maxCandidates = defaultMaxCandidates
	}

	ppm := cfg.PPM
	if ppm == nil {
		ppm = geometry.BuildPlanePolygonMap(mesh)
	}

	// Order-0 source is the original position.
	result := []MeshImageSource{{
		Position:     src,
		Order:        0,
		TriangleHits: nil,
		PlaneHits:    nil,
	}}

	if cfg.MaxOrder == 0 {
		return result
	}

	// BFS queue: process sources order by order.
	type candidate struct {
		pos          geometry.Vec3
		order        int
		triangleHits []int
		planeHits    []int
	}

	queue := []candidate{{pos: src, order: 0, triangleHits: nil, planeHits: nil}}

	for len(queue) > 0 && len(result) < maxCandidates {
		current := queue[0]
		queue = queue[1:]

		if current.order >= cfg.MaxOrder {
			continue
		}

		nextOrder := current.order + 1

		for planeIndex, plane := range ppm.Planes {
			// Only mirror if the source is on the normal side of the plane.
			if plane.SideOf(current.pos) <= pathEpsilon {
				continue
			}

			mirrored := plane.ReflectPoint(current.pos)

			// Prune by distance from original source.
			if cfg.MaxDistance > 0 && mirrored.Distance(src) > cfg.MaxDistance {
				continue
			}

			if len(result) >= maxCandidates {
				break
			}

			// Representative triangle of the plane, kept for consumers that
			// work with triangle indices.
			representative := planeIndex

			if tris := ppm.TrianglesOn(planeIndex); len(tris) > 0 {
				representative = tris[0]
			}

			hits := make([]int, len(current.triangleHits)+1)
			copy(hits, current.triangleHits)
			hits[len(hits)-1] = representative

			planeHits := make([]int, len(current.planeHits)+1)
			copy(planeHits, current.planeHits)
			planeHits[len(planeHits)-1] = planeIndex

			imgSrc := MeshImageSource{
				Position:     mirrored,
				Order:        nextOrder,
				TriangleHits: hits,
				PlaneHits:    planeHits,
			}

			result = append(result, imgSrc)

			if nextOrder < cfg.MaxOrder {
				queue = append(queue, candidate{
					pos:          mirrored,
					order:        nextOrder,
					triangleHits: hits,
					planeHits:    planeHits,
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
