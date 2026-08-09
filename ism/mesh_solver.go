package ism

import (
	"math"
	"sort"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

const (
	meshCoplanarEpsilon = 1e-6
	meshRayOffset       = 1e-6
)

// meshReflectionPoint records one validated reflection on a mesh triangle.
type meshReflectionPoint struct {
	Point    geometry.Vec3
	Normal   geometry.Vec3
	TriIndex int
}

// solveMesh computes direct and specular events for a mesh room.
func solveMesh(sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	bandSpec := cfg.BandSpec
	if bandSpec.BandCount() == 0 {
		bandSpec = sc.BandSpec
	}

	speedOfSound := cfg.SpeedOfSound
	if speedOfSound <= 0 {
		speedOfSound = acoustics.SpeedOfSound
	}

	bvh := geometry.BuildBVH(sc.Room.Mesh)
	material := meshMaterial(sc)
	receiver := sc.Receivers[0]

	events := make([]ir.Event, 0)

	for _, source := range sc.Sources {
		direct, ok := directEvent(source, receiver, bandSpec, speedOfSound)
		if ok {
			events = append(events, direct)
		}

		imgCfg := MeshISMConfig{
			MaxOrder:    cfg.MaxOrder,
			MaxDistance: meshMaxDistance(sc, speedOfSound),
		}

		imageSources := GenerateMeshImageSources(source.Position, sc.Room.Mesh, imgCfg)

		for _, imgSrc := range imageSources {
			if imgSrc.Order == 0 {
				continue
			}

			event, ok := meshSpecularEvent(source, receiver, imgSrc, sc.Room.Mesh, bvh, material, bandSpec, speedOfSound)
			if ok {
				events = append(events, event)
			}
		}
	}

	if cfg.EnableDiffraction {
		events = append(events, meshDiffractionEvents(sc, cfg, bvh, material, bandSpec, speedOfSound)...)
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].TimeSeconds != events[j].TimeSeconds {
			return events[i].TimeSeconds < events[j].TimeSeconds
		}

		if events[i].Kind != events[j].Kind {
			return events[i].Kind < events[j].Kind
		}

		return events[i].DistanceMeters < events[j].DistanceMeters
	})

	return events, nil
}

func meshSpecularEvent(
	source scene.Source,
	receiver scene.Receiver,
	imgSrc MeshImageSource,
	mesh *geometry.Mesh,
	bvh *geometry.BVHNode,
	material scene.Material,
	bandSpec acoustics.BandSpec,
	speedOfSound float64,
) (ir.Event, bool) {
	distance := receiver.Position.Distance(imgSrc.Position)
	if distance <= pathEpsilon {
		return ir.Event{}, false
	}

	path, ok := meshReflectionPath(imgSrc, receiver.Position, mesh, bvh)
	if !ok {
		return ir.Event{}, false
	}

	target := receiver.Position
	if len(path) > 0 {
		target = path[0].Point
	}

	bandGain := directivityBandGain(source, bandSpec, target)
	if bandGainSilent(bandGain) {
		return ir.Event{}, false
	}

	for bandIndex := range bandGain {
		bandGain[bandIndex] *= meshPathReflectance(path, source.Position, material, bandIndex)
	}

	if bandGainSilent(bandGain) {
		return ir.Event{}, false
	}

	return ir.Event{
		TimeSeconds:    distance / speedOfSound,
		Amplitude:      sourceAmplitude(source) / distance,
		Direction:      imgSrc.Position.Sub(receiver.Position).Normalize(),
		DistanceMeters: distance,
		BandGain:       bandGain,
		Kind:           ir.EventSpecular,
	}, true
}

