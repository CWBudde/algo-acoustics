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
	fmt.Fprintf(&b, "room: %s", sc.Room.Kind)

	switch sc.Room.Kind {
	case RoomKindShoebox:
		if sc.Room.Shoebox != nil {
			fmt.Fprintf(&b, " (%.3fm x %.3fm x %.3fm)", sc.Room.Shoebox.Width, sc.Room.Shoebox.Depth, sc.Room.Shoebox.Height)
		}
	case RoomKindMesh:
		if sc.Room.MeshPath != "" {
			fmt.Fprintf(&b, " (%s)", sc.Room.MeshPath)
		}

		if sc.Room.Mesh != nil {
			fmt.Fprintf(&b, " [%d triangles]", len(sc.Room.Mesh.Triangles))
		}
	}

	b.WriteString("\n")
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
