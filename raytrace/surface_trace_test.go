package raytrace

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestTraceSurfacesCapturesPortalWallEnergy(t *testing.T) {
	t.Parallel()

	detector, err := NewSurfaceReceiver([]geometry.Vec3{
		{X: 6},
		{X: 6, Y: 4},
		{X: 6, Y: 4, Z: 3},
		{X: 6, Z: 3},
	})
	if err != nil {
		t.Fatalf("NewSurfaceReceiver() error = %v", err)
	}

	sc := secondaryRaytraceScene()
	sc.Sources = []scene.Source{{Position: geometry.Vec3{X: 1, Y: 2, Z: 1.5}}}
	sc.Receivers = nil
	tracer := &RayTracer{
		Config: LaunchConfig{
			NumRays:        2048,
			MaxBounces:     0,
			MaxTimeSeconds: 0.1,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene: sc,
	}

	histograms, err := tracer.TraceSurfaces([]SurfaceReceiver{detector})
	if err != nil {
		t.Fatalf("TraceSurfaces() error = %v", err)
	}

	if len(histograms) != 1 || histogramEnergy(histograms[0]) <= 0 {
		t.Fatalf("TraceSurfaces() histograms = %#v, want one non-silent result", histograms)
	}
}

func histogramEnergy(histogram *EnergyHistogram) float64 {
	var total float64

	for _, bin := range histogram.Bins {
		for _, energy := range bin.BandEnergy {
			total += energy
		}
	}

	return total
}
