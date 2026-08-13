package ism

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestGenerateMeshImageSourcesOrder0(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 2, Y: 2, Z: 2},
	)
	src := geometry.Vec3{X: 1, Y: 1, Z: 1}

	sources := GenerateMeshImageSources(src, mesh, MeshISMConfig{
		MaxOrder: 0,
	})

	if len(sources) != 1 {
		t.Fatalf("order 0: got %d sources, want 1", len(sources))
	}

	if sources[0].Position != src {
		t.Fatalf("order 0: position = %v, want %v", sources[0].Position, src)
	}

	if sources[0].Order != 0 {
		t.Fatalf("order 0: order = %d, want 0", sources[0].Order)
	}
}

func TestGenerateMeshImageSourcesOrder1(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 2, Y: 2, Z: 2},
	)
	src := geometry.Vec3{X: 1, Y: 1, Z: 1}

	sources := GenerateMeshImageSources(src, mesh, MeshISMConfig{
		MaxOrder: 1,
	})

	// Order 0: 1 source. Order 1: at least 6 (one per face plane).
	order1Count := 0

	for _, s := range sources {
		if s.Order == 1 {
			order1Count++
		}
	}

	if order1Count < 6 {
		t.Fatalf("order 1: got %d sources, want >= 6", order1Count)
	}

	// Total should be at least 7 (1 order-0 + 6 order-1).
	if len(sources) < 7 {
		t.Fatalf("total sources = %d, want >= 7", len(sources))
	}
}

func TestGenerateMeshImageSourcesRespectsMaxDistance(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 2, Y: 2, Z: 2},
	)
	src := geometry.Vec3{X: 1, Y: 1, Z: 1}

	far := GenerateMeshImageSources(src, mesh, MeshISMConfig{
		MaxOrder:    3,
		MaxDistance: 100,
	})

	near := GenerateMeshImageSources(src, mesh, MeshISMConfig{
		MaxOrder:    3,
		MaxDistance: 1.5,
	})

	if len(near) >= len(far) {
		t.Fatalf("short distance produced %d sources, long distance produced %d; expected fewer", len(near), len(far))
	}
}

func TestGenerateMeshImageSourcesRespectsMaxCandidates(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 2, Y: 2, Z: 2},
	)
	src := geometry.Vec3{X: 1, Y: 1, Z: 1}

	maxCandidates := 10

	sources := GenerateMeshImageSources(src, mesh, MeshISMConfig{
		MaxOrder:      5,
		MaxCandidates: maxCandidates,
	})

	if len(sources) > maxCandidates {
		t.Fatalf("got %d sources, want <= %d", len(sources), maxCandidates)
	}
}

// imageSourceSeries returns the theoretical image-source count for `order`
// reflections when mirroring across `count` distinct surfaces:
//
//	N = sum_{j=1..order} count * (count-1)^(j-1)
func imageSourceSeries(count, order int) int {
	total := 0

	for j := 1; j <= order; j++ {
		term := count

		for k := 1; k < j; k++ {
			term *= count - 1
		}

		total += term
	}

	return total
}

// TestGenerateMeshImageSourcesOrder4PlaneReduction verifies the plane-polygon
// map optimisation from docs/raven.md section 2.4: mirroring across the 6
// distinct planes of a box instead of its 12 polygons.
//
// The generator additionally prunes candidates that lie behind a plane, so the
// plane-based series is an upper bound rather than an exact count. The test
// therefore asserts:
//   - the generated (order >= 1) count never exceeds the plane-based series,
//   - it is at least 4x smaller than the polygon-based series.
func TestGenerateMeshImageSourcesOrder4PlaneReduction(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 4, Y: 5, Z: 3},
	)
	src := geometry.Vec3{X: 1.3, Y: 2.1, Z: 1.7}

	ppm := geometry.BuildPlanePolygonMap(mesh)

	planeCount := ppm.PlaneCount()
	if planeCount != 6 {
		t.Fatalf("plane count = %d, want 6", planeCount)
	}

	triangleCount := len(mesh.Triangles)
	if triangleCount != 12 {
		t.Fatalf("triangle count = %d, want 12", triangleCount)
	}

	const order = 4

	sources := GenerateMeshImageSources(src, mesh, MeshISMConfig{
		MaxOrder:      order,
		MaxDistance:   0, // no distance pruning
		MaxCandidates: 1_000_000,
	})

	generated := 0

	for _, s := range sources {
		if s.Order == 0 {
			continue
		}

		generated++

		if len(s.PlaneHits) != s.Order {
			t.Fatalf("order %d image source has %d plane hits", s.Order, len(s.PlaneHits))
		}

		if len(s.TriangleHits) != s.Order {
			t.Fatalf("order %d image source has %d triangle hits", s.Order, len(s.TriangleHits))
		}

		for i, planeIndex := range s.PlaneHits {
			if ppm.PlaneOf(s.TriangleHits[i]) != planeIndex {
				t.Fatalf("triangle hit %d is not on plane %d", s.TriangleHits[i], planeIndex)
			}
		}
	}

	planeSeries := imageSourceSeries(planeCount, order)      // 936
	polygonSeries := imageSourceSeries(triangleCount, order) // 17568

	if generated > planeSeries {
		t.Fatalf("generated %d image sources, want <= plane series %d", generated, planeSeries)
	}

	if generated*4 > polygonSeries {
		t.Fatalf("generated %d image sources, want at least 4x below the polygon series %d", generated, polygonSeries)
	}
}

func BenchmarkGenerateMeshImageSourcesOrder4(b *testing.B) {
	mesh := geometry.MeshFromBox(
		geometry.Vec3{X: 0, Y: 0, Z: 0},
		geometry.Vec3{X: 4, Y: 5, Z: 3},
	)
	src := geometry.Vec3{X: 1.3, Y: 2.1, Z: 1.7}
	cfg := MeshISMConfig{
		MaxOrder:      4,
		MaxCandidates: 1_000_000,
		PPM:           geometry.BuildPlanePolygonMap(mesh),
	}

	b.ResetTimer()

	for range b.N {
		_ = GenerateMeshImageSources(src, mesh, cfg)
	}
}
