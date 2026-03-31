package main

import (
	"fmt"

	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

type validationError struct {
	message string
}

func (err *validationError) Error() string {
	return err.message
}

func newValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <scene.json>",
		Short: "Validate a scene file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sceneFile := args[0]

			sc, err := scene.LoadSceneFile(sceneFile)
			if err != nil {
				return fmt.Errorf("load scene %q: %w", sceneFile, err)
			}

			if err := scene.Validate(sc); err != nil {
				return &validationError{message: err.Error()}
			}

			fmt.Fprintln(cmd.OutOrStdout(), "✓ scene is valid")
			return nil
		},
	}
}
