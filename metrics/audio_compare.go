package metrics

import (
	"errors"
	"fmt"
	"io"
	"math"
	"text/tabwriter"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/ir"
	algofft "github.com/cwbudde/algo-fft"
)

// ComparisonRow captures a side-by-side comparison between two values.
type ComparisonRow struct {
	Name     string  `json:"name"`
	Expected float64 `json:"expected"`
	Actual   float64 `json:"actual"`
	Delta    float64 `json:"delta"`
	Unit     string  `json:"unit"`
}

// CompareBuffers compares two dense IR buffers and returns summary rows for
// peak, RMS, correlation, and band-limited energy deltas.
func CompareBuffers(left, right *ir.Buffer, bandSpec acoustics.BandSpec) ([]ComparisonRow, error) {
	if left == nil || right == nil {
		return nil, errors.New("buffers must not be nil")
	}

	if left.SampleRate <= 0 || right.SampleRate <= 0 {
		return nil, errors.New("buffer sample rates must be positive")
	}

	if left.SampleRate != right.SampleRate {
		return nil, errors.New("buffer sample rates must match")
	}

	if len(left.Samples) == 0 || len(right.Samples) == 0 {
		return nil, errors.New("buffers must not be empty")
	}

	rows := make([]ComparisonRow, 0, 3+bandSpec.BandCount())

	leftPeak := bufferPeak(left.Samples)
	rightPeak := bufferPeak(right.Samples)
	rows = append(rows, ComparisonRow{
		Name:     "peak amplitude",
		Expected: leftPeak,
		Actual:   rightPeak,
		Delta:    rightPeak - leftPeak,
		Unit:     "linear",
	})

	leftRMS := bufferRMS(left.Samples)
	rightRMS := bufferRMS(right.Samples)
	rows = append(rows, ComparisonRow{
		Name:     "rms amplitude",
		Expected: leftRMS,
		Actual:   rightRMS,
		Delta:    rightRMS - leftRMS,
		Unit:     "linear",
	})

	correlation, err := bufferCorrelation(left.Samples, right.Samples)
	if err != nil {
		return nil, err
	}

	rows = append(rows, ComparisonRow{
		Name:     "correlation",
		Expected: 1,
		Actual:   correlation,
		Delta:    correlation - 1,
		Unit:     "coefficient",
	})

	if bandSpec.BandCount() == 0 {
		return rows, nil
	}

	nfft := nextPowerOf2(maxInt(len(left.Samples), len(right.Samples)))

	leftBands, err := bandLevels(left.Samples, left.SampleRate, nfft, bandSpec)
	if err != nil {
		return nil, fmt.Errorf("compute left band levels: %w", err)
	}

	rightBands, err := bandLevels(right.Samples, right.SampleRate, nfft, bandSpec)
	if err != nil {
		return nil, fmt.Errorf("compute right band levels: %w", err)
	}

	for index, centerHz := range bandSpec.CenterFreqs {
		expected := leftBands[index]
		actual := rightBands[index]

		delta := actual - expected
		if math.IsInf(expected, -1) && math.IsInf(actual, -1) {
			delta = 0
		}

		rows = append(rows, ComparisonRow{
			Name:     fmt.Sprintf("%.0f Hz band", centerHz),
			Expected: expected,
			Actual:   actual,
			Delta:    delta,
			Unit:     "dB",
		})
	}

	return rows, nil
}

// PrintComparisonReport renders a compact tabular comparison report.
func PrintComparisonReport(rows []ComparisonRow, w io.Writer) {
	if w == nil {
		return
	}

	writer := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "Metric\tExpected\tActual\tDelta\tUnit")

	for _, row := range rows {
		_, _ = fmt.Fprintf(writer, "%s\t%.6f\t%.6f\t%.6f\t%s\n", row.Name, row.Expected, row.Actual, row.Delta, row.Unit)
	}

	_ = writer.Flush()
}

func bufferPeak(samples []float64) float64 {
	peak := 0.0

	for _, sample := range samples {
		magnitude := math.Abs(sample)
		if magnitude > peak {
			peak = magnitude
		}
	}

	return peak
}

func bufferRMS(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}

	energy := 0.0
	for _, sample := range samples {
		energy += sample * sample
	}

	return math.Sqrt(energy / float64(len(samples)))
}

func bufferCorrelation(left, right []float64) (float64, error) {
	if len(left) == 0 || len(right) == 0 {
		return 0, errors.New("buffers must not be empty")
	}

	n := maxInt(len(left), len(right))
	var sumXY, sumXX, sumYY float64

	for i := range n {
		var lx, rx float64
		if i < len(left) {
			lx = left[i]
		}

		if i < len(right) {
			rx = right[i]
		}

		sumXY += lx * rx
		sumXX += lx * lx
		sumYY += rx * rx
	}

	denominator := math.Sqrt(sumXX * sumYY)
	if denominator == 0 {
		return 0, errors.New("buffers contain no energy")
	}

	return sumXY / denominator, nil
}

func bandLevels(samples []float64, sampleRate int, nfft int, spec acoustics.BandSpec) ([]float64, error) {
	if sampleRate <= 0 {
		return nil, errors.New("sample rate must be positive")
	}

	if nfft < 2 {
		nfft = 2
	}

	padded := make([]float64, nfft)
	copy(padded, samples)

	plan, err := algofft.NewPlanReal64(nfft)
	if err != nil {
		return nil, fmt.Errorf("create FFT plan: %w", err)
	}

	spectrum := make([]complex128, nfft/2+1)
	if err := plan.Forward(spectrum, padded); err != nil {
		return nil, fmt.Errorf("FFT forward: %w", err)
	}

	binWidth := float64(sampleRate) / float64(nfft)
	levels := make([]float64, spec.BandCount())

	for i := range spec.CenterFreqs {
		energy := 0.0
		lower := spec.LowerEdges[i]
		upper := spec.UpperEdges[i]

		for bin, coefficient := range spectrum {
			freq := float64(bin) * binWidth
			if freq < lower || freq > upper {
				continue
			}

			energy += real(coefficient)*real(coefficient) + imag(coefficient)*imag(coefficient)
		}

		if energy <= 0 {
			levels[i] = math.Inf(-1)
			continue
		}

		levels[i] = 10 * math.Log10(energy)
	}

	return levels, nil
}

func nextPowerOf2(n int) int {
	if n <= 1 {
		return 1
	}

	power := 1
	for power < n {
		power <<= 1
	}

	return power
}

func maxInt(values ...int) int {
	maximum := 0
	for _, value := range values {
		if value > maximum {
			maximum = value
		}
	}

	return maximum
}
