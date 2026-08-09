package scene

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

const portalGeometryTolerance = 1e-7

// PortalState describes whether a portal couples two rooms directly or uses
// its material transmission loss.
type PortalState string

const (
	// PortalOpen makes the portal fully transmissive in every band.
	PortalOpen PortalState = "open"
	// PortalClosed applies the referenced material's transmission coefficients.
	PortalClosed PortalState = "closed"
)

// Portal describes an ordered planar polygon shared by two rooms. Polygon
// winding points from RoomIndices[0] toward RoomIndices[1].
//
//nolint:tagliatelle // Camel-case tags are part of the established public scene schema.
type Portal struct {
	RoomIndices [2]int          `json:"roomIndices"`
	Polygon     []geometry.Vec3 `json:"polygon"`
	Material    string          `json:"material"`
	State       PortalState     `json:"state"`
}

// Normal returns the unit normal implied by the polygon winding.
func (p Portal) Normal() geometry.Vec3 {
	if len(p.Polygon) < 3 {
		return geometry.Vec3Zero
	}

	origin := p.Polygon[0]
	for index := 1; index+1 < len(p.Polygon); index++ {
		normal := p.Polygon[index].Sub(origin).Cross(p.Polygon[index+1].Sub(origin))
		if normal.Norm() > portalGeometryTolerance {
			return normal.Normalize()
		}
	}

	return geometry.Vec3Zero
}

// Area returns the area of the ordered planar polygon.
func (p Portal) Area() float64 {
	normal := p.Normal()
	if normal == geometry.Vec3Zero {
		return 0
	}

	doubleArea := 0.0

	for index, vertex := range p.Polygon {
		next := p.Polygon[(index+1)%len(p.Polygon)]
		doubleArea += vertex.Cross(next).Dot(normal)
	}

	return math.Abs(doubleArea) * 0.5
}

// Center returns the area-weighted centroid of the ordered planar polygon.
func (p Portal) Center() geometry.Vec3 {
	if len(p.Polygon) == 0 {
		return geometry.Vec3Zero
	}

	normal := p.Normal()
	origin := p.Polygon[0]
	weighted := geometry.Vec3Zero
	totalWeight := 0.0

	for index := 1; index+1 < len(p.Polygon); index++ {
		a := p.Polygon[index]
		b := p.Polygon[index+1]
		weight := a.Sub(origin).Cross(b.Sub(origin)).Dot(normal)
		centroid := origin.Add(a).Add(b).Scale(1.0 / 3)
		weighted = weighted.Add(centroid.Scale(weight))
		totalWeight += weight
	}

	if math.Abs(totalWeight) > portalGeometryTolerance {
		return weighted.Scale(1 / totalWeight)
	}

	center := geometry.Vec3Zero
	for _, vertex := range p.Polygon {
		center = center.Add(vertex)
	}

	return center.Scale(1 / float64(len(p.Polygon)))
}

// TransmissionAt returns the portal's effective transmission coefficient for
// a band. Open portals are fully transmissive without changing their material.
func (p Portal) TransmissionAt(materials map[string]Material, bandIndex int) float64 {
	if p.State == PortalOpen {
		return 1
	}

	material, ok := materials[p.Material]
	if !ok {
		return 0
	}

	return material.TransmissionAt(bandIndex)
}
