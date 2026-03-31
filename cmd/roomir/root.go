package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "roomir",
		Short:         "Validate and render room acoustics scenes.",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newValidateCommand())
	cmd.AddCommand(newRenderCommand())
	cmd.AddCommand(newRenderStereoCommand())
	cmd.AddCommand(newDumpEventsCommand())

	return cmd
}

func run(cmd *cobra.Command) int {
	if err := cmd.Execute(); err != nil {
		var validationErr *validationError
		if errors.As(err, &validationErr) {
			fmt.Fprintln(cmd.OutOrStdout(), validationErr.message)
		} else {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
		}

		return 1
	}

	return 0
}
