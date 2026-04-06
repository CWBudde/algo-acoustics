package scene

import (
	"encoding/binary"
	"hash/fnv"
	"math"
)

// GeometryHash returns a uint64 FNV-64a hash of the scene's geometry and
// source/receiver positions. Material changes do not affect this hash,
// making it suitable for cache invalidation of geometry-dependent results.
func (s *Scene) GeometryHash() uint64 {
	h := fnv.New64a()
	buf := make([]byte, 8) //nolint:mnd // 8 bytes for float64

	writeFloat := func(v float64) {
		binary.LittleEndian.PutUint64(buf, math.Float64bits(v))
		_, _ = h.Write(buf)
	}

	writeVec3 := func(x, y, z float64) {
		writeFloat(x)
		writeFloat(y)
		writeFloat(z)
	}

	// Room kind.
	_, _ = h.Write([]byte(s.Room.Kind))

	// Shoebox dimensions.
	if s.Room.Shoebox != nil {
		writeVec3(s.Room.Shoebox.Width, s.Room.Shoebox.Depth, s.Room.Shoebox.Height)
	}

	// Mesh triangle vertices.
	if s.Room.Mesh != nil {
		for i := range s.Room.Mesh.Triangles {
			tri := &s.Room.Mesh.Triangles[i]
			writeVec3(tri.V0.X, tri.V0.Y, tri.V0.Z)
			writeVec3(tri.V1.X, tri.V1.Y, tri.V1.Z)
			writeVec3(tri.V2.X, tri.V2.Y, tri.V2.Z)
		}
	}

	// Source positions.
	for i := range s.Sources {
		p := &s.Sources[i].Position
		writeVec3(p.X, p.Y, p.Z)
	}

	// Receiver positions.
	for i := range s.Receivers {
		p := &s.Receivers[i].Position
		writeVec3(p.X, p.Y, p.Z)
	}

	return h.Sum64()
}
