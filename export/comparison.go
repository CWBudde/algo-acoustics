package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cwbudde/algo-acoustics/metrics"
)

// WriteComparisonCSV writes audio comparison rows to a CSV file.
func WriteComparisonCSV(path string, rows []metrics.ComparisonRow) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv file: %w", err)
	}
	defer file.Close()

	return WriteComparisonCSVTo(file, rows)
}

// WriteComparisonMarkdown writes audio comparison rows to a Markdown table.
func WriteComparisonMarkdown(path string, rows []metrics.ComparisonRow) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create markdown file: %w", err)
	}
	defer file.Close()

	return WriteComparisonMarkdownTo(file, rows)
}

// WriteComparisonCSVTo writes audio comparison rows to a CSV writer.
func WriteComparisonCSVTo(w io.Writer, rows []metrics.ComparisonRow) error {
	writer := csv.NewWriter(w)

	err := writer.Write([]string{"metric", "expected", "actual", "delta", "unit"})
	if err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, row := range rows {
		err := writer.Write([]string{
			row.Name,
			fmt.Sprintf("%.6f", row.Expected),
			fmt.Sprintf("%.6f", row.Actual),
			fmt.Sprintf("%.6f", row.Delta),
			row.Unit,
		})
		if err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	writer.Flush()

	err = writer.Error()
	if err != nil {
		return fmt.Errorf("flush csv writer: %w", err)
	}

	return nil
}

// WriteComparisonMarkdownTo writes audio comparison rows to a Markdown writer.
func WriteComparisonMarkdownTo(w io.Writer, rows []metrics.ComparisonRow) error {
	_, err := fmt.Fprintln(w, "| Metric | Expected | Actual | Delta | Unit |")
	if err != nil {
		return fmt.Errorf("write markdown header: %w", err)
	}

	_, err = fmt.Fprintln(w, "| --- | ---: | ---: | ---: | --- |")
	if err != nil {
		return fmt.Errorf("write markdown header: %w", err)
	}

	for _, row := range rows {
		_, err = fmt.Fprintf(w, "| %s | %.6f | %.6f | %.6f | %s |\n", escapeMarkdown(row.Name), row.Expected, row.Actual, row.Delta, escapeMarkdown(row.Unit))
		if err != nil {
			return fmt.Errorf("write markdown row: %w", err)
		}
	}

	return nil
}

func escapeMarkdown(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.ReplaceAll(value, "\n", " ")
}
