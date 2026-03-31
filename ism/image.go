package ism

import (
	"sort"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

const (
	wallNegX = iota
	wallPosX
	wallNegY
	wallPosY
	wallNegZ
	wallPosZ
)

const (
	wallBitNegX uint8 = 1 << iota
	wallBitPosX
	wallBitNegY
	wallBitPosY
	wallBitNegZ
	wallBitPosZ
)

// ImageSource describes one unfolded source position for a shoebox room.
type ImageSource struct {
	Position geometry.Vec3
	Order    int
	WallMask uint8

	orderX   int
	orderY   int
	orderZ   int
	roomDims geometry.Vec3
}

// GenerateImageSources enumerates shoebox image sources up to maxOrder.
func GenerateImageSources(src geometry.Vec3, room *scene.Shoebox, maxOrder int) []ImageSource {
	if room == nil || maxOrder < 0 {
		return nil
	}

	dims := geometry.Vec3{X: room.Width, Y: room.Depth, Z: room.Height}
	sources := make([]ImageSource, 0)

	for orderX := -maxOrder; orderX <= maxOrder; orderX++ {
		for orderY := -maxOrder; orderY <= maxOrder; orderY++ {
			for orderZ := -maxOrder; orderZ <= maxOrder; orderZ++ {
				order := absInt(orderX) + absInt(orderY) + absInt(orderZ)
				if order > maxOrder {
					continue
				}

				wallCounts := wallHitCounts(orderX, orderY, orderZ)
				sources = append(sources, ImageSource{
					Position: geometry.Vec3{
						X: imageSourceCoordinate(orderX, src.X, dims.X),
						Y: imageSourceCoordinate(orderY, src.Y, dims.Y),
						Z: imageSourceCoordinate(orderZ, src.Z, dims.Z),
					},
					Order:    order,
					WallMask: wallMaskFromCounts(wallCounts),
					orderX:   orderX,
					orderY:   orderY,
					orderZ:   orderZ,
					roomDims: dims,
				})
			}
		}
	}

	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Order != sources[j].Order {
			return sources[i].Order < sources[j].Order
		}
		if sources[i].Position.X != sources[j].Position.X {
			return sources[i].Position.X < sources[j].Position.X
		}
		if sources[i].Position.Y != sources[j].Position.Y {
			return sources[i].Position.Y < sources[j].Position.Y
		}

		return sources[i].Position.Z < sources[j].Position.Z
	})

	return sources
}

func imageSourceCoordinate(order int, source, dimension float64) float64 {
	if order%2 == 0 {
		return float64(order)*dimension + source
	}

	return float64(order+1)*dimension - source
}

func wallHitCounts(orderX, orderY, orderZ int) [6]int {
	var counts [6]int
	accumulateAxisWallHits(&counts, orderX, wallNegX, wallPosX)
	accumulateAxisWallHits(&counts, orderY, wallNegY, wallPosY)
	accumulateAxisWallHits(&counts, orderZ, wallNegZ, wallPosZ)

	return counts
}

func accumulateAxisWallHits(counts *[6]int, order, negativeWall, positiveWall int) {
	if order == 0 {
		return
	}

	steps := absInt(order)
	firstWall := positiveWall
	secondWall := negativeWall
	if order < 0 {
		firstWall = negativeWall
		secondWall = positiveWall
	}

	for step := 0; step < steps; step++ {
		if step%2 == 0 {
			counts[firstWall]++
			continue
		}

		counts[secondWall]++
	}
}

func wallMaskFromCounts(counts [6]int) uint8 {
	var mask uint8
	if counts[wallNegX] > 0 {
		mask |= wallBitNegX
	}
	if counts[wallPosX] > 0 {
		mask |= wallBitPosX
	}
	if counts[wallNegY] > 0 {
		mask |= wallBitNegY
	}
	if counts[wallPosY] > 0 {
		mask |= wallBitPosY
	}
	if counts[wallNegZ] > 0 {
		mask |= wallBitNegZ
	}
	if counts[wallPosZ] > 0 {
		mask |= wallBitPosZ
	}

	return mask
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}

	return value
}
