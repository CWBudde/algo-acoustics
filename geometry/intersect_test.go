package geometry_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// ---- RayPlane ---------------------------------------------------------------

func TestRayPlaneHit(t *testing.T) {
	r := geometry.NewRay(geometry.Vec3{0, 0, -1}, geometry.Vec3{0, 0, 1})
	p := geometry.NewPlaneFromPointNormal(geometry.Vec3Zero, geometry.Vec3{0, 0, 1})

	tVal, hit := geometry.RayPlane(r, p)
	if !hit {
		t.Fatal("expected hit, got miss")
	}

	if math.Abs(tVal-1) > 1e-12 {
		t.Errorf("t = %v, want 1", tVal)
	}
}

func TestRayPlaneParallel(t *testing.T) {
	r := geometry.NewRay(geometry.Vec3{0, 0, 1}, geometry.Vec3{1, 0, 0})
	p := geometry.NewPlaneFromPointNormal(geometry.Vec3Zero, geometry.Vec3{0, 0, 1})

	_, hit := geometry.RayPlane(r, p)
	if hit {
		t.Error("parallel ray should not hit plane")
	}
}

func TestRayPlaneHitPoint(t *testing.T) {
	r := geometry.NewRay(geometry.Vec3{1, 2, -3}, geometry.Vec3{0, 0, 1})
	p := geometry.NewPlaneFromPointNormal(geometry.Vec3{0, 0, 5}, geometry.Vec3{0, 0, 1})

	tVal, hit := geometry.RayPlane(r, p)
	if !hit {
		t.Fatal("expected hit")
	}

	got := r.At(tVal)
	want := geometry.Vec3{1, 2, 5}

	if !vec3Near(got, want, 1e-10) {
		t.Errorf("hit point = %v, want %v", got, want)
	}
}

// ---- RayBox -----------------------------------------------------------------

func TestRayBoxHitFront(t *testing.T) {
	box := geometry.NewBox(geometry.Vec3{-1, -1, -1}, geometry.Vec3{1, 1, 1})
	r := geometry.NewRay(geometry.Vec3{0, 0, -3}, geometry.Vec3{0, 0, 1})

	tMin, _, hit := geometry.RayBox(r, box)
	if !hit {
		t.Fatal("expected hit")
	}

	if math.Abs(tMin-2) > 1e-12 {
		t.Errorf("tMin = %v, want 2", tMin)
	}
}

func TestRayBoxMiss(t *testing.T) {
	box := geometry.NewBox(geometry.Vec3{-1, -1, -1}, geometry.Vec3{1, 1, 1})
	r := geometry.NewRay(geometry.Vec3{0, 3, -3}, geometry.Vec3{0, 0, 1})

	_, _, hit := geometry.RayBox(r, box)
	if hit {
		t.Error("ray aimed past box should miss")
	}
}

func TestRayBoxRayInsideBox(t *testing.T) {
	box := geometry.NewBox(geometry.Vec3{-1, -1, -1}, geometry.Vec3{1, 1, 1})
	r := geometry.NewRay(geometry.Vec3Zero, geometry.Vec3{0, 0, 1})

	_, tMax, hit := geometry.RayBox(r, box)
	if !hit {
		t.Fatal("ray inside box should hit")
	}

	if math.Abs(tMax-1) > 1e-12 {
		t.Errorf("tMax = %v, want 1", tMax)
	}
}

// ---- RayTriangle ------------------------------------------------------------

func TestRayTriangleHit(t *testing.T) {
	tri := geometry.Triangle{
		V0: geometry.Vec3{-1, 0, 0},
		V1: geometry.Vec3{1, 0, 0},
		V2: geometry.Vec3{0, 1, 0},
	}
	r := geometry.NewRay(geometry.Vec3{0, 0.2, -1}, geometry.Vec3{0, 0, 1})

	tVal, hit := geometry.RayTriangle(r, tri)
	if !hit {
		t.Fatal("expected hit")
	}

	got := r.At(tVal)
	if math.Abs(got.Z) > 1e-10 {
		t.Errorf("hit point Z = %v, want 0", got.Z)
	}
}

func TestRayTriangleMiss(t *testing.T) {
	tri := geometry.Triangle{
		V0: geometry.Vec3{-1, 0, 0},
		V1: geometry.Vec3{1, 0, 0},
		V2: geometry.Vec3{0, 1, 0},
	}
	r := geometry.NewRay(geometry.Vec3{5, 5, -1}, geometry.Vec3{0, 0, 1})

	_, hit := geometry.RayTriangle(r, tri)
	if hit {
		t.Error("ray missing triangle should not hit")
	}
}

