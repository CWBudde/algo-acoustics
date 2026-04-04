package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

func newInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <scene.json>",
		Short: "Print a normalized summary of a scene.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, err := scene.LoadSceneFile(args[0])
			if err != nil {
				return fmt.Errorf("load scene %q: %w", args[0], err)
			}

			err = scene.Validate(sc)
			if err != nil {
				return &validationError{message: err.Error()}
			}

			fmt.Fprint(cmd.OutOrStdout(), sceneSummary(sc))

			return nil
		},
	}
}

func sceneSummary(sc *scene.Scene) string {
	var b strings.Builder

	b.WriteString("scene summary\n")
	b.WriteString(fmt.Sprintf("room: %s", sc.Room.Kind))

	switch sc.Room.Kind {
	case scene.RoomKindShoebox:
		if sc.Room.Shoebox != nil {
			b.WriteString(fmt.Sprintf(" (%.3fm x %.3fm x %.3fm)", sc.Room.Shoebox.Width, sc.Room.Shoebox.Depth, sc.Room.Shoebox.Height))
		}
	case scene.RoomKindMesh:
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
