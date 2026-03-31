package geometry_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestBuildBVHEmptyMesh(t *testing.T) {
	if got := geometry.BuildBVH(nil); got != nil {
		t.Fatalf("BuildBVH(nil) = %#v, want nil", got)
	}

	mesh := &geometry.Mesh{}
	if got := geometry.BuildBVH(mesh); got != nil {
		t.Fatalf("BuildBVH(empty) = %#v, want nil", got)
	}
}

func TestBVHIntersectNearestTriangle(t *testing.T) {
	mesh := &geometry.Mesh{Triangles: []geometry.Triangle{
		{
			V0: geometry.Vec3{X: -1, Y: -1, Z: 1},
			V1: geometry.Vec3{X: 1, Y: -1, Z: 1},
			V2: geometry.Vec3{X: 0, Y: 1, Z: 1},
		},
		{
			V0: geometry.Vec3{X: -1, Y: -1, Z: 3},
			V1: geometry.Vec3{X: 1, Y: -1, Z: 3},
			V2: geometry.Vec3{X: 0, Y: 1, Z: 3},
		},
	}}

	bvh := geometry.BuildBVH(mesh)
	if bvh == nil {
		t.Fatal("BuildBVH() returned nil for populated mesh")
	}

	r := geometry.NewRay(geometry.Vec3Zero, geometry.Vec3{Z: 1})

	tHit, triIdx, hit := bvh.Intersect(r)
	if !hit {
		t.Fatal("Intersect() reported miss, want hit")
	}

	if triIdx != 0 {
		t.Fatalf("Intersect() triangle index = %d, want 0", triIdx)
	}

	if math.Abs(tHit-1) > 1e-12 {
		t.Fatalf("Intersect() t = %v, want 1", tHit)
	}

	if bvh.AABB != mesh.BoundingBox() {
		t.Fatalf("root AABB = %#v, want %#v", bvh.AABB, mesh.BoundingBox())
	}
}

func TestBVHIntersectMatchesBruteForceRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(10))
	mesh := randomTriangleMesh(rng, 1000)

	bvh := geometry.BuildBVH(mesh)
	if bvh == nil {
		t.Fatal("BuildBVH() returned nil for random mesh")
	}

	for index := range 2000 {
		ray := randomComparisonRay(rng, mesh, index)

		wantT, wantTri, wantHit := bruteForceIntersect(mesh, ray)
		gotT, gotTri, gotHit := bvh.Intersect(ray)

		if gotHit != wantHit {
			t.Fatalf("ray[%d] hit = %v, want %v", index, gotHit, wantHit)
		}

		if !wantHit {
			continue
		}

		if gotTri != wantTri {
			t.Fatalf("ray[%d] triangle = %d, want %d", index, gotTri, wantTri)
		}

		if math.Abs(gotT-wantT) > 1e-9 {
			t.Fatalf("ray[%d] t = %v, want %v", index, gotT, wantT)
		}
	}
}

func BenchmarkMeshIntersect(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	mesh := randomTriangleMesh(rng, 10000)
	bvh := geometry.BuildBVH(mesh)

	rays := make([]geometry.Ray, 1024)
	for index := range rays {
		rays[index] = randomComparisonRay(rng, mesh, index)
	}

	b.Run("bvh", func(b *testing.B) {
		b.ReportAllocs()

		for index := range b.N {
			benchmarkT, benchmarkTriIdx, benchmarkHit = bvh.Intersect(rays[index%len(rays)])
		}
	})

	b.Run("brute_force", func(b *testing.B) {
		b.ReportAllocs()

		for index := range b.N {
			benchmarkT, benchmarkTriIdx, benchmarkHit = bruteForceIntersect(mesh, rays[index%len(rays)])
		}
	})
}

var (
	benchmarkT      float64
	benchmarkTriIdx int
	benchmarkHit    bool
)

func bruteForceIntersect(mesh *geometry.Mesh, r geometry.Ray) (t float64, triIdx int, hit bool) {
	bestT := math.Inf(1)
	bestIdx := -1

	for index, tri := range mesh.Triangles {
		candidateT, candidateHit := geometry.RayTriangle(r, tri)
		if !candidateHit || candidateT >= bestT {
			continue
		}

		bestT = candidateT
		bestIdx = index
	}

	if bestIdx < 0 {
		return 0, 0, false
	}

	return bestT, bestIdx, true
}

func randomTriangleMesh(rng *rand.Rand, count int) *geometry.Mesh {
	triangles := make([]geometry.Triangle, count)
	for index := range triangles {
		triangles[index] = randomTriangle(rng)
	}

	return &geometry.Mesh{Triangles: triangles}
}

func randomTriangle(rng *rand.Rand) geometry.Triangle {
	center := randomVecInRange(rng, 100)
	for {
		u := randomDirection(rng).Scale(0.5 + rng.Float64()*1.5)
		v := randomDirection(rng).Scale(0.5 + rng.Float64()*1.5)

		tri := geometry.Triangle{
			V0: center.Sub(u),
			V1: center.Add(u),
			V2: center.Add(v),
		}
		if tri.Area() > 1e-4 {
			return tri
		}
	}
}

func randomComparisonRay(rng *rand.Rand, mesh *geometry.Mesh, index int) geometry.Ray {
	if index%3 == 0 {
		origin := randomVecInRange(rng, 160)
		direction := randomDirection(rng)

		return geometry.NewRay(origin, direction)
	}

	tri := mesh.Triangles[rng.Intn(len(mesh.Triangles))]
	target := tri.Centroid().Add(randomDirection(rng).Scale(0.15))
	origin := target.Add(randomDirection(rng).Scale(150 + rng.Float64()*50))

	return geometry.NewRay(origin, target.Sub(origin))
}

func randomVecInRange(rng *rand.Rand, extent float64) geometry.Vec3 {
	return geometry.Vec3{
		X: (rng.Float64()*2 - 1) * extent,
		Y: (rng.Float64()*2 - 1) * extent,
		Z: (rng.Float64()*2 - 1) * extent,
	}
}

func randomDirection(rng *rand.Rand) geometry.Vec3 {
	for {
		v := geometry.Vec3{
			X: rng.Float64()*2 - 1,
			Y: rng.Float64()*2 - 1,
			Z: rng.Float64()*2 - 1,
		}
		if v.Norm() > 1e-9 {
			return v.Normalize()
		}
	}
}
