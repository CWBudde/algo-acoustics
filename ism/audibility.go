package ism

import (
	"math"
	"sort"

	"github.com/cwbudde/algo-acoustics/geometry"
)

const pathEpsilon = 1e-9

type reflectionPoint struct {
	Point geometry.Vec3
	Wall  int
}

type pathCrossing struct {
	t    float64
	wall int
}

// IsAudible reports whether the image source produces a valid specular path.
func IsAudible(imgSrc ImageSource, receiver geometry.Vec3) bool {
	_, ok := reflectionPath(imgSrc, receiver)
	return ok
}

func reflectionPath(imgSrc ImageSource, receiver geometry.Vec3) ([]reflectionPoint, bool) {
	if imgSrc.Position.Distance(receiver) <= pathEpsilon {
		return nil, false
	}

	crossings, ok := crossingEvents(imgSrc, receiver)
	if !ok {
		return nil, false
	}

	if len(crossings) != imgSrc.Order {
		return nil, false
	}

	sort.Slice(crossings, func(i, j int) bool {
		return crossings[i].t < crossings[j].t
	})

	for index := 1; index < len(crossings); index++ {
		if math.Abs(crossings[index].t-crossings[index-1].t) <= pathEpsilon {
			return nil, false
		}
	}

	delta := imgSrc.Position.Sub(receiver)

	path := make([]reflectionPoint, 0, len(crossings))
	for _, crossing := range crossings {
		if crossing.t <= pathEpsilon || crossing.t >= 1-pathEpsilon {
			return nil, false
		}

		unfolded := receiver.Add(delta.Scale(crossing.t))

		point := foldPoint(unfolded, imgSrc.roomDims)
		if !pointOnWallInterior(point, crossing.wall, imgSrc.roomDims) {
			return nil, false
		}

		path = append(path, reflectionPoint{Point: point, Wall: crossing.wall})
	}

	return path, true
}

func crossingEvents(imgSrc ImageSource, receiver geometry.Vec3) ([]pathCrossing, bool) {
	events := make([]pathCrossing, 0, imgSrc.Order)

	var ok bool

	events, ok = appendAxisCrossings(events, receiver.X, imgSrc.Position.X, imgSrc.roomDims.X, imgSrc.orderX, wallNegX, wallPosX)
	if !ok {
		return nil, false
	}

	events, ok = appendAxisCrossings(events, receiver.Y, imgSrc.Position.Y, imgSrc.roomDims.Y, imgSrc.orderY, wallNegY, wallPosY)
	if !ok {
		return nil, false
	}

	events, ok = appendAxisCrossings(events, receiver.Z, imgSrc.Position.Z, imgSrc.roomDims.Z, imgSrc.orderZ, wallNegZ, wallPosZ)
	if !ok {
		return nil, false
	}

	return events, true
}

func appendAxisCrossings(events []pathCrossing, receiverCoord, imageCoord, dimension float64, order, negativeWall, positiveWall int) ([]pathCrossing, bool) {
	if order == 0 {
		return events, true
	}

	delta := imageCoord - receiverCoord
	if math.Abs(delta) <= pathEpsilon {
		return nil, false
	}

	steps := absInt(order)
	for step := range steps {
		planeIndex := step + 1

		wall := positiveWall
		if step%2 == 1 {
			wall = negativeWall
		}

		if order < 0 {
			planeIndex = -step

			wall = negativeWall
			if step%2 == 1 {
				wall = positiveWall
			}
		}

		plane := float64(planeIndex) * dimension
		t := (plane - receiverCoord) / delta
		events = append(events, pathCrossing{t: t, wall: wall})
	}

	return events, true
}

func foldPoint(point, dims geometry.Vec3) geometry.Vec3 {
	return geometry.Vec3{
		X: foldCoordinate(point.X, dims.X),
		Y: foldCoordinate(point.Y, dims.Y),
		Z: foldCoordinate(point.Z, dims.Z),
	}
}

func foldCoordinate(value, dimension float64) float64 {
	period := 2 * dimension

	folded := math.Mod(value, period)
	if folded < 0 {
		folded += period
	}

	switch {
	case folded <= pathEpsilon:
		return 0
	case math.Abs(folded-dimension) <= pathEpsilon:
		return dimension
	case folded < dimension:
		return folded
	default:
		mirrored := period - folded
		if mirrored <= pathEpsilon {
			return 0
		}

		if math.Abs(mirrored-dimension) <= pathEpsilon {
			return dimension
		}

		return mirrored
	}
}

func pointOnWallInterior(point geometry.Vec3, wall int, dims geometry.Vec3) bool {
	switch wall {
	case wallNegX:
		return math.Abs(point.X) <= pathEpsilon && withinOpenInterval(point.Y, dims.Y) && withinOpenInterval(point.Z, dims.Z)
	case wallPosX:
		return math.Abs(point.X-dims.X) <= pathEpsilon && withinOpenInterval(point.Y, dims.Y) && withinOpenInterval(point.Z, dims.Z)
	case wallNegY:
		return withinOpenInterval(point.X, dims.X) && math.Abs(point.Y) <= pathEpsilon && withinOpenInterval(point.Z, dims.Z)
	case wallPosY:
		return withinOpenInterval(point.X, dims.X) && math.Abs(point.Y-dims.Y) <= pathEpsilon && withinOpenInterval(point.Z, dims.Z)
	case wallNegZ:
		return withinOpenInterval(point.X, dims.X) && withinOpenInterval(point.Y, dims.Y) && math.Abs(point.Z) <= pathEpsilon
	case wallPosZ:
		return withinOpenInterval(point.X, dims.X) && withinOpenInterval(point.Y, dims.Y) && math.Abs(point.Z-dims.Z) <= pathEpsilon
	default:
		return false
	}
}

func withinOpenInterval(value, dimension float64) bool {
	return value > pathEpsilon && value < dimension-pathEpsilon
}