// meshReflectionPath validates the specular path by tracing from receiver
// backward through each reflection plane, unfolding the image source at each
// step. Returns reflection points in source-to-receiver order.
//
// For an order-N image source with TriangleHits [t0, t1, ..., tN-1]:
//   - t0 is the first reflection (closest to source)
//   - tN-1 is the last reflection (closest to receiver)
//
// We trace from receiver and unfold backward:
//  1. target = imgSrc.Position; trace receiver → target, hit tN-1 at pN-1
//  2. target = reflect(target, plane of tN-1); trace pN-1 → target, hit tN-2
//  3. ... until all reflections are found
func meshReflectionPath(
	imgSrc MeshImageSource,
	receiver geometry.Vec3,
	mesh *geometry.Mesh,
	bvh *geometry.BVHNode,
) ([]meshReflectionPoint, bool) {
	order := imgSrc.Order
	if order == 0 || len(imgSrc.TriangleHits) != order {
		return nil, false
	}

	points := make([]meshReflectionPoint, 0, order)
	current := receiver
	target := imgSrc.Position

	for i := order - 1; i >= 0; i-- {
		direction := target.Sub(current)
		dist := direction.Norm()

		if dist <= pathEpsilon {
			return nil, false
		}

		ray := geometry.Ray{
			Origin:    current,
			Direction: direction.Scale(1 / dist),
		}

		hitT, triIdx, hit := bvh.Intersect(ray)
		if !hit {
			return nil, false
		}

		expectedTri := imgSrc.TriangleHits[i]
		if !meshTrianglesCoplanar(mesh, triIdx, expectedTri) {
			return nil, false
		}

		hitPoint := ray.At(hitT)
		normal := mesh.Triangles[triIdx].Normal()

		points = append(points, meshReflectionPoint{
			Point:    hitPoint,
			Normal:   normal,
			TriIndex: triIdx,
		})

		// Unfold: reflect the target back through the plane we just hit,
		// so the next segment traces toward the previous image source.
		plane := geometry.NewPlaneFromPointNormal(hitPoint, normal)
		target = plane.ReflectPoint(target)

		// Offset along the inward normal to avoid self-intersection.
		// The normal points into the room, so this moves us to the
		// interior side of the surface.
		current = hitPoint.Add(normal.Scale(meshRayOffset))
	}

	// Reverse to source-to-receiver order.
	for left, right := 0, len(points)-1; left < right; left, right = left+1, right-1 {
		points[left], points[right] = points[right], points[left]
	}

	return points, true
}

func meshPathReflectance(path []meshReflectionPoint, source geometry.Vec3, material scene.Material, bandIndex int) float64 {
	if len(path) == 0 {
		return 1
	}

	pressure := 1.0
	previous := source

	for _, rp := range path {
		direction := rp.Point.Sub(previous).Normalize()
		if direction == geometry.Vec3Zero {
			return 0
		}

		cosAngle := math.Abs(direction.Dot(rp.Normal))

		if cosAngle > 1 {
			cosAngle = 1
		}

		pressure *= wayverbPressureReflectance(material.AbsorptionAt(bandIndex), cosAngle)
		previous = rp.Point
	}

	return pressure
}

// meshTrianglesCoplanar reports whether triangles a and b share the same
// geometric plane (same normal direction and same signed distance from origin).
func meshTrianglesCoplanar(mesh *geometry.Mesh, a, b int) bool {
	if a == b {
		return true
	}

	triA := mesh.Triangles[a]
	triB := mesh.Triangles[b]

	nA := triA.Normal()
	nB := triB.Normal()

	if math.Abs(nA.Dot(nB)-1) > meshCoplanarEpsilon {
		return false
	}

	dA := nA.Dot(triA.V0)
	dB := nB.Dot(triB.V0)

	return math.Abs(dA-dB) < meshCoplanarEpsilon
}

// meshMaterial looks up the mesh material from the scene, falling back to
// fully reflective if not set or not found.
func meshMaterial(sc *scene.Scene) scene.Material {
	if sc.Room.MeshMaterial == "" {
		return scene.MaterialFullyReflective()
	}

	mat, ok := sc.Materials[sc.Room.MeshMaterial]
	if !ok {
		return scene.MaterialFullyReflective()
	}

	return mat
}

// meshMaxDistance returns a safe maximum image source distance based on a 2-second cap.
func meshMaxDistance(_ *scene.Scene, speedOfSound float64) float64 {
	return speedOfSound * 2.0
}
