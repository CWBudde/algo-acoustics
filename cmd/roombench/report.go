package main

import (
	"fmt"

	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/spf13/cobra"
)

func newReportCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "report <metrics.json>",
		Short: "Render a metrics report as a table.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := loadMetricResults(args[0])
			if err != nil {
				return fmt.Errorf("load metrics: %w", err)
			}

			metrics.PrintReport(results, cmd.OutOrStdout())
			return nil
		},
	}
}
