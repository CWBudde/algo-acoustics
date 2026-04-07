package ism

import "github.com/cwbudde/algo-acoustics/scene"

// ShoeboxCache stores pre-computed shoebox image sources and a geometry hash
// for cache invalidation.
type ShoeboxCache struct {
	Sources      []ImageSource
	GeometryHash uint64
}

// NewShoeboxCache creates a ShoeboxCache from the given image sources and
// records the scene's geometry hash for later validity checks.
func NewShoeboxCache(sources []ImageSource, sc *scene.Scene) *ShoeboxCache {
	return &ShoeboxCache{
		Sources:      sources,
		GeometryHash: sc.GeometryHash(),
	}
}

// ValidFor reports whether the cache is still valid for the given scene.
// It compares the stored geometry hash against the scene's current hash.
func (c *ShoeboxCache) ValidFor(sc *scene.Scene) bool {
	return c.GeometryHash == sc.GeometryHash()
}

// MeshCache stores pre-computed mesh image sources and a geometry hash
// for cache invalidation.
type MeshCache struct {
	Sources      []MeshImageSource
	GeometryHash uint64
}

// NewMeshCache creates a MeshCache from the given mesh image sources and
// records the scene's geometry hash for later validity checks.
func NewMeshCache(sources []MeshImageSource, sc *scene.Scene) *MeshCache {
	return &MeshCache{
		Sources:      sources,
		GeometryHash: sc.GeometryHash(),
	}
}

// ValidFor reports whether the cache is still valid for the given scene.
// It compares the stored geometry hash against the scene's current hash.
func (c *MeshCache) ValidFor(sc *scene.Scene) bool {
	return c.GeometryHash == sc.GeometryHash()
}
