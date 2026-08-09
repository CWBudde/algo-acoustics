package scene

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// Summary returns a normalized textual description of the scene.
func Summary(sc *Scene) string {
	if sc == nil {
		return "scene summary\n<nil>\n"
	}

	var b strings.Builder

	b.WriteString("scene summary\n")

	if len(sc.Rooms) == 0 {
		writeRoomSummary(&b, "room", sc.Room)
	} else {
		fmt.Fprintf(&b, "rooms: %d\n", len(sc.Rooms))

		for index, room := range sc.Rooms {
			writeRoomSummary(&b, fmt.Sprintf("room[%d]", index), room)
		}

		fmt.Fprintf(&b, "portals: %d\n", len(sc.Portals))
	}

	fmt.Fprintf(&b, "materials: %d", len(sc.Materials))

	if len(sc.Materials) > 0 {
		names := make([]string, 0, len(sc.Materials))
		for name := range sc.Materials {
			names = append(names, name)
		}

		sort.Strings(names)
		b.WriteString(" [")
		b.WriteString(strings.Join(names, ", "))
		b.WriteString("]")
	}

	b.WriteString("\n")
	fmt.Fprintf(&b, "sources: %d", len(sc.Sources))
	b.WriteString("\n")
	fmt.Fprintf(&b, "receivers: %d", len(sc.Receivers))
	b.WriteString("\n")
	fmt.Fprintf(&b, "band count: %d", sc.BandSpec.BandCount())

	writeBandSummary(&b, sc.BandSpec.CenterFreqs)

	b.WriteString("\n")
	fmt.Fprintf(&b, "sample rate: %d", sc.SampleRate)
	b.WriteString(" Hz\n")

	return b.String()
}

func writeRoomSummary(b *strings.Builder, label string, room Room) {
	fmt.Fprintf(b, "%s: %s", label, room.Kind)

	switch room.Kind {
	case RoomKindShoebox:
		if room.Shoebox != nil {
			fmt.Fprintf(b, " (%.3fm x %.3fm x %.3fm)", room.Shoebox.Width, room.Shoebox.Depth, room.Shoebox.Height)

			if room.Shoebox.Origin != (geometry.Vec3{}) {
				fmt.Fprintf(b, " at (%.3f, %.3f, %.3f)", room.Shoebox.Origin.X, room.Shoebox.Origin.Y, room.Shoebox.Origin.Z)
			}
		}
	case RoomKindMesh:
		if room.MeshPath != "" {
			fmt.Fprintf(b, " (%s)", room.MeshPath)
		}

		if room.Mesh != nil {
			fmt.Fprintf(b, " [%d triangles]", len(room.Mesh.Triangles))
		}
	}

	b.WriteString("\n")
}

func writeBandSummary(b *strings.Builder, centerFreqs []float64) {
	if len(centerFreqs) == 0 {
		return
	}

	b.WriteString(" [")

	for i, freq := range centerFreqs {
		if i > 0 {
			b.WriteString(", ")
		}

		fmt.Fprintf(b, "%.0f", freq)
	}

	b.WriteString(" Hz]")
}
