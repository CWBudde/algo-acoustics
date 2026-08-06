package metrics

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/ir"
)

func TestCompareBuffersProducesPeakRMSCorrelationAndBandRows(t *testing.T) {
	t.Parallel()

	left := &ir.Buffer{
		SampleRate: 48000,
		Samples:    make([]float64, 2048),
	}
	right := &ir.Buffer{
		SampleRate: 48000,
		Samples:    make([]float64, 2048),
	}
	left.Samples[0] = 1
	right.Samples[0] = 0.5
	right.Samples[1] = 0

	rows, err := CompareBuffers(left, right, acoustics.Octave6)
	if err != nil {
		t.Fatalf("CompareBuffers() error = %v", err)
	}

	if got, want := len(rows), 3+acoustics.Octave6.BandCount(); got != want {
		t.Fatalf("CompareBuffers() rows = %d, want %d", got, want)
	}

	if rows[0].Name != "peak amplitude" {
		t.Fatalf("first row = %q, want peak amplitude", rows[0].Name)
	}

	if math.Abs(rows[0].Expected-1) > 1e-12 || math.Abs(rows[0].Actual-0.5) > 1e-12 {
		t.Fatalf("peak row = %#v, want 1.0 vs 0.5", rows[0])
	}

	if rows[1].Name != "rms amplitude" {
		t.Fatalf("second row = %q, want rms amplitude", rows[1].Name)
	}

	if rows[2].Name != "correlation" {
		t.Fatalf("third row = %q, want correlation", rows[2].Name)
	}

	if math.Abs(rows[2].Actual-1) > 1e-12 {
		t.Fatalf("correlation = %v, want 1", rows[2].Actual)
	}

	for _, row := range rows[3:] {
		if !strings.HasSuffix(row.Name, "Hz band") {
			t.Fatalf("band row name = %q, want frequency band", row.Name)
		}

		if math.Abs(row.Delta+6.020599913) > 1e-6 {
			t.Fatalf("band delta = %v, want about -6.0206 dB", row.Delta)
		}
	}

	var buf bytes.Buffer
	PrintComparisonReport(rows, &buf)

	output := buf.String()
	if !strings.Contains(output, "peak amplitude") || !strings.Contains(output, "correlation") || !strings.Contains(output, "125 Hz band") {
		t.Fatalf("PrintComparisonReport() output missing rows:\n%s", output)
	}
}

func TestCompareBuffersRejectsMalformedBandSpecs(t *testing.T) {
	t.Parallel()

	left := &ir.Buffer{SampleRate: 48000, Samples: []float64{1}}
	right := &ir.Buffer{SampleRate: 48000, Samples: []float64{1}}

	tests := []struct {
		name string
		spec acoustics.BandSpec
	}{
		{
			name: "missing lower edge",
			spec: acoustics.BandSpec{CenterFreqs: []float64{1000}, UpperEdges: []float64{1400}},
		},
		{
			name: "extra upper edge",
			spec: acoustics.BandSpec{UpperEdges: []float64{1400}},
		},
		{
			name: "non-positive center",
			spec: acoustics.BandSpec{CenterFreqs: []float64{0}, LowerEdges: []float64{700}, UpperEdges: []float64{1400}},
		},
		{
			name: "non-finite edge",
			spec: acoustics.BandSpec{CenterFreqs: []float64{1000}, LowerEdges: []float64{700}, UpperEdges: []float64{math.Inf(1)}},
		},
		{
			name: "center outside edges",
			spec: acoustics.BandSpec{CenterFreqs: []float64{1000}, LowerEdges: []float64{1000}, UpperEdges: []float64{1400}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := CompareBuffers(left, right, test.spec)
			if err == nil {
				t.Fatal("CompareBuffers() error = nil, want error")
			}
		})
	}
}
