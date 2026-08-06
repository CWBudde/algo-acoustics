package main

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/internal/pipeline"
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

const reportRayCount = 4096

// These are regression envelopes for the deterministic hybrid report at
// max-order 3. They allow roughly 20-30% decay-time drift and about 4 dB of
// clarity drift while preserving the corpus ordering: the small treated rooms
// decay quickly and remain clear, while the larger rooms carry a stronger late
// field. The former early-only report could not exercise those late-field
// properties, so its clarity limits are not meaningful hybrid baselines.
var defaultCorpusCases = []corpusCase{
	{name: "tiny room", fixture: "tiny_room.json", t60Range: [2]float64{0.12, 0.18}, edtRange: [2]float64{0.05, 0.08}, c80Range: [2]float64{47, 55}},
	{name: "control room", fixture: "control_room.json", t60Range: [2]float64{0.30, 0.42}, edtRange: [2]float64{0.13, 0.20}, c80Range: [2]float64{14, 22}},
	{name: "lecture room", fixture: "lecture_room.json", t60Range: [2]float64{0.62, 0.82}, edtRange: [2]float64{0.75, 1.20}, c80Range: [2]float64{-13, -5}},
	{name: "pa room", fixture: "pa_room.json", t60Range: [2]float64{0.48, 0.68}, edtRange: [2]float64{0.75, 1.12}, c80Range: [2]float64{-10, -2}},
}

func newReportCommand() *cobra.Command {
	return newReportCommandWithBuilder(buildCorpusReport)
}

func newReportCommandWithBuilder(buildReport func(string, int) ([]corpusRow, error)) *cobra.Command {
	var format string
	var outputPath string
	var fixtureDir string
	var maxOrder int

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render the benchmark corpus report.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rows, err := buildReport(fixtureDir, maxOrder)
			if err != nil {
				return err
			}

			switch strings.ToLower(format) {
			case "table":
				renderCorpusTable(cmd.OutOrStdout(), rows)
			case "markdown":
				if outputPath == "" {
					outputPath = "bench_report.md"
				}

				err := writeCorpusMarkdown(outputPath, rows)
				if err != nil {
					return err
				}

				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", outputPath)
			default:
				return fmt.Errorf("unknown report format %q", format)
			}

			if corpusRowsPass(rows) {
				return nil
			}

			return errors.New("one or more corpus rooms are outside their expected metric ranges")
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
		row, err := buildCorpusRow(fixtureDir, maxOrder, corpusCase)
		if err != nil {
			return nil, err
		}

		rows = append(rows, row)
	}

	return rows, nil
}

func buildCorpusRow(fixtureDir string, maxOrder int, corpusCase corpusCase) (corpusRow, error) {
	if !validRange(corpusCase.t60Range) || !validRange(corpusCase.edtRange) || !validRange(corpusCase.c80Range) {
		return corpusRow{}, fmt.Errorf("corpus case %q has an invalid metric range", corpusCase.name)
	}

	fixturePath := filepath.Join(fixtureDir, corpusCase.fixture)

	sc, err := scene.LoadSceneFile(fixturePath)
	if err != nil {
		return corpusRow{}, fmt.Errorf("load fixture %s: %w", fixturePath, err)
	}

	err = scene.Validate(sc)
	if err != nil {
		return corpusRow{}, fmt.Errorf("validate fixture %s: %w", fixturePath, err)
	}

	events, err := solveFixture(sc, maxOrder)
	if err != nil {
		return corpusRow{}, fmt.Errorf("solve fixture %s: %w", fixturePath, err)
	}

	buf, err := renderCorpusFixture(sc, events, maxOrder)
	if err != nil {
		return corpusRow{}, fmt.Errorf("render fixture %s: %w", fixturePath, err)
	}

	t60, err := metrics.T60FromDecaySlope(buf)
	if err != nil {
		return corpusRow{}, fmt.Errorf("compute T60 for %s: %w", fixturePath, err)
	}

	edt, err := metrics.EDT(buf)
	if err != nil {
		return corpusRow{}, fmt.Errorf("compute EDT for %s: %w", fixturePath, err)
	}

	c80, err := metrics.C80(buf)
	if err != nil {
		return corpusRow{}, fmt.Errorf("compute C80 for %s: %w", fixturePath, err)
	}

	return corpusRow{
		name:     corpusCase.name,
		t60:      t60,
		edt:      edt,
		c80:      c80,
		rangeStr: formatRange(corpusCase.t60Range, corpusCase.edtRange, corpusCase.c80Range),
		pass:     inRange(t60, corpusCase.t60Range) && inRange(edt, corpusCase.edtRange) && inRange(c80, corpusCase.c80Range),
	}, nil
}

func renderCorpusFixture(sc *scene.Scene, events []ir.Event, maxOrder int) (*ir.Buffer, error) {
	renderCfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 2.5, BandSpec: sc.BandSpec}

	early, err := ir.RenderMono(events, renderCfg)
	if err != nil {
		return nil, fmt.Errorf("render early IR: %w", err)
	}

	late, err := pipeline.RenderLateBuffer(sc, pipeline.LateConfig{
		NumRays:            reportRayCount,
		MaxOrder:           maxOrder,
		DurationSeconds:    renderCfg.DurationSeconds,
		ReceiverRadius:     0.25,
		BinDurationSeconds: 0.01,
	})
	if err != nil {
		return nil, fmt.Errorf("render late IR: %w", err)
	}

	buf, err := pipeline.RenderHybrid(early, late, events, hybrid.HybridConfig{
		CrossoverTimeSeconds: 0.08,
		CrossoverMode:        hybrid.TimeBased,
		SmoothenCrossover:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("render hybrid IR: %w", err)
	}

	return buf, nil
}

func corpusRowsPass(rows []corpusRow) bool {
	if len(rows) != len(defaultCorpusCases) {
		return false
	}

	expected := make(map[string]struct{}, len(defaultCorpusCases))
	for _, corpusCase := range defaultCorpusCases {
		expected[corpusCase.name] = struct{}{}
	}

	for _, row := range rows {
		if !row.pass || row.rangeStr == "" {
			return false
		}

		if _, ok := expected[row.name]; !ok {
			return false
		}

		delete(expected, row.name)
	}

	return len(expected) == 0
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

func validRange(bounds [2]float64) bool {
	return !math.IsNaN(bounds[0]) && !math.IsNaN(bounds[1]) &&
		!math.IsInf(bounds[0], 0) && !math.IsInf(bounds[1], 0) &&
		bounds[0] < bounds[1]
}

func formatRange(t60Range, edtRange, c80Range [2]float64) string {
	return fmt.Sprintf("T60 %.2f-%.2f s, EDT %.2f-%.2f s, C80 %.2f-%.2f dB", t60Range[0], t60Range[1], edtRange[0], edtRange[1], c80Range[0], c80Range[1])
}

func escapeMarkdown(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.ReplaceAll(value, "\n", " ")
}
