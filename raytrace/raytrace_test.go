package raytrace

import (
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestRayTracerTraceProducesLateEnergy(t *testing.T) {
	t.Parallel()

	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6,
				Depth:  4.5,
				Height: 2.8,
				WallMaterials: [6]string{
					"reflective", "reflective", "reflective", "reflective", "reflective", "reflective",
				},
			},
		},
		Materials: map[string]scene.Material{
			"reflective": scene.MaterialFullyReflective(),
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1.2, Y: 1.0, Z: 1.2}, GainDB: -12}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 3.5, Y: 2.2, Z: 1.2}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	tracer := RayTracer{
		Config: LaunchConfig{NumRays: 10000, MaxBounces: 8, MaxTimeSeconds: 1, SpeedOfSound: acoustics.SpeedOfSound},
		Scene:  sc,
	}

	hist, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	if len(hist.Bins) == 0 {
		t.Fatal("Trace() returned empty histogram")
	}

	var total float64

	for _, bin := range hist.Bins {
		for _, energy := range bin.BandEnergy {
			total += energy
		}
	}

	if total <= 0 {
		t.Fatal("Trace() produced zero total energy")
	}
}

func TestRayTracerTraceSupportsMeshRoom(t *testing.T) {
	t.Parallel()

	mesh := cubeMesh(geometry.Vec3Zero, geometry.Vec3{X: 4, Y: 4, Z: 4})
	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindMesh,
			Mesh: mesh,
		},
		Materials: map[string]scene.Material{
			"reflective": scene.MaterialFullyReflective(),
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1, Y: 1, Z: 1}, GainDB: -12}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 1.8, Y: 1, Z: 1}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	tracer := RayTracer{
		Config:         LaunchConfig{NumRays: 4096, MaxBounces: 6, MaxTimeSeconds: 1, SpeedOfSound: acoustics.SpeedOfSound},
		Scene:          sc,
		ReceiverRadius: 0.5,
	}

	hist, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	if len(hist.Bins) == 0 {
		t.Fatal("Trace() returned empty histogram")
	}

	var total float64

	for _, bin := range hist.Bins {
		for _, energy := range bin.BandEnergy {
			total += energy
		}
	}

	if total <= 0 {
		t.Fatal("Trace() produced zero total energy for mesh room")
	}
}

func TestRayTracerLoadedCubeMeshKeepsBouncesInsideBounds(t *testing.T) {
	t.Parallel()

	sc := loadMeshCubeScene(t)

	bounds, ok := sc.Room.Bounds()
	if !ok {
		t.Fatal("mesh room bounds unavailable")
	}

	rt := RayTracer{
		Config: LaunchConfig{NumRays: 1000, MaxBounces: 8, MaxTimeSeconds: 0.25, SpeedOfSound: acoustics.SpeedOfSound},
		Scene:  sc,
	}

	tracer, err := rt.sceneTracer()
	if err != nil {
		t.Fatalf("sceneTracer() error = %v", err)
	}

	rays := LaunchRays(sc.Sources[0].Position, rt.Config)
	for rayIndex, ray := range rays {
		currentRay := ray

		for bounce := 0; bounce <= rt.Config.MaxBounces; bounce++ {
			hitPoint, hitNormal, _, hit := tracer.NextHit(currentRay)
			if !hit {
				t.Fatalf("ray %d bounce %d missed closed mesh", rayIndex, bounce)
			}

			if !boxContainsWithin(bounds, hitPoint, 1e-5) {
				t.Fatalf("ray %d bounce %d hit point %#v outside bounds %#v", rayIndex, bounce, hitPoint, bounds)
			}

			nextDir := SelectReflection(0, currentRay.Direction, hitNormal, nil)

			nextOrigin := hitPoint.Add(nextDir.Scale(wallEpsilon))
			if !boxContainsWithin(bounds, nextOrigin, 1e-5) {
				t.Fatalf("ray %d bounce %d next origin %#v outside bounds %#v", rayIndex, bounce, nextOrigin, bounds)
			}

			currentRay = geometry.NewRay(nextOrigin, nextDir)
		}
	}
}