func TestRayTriangleDoubleSided(t *testing.T) {
	// RayTriangle is double-sided: room acoustic rays bounce off both sides
	// of a surface, so a ray from either side of the triangle should hit.
	tri := geometry.Triangle{
		V0: geometry.Vec3{-1, 0, 0},
		V1: geometry.Vec3{1, 0, 0},
		V2: geometry.Vec3{0, 1, 0},
	}

	rFront := geometry.NewRay(geometry.Vec3{0, 0.2, -1}, geometry.Vec3{0, 0, 1})
	rBack := geometry.NewRay(geometry.Vec3{0, 0.2, 1}, geometry.Vec3{0, 0, -1})

	if _, hit := geometry.RayTriangle(rFront, tri); !hit {
		t.Error("ray from front should hit")
	}

	if _, hit := geometry.RayTriangle(rBack, tri); !hit {
		t.Error("ray from back should hit (double-sided surface)")
	}
}

func TestRayTriangleOppositeDirectionMiss(t *testing.T) {
	// A ray aimed away from the triangle (negative t) should not hit.
	tri := geometry.Triangle{
		V0: geometry.Vec3{-1, 0, 0},
		V1: geometry.Vec3{1, 0, 0},
		V2: geometry.Vec3{0, 1, 0},
	}
	r := geometry.NewRay(geometry.Vec3{0, 0.2, -1}, geometry.Vec3{0, 0, -1}) // aimed away

	_, hit := geometry.RayTriangle(r, tri)
	if hit {
		t.Error("ray aimed away from triangle should not hit")
	}
}

// ---- Plane helpers ----------------------------------------------------------

func TestPlaneReflect(t *testing.T) {
	// Floor plane (normal pointing up). Downward vector should reflect upward.
	p := geometry.NewPlaneFromPointNormal(geometry.Vec3Zero, geometry.Vec3{0, 1, 0})
	v := geometry.Vec3{0, -1, 0}
	got := p.Reflect(v)
	want := geometry.Vec3{0, 1, 0}

	if !vec3Near(got, want, 1e-12) {
		t.Errorf("Reflect = %v, want %v", got, want)
	}
}

func TestPlaneReflectPreservesLength(t *testing.T) {
	p := geometry.NewPlaneFromPointNormal(geometry.Vec3Zero, geometry.Vec3{1, 1, 0}.Normalize())
	v := geometry.Vec3{3, 1, 2}
	got := p.Reflect(v)

	if math.Abs(got.Norm()-v.Norm()) > 1e-12 {
		t.Errorf("Reflect changed vector length: %v → %v", v.Norm(), got.Norm())
	}
}

func TestPlaneReflectPoint(t *testing.T) {
	t.Parallel()

	// Floor plane at z=0, normal pointing up.
	p := geometry.NewPlaneFromPointNormal(geometry.Vec3Zero, geometry.Vec3{Z: 1})

	got := p.ReflectPoint(geometry.Vec3{X: 1, Y: 2, Z: 3})
	want := geometry.Vec3{X: 1, Y: 2, Z: -3}

	if !vec3Near(got, want, 1e-12) {
		t.Errorf("ReflectPoint = %v, want %v", got, want)
	}
}

func TestPlaneReflectPointOnPlane(t *testing.T) {
	t.Parallel()

	p := geometry.NewPlaneFromPointNormal(geometry.Vec3{Z: 5}, geometry.Vec3{Z: 1})
	got := p.ReflectPoint(geometry.Vec3{X: 3, Y: 4, Z: 5})
	want := geometry.Vec3{X: 3, Y: 4, Z: 5}

	if !vec3Near(got, want, 1e-12) {
		t.Errorf("ReflectPoint on plane = %v, want %v", got, want)
	}
}

func TestPlaneReflectPointPreservesDistance(t *testing.T) {
	t.Parallel()

	p := geometry.NewPlaneFromPointNormal(geometry.Vec3{X: 2}, geometry.Vec3{X: 1})
	point := geometry.Vec3{X: 5, Y: 1, Z: -1}
	reflected := p.ReflectPoint(point)

	distBefore := math.Abs(p.SideOf(point))
	distAfter := math.Abs(p.SideOf(reflected))

	if math.Abs(distBefore-distAfter) > 1e-12 {
		t.Errorf("distance changed: before=%v, after=%v", distBefore, distAfter)
	}
}

func TestPlaneSideOf(t *testing.T) {
	p := geometry.NewPlaneFromPointNormal(geometry.Vec3{0, 1, 0}, geometry.Vec3{0, 1, 0})

	if p.SideOf(geometry.Vec3{0, 2, 0}) <= 0 {
		t.Error("point above plane should be positive side")
	}

	if p.SideOf(geometry.Vec3{0, 0, 0}) >= 0 {
		t.Error("point below plane should be negative side")
	}
}
