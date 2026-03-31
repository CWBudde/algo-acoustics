package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

type corpusCase struct {
	name     string
	fixture  string
	t60Range [2]float64
	edtRange [2]float64
	c80Range [2]float64
}

type corpusRow struct {
	name     string
	t60      float64
	edt      float64
	c80      float64
	rangeStr string
	pass     bool
}

var defaultCorpusCases = []corpusCase{
	{name: "tiny room", fixture: "tiny_room.json", t60Range: [2]float64{0.03, 0.18}, edtRange: [2]float64{0.02, 0.20}, c80Range: [2]float64{-2, 14}},
	{name: "control room", fixture: "control_room.json", t60Range: [2]float64{0.18, 0.75}, edtRange: [2]float64{0.15, 0.90}, c80Range: [2]float64{-8, 10}},
	{name: "lecture room", fixture: "lecture_room.json", t60Range: [2]float64{0.70, 2.20}, edtRange: [2]float64{0.50, 2.80}, c80Range: [2]float64{-14, 6}},
	{name: "pa room", fixture: "pa_room.json", t60Range: [2]float64{0.25, 1.40}, edtRange: [2]float64{0.20, 1.80}, c80Range: [2]float64{-10, 8}},
}

func newReportCommand() *cobra.Command {
	var format string
	var outputPath string
	var fixtureDir string
	var maxOrder int

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render the benchmark corpus report.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rows, err := buildCorpusReport(fixtureDir, maxOrder)
			if err != nil {
				return err
			}

			switch strings.ToLower(format) {
			case "table":
				renderCorpusTable(cmd.OutOrStdout(), rows)
				return nil
			case "markdown":
				if outputPath == "" {
					outputPath = "bench_report.md"
				}

				err := writeCorpusMarkdown(outputPath, rows)
				if err != nil {
					return err
				}

				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", outputPath)

				return nil
			default:
				return fmt.Errorf("unknown report format %q", format)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "report format: table or markdown")
	cmd.Flags().StringVar(&outputPath, "output", "", "output path for markdown reports")
	cmd.Flags().StringVar(&fixtureDir, "fixtures", defaultFixtureDir, "fixture directory")
	cmd.Flags().IntVar(&maxOrder, "max-order", 3, "maximum reflection order")

	return cmd
}

func buildCorpusReport(fixtureDir string, maxOrder int) ([]corpusRow, error) {
	rows := make([]corpusRow, 0, len(defaultCorpusCases))
	for _, corpusCase := range defaultCorpusCases {
		fixturePath := filepath.Join(fixtureDir, corpusCase.fixture)

		sc, err := scene.LoadSceneFile(fixturePath)
		if err != nil {
			return nil, fmt.Errorf("load fixture %s: %w", fixturePath, err)
		}

		err = scene.Validate(sc)
		if err != nil {
			return nil, fmt.Errorf("validate fixture %s: %w", fixturePath, err)
		}

		events, err := solveFixture(sc, maxOrder)
		if err != nil {
			return nil, fmt.Errorf("solve fixture %s: %w", fixturePath, err)
		}

		buf, err := ir.RenderMono(events, ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 2.5, BandSpec: sc.BandSpec})
		if err != nil {
			return nil, fmt.Errorf("render fixture %s: %w", fixturePath, err)
		}

		t60, err := metrics.T60FromDecaySlope(buf)
		if err != nil {
			return nil, fmt.Errorf("compute T60 for %s: %w", fixturePath, err)
		}

		edt, err := metrics.EDT(buf)
		if err != nil {
			return nil, fmt.Errorf("compute EDT for %s: %w", fixturePath, err)
		}

		c80, err := metrics.C80(buf)
		if err != nil {
			return nil, fmt.Errorf("compute C80 for %s: %w", fixturePath, err)
		}

		pass := inRange(t60, corpusCase.t60Range) && inRange(edt, corpusCase.edtRange) && inRange(c80, corpusCase.c80Range)
		rows = append(rows, corpusRow{
			name:     corpusCase.name,
			t60:      t60,
			edt:      edt,
			c80:      c80,
			rangeStr: formatRange(corpusCase.t60Range, corpusCase.edtRange, corpusCase.c80Range),
			pass:     pass,
		})
	}

	return rows, nil
}

func renderCorpusTable(w io.Writer, rows []corpusRow) {
	if w == nil {
		return
	}

	_, _ = fmt.Fprintln(w, "Room\tT60\tEDT\tC80\tExpected range\tPass")

	for _, row := range rows {
		status := "FAIL"
		if row.pass {
			status = "PASS"
		}

		_, _ = fmt.Fprintf(w, "%s\t%.3f\t%.3f\t%.3f\t%s\t%s\n", row.name, row.t60, row.edt, row.c80, row.rangeStr, status)
	}
}

func writeCorpusMarkdown(path string, rows []corpusRow) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create report file: %w", err)
	}
	defer file.Close()

	_, err = fmt.Fprintln(file, "| Room | T60 | EDT | C80 | Expected range | Pass |")
	if err != nil {
		return fmt.Errorf("write report header: %w", err)
	}

	_, err = fmt.Fprintln(file, "| --- | ---: | ---: | ---: | --- | --- |")
	if err != nil {
		return fmt.Errorf("write report header: %w", err)
	}

	for _, row := range rows {
		status := "FAIL"
		if row.pass {
			status = "PASS"
		}

		_, err = fmt.Fprintf(file, "| %s | %.3f | %.3f | %.3f | %s | %s |\n", escapeMarkdown(row.name), row.t60, row.edt, row.c80, escapeMarkdown(row.rangeStr), status)
		if err != nil {
			return fmt.Errorf("write report row: %w", err)
		}
	}

	return nil
}

func inRange(value float64, bounds [2]float64) bool {
	return value >= bounds[0] && value <= bounds[1]
}

func formatRange(t60Range, edtRange, c80Range [2]float64) string {
	return fmt.Sprintf("T60 %.2f-%.2f s, EDT %.2f-%.2f s, C80 %.2f-%.2f dB", t60Range[0], t60Range[1], edtRange[0], edtRange[1], c80Range[0], c80Range[1])
}

func escapeMarkdown(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.ReplaceAll(value, "\n", " ")
}