func TestRayTracerLoadedCubeMeshDecayMatchesShoebox(t *testing.T) {
	t.Parallel()

	meshScene := loadMeshCubeScene(t)
	shoeboxScene := equivalentShoeboxScene(meshScene)
	config := LaunchConfig{NumRays: 1000, MaxBounces: 8, MaxTimeSeconds: 0.25, SpeedOfSound: acoustics.SpeedOfSound}

	meshHist, err := (&RayTracer{Config: config, Scene: meshScene, ReceiverRadius: 0.4}).Trace()
	if err != nil {
		t.Fatalf("mesh Trace() error = %v", err)
	}

	shoeboxHist, err := (&RayTracer{Config: config, Scene: shoeboxScene, ReceiverRadius: 0.4}).Trace()
	if err != nil {
		t.Fatalf("shoebox Trace() error = %v", err)
	}

	meshDecay := normalizedTailEnergy(meshHist)

	shoeboxDecay := normalizedTailEnergy(shoeboxHist)
	if len(meshDecay) != len(shoeboxDecay) {
		t.Fatalf("tail energy length mismatch: mesh=%d shoebox=%d", len(meshDecay), len(shoeboxDecay))
	}

	if diff := meanAbsDifference(meshDecay, shoeboxDecay); diff > 0.08 {
		t.Fatalf("mean decay difference = %g, want <= 0.08", diff)
	}

	if diff := maxAbsDifference(meshDecay, shoeboxDecay); diff > 0.2 {
		t.Fatalf("max decay difference = %g, want <= 0.2", diff)
	}
}

func loadMeshCubeScene(t *testing.T) *scene.Scene {
	t.Helper()

	path := filepath.Join("..", "testdata", "rooms", "mesh_cube.json")

	sc, err := scene.LoadSceneFile(path)
	if err != nil {
		t.Fatalf("LoadSceneFile(%q) error = %v", path, err)
	}

	return sc
}

func equivalentShoeboxScene(meshScene *scene.Scene) *scene.Scene {
	bounds, ok := meshScene.Room.Bounds()
	if !ok {
		return nil
	}

	dims := bounds.Dimensions()

	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  dims.X,
				Depth:  dims.Y,
				Height: dims.Z,
				WallMaterials: [6]string{
					"reflective", "reflective", "reflective", "reflective", "reflective", "reflective",
				},
			},
		},
		Materials: map[string]scene.Material{
			"reflective": scene.MaterialFullyReflective(),
		},
		Sources:    append([]scene.Source(nil), meshScene.Sources...),
		Receivers:  append([]scene.Receiver(nil), meshScene.Receivers...),
		BandSpec:   meshScene.BandSpec,
		SampleRate: meshScene.SampleRate,
	}
}

func normalizedTailEnergy(hist *EnergyHistogram) []float64 {
	totals := histogramTotalEnergy(hist)
	tail := make([]float64, len(totals))

	var sum float64
	for i := len(totals) - 1; i >= 0; i-- {
		sum += totals[i]
		tail[i] = sum
	}

	if len(tail) == 0 || tail[0] <= 0 {
		return tail
	}

	for i := range tail {
		tail[i] /= tail[0]
	}

	return tail
}

func histogramTotalEnergy(hist *EnergyHistogram) []float64 {
	if hist == nil {
		return nil
	}

	totals := make([]float64, len(hist.Bins))
	for i, bin := range hist.Bins {
		for _, energy := range bin.BandEnergy {
			totals[i] += energy
		}
	}

	return totals
}

func meanAbsDifference(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 1
	}

	var sum float64

	for i := range a {
		delta := a[i] - b[i]
		if delta < 0 {
			delta = -delta
		}

		sum += delta
	}

	return sum / float64(len(a))
}

func maxAbsDifference(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 1
	}

	var maxDelta float64

	for i := range a {
		delta := a[i] - b[i]
		if delta < 0 {
			delta = -delta
		}

		if delta > maxDelta {
			maxDelta = delta
		}
	}

	return maxDelta
}

func boxContainsWithin(bounds geometry.Box, point geometry.Vec3, tolerance float64) bool {
	return point.X >= bounds.Min.X-tolerance && point.X <= bounds.Max.X+tolerance &&
		point.Y >= bounds.Min.Y-tolerance && point.Y <= bounds.Max.Y+tolerance &&
		point.Z >= bounds.Min.Z-tolerance && point.Z <= bounds.Max.Z+tolerance
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
		{V0: v000, V1: v010, V2: v001},
		{V0: v001, V1: v010, V2: v011},
		{V0: v100, V1: v101, V2: v110},
		{V0: v101, V1: v111, V2: v110},
		{V0: v000, V1: v001, V2: v100},
		{V0: v001, V1: v101, V2: v100},
		{V0: v010, V1: v110, V2: v011},
		{V0: v011, V1: v110, V2: v111},
		{V0: v000, V1: v100, V2: v010},
		{V0: v010, V1: v100, V2: v110},
		{V0: v001, V1: v011, V2: v101},
		{V0: v011, V1: v111, V2: v101},
	}}
}
