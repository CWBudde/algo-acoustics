package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/spf13/cobra"
)

func newCompareCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "compare <baseline.json> <current.json>",
		Short: "Compare two metric reports.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseline, err := loadMetricResults(args[0])
			if err != nil {
				return fmt.Errorf("load baseline metrics: %w", err)
			}

			current, err := loadMetricResults(args[1])
			if err != nil {
				return fmt.Errorf("load current metrics: %w", err)
			}

			results := compareMetricReports(baseline, current)
			metrics.PrintReport(results, cmd.OutOrStdout())

			if !metrics.CompareAll(results) {
				return errors.New("one or more metrics differ")
			}

			return nil
		},
	}
}

func loadMetricResults(path string) ([]metrics.MetricResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var results []metrics.MetricResult
	err = json.Unmarshal(data, &results)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func compareMetricReports(baseline, current []metrics.MetricResult) []metrics.MetricResult {
	lookup := make(map[string]metrics.MetricResult, len(current))
	for _, result := range current {
		lookup[result.Name] = result
	}

	results := make([]metrics.MetricResult, 0, len(baseline))
	for _, expected := range baseline {
		actual, ok := lookup[expected.Name]
		if !ok {
			results = append(results, metrics.MetricResult{Name: expected.Name, Expected: expected.Actual, Actual: math.NaN(), Tolerance: expected.Tolerance, Pass: false})
			continue
		}

		tolerance := expected.Tolerance
		if actual.Tolerance > tolerance {
			tolerance = actual.Tolerance
		}

		results = append(results, metrics.CompareMetric(expected.Name, expected.Actual, actual.Actual, tolerance))
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })

	return results
}
