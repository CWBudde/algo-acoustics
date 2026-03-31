package raytrace

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

const wallEpsilon = 1e-6

// Tracer exposes the wall-intersection primitive used by the ray tracer.
type Tracer interface {
	NextHit(r geometry.Ray) (hitPoint, normal geometry.Vec3, wallIdx int, ok bool)
}

// ShoeboxTracer holds the six wall planes of an axis-aligned room.
type ShoeboxTracer struct {
	Bounds geometry.Box
	Walls  [6]geometry.Plane
}

// NewShoeboxTracer constructs a wall tracer for a shoebox room.
func NewShoeboxTracer(room *scene.Shoebox) (ShoeboxTracer, error) {
	if room == nil {
		return ShoeboxTracer{}, errors.New("shoebox room is nil")
	}

	bounds := room.Bounds()
	tracer := ShoeboxTracer{Bounds: bounds}
	tracer.Walls[0] = geometry.NewPlaneFromPointNormal(geometry.Vec3{X: bounds.Min.X}, geometry.Vec3{X: -1})
	tracer.Walls[1] = geometry.NewPlaneFromPointNormal(geometry.Vec3{X: bounds.Max.X}, geometry.Vec3{X: 1})
	tracer.Walls[2] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Y: bounds.Min.Y}, geometry.Vec3{Y: -1})
	tracer.Walls[3] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Y: bounds.Max.Y}, geometry.Vec3{Y: 1})
	tracer.Walls[4] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Z: bounds.Min.Z}, geometry.Vec3{Z: -1})
	tracer.Walls[5] = geometry.NewPlaneFromPointNormal(geometry.Vec3{Z: bounds.Max.Z}, geometry.Vec3{Z: 1})

	return tracer, nil
}

// NextHit returns the nearest wall hit for a ray inside the shoebox.
func (t ShoeboxTracer) NextHit(r geometry.Ray) (geometry.Vec3, geometry.Vec3, int, bool) {
	bestT := math.Inf(1)
	bestIdx := -1

	for idx, wall := range t.Walls {
		denom := wall.Normal.Dot(r.Direction)
		if math.Abs(denom) < 1e-12 {
			continue
		}

		tHit := (wall.Distance - wall.Normal.Dot(r.Origin)) / denom
		if tHit <= wallEpsilon || tHit >= bestT {
			continue
		}

		point := r.At(tHit)
		if !t.pointOnWall(point, idx) {
			continue
		}

		bestT = tHit
		bestIdx = idx
	}

	if bestIdx < 0 {
		return geometry.Vec3{}, geometry.Vec3{}, 0, false
	}

	point := r.At(bestT)
	return point, t.Walls[bestIdx].Normal, bestIdx, true
}

func (t ShoeboxTracer) pointOnWall(p geometry.Vec3, wallIdx int) bool {
	switch wallIdx {
	case 0, 1:
		return p.Y >= t.Bounds.Min.Y-wallEpsilon && p.Y <= t.Bounds.Max.Y+wallEpsilon &&
			p.Z >= t.Bounds.Min.Z-wallEpsilon && p.Z <= t.Bounds.Max.Z+wallEpsilon
	case 2, 3:
		return p.X >= t.Bounds.Min.X-wallEpsilon && p.X <= t.Bounds.Max.X+wallEpsilon &&
			p.Z >= t.Bounds.Min.Z-wallEpsilon && p.Z <= t.Bounds.Max.Z+wallEpsilon
	case 4, 5:
		return p.X >= t.Bounds.Min.X-wallEpsilon && p.X <= t.Bounds.Max.X+wallEpsilon &&
			p.Y >= t.Bounds.Min.Y-wallEpsilon && p.Y <= t.Bounds.Max.Y+wallEpsilon
	default:
		return false
	}
}
