package algoacoustics

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

// makeEarlyBuffer builds a simple early-reflection buffer: a direct-path
// impulse at time 0, a reflection at 10 ms, then silence out to the default
// 100 ms early window.
func makeEarlyBuffer(sampleRate int) *ir.Buffer {
	const durationSeconds = 0.1

	length := int(math.Ceil(durationSeconds * float64(sampleRate)))

	samples := make([]float64, length)
	if length > 0 {
		samples[0] = 1.0
	}

	// Add a second "early reflection" at 10 ms to give the reference-energy
	// window something to measure.
	if reflectSample := int(0.01 * float64(sampleRate)); reflectSample < length {
		samples[reflectSample] = 0.5
	}

	return &ir.Buffer{SampleRate: sampleRate, Samples: samples}
}

func TestSynthesizeStatisticalTail_Basic(t *testing.T) {
	t.Parallel()

	sc := progressiveTestScene()
	early := makeEarlyBuffer(sc.SampleRate)

	cfg := StatisticalTailConfig{
		SampleRate:       sc.SampleRate,
		DurationSeconds:  0.8,
		CrossoverSeconds: 0.05,
		CrossfadeSeconds: 0.005,
		BandIndex:        2,
	}

	buf, err := SynthesizeStatisticalTail(sc, early, cfg)
	if err != nil {
		t.Fatalf("SynthesizeStatisticalTail: %v", err)
	}

	wantLen := int(math.Ceil(cfg.DurationSeconds * float64(sc.SampleRate)))
	if buf.Len() != wantLen {
		t.Errorf("buffer length = %d, want %d", buf.Len(), wantLen)
	}

	// Early portion should be preserved (with a possible crossfade near the
	// boundary).
	crossoverSample := int(0.05 * float64(sc.SampleRate))

	fadeStart := max(crossoverSample-int(0.0025*float64(sc.SampleRate)), 0)

	for i := range fadeStart {
		if buf.Samples[i] != early.Samples[i] {
			t.Errorf("sample %d before fade: got %v, want %v (early)", i, buf.Samples[i], early.Samples[i])

			break
		}
	}

	// Tail region should contain non-zero noise.
	tailStart := crossoverSample + int(0.01*float64(sc.SampleRate))

	var nonZero int

	for i := tailStart; i < buf.Len(); i++ {
		if buf.Samples[i] != 0 {
			nonZero++
		}
	}

	if nonZero == 0 {
		t.Error("statistical tail has no non-zero samples")
	}
}

func TestSynthesizeStatisticalTail_DecaysOverTime(t *testing.T) {
	t.Parallel()

	sc := progressiveTestScene()
	early := makeEarlyBuffer(sc.SampleRate)

	cfg := StatisticalTailConfig{
		SampleRate:       sc.SampleRate,
		DurationSeconds:  1.2,
		CrossoverSeconds: 0.05,
		BandIndex:        2,
		Seed:             42,
	}

	buf, err := SynthesizeStatisticalTail(sc, early, cfg)
	if err != nil {
		t.Fatalf("SynthesizeStatisticalTail: %v", err)
	}

	// Compute RMS of early tail window vs late tail window.
	measureRMS := func(start, end int) float64 {
		var sum float64

		n := 0

		for i := start; i < end && i < buf.Len(); i++ {
			sum += buf.Samples[i] * buf.Samples[i]
			n++
		}

		if n == 0 {
			return 0
		}

		return math.Sqrt(sum / float64(n))
	}

	earlyWindowStart := int(0.1 * float64(sc.SampleRate))
	earlyWindowEnd := int(0.2 * float64(sc.SampleRate))
	lateWindowStart := int(0.9 * float64(sc.SampleRate))
	lateWindowEnd := int(1.0 * float64(sc.SampleRate))

	earlyRMS := measureRMS(earlyWindowStart, earlyWindowEnd)
	lateRMS := measureRMS(lateWindowStart, lateWindowEnd)

	if earlyRMS <= 0 || lateRMS <= 0 {
		t.Fatalf("tail RMS must be positive, got early=%g late=%g", earlyRMS, lateRMS)
	}

	if lateRMS >= earlyRMS {
		t.Errorf("tail must decay: earlyRMS=%g lateRMS=%g", earlyRMS, lateRMS)
	}
}

