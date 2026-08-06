package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"

	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/spf13/cobra"
)

const (
	defaultSourceDirectivityFreqHz    = 1000
	defaultSourceDirectivityStepDeg   = 5
	defaultSourceDirectivityElevation = 0
	outputFormatCSV                   = "csv"
	outputFormatTable                 = "table"
)

type sourceDirectivityRow struct {
	AzimuthDeg float64
	GainLinear float64
	GainDB     float64
}

var loadGLLModel = directivity.LoadGLL

func newSourceDirectivityCommand() *cobra.Command {
	var outputPath string
	var preset string
	var freqHz float64
	var format string
	var elevationDeg float64
	var stepDeg float64

	cmd := &cobra.Command{
		Use:   "source-directivity <gll-file>",
		Short: "Print source directivity gain by azimuth.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := validateSourceDirectivityOptions(format, freqHz, stepDeg, elevationDeg)
			if err != nil {
				return err
			}

			gllPath := args[0]

			model, err := loadGLLModel(gllPath, preset)
			if err != nil {
				return fmt.Errorf("load gll %q: %w", gllPath, err)
			}

			rows := buildSourceDirectivityRows(model, freqHz, elevationDeg, stepDeg)

			writer := cmd.OutOrStdout()

			var file *os.File
			if outputPath != "" {
				file, err = os.Create(outputPath)
				if err != nil {
					return fmt.Errorf("create output %q: %w", outputPath, err)
				}
				defer file.Close()

				writer = file
			}

			switch format {
			case outputFormatCSV:
				err := writeSourceDirectivityCSV(writer, rows)
				if err != nil {
					return fmt.Errorf("write csv: %w", err)
				}
			case outputFormatTable:
				err := writeSourceDirectivityTable(writer, rows)
				if err != nil {
					return fmt.Errorf("write table: %w", err)
				}
			}

			destination := "stdout"
			if outputPath != "" {
				destination = outputPath
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d source-directivity rows to %s (%s)\n", len(rows), destination, format)

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output file (defaults to stdout)")
	cmd.Flags().StringVar(&preset, "preset", "", "GLL preset selector")
	cmd.Flags().Float64Var(&freqHz, "freq", defaultSourceDirectivityFreqHz, "frequency in Hz")
	cmd.Flags().StringVar(&format, "format", outputFormatCSV, "output format (csv|table)")
	cmd.Flags().Float64Var(&elevationDeg, "elevation", defaultSourceDirectivityElevation, "elevation angle in degrees")
	cmd.Flags().Float64Var(&stepDeg, "step-deg", defaultSourceDirectivityStepDeg, "azimuth step in degrees")

	return cmd
}

func validateSourceDirectivityOptions(format string, freqHz, stepDeg, elevationDeg float64) error {
	if format != outputFormatCSV && format != outputFormatTable {
		return fmt.Errorf("unsupported format %q", format)
	}

	if freqHz <= 0 || math.IsNaN(freqHz) || math.IsInf(freqHz, 0) {
		return fmt.Errorf("frequency must be a finite positive value in Hz, got %g", freqHz)
	}

	if stepDeg <= 0 || math.IsNaN(stepDeg) || math.IsInf(stepDeg, 0) {
		return fmt.Errorf("azimuth step must be a finite positive value in degrees, got %g", stepDeg)
	}

	if math.IsNaN(elevationDeg) || math.IsInf(elevationDeg, 0) {
		return fmt.Errorf("elevation must be finite, got %g", elevationDeg)
	}

	return nil
}

func buildSourceDirectivityRows(model directivity.Model, freqHz, elevationDeg, stepDeg float64) []sourceDirectivityRow {
	if stepDeg <= 0 {
		stepDeg = defaultSourceDirectivityStepDeg
	}

	rows := make([]sourceDirectivityRow, 0, int(math.Ceil(360/stepDeg)))
	elevationRad := elevationDeg * math.Pi / 180
	cosElevation := math.Cos(elevationRad)
	sinElevation := math.Sin(elevationRad)

	for azimuthDeg := 0.0; azimuthDeg < 360; azimuthDeg += stepDeg {
		azimuthRad := azimuthDeg * math.Pi / 180
		dir := geometry.Vec3{
			X: cosElevation * math.Cos(azimuthRad),
			Y: cosElevation * math.Sin(azimuthRad),
			Z: sinElevation,
		}.Normalize()

		gainLinear := model.GainLinear(freqHz, dir)

		gainDB := math.Inf(-1)
		if gainLinear > 0 {
			gainDB = 20 * math.Log10(gainLinear)
		}

		rows = append(rows, sourceDirectivityRow{
			AzimuthDeg: azimuthDeg,
			GainLinear: gainLinear,
			GainDB:     gainDB,
		})
	}

	return rows
}

func writeSourceDirectivityCSV(w io.Writer, rows []sourceDirectivityRow) error {
	writer := csv.NewWriter(w)

	err := writer.Write([]string{"azimuth_deg", "gain_linear", "gain_db"})
	if err != nil {
		return err
	}

	for _, row := range rows {
		err := writer.Write([]string{
			strconv.FormatFloat(row.AzimuthDeg, 'f', 1, 64),
			strconv.FormatFloat(row.GainLinear, 'g', -1, 64),
			strconv.FormatFloat(row.GainDB, 'g', -1, 64),
		})
		if err != nil {
			return err
		}
	}

	writer.Flush()

	return writer.Error()
}

func writeSourceDirectivityTable(w io.Writer, rows []sourceDirectivityRow) error {
	_, err := fmt.Fprintln(w, "azimuth_deg gain_linear gain_db")
	if err != nil {
		return err
	}

	for _, row := range rows {
		_, err = fmt.Fprintf(w, "%10.1f %11.6g %8.3f\n", row.AzimuthDeg, row.GainLinear, row.GainDB)
		if err != nil {
			return err
		}
	}

	return nil
}
