package scene

import (
	"encoding/binary"
	"hash"
	"hash/fnv"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// GeometryHash returns a uint64 FNV-64a hash of the scene's room and portal
// geometry, portal state, and source/receiver positions. Material changes do
// not affect this hash, making it suitable for cache invalidation of
// geometry-dependent results.
func (s *Scene) GeometryHash() uint64 {
	writer := newSceneHashWriter()
	writer.writeRoomsAndPortals(s)

	for index := range s.Sources {
		writer.writeVec3(s.Sources[index].Position)
	}

	for index := range s.Receivers {
		writer.writeVec3(s.Receivers[index].Position)
	}

	return writer.hash.Sum64()
}

// RoomHash returns a uint64 FNV-64a hash of room and portal geometry and portal
// state only. Unlike GeometryHash, this excludes source and receiver positions,
// making it suitable for caches whose validity depends on room shape alone.
func (s *Scene) RoomHash() uint64 {
	writer := newSceneHashWriter()
	writer.writeRoomsAndPortals(s)

	return writer.hash.Sum64()
}

type sceneHashWriter struct {
	hash   hash.Hash64
	buffer [8]byte
}

func newSceneHashWriter() *sceneHashWriter {
	return &sceneHashWriter{hash: fnv.New64a()}
}

func (w *sceneHashWriter) writeRoomsAndPortals(s *Scene) {
	for roomIndex := range s.RoomCount() {
		room, ok := s.RoomAt(roomIndex)
		if !ok {
			continue
		}

		w.writeFloat(float64(roomIndex))
		_, _ = w.hash.Write([]byte(room.Kind))

		if room.Shoebox != nil {
			w.writeVec3(room.Shoebox.Origin)
			w.writeVec3(geometry.Vec3{X: room.Shoebox.Width, Y: room.Shoebox.Depth, Z: room.Shoebox.Height})
		}

		if room.Mesh != nil {
			for index := range room.Mesh.Triangles {
				triangle := &room.Mesh.Triangles[index]
				w.writeVec3(triangle.V0)
				w.writeVec3(triangle.V1)
				w.writeVec3(triangle.V2)
			}
		}
	}

	for portalIndex, portal := range s.Portals {
		w.writeFloat(float64(portalIndex))
		w.writeFloat(float64(portal.RoomIndices[0]))
		w.writeFloat(float64(portal.RoomIndices[1]))
		_, _ = w.hash.Write([]byte(portal.State))

		for _, vertex := range portal.Polygon {
			w.writeVec3(vertex)
		}
	}
}

func (w *sceneHashWriter) writeFloat(value float64) {
	binary.LittleEndian.PutUint64(w.buffer[:], math.Float64bits(value))
	_, _ = w.hash.Write(w.buffer[:])
}

func (w *sceneHashWriter) writeVec3(value geometry.Vec3) {
	w.writeFloat(value.X)
	w.writeFloat(value.Y)
	w.writeFloat(value.Z)
}