func TestSynthesizeStatisticalTail_Deterministic(t *testing.T) {
	t.Parallel()

	sc := progressiveTestScene()
	early := makeEarlyBuffer(sc.SampleRate)

	cfg := StatisticalTailConfig{
		SampleRate:       sc.SampleRate,
		DurationSeconds:  0.5,
		CrossoverSeconds: 0.05,
		BandIndex:        2,
		Seed:             123,
	}

	buf1, err := SynthesizeStatisticalTail(sc, early, cfg)
	if err != nil {
		t.Fatal(err)
	}

	buf2, err := SynthesizeStatisticalTail(sc, early, cfg)
	if err != nil {
		t.Fatal(err)
	}

	if buf1.Len() != buf2.Len() {
		t.Fatalf("length mismatch: %d vs %d", buf1.Len(), buf2.Len())
	}

	for i := range buf1.Samples {
		if buf1.Samples[i] != buf2.Samples[i] {
			t.Errorf("sample %d: %v != %v (seed should be deterministic)", i, buf1.Samples[i], buf2.Samples[i])

			break
		}
	}
}

func TestSynthesizeStatisticalTail_NilScene(t *testing.T) {
	t.Parallel()

	early := makeEarlyBuffer(48000)

	_, err := SynthesizeStatisticalTail(nil, early, StatisticalTailConfig{
		SampleRate:      48000,
		DurationSeconds: 0.5,
	})
	if err == nil {
		t.Fatal("expected error for nil scene")
	}
}

func TestSynthesizeStatisticalTail_NilBuffer(t *testing.T) {
	t.Parallel()

	sc := progressiveTestScene()

	_, err := SynthesizeStatisticalTail(sc, nil, StatisticalTailConfig{
		SampleRate:      48000,
		DurationSeconds: 0.5,
	})
	if err == nil {
		t.Fatal("expected error for nil buffer")
	}
}

func TestReplaceStatisticalTail_Crossfade(t *testing.T) {
	t.Parallel()

	sampleRate := 48000
	length := sampleRate / 2 // 0.5 s

	// Statistical buffer: all ones.
	stat := &ir.Buffer{
		SampleRate: sampleRate,
		Samples:    make([]float64, length),
	}
	for i := range stat.Samples {
		stat.Samples[i] = 1.0
	}

	// Ray-traced buffer: all 3s.
	ray := &ir.Buffer{
		SampleRate: sampleRate,
		Samples:    make([]float64, length),
	}
	for i := range ray.Samples {
		ray.Samples[i] = 3.0
	}

	// Crossover at 0.2 s, 0.05 s crossfade.
	out, err := ReplaceStatisticalTail(stat, ray, 0.2, 0.05)
	if err != nil {
		t.Fatalf("ReplaceStatisticalTail: %v", err)
	}

	if out.Len() != length {
		t.Fatalf("length = %d, want %d", out.Len(), length)
	}

	// Before fade: must be stat (=1).
	// After fade: must be ray (=3).
	if out.Samples[0] != 1.0 {
		t.Errorf("early sample = %v, want 1.0 (stat)", out.Samples[0])
	}

	if out.Samples[length-1] != 3.0 {
		t.Errorf("late sample = %v, want 3.0 (ray)", out.Samples[length-1])
	}

	// Middle of crossfade should be between 1 and 3.
	crossoverSample := int(0.2 * float64(sampleRate))
	mid := out.Samples[crossoverSample]

	if mid <= 1.0 || mid >= 3.0 {
		t.Errorf("crossfade midpoint = %v, want strictly between 1 and 3", mid)
	}
}

func TestReplaceStatisticalTail_HardCut(t *testing.T) {
	t.Parallel()

	sampleRate := 48000
	length := sampleRate / 2

	stat := &ir.Buffer{SampleRate: sampleRate, Samples: make([]float64, length)}
	for i := range stat.Samples {
		stat.Samples[i] = 1.0
	}

	ray := &ir.Buffer{SampleRate: sampleRate, Samples: make([]float64, length)}
	for i := range ray.Samples {
		ray.Samples[i] = 2.0
	}

	// Zero crossfade = hard cut at the crossover sample.
	out, err := ReplaceStatisticalTail(stat, ray, 0.1, 0)
	if err != nil {
		t.Fatal(err)
	}

	crossoverSample := int(0.1 * float64(sampleRate))

	if out.Samples[crossoverSample-1] != 1.0 {
		t.Errorf("sample just before crossover = %v, want 1.0", out.Samples[crossoverSample-1])
	}

	if out.Samples[crossoverSample] != 2.0 {
		t.Errorf("sample at crossover = %v, want 2.0", out.Samples[crossoverSample])
	}
}

func TestReplaceStatisticalTail_SampleRateMismatch(t *testing.T) {
	t.Parallel()

	stat := &ir.Buffer{SampleRate: 48000, Samples: []float64{0}}
	ray := &ir.Buffer{SampleRate: 44100, Samples: []float64{0}}

	_, err := ReplaceStatisticalTail(stat, ray, 0.1, 0)
	if err == nil {
		t.Fatal("expected sample rate mismatch error")
	}
}
