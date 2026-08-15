package geometry

import (
	"math"
	"testing"
)

func TestNewPlaneFrameIsOrthonormalAndRightHanded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		normal Vec3
	}{
		{name: "plus x", normal: Vec3{X: 1}},
		{name: "minus x", normal: Vec3{X: -1}},
		{name: "plus y", normal: Vec3{Y: 1}},
		{name: "minus y", normal: Vec3{Y: -1}},
		{name: "plus z", normal: Vec3{Z: 1}},
		{name: "minus z", normal: Vec3{Z: -1}},
		{name: "oblique", normal: Vec3{X: 1, Y: 2, Z: -3}},
		{name: "unnormalized", normal: Vec3{X: 0, Y: 0, Z: 7}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			frame := NewPlaneFrame(Vec3{X: 1, Y: 2, Z: 3}, test.normal)

			for name, value := range map[string]float64{
				"|U|": frame.U.Norm(),
				"|V|": frame.V.Norm(),
				"|N|": frame.Normal.Norm(),
			} {
				if math.Abs(value-1) > 1e-12 {
					t.Fatalf("%s = %v, want 1", name, value)
				}
			}

			for name, value := range map[string]float64{
				"U·V": frame.U.Dot(frame.V),
				"U·N": frame.U.Dot(frame.Normal),
				"V·N": frame.V.Dot(frame.Normal),
			} {
				if math.Abs(value) > 1e-12 {
					t.Fatalf("%s = %v, want 0", name, value)
				}
			}

			// (U, V, Normal) must be right-handed so face_cut's winding
			// matches the frame normal.
			cross := frame.U.Cross(frame.V)
			if cross.Sub(frame.Normal).Norm() > 1e-12 {
				t.Fatalf("U x V = %+v, want Normal %+v", cross, frame.Normal)
			}
		})
	}
}

func TestNewPlaneFrameIsDeterministic(t *testing.T) {
	t.Parallel()

	// A basis that flipped between constructions would spuriously invalidate
	// the geometry hashes that downstream caches key on.
	normal := Vec3{X: 0.5, Y: 0.5, Z: 0.5}
	first := NewPlaneFrame(Vec3Zero, normal)

	for range 16 {
		again := NewPlaneFrame(Vec3Zero, normal)
		if again != first {
			t.Fatalf("NewPlaneFrame is not deterministic: %+v vs %+v", again, first)
		}
	}
}

func TestNewPlaneFrameRejectsZeroNormal(t *testing.T) {
	t.Parallel()

	frame := NewPlaneFrame(Vec3{X: 1}, Vec3Zero)
	if frame.U != Vec3Zero || frame.V != Vec3Zero || frame.Normal != Vec3Zero {
		t.Fatalf("NewPlaneFrame(zero normal) = %+v, want a zero basis", frame)
	}
}

func TestPlaneFrameProjectionRoundTrips(t *testing.T) {
	t.Parallel()

	frame := NewPlaneFrame(Vec3{X: 2, Y: -1, Z: 4}, Vec3{X: 1, Y: 1, Z: 1})

	for _, point := range []Vec2{{}, {U: 3, V: -2}, {U: -7.5, V: 0.25}} {
		got := frame.To2D(frame.To3D(point))
		if math.Abs(got.U-point.U) > 1e-12 || math.Abs(got.V-point.V) > 1e-12 {
			t.Fatalf("To2D(To3D(%+v)) = %+v", point, got)
		}
	}
}

func TestPlaneFrameDistanceIsSigned(t *testing.T) {
	t.Parallel()

	frame := NewPlaneFrame(Vec3Zero, Vec3{Z: 1})

	if got := frame.Distance(Vec3{Z: 2}); math.Abs(got-2) > 1e-12 {
		t.Fatalf("Distance(+2z) = %v, want 2", got)
	}

	if got := frame.Distance(Vec3{Z: -3}); math.Abs(got+3) > 1e-12 {
		t.Fatalf("Distance(-3z) = %v, want -3", got)
	}
}

