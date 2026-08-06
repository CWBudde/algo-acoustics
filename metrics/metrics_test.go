package metrics

import (
	"bytes"
	"math"
	"strings"
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

	if got := buffer.String(); !strings.Contains(got, "T60") {
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

	got, err := T60FromDecaySlope(buf)
	if err != nil {
		t.Fatalf("T60FromDecaySlope() error = %v", err)
	}

	if math.Abs(got-targetT60) > 0.05 {
		t.Fatalf("T60FromDecaySlope() = %v, want %v", got, targetT60)
	}

	got, err = EDT(buf)
	if err != nil {
		t.Fatalf("EDT() error = %v", err)
	}

	if math.Abs(got-targetT60) > 0.05 {
		t.Fatalf("EDT() = %v, want %v", got, targetT60)
	}

	got, err = T20(buf)
	if err != nil {
		t.Fatalf("T20() error = %v", err)
	}

	if math.Abs(got-targetT60) > 0.05 {
		t.Fatalf("T20() = %v, want %v", got, targetT60)
	}

	got, err = T30(buf)
	if err != nil {
		t.Fatalf("T30() error = %v", err)
	}

	if math.Abs(got-targetT60) > 0.05 {
		t.Fatalf("T30() = %v, want %v", got, targetT60)
	}

	got, err = D50(buf)
	if err != nil {
		t.Fatalf("D50() error = %v", err)
	}

	if got <= 0 || got >= 1 {
		t.Fatalf("D50() = %v, want between 0 and 1", got)
	}

	got, err = C50(buf)
	if err != nil {
		t.Fatalf("C50() error = %v", err)
	}

	wantC50 := 10 * math.Log10((1-math.Exp(-60*math.Ln10/10*0.05))/math.Exp(-60*math.Ln10/10*0.05))
	if math.Abs(got-wantC50) > 0.01 {
		t.Fatalf("C50() = %v, want %v", got, wantC50)
	}

	got, err = C80(buf)
	if err != nil {
		t.Fatalf("C80() error = %v", err)
	}

	if !math.IsInf(got, 1) && got <= 0 {
		t.Fatalf("C80() = %v, want positive or +Inf", got)
	}
}

func TestDecayTimesIgnoreLeadingSilence(t *testing.T) {
	t.Parallel()

	const sampleRate = 1000
	const decaySamples = 1500
	const delaySamples = 250
	const targetT60 = 1.0
	k := 60 * math.Ln10 / 20

	decay := make([]float64, decaySamples)
	for index := range decay {
		decay[index] = math.Exp(-k * float64(index) / sampleRate)
	}

	undelayed := &ir.Buffer{SampleRate: sampleRate, Samples: decay}
	delayedSamples := make([]float64, delaySamples+decaySamples)
	copy(delayedSamples[delaySamples:], decay)
	delayed := &ir.Buffer{SampleRate: sampleRate, Samples: delayedSamples}

	tests := []struct {
		name   string
		metric func(*ir.Buffer) (float64, error)
	}{
		{name: "T60", metric: T60FromDecaySlope},
		{name: "EDT", metric: EDT},
		{name: "T20", metric: T20},
		{name: "T30", metric: T30},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			want, err := test.metric(undelayed)
			if err != nil {
				t.Fatalf("undelayed metric error = %v", err)
			}

			got, err := test.metric(delayed)
			if err != nil {
				t.Fatalf("delayed metric error = %v", err)
			}

			if math.Abs(got-want) > 1e-9 {
				t.Fatalf("delayed metric = %v, undelayed = %v", got, want)
			}

			if math.Abs(got-targetT60) > 0.05 {
				t.Fatalf("metric = %v, want %v", got, targetT60)
			}
		})
	}
}

func TestSplitEnergyUsesHalfOpenEarlyWindow(t *testing.T) {
	t.Parallel()

	buf := &ir.Buffer{SampleRate: 1000, Samples: make([]float64, 52)}
	buf.Samples[49] = 1
	buf.Samples[50] = 2

	early, late, err := splitEnergy(buf, 0.05)
	if err != nil {
		t.Fatalf("splitEnergy() error = %v", err)
	}

	if early != 1 || late != 4 {
		t.Fatalf("splitEnergy() = (%v, %v), want (1, 4)", early, late)
	}
}
