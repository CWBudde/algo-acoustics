package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/spf13/cobra"
)

type materialBandRow struct {
	BandIndex  int
	CenterHz   float64
	Absorption float64
	Scattering float64
}

func newMaterialsCommand() *cobra.Command {
	var outputPath string
	var format string

	cmd := &cobra.Command{
		Use:   "materials <material|file>",
		Short: "Print a band table for a material library entry or material file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			material, source, err := loadMaterialSpec(args[0])
			if err != nil {
				return err
			}

			rows := buildMaterialBandRows(material)

			writer := cmd.OutOrStdout()

			if outputPath != "" {
				file, err := os.Create(outputPath)
				if err != nil {
					return fmt.Errorf("create output %q: %w", outputPath, err)
				}
				defer file.Close()

				writer = file
			}

			switch format {
			case "table":
				err := writeMaterialTable(writer, source, material, rows)
				if err != nil {
					return fmt.Errorf("write table: %w", err)
				}
			case "csv":
				err := writeMaterialCSV(writer, source, rows)
				if err != nil {
					return fmt.Errorf("write csv: %w", err)
				}
			default:
				return fmt.Errorf("unsupported format %q", format)
			}

			destination := "stdout"
			if outputPath != "" {
				destination = outputPath
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d material bands for %s to %s (%s)\n", len(rows), source, destination, format)

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output file (defaults to stdout)")
	cmd.Flags().StringVar(&format, "format", "table", "output format (table|csv)")

	return cmd
}

func loadMaterialSpec(arg string) (scene.Material, string, error) {
	if info, err := os.Stat(arg); err == nil && !info.IsDir() {
		material, loadErr := scene.LoadMaterialFile(arg)
		if loadErr != nil {
			return scene.Material{}, "", loadErr
		}

		return material, filepath.Base(arg), nil
	}

	material, ok := scene.MaterialLibrary[arg]
	if !ok {
		return scene.Material{}, "", fmt.Errorf("unknown material %q", arg)
	}

	return material, arg, nil
}

func buildMaterialBandRows(material scene.Material) []materialBandRow {
	rows := make([]materialBandRow, 0, scene.NumBands)
	scattering := material.ScatteringCoefficients(scene.NumBands)

	for index, centerHz := range acoustics.Octave6.CenterFreqs {
		rows = append(rows, materialBandRow{
			BandIndex:  index,
			CenterHz:   centerHz,
			Absorption: material.AbsorptionAt(index),
			Scattering: scattering[index],
		})
	}

	return rows
}

func writeMaterialTable(w io.Writer, source string, material scene.Material, rows []materialBandRow) error {
	if _, err := fmt.Fprintf(w, "material: %s\n", material.Name); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "source: %s\n", source); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w, "band center_hz absorption scattering"); err != nil {
		return err
	}

	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "%d %g %g %g\n", row.BandIndex, row.CenterHz, row.Absorption, row.Scattering); err != nil {
			return err
		}
	}

	return nil
}

func writeMaterialCSV(w io.Writer, source string, rows []materialBandRow) error {
	writer := csv.NewWriter(w)

	err := writer.Write([]string{"name", "band", "center_hz", "absorption", "scattering"})
	if err != nil {
		return err
	}

	name := source
	if ext := filepath.Ext(source); ext != "" {
		name = strings.TrimSuffix(filepath.Base(source), ext)
	}

	for _, row := range rows {
		err := writer.Write([]string{
			name,
			strconv.Itoa(row.BandIndex),
			strconv.FormatFloat(row.CenterHz, 'f', -1, 64),
			strconv.FormatFloat(row.Absorption, 'g', -1, 64),
			strconv.FormatFloat(row.Scattering, 'g', -1, 64),
		})
		if err != nil {
			return err
		}
	}

	writer.Flush()

	return writer.Error()
}
