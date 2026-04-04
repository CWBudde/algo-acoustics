package scene

import (
	"fmt"
	"sort"
	"strings"
)

// Summary returns a normalized textual description of the scene.
func Summary(sc *Scene) string {
	if sc == nil {
		return "scene summary\n<nil>\n"
	}

	var b strings.Builder

	b.WriteString("scene summary\n")
	b.WriteString(fmt.Sprintf("room: %s", sc.Room.Kind))

	switch sc.Room.Kind {
	case RoomKindShoebox:
		if sc.Room.Shoebox != nil {
			b.WriteString(fmt.Sprintf(" (%.3fm x %.3fm x %.3fm)", sc.Room.Shoebox.Width, sc.Room.Shoebox.Depth, sc.Room.Shoebox.Height))
		}
	case RoomKindMesh:
		if sc.Room.MeshPath != "" {
			b.WriteString(fmt.Sprintf(" (%s)", sc.Room.MeshPath))
		}

		if sc.Room.Mesh != nil {
			b.WriteString(fmt.Sprintf(" [%d triangles]", len(sc.Room.Mesh.Triangles)))
		}
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("materials: %d", len(sc.Materials)))

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
	b.WriteString(fmt.Sprintf("sources: %d", len(sc.Sources)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("receivers: %d", len(sc.Receivers)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("band count: %d", sc.BandSpec.BandCount()))

	if count := sc.BandSpec.BandCount(); count > 0 {
		b.WriteString(" [")
		for i, freq := range sc.BandSpec.CenterFreqs {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%.0f", freq))
		}
		b.WriteString(" Hz]")
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("sample rate: %d", sc.SampleRate))
	b.WriteString(" Hz\n")

	return b.String()
}
