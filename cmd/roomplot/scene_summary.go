package main

import (
	"fmt"

	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

func newSceneSummaryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "scene-summary <scene.json>",
		Short: "Print a normalized summary of a scene.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, err := scene.LoadSceneFile(args[0])
			if err != nil {
				return fmt.Errorf("load scene %q: %w", args[0], err)
			}

			err = scene.Validate(sc)
			if err != nil {
				return err
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), scene.Summary(sc))

			return err
		},
	}
}
