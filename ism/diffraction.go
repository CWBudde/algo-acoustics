package ism

import (
	"github.com/cwbudde/algo-acoustics/geometry"
)

// EnumerateFirstOrderDiffractionPaths returns the first-order diffraction paths
// for a real source/receiver pair. It is a thin wrapper over geometry so later
// stages can keep solver-specific orchestration in ism/.
func EnumerateFirstOrderDiffractionPaths(source, receiver geometry.Vec3, edges []geometry.DiffractionEdge, mesh *geometry.Mesh) []geometry.DiffractionPath {
	return geometry.EnumerateDiffractionPaths(source, receiver, edges, mesh)
}

// EnumerateReflectionDiffractionPaths treats each image source as a virtual
// source for diffraction enumeration. The returned paths preserve the virtual
// source position so later stages can map them back to the corresponding
// specular reflection sequence.
func EnumerateReflectionDiffractionPaths(imageSources []geometry.Vec3, receiver geometry.Vec3, edges []geometry.DiffractionEdge, mesh *geometry.Mesh) []geometry.DiffractionPath {
	paths := make([]geometry.DiffractionPath, 0, len(imageSources))
	for _, virtualSource := range imageSources {
		paths = append(paths, geometry.EnumerateDiffractionPaths(virtualSource, receiver, edges, mesh)...)
	}

	return paths
}
