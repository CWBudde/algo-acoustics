package metrics

import (
	"fmt"
	"io"
	"math"
	"text/tabwriter"
)

// MetricResult captures the comparison between an expected and actual metric value.
type MetricResult struct {
	Name      string  `json:"name"`
	Expected  float64 `json:"expected"`
	Actual    float64 `json:"actual"`
	Tolerance float64 `json:"tolerance"`
	Pass      bool    `json:"pass"`
}

// CompareMetric evaluates a single metric result against a tolerance.
func CompareMetric(name string, expected, actual, tolerance float64) MetricResult {
	if tolerance < 0 {
		tolerance = math.Abs(tolerance)
	}

	return MetricResult{
		Name:      name,
		Expected:  expected,
		Actual:    actual,
		Tolerance: tolerance,
		Pass:      math.Abs(actual-expected) <= tolerance,
	}
}

// CompareAll returns true when all metric results passed.
func CompareAll(results []MetricResult) bool {
	for _, result := range results {
		if !result.Pass {
			return false
		}
	}

	return true
}

// PrintReport renders a compact tabular comparison report.
func PrintReport(results []MetricResult, w io.Writer) {
	if w == nil {
		return
	}

	writer := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "Metric\tExpected\tActual\tTolerance\tPass")

	for _, result := range results {
		pass := "FAIL"
		if result.Pass {
			pass = "PASS"
		}

		_, _ = fmt.Fprintf(writer, "%s\t%.6f\t%.6f\t%.6f\t%s\n", result.Name, result.Expected, result.Actual, result.Tolerance, pass)
	}

	_ = writer.Flush()
}