func TestRect2FromPolygonAcceptsRectanglesAndRejectsOthers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		points []Vec2
		want   Rect2
		wantOK bool
	}{
		{
			name:   "counter-clockwise rectangle",
			points: []Vec2{{U: 0, V: 0}, {U: 4, V: 0}, {U: 4, V: 3}, {U: 0, V: 3}},
			want:   Rect2{UMax: 4, VMax: 3},
			wantOK: true,
		},
		{
			name:   "clockwise rectangle",
			points: []Vec2{{U: 0, V: 3}, {U: 4, V: 3}, {U: 4, V: 0}, {U: 0, V: 0}},
			want:   Rect2{UMax: 4, VMax: 3},
			wantOK: true,
		},
		{
			name:   "rectangle with a duplicated corner",
			points: []Vec2{{U: 0, V: 0}, {U: 4, V: 0}, {U: 4, V: 3}, {U: 0, V: 3}, {U: 0, V: 0}},
			want:   Rect2{UMax: 4, VMax: 3},
			wantOK: true,
		},
		{
			name:   "rectangle with a subdivided edge",
			points: []Vec2{{U: 0, V: 0}, {U: 2, V: 0}, {U: 4, V: 0}, {U: 4, V: 3}, {U: 0, V: 3}},
			want:   Rect2{UMax: 4, VMax: 3},
			wantOK: true,
		},
		{
			name:   "corners visited over a diagonal",
			points: []Vec2{{U: 0, V: 0}, {U: 4, V: 0}, {U: 4, V: 3}, {U: 0, V: 0}, {U: 0, V: 3}},
			wantOK: false,
		},
		{
			name:   "bowtie over all four corners",
			points: []Vec2{{U: 0, V: 0}, {U: 4, V: 0}, {U: 0, V: 3}, {U: 4, V: 3}},
			wantOK: false,
		},
		{
			name:   "triangle",
			points: []Vec2{{U: 0, V: 0}, {U: 4, V: 0}, {U: 0, V: 3}},
			wantOK: false,
		},
		{
			name:   "pentagon with an off-outline vertex",
			points: []Vec2{{U: 0, V: 0}, {U: 4, V: 0}, {U: 4, V: 3}, {U: 2, V: 1.5}, {U: 0, V: 3}},
			wantOK: false,
		},
		{
			name:   "rotated square",
			points: []Vec2{{U: 0, V: 1}, {U: 1, V: 0}, {U: 2, V: 1}, {U: 1, V: 2}},
			wantOK: false,
		},
		{
			name:   "missing a corner",
			points: []Vec2{{U: 0, V: 0}, {U: 4, V: 0}, {U: 4, V: 3}, {U: 4, V: 3}},
			wantOK: false,
		},
		{
			name:   "degenerate strip",
			points: []Vec2{{U: 0, V: 0}, {U: 4, V: 0}, {U: 4, V: 0}, {U: 0, V: 0}},
			wantOK: false,
		},
		{
			name:   "too few vertices",
			points: []Vec2{{U: 0, V: 0}, {U: 4, V: 0}, {U: 4, V: 3}},
			wantOK: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, ok := Rect2FromPolygon(test.points, 1e-9)
			if ok != test.wantOK {
				t.Fatalf("Rect2FromPolygon ok = %v, want %v", ok, test.wantOK)
			}

			if ok && got != test.want {
				t.Fatalf("Rect2FromPolygon = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestRect2OverlapsIgnoresSharedEdges(t *testing.T) {
	t.Parallel()

	left := Rect2{UMax: 1, VMax: 1}
	right := Rect2{UMin: 1, UMax: 2, VMax: 1}
	inner := Rect2{UMin: 0.5, VMin: 0.5, UMax: 1.5, VMax: 1.5}

	if left.Overlaps(right, 1e-9) {
		t.Fatal("edge-adjacent rectangles must not overlap")
	}

	if !left.Overlaps(inner, 1e-9) {
		t.Fatal("rectangles sharing positive area must overlap")
	}
}

func TestRect2ContainsAllowsSharedEdges(t *testing.T) {
	t.Parallel()

	face := Rect2{UMax: 4, VMax: 3}

	if !face.Contains(Rect2{UMin: 1, UMax: 2, VMax: 2}, 1e-9) {
		t.Fatal("a floor-flush hole must count as contained")
	}

	if face.Contains(Rect2{UMin: 1, UMax: 5, VMax: 2}, 1e-9) {
		t.Fatal("a hole extending past the face must not count as contained")
	}
}

func TestBoundingRect2OfEmptyInput(t *testing.T) {
	t.Parallel()

	if got := BoundingRect2(nil); got != (Rect2{}) {
		t.Fatalf("BoundingRect2(nil) = %+v, want the zero rectangle", got)
	}
}
