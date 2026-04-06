package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestNewDirectivityGroupsCount(t *testing.T) {
	tests := []struct {
		name    string
		azSteps int
		elSteps int
		want    int
	}{
		{"default 12x6", 12, 6, 72},
		{"4x2", 4, 2, 8},
		{"1x1", 1, 1, 1},
		{"6x3", 6, 3, 18},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dgs := NewDirectivityGroups(tt.azSteps, tt.elSteps)
			if len(dgs) != tt.want {
				t.Errorf("NewDirectivityGroups(%d, %d) returned %d groups, want %d",
					tt.azSteps, tt.elSteps, len(dgs), tt.want)
			}
		})
	}
}

func TestNewDirectivityGroupsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		azSteps int
		elSteps int
	}{
		{"zero azimuth", 0, 6},
		{"zero elevation", 12, 0},
		{"negative azimuth", -1, 6},
		{"negative elevation", 12, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dgs := NewDirectivityGroups(tt.azSteps, tt.elSteps)
			if dgs != nil {
				t.Errorf("NewDirectivityGroups(%d, %d) should return nil for invalid input",
					tt.azSteps, tt.elSteps)
			}
		})
	}
}

func TestDirectivityGroupAzimuthCenters(t *testing.T) {
	dgs := NewDirectivityGroups(4, 1)

	// 4 azimuth steps over [0, 2*pi): centers at 45, 135, 225, 315 degrees
	wantAzDeg := []float64{45, 135, 225, 315}

	for i, dg := range dgs {
		gotDeg := dg.AzimuthCenter * 180 / math.Pi
		wantDeg := wantAzDeg[i]

		if math.Abs(gotDeg-wantDeg) > 0.01 {
			t.Errorf("DG[%d] azimuth center = %.2f deg, want %.2f deg", i, gotDeg, wantDeg)
		}
	}
}

func TestDirectivityGroupElevationCenters(t *testing.T) {
	dgs := NewDirectivityGroups(1, 4)

	// 4 elevation steps over [-pi/2, pi/2]: centers at -67.5, -22.5, 22.5, 67.5 degrees
	wantElDeg := []float64{-67.5, -22.5, 22.5, 67.5}

	for i, dg := range dgs {
		gotDeg := dg.ElevationCenter * 180 / math.Pi
		wantDeg := wantElDeg[i]

		if math.Abs(gotDeg-wantDeg) > 0.01 {
			t.Errorf("DG[%d] elevation center = %.2f deg, want %.2f deg", i, gotDeg, wantDeg)
		}
	}
}

func TestDirectivityGroupAngularExtent(t *testing.T) {
	dgs := NewDirectivityGroups(12, 6)

	azExtent := 2 * math.Pi / 12
	elExtent := math.Pi / 6

	for i, dg := range dgs {
		if math.Abs(dg.AzimuthExtent-azExtent) > 1e-10 {
			t.Errorf("DG[%d] azimuth extent = %f, want %f", i, dg.AzimuthExtent, azExtent)
		}

		if math.Abs(dg.ElevationExtent-elExtent) > 1e-10 {
			t.Errorf("DG[%d] elevation extent = %f, want %f", i, dg.ElevationExtent, elExtent)
		}
	}
}

func TestDirectivityGroupRepresentativeDirection(t *testing.T) {
	dgs := NewDirectivityGroups(4, 2)

	for i, dg := range dgs {
		// Direction should be a unit vector
		norm := dg.Direction.Norm()
		if math.Abs(norm-1.0) > 1e-10 {
			t.Errorf("DG[%d] direction norm = %f, want 1.0", i, norm)
		}

		// Direction should correspond to the azimuth/elevation center
		wantX := math.Cos(dg.ElevationCenter) * math.Cos(dg.AzimuthCenter)
		wantY := math.Cos(dg.ElevationCenter) * math.Sin(dg.AzimuthCenter)
		wantZ := math.Sin(dg.ElevationCenter)

		if math.Abs(dg.Direction.X-wantX) > 1e-10 ||
			math.Abs(dg.Direction.Y-wantY) > 1e-10 ||
			math.Abs(dg.Direction.Z-wantZ) > 1e-10 {
			t.Errorf("DG[%d] direction = (%f, %f, %f), want (%f, %f, %f)",
				i, dg.Direction.X, dg.Direction.Y, dg.Direction.Z, wantX, wantY, wantZ)
		}
	}
}

func TestDirectivityGroupsFrontDirectionIsFirstAzimuth(t *testing.T) {
	// Azimuth=0 should correspond to +X direction (frontal in head-related coords)
	// With 4 az steps, center of first is at 45 deg
	dgs := NewDirectivityGroups(4, 1)

	// DG[0]: az center = 45 deg, el center = 0 deg → direction should be in +X,+Y quadrant
	if dgs[0].Direction.X <= 0 {
		t.Errorf("DG[0] should have positive X component (frontal), got %f", dgs[0].Direction.X)
	}
}

