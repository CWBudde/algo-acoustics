package scene

// GroupSignature hashes everything that determines a room group's acoustics:
// its member rooms, the open portals inside it, and the closed portals that
// bound it, including their materials.
//
// Keying caches on the signature rather than on the GroupID is what makes
// eviction sound. Opening a portal renumbers every group, so an ID-keyed cache
// misses on groups that did not change at all; a signature-keyed one keeps them
// warm, which is the whole basis of the incremental re-simulation in
// docs/raven.md section 8.
func (g *AcousticSceneGraph) GroupSignature(id GroupID) (uint64, bool) {
	rooms, ok := g.GroupRooms(id)
	if !ok {
		return 0, false
	}

	writer := newSceneHashWriter()
	for _, roomIndex := range rooms {
		writer.writeRoom(g.scene, roomIndex)
		writer.writeShoeboxMaterials(g.scene, roomIndex)
	}

	member := make(map[int]bool, len(rooms))
	for _, roomIndex := range rooms {
		member[roomIndex] = true
	}

	for portalIndex, portal := range g.scene.Portals {
		if !member[portal.RoomIndices[0]] && !member[portal.RoomIndices[1]] {
			continue
		}

		writer.writePortal(portalIndex, portal)
		writer.writePortalMaterial(g.scene, portal)
	}

	return writer.hash.Sum64(), true
}

// GroupSignatures returns the signature of every current group.
func (g *AcousticSceneGraph) GroupSignatures() map[GroupID]uint64 {
	signatures := make(map[GroupID]uint64, g.GroupCount())

	for index := range g.GroupCount() {
		id := GroupID(index)
		if signature, ok := g.GroupSignature(id); ok {
			signatures[id] = signature
		}
	}

	return signatures
}

// writeShoeboxMaterials folds a room's surface materials into the hash. Unlike
// GeometryHash, a group signature must react to a material change, because the
// cached responses it guards depend on absorption.
func (w *sceneHashWriter) writeShoeboxMaterials(s *Scene, roomIndex int) {
	room, ok := s.RoomAt(roomIndex)
	if !ok {
		return
	}

	if room.Shoebox != nil {
		for _, name := range room.Shoebox.WallMaterials {
			w.writeMaterial(s, name)
		}
	}

	w.writeMaterial(s, room.MeshMaterial)

	for _, name := range room.TriangleMaterials {
		w.writeMaterial(s, name)
	}
}

func (w *sceneHashWriter) writePortalMaterial(s *Scene, portal Portal) {
	w.writeMaterial(s, portal.Material)
}

func (w *sceneHashWriter) writeMaterial(s *Scene, name string) {
	_, _ = w.hash.Write([]byte(name))

	material, ok := s.Materials[name]
	if !ok {
		return
	}

	for _, values := range [][]float64{
		material.AbsorptionByBand,
		material.ScatteringByBand,
		material.TransmissionByBand,
		material.SoundReductionIndex,
	} {
		for _, value := range values {
			w.writeFloat(value)
		}
	}

	for _, value := range material.Scattering {
		w.writeFloat(value)
	}
}
