package raytrace

import (
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// PathStep records one bounce in a traced ray path.
type PathStep struct {
	HitPoint      geometry.Vec3
	Normal        geometry.Vec3
	WallIndex     int
	SegmentLength float64
}

// TracedPath is a single ray's complete geometric path.
type TracedPath struct {
	LaunchDir geometry.Vec3
	DGIndex   int // directivity group index, -1 if none
	Steps     []PathStep
}

// PathCache holds all traced paths plus a validity key.
type PathCache struct {
	Paths          []TracedPath
	GeometryHash   uint64
	ReceiverRadius float64
	MaxBounces     int
	MaxPathLength  float64
}

// ValidFor reports whether the cache is still valid for the given scene and
// receiver radius. A cache is valid when the geometry hash matches and the
// receiver radius has not changed.
func (c *PathCache) ValidFor(sc *scene.Scene, receiverRadius float64) bool {
	if c == nil || sc == nil {
		return false
	}

	return c.GeometryHash == sc.GeometryHash() && c.ReceiverRadius == receiverRadius
}
