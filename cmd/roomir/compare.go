package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/wav"
	"github.com/spf13/cobra"
)

func newCompareCommand() *cobra.Command {
	var format string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "compare <left.wav> <right.wav>",
		Short: "Compare two WAV files and report peak, RMS, correlation, and band deltas.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			left, err := loadComparisonBuffer(args[0])
			if err != nil {
				return fmt.Errorf("load left WAV %q: %w", args[0], err)
			}

			right, err := loadComparisonBuffer(args[1])
			if err != nil {
				return fmt.Errorf("load right WAV %q: %w", args[1], err)
			}

			rows, err := metrics.CompareBuffers(left, right, acoustics.Octave6)
			if err != nil {
				return err
			}

			switch format {
			case "table":
				writer := cmd.OutOrStdout()

				if outputPath != "" {
					file, err := os.Create(outputPath)
					if err != nil {
						return fmt.Errorf("create output %q: %w", outputPath, err)
					}
					defer file.Close()

					writer = file
				}

				metrics.PrintComparisonReport(rows, writer)
			case "csv":
				if outputPath == "" {
					err := export.WriteComparisonCSVTo(cmd.OutOrStdout(), rows)
					if err != nil {
						return err
					}

					return nil
				}

				err := export.WriteComparisonCSV(outputPath, rows)
				if err != nil {
					return err
				}
			case "markdown":
				if outputPath == "" {
					err := export.WriteComparisonMarkdownTo(cmd.OutOrStdout(), rows)
					if err != nil {
						return err
					}

					return nil
				}

				err := export.WriteComparisonMarkdown(outputPath, rows)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported format %q", format)
			}

			if outputPath != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote comparison report to %s (%s)\n", outputPath, format)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "output format (table|csv|markdown)")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output file (defaults to stdout)")

	return cmd
}

func loadComparisonBuffer(path string) (*ir.Buffer, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open wav file: %w", err)
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)

	decoded, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, fmt.Errorf("decode wav: %w", err)
	}

	sampleRate := int(decoder.SampleRate)
	if sampleRate <= 0 {
		return nil, errors.New("wav sample rate must be positive")
	}

	channels := int(decoder.NumChans)
	if channels <= 0 && decoded.Format != nil {
		channels = decoded.Format.NumChannels
	}

	if channels <= 0 {
		channels = 1
	}

	frames := len(decoded.Data) / channels

	buffer := ir.NewBuffer(sampleRate, float64(frames)/float64(sampleRate))
	for frame := range frames {
		sum := 0.0
		for channel := 0; channel < channels; channel++ {
			sum += float64(decoded.Data[frame*channels+channel])
		}

		buffer.Samples[frame] = sum / float64(channels)
	}

	return buffer, nil
}
