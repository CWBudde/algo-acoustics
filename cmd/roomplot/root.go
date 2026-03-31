package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "roomplot",
		Short:         "Inspect room-acoustics diagnostics and plots.",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newSourceDirectivityCommand())

	return cmd
}

func run(cmd *cobra.Command) int {
	err := cmd.Execute()
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err)

		return 1
	}

	return 0
}
