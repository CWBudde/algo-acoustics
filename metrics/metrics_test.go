package metrics

import (
	"bytes"
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

func TestCompareMetricAndReport(t *testing.T) {
	t.Parallel()

	result := CompareMetric("T60", 1.0, 1.02, 0.05)
	if !result.Pass {
		t.Fatalf("CompareMetric() pass = false, want true")
	}

	buffer := &bytes.Buffer{}
	PrintReport([]MetricResult{result}, buffer)

	if got := buffer.String(); !bytes.Contains([]byte(got), []byte("T60")) {
		t.Fatalf("PrintReport() output = %q, want metric name", got)
	}

	if !CompareAll([]MetricResult{result}) {
		t.Fatal("CompareAll() = false, want true")
	}
}

func TestDecayAndClarityMetrics(t *testing.T) {
	t.Parallel()

	const sampleRate = 1000
	const durationSeconds = 1.5
	const targetT60 = 1.0
	const slopeDBPerSecond = 60.0
	k := slopeDBPerSecond * math.Ln10 / 20

	buf := ir.NewBuffer(sampleRate, durationSeconds)
	for index := range buf.Samples {
		timeSeconds := float64(index) / float64(sampleRate)
		buf.Samples[index] = math.Exp(-k * timeSeconds)
	}

	if got, err := T60FromDecaySlope(buf); err != nil {
		t.Fatalf("T60FromDecaySlope() error = %v", err)
	} else if math.Abs(got-targetT60) > 0.05 {
		t.Fatalf("T60FromDecaySlope() = %v, want %v", got, targetT60)
	}

	if got, err := EDT(buf); err != nil {
		t.Fatalf("EDT() error = %v", err)
	} else if math.Abs(got-targetT60) > 0.05 {
		t.Fatalf("EDT() = %v, want %v", got, targetT60)
	}

	if got, err := T20(buf); err != nil {
		t.Fatalf("T20() error = %v", err)
	} else if math.Abs(got-targetT60) > 0.05 {
		t.Fatalf("T20() = %v, want %v", got, targetT60)
	}

	if got, err := T30(buf); err != nil {
		t.Fatalf("T30() error = %v", err)
	} else if math.Abs(got-targetT60) > 0.05 {
		t.Fatalf("T30() = %v, want %v", got, targetT60)
	}

	if got, err := D50(buf); err != nil {
		t.Fatalf("D50() error = %v", err)
	} else if got <= 0 || got >= 1 {
		t.Fatalf("D50() = %v, want between 0 and 1", got)
	}

	if got, err := C50(buf); err != nil {
		t.Fatalf("C50() error = %v", err)
	} else if !math.IsInf(got, 1) && got <= 0 {
		t.Fatalf("C50() = %v, want positive or +Inf", got)
	}

	if got, err := C80(buf); err != nil {
		t.Fatalf("C80() error = %v", err)
	} else if !math.IsInf(got, 1) && got <= 0 {
		t.Fatalf("C80() = %v, want positive or +Inf", got)
	}
}