func TestClassifyFrontDirection(t *testing.T) {
	dgs := NewDirectivityGroups(4, 2)

	// +X direction (front): azimuth=0, elevation=0
	front := geometry.Vec3{X: 1, Y: 0, Z: 0}
	idx := ClassifyDirection(dgs, front)

	if idx < 0 || idx >= len(dgs) {
		t.Fatalf("ClassifyDirection returned invalid index %d", idx)
	}

	// The classified DG should have azimuth center near 0 and elevation center near 0
	dg := dgs[idx]
	azDeg := dg.AzimuthCenter * 180 / math.Pi
	elDeg := dg.ElevationCenter * 180 / math.Pi

	if azDeg > 90 && azDeg < 270 {
		t.Errorf("front direction classified to DG with azimuth=%.1f deg, expected near 0", azDeg)
	}

	if math.Abs(elDeg) > 45 {
		t.Errorf("front direction classified to DG with elevation=%.1f deg, expected near 0", elDeg)
	}
}

func TestClassifyCardinalDirections(t *testing.T) {
	dgs := NewDirectivityGroups(4, 2)

	tests := []struct {
		name   string
		dir    geometry.Vec3
		wantAz float64 // approximate azimuth in degrees
		wantEl float64 // approximate elevation in degrees
	}{
		{"front", geometry.Vec3{X: 1, Y: 0, Z: 0}, 0, 0},
		{"left", geometry.Vec3{X: 0, Y: 1, Z: 0}, 90, 0},
		{"back", geometry.Vec3{X: -1, Y: 0, Z: 0}, 180, 0},
		{"right", geometry.Vec3{X: 0, Y: -1, Z: 0}, 270, 0},
		{"up", geometry.Vec3{X: 0, Y: 0, Z: 1}, 0, 90},
		{"down", geometry.Vec3{X: 0, Y: 0, Z: -1}, 0, -90},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := ClassifyDirection(dgs, tt.dir)
			if idx < 0 || idx >= len(dgs) {
				t.Fatalf("invalid index %d", idx)
			}

			dg := dgs[idx]
			elDeg := dg.ElevationCenter * 180 / math.Pi

			// Check elevation is in the right half
			if tt.wantEl > 0 && elDeg <= 0 {
				t.Errorf("expected positive elevation, got %.1f", elDeg)
			}

			if tt.wantEl < 0 && elDeg >= 0 {
				t.Errorf("expected negative elevation, got %.1f", elDeg)
			}
		})
	}
}

func TestClassifyZeroVector(t *testing.T) {
	dgs := NewDirectivityGroups(4, 2)
	idx := ClassifyDirection(dgs, geometry.Vec3{})

	// Zero vector should return a valid index (fallback to 0)
	if idx != 0 {
		t.Errorf("ClassifyDirection for zero vector = %d, want 0", idx)
	}
}

func TestClassifyAllDirectionsHaveValidIndex(t *testing.T) {
	dgs := NewDirectivityGroups(12, 6)

	// Sample many random directions on the sphere
	steps := 36
	for azStep := range steps {
		for elStep := range steps {
			az := float64(azStep) * 2 * math.Pi / float64(steps)
			el := -math.Pi/2 + float64(elStep)*math.Pi/float64(steps)

			dir := geometry.Vec3{
				X: math.Cos(el) * math.Cos(az),
				Y: math.Cos(el) * math.Sin(az),
				Z: math.Sin(el),
			}

			idx := ClassifyDirection(dgs, dir)
			if idx < 0 || idx >= len(dgs) {
				t.Errorf("ClassifyDirection(az=%.0f, el=%.0f) = %d, out of range [0, %d)",
					az*180/math.Pi, el*180/math.Pi, idx, len(dgs))
			}
		}
	}
}

func TestDirectivityGroupHistogramIsNil(t *testing.T) {
	// At construction, histograms should be nil (allocated later during tracing)
	dgs := NewDirectivityGroups(4, 2)
	for i, dg := range dgs {
		if dg.Histogram != nil {
			t.Errorf("DG[%d] histogram should be nil at construction", i)
		}
	}
}

func TestDirectivityGroupOrderingElevationMajor(t *testing.T) {
	// DGs should be ordered elevation-major: all azimuth steps for el[0], then el[1], etc.
	dgs := NewDirectivityGroups(4, 3)

	// First 4 DGs should have the same (lowest) elevation
	el0 := dgs[0].ElevationCenter
	for i := 1; i < 4; i++ {
		if math.Abs(dgs[i].ElevationCenter-el0) > 1e-10 {
			t.Errorf("DG[%d] elevation = %f, want %f (same as DG[0])", i, dgs[i].ElevationCenter, el0)
		}
	}

	// DGs 4-7 should have the next elevation
	el1 := dgs[4].ElevationCenter
	if el1 <= el0 {
		t.Errorf("second elevation band (%f) should be above first (%f)", el1, el0)
	}
}
