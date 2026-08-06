package algoacoustics

import (
	"errors"
	"fmt"
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/scene"
)

// logDecay60dB is 6·ln(10); converts T60 (seconds to reach −60 dB energy
// decay) into the exponential decay rate γ such that E(t) = E₀·exp(−γt).
const logDecay60dB = 13.815510557964274

// StatisticalTailConfig parameters the synthesis of a statistical late-field
// tail for preview tiers.
type StatisticalTailConfig struct {
	// SampleRate of the output buffer in Hz.
	SampleRate int
	// DurationSeconds is the total duration of the returned buffer (early +
	// tail), measured from t=0.
	DurationSeconds float64
	// CrossoverSeconds is the time at which the early reflection region ends
	// and the statistical tail begins. Typical values: 0.05–0.1 seconds.
	CrossoverSeconds float64
	// CrossfadeSeconds is the length of the cosine crossfade applied across
	// the early/tail boundary to avoid audible seams. Zero = hard cut.
	CrossfadeSeconds float64
	// BandIndex selects which octave band's RT60 drives the decay rate.
	// Typically mid-band (band 2 for Octave6 ≈ 500 Hz).
	BandIndex int
	// Seed controls the Gaussian noise generator. Zero = deterministic default.
	Seed int64
}

// SynthesizeStatisticalTail appends an exponentially decaying noise tail to
// an early reflection buffer using the Eyring RT60 from scene geometry and
// materials. The result is a mono buffer of length DurationSeconds that
// splices the early buffer (up to CrossoverSeconds) with Gaussian noise
// shaped by the Eyring decay envelope.
//
// This is intended for preview tiers (Tier 2 / Tier 3) where a full ray
// trace is too expensive — the statistical tail provides a plausible RT60
// without simulating late-field paths. Tier 4 should replace the tail with
// a ray-traced result via ReplaceStatisticalTail.
func SynthesizeStatisticalTail(sc *scene.Scene, earlyBuf *ir.Buffer, cfg StatisticalTailConfig) (*ir.Buffer, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	if earlyBuf == nil {
		return nil, errors.New("early buffer is nil")
	}

	if cfg.SampleRate <= 0 {
		cfg.SampleRate = earlyBuf.SampleRate
	}

	if earlyBuf.SampleRate <= 0 {
		return nil, errors.New("early buffer sample rate must be positive")
	}

	if cfg.SampleRate <= 0 {
		return nil, errors.New("sample rate must be positive")
	}

	if cfg.SampleRate != earlyBuf.SampleRate {
		return nil, fmt.Errorf("sample rate mismatch: output %d vs early buffer %d", cfg.SampleRate, earlyBuf.SampleRate)
	}

	if cfg.DurationSeconds <= 0 {
		return nil, errors.New("duration must be positive")
	}

	if cfg.CrossoverSeconds < 0 {
		return nil, errors.New("crossover must be non-negative")
	}

	if cfg.CrossfadeSeconds < 0 {
		return nil, errors.New("crossfade must be non-negative")
	}

	stats, err := metrics.ShoeboxStatsFromScene(sc)
	if err != nil {
		return nil, fmt.Errorf("shoebox stats: %w", err)
	}

	bandIndex := cfg.BandIndex
	if bandIndex < 0 || bandIndex >= len(stats.AlphaByBand) {
		bandIndex = len(stats.AlphaByBand) / 2
	}

	t60, err := metrics.EyringRT60(stats, bandIndex)
	if err != nil {
		return nil, fmt.Errorf("eyring rt60: %w", err)
	}

	return buildStatisticalTailBuffer(earlyBuf, t60, cfg)
}

// ReplaceStatisticalTail crossfades between a buffer with a statistical tail
// (produced by SynthesizeStatisticalTail) and a buffer containing a real
// ray-traced late field. The crossfade is applied around crossoverSeconds
// with a total width of crossfadeSeconds, producing a single mono buffer
// whose early region comes from `statisticalBuf` and whose late region
// comes from `raytracedBuf`.
//
// Both buffers must share the same SampleRate. The result has length
// max(len(statisticalBuf), len(raytracedBuf)).
func ReplaceStatisticalTail(statisticalBuf, raytracedBuf *ir.Buffer, crossoverSeconds, crossfadeSeconds float64) (*ir.Buffer, error) {
	if statisticalBuf == nil || raytracedBuf == nil {
		return nil, errors.New("both buffers required")
	}

	if statisticalBuf.SampleRate != raytracedBuf.SampleRate {
		return nil, fmt.Errorf("sample rate mismatch: %d vs %d", statisticalBuf.SampleRate, raytracedBuf.SampleRate)
	}

	if crossoverSeconds < 0 || crossfadeSeconds < 0 {
		return nil, errors.New("crossover and crossfade must be non-negative")
	}

	sampleRate := statisticalBuf.SampleRate
	length := max(len(statisticalBuf.Samples), len(raytracedBuf.Samples))

	out := &ir.Buffer{
		SampleRate: sampleRate,
		Samples:    make([]float64, length),
	}

	crossoverSample := int(math.Round(crossoverSeconds * float64(sampleRate)))
	fadeSamples := int(math.Round(crossfadeSeconds * float64(sampleRate)))

	fadeStart := max(crossoverSample-fadeSamples/2, 0)
	fadeEnd := min(fadeStart+fadeSamples, length)

	for i := range length {
		stat := sampleOrZero(statisticalBuf.Samples, i)
		ray := sampleOrZero(raytracedBuf.Samples, i)

		switch {
		case i < fadeStart:
			out.Samples[i] = stat
		case i >= fadeEnd:
			out.Samples[i] = ray
		default:
			weight := crossfadeWeight(i-fadeStart, fadeEnd-fadeStart)
			out.Samples[i] = stat*(1-weight) + ray*weight
		}
	}

	return out, nil
}

func buildStatisticalTailBuffer(earlyBuf *ir.Buffer, t60 float64, cfg StatisticalTailConfig) (*ir.Buffer, error) {
	if t60 <= 0 {
		return nil, fmt.Errorf("invalid rt60: %g", t60)
	}

	sampleRate := cfg.SampleRate
	totalLen := int(math.Ceil(cfg.DurationSeconds * float64(sampleRate)))

	out := &ir.Buffer{
		SampleRate: sampleRate,
		Samples:    make([]float64, totalLen),
	}

	// Copy the early portion.
	earlyCount := min(len(earlyBuf.Samples), totalLen)
	copy(out.Samples[:earlyCount], earlyBuf.Samples[:earlyCount])

	// Decay constant: E(t) = exp(-gamma * t). Amplitude envelope is
	// sqrt(E(t)) = exp(-gamma * t / 2).
	gamma := logDecay60dB / t60

	// Match tail level to the RMS energy of the early buffer right before
	// crossover, so the tail enters at a physically plausible magnitude.
	crossoverSample := int(math.Round(cfg.CrossoverSeconds * float64(sampleRate)))
	crossoverSample = max(crossoverSample, 0)
	crossoverSample = min(crossoverSample, totalLen)

	referenceEnergy := measureReferenceEnergy(earlyBuf.Samples, crossoverSample, sampleRate)
	referenceAmp := math.Sqrt(referenceEnergy)

	seed := cfg.Seed
	if seed == 0 {
		seed = 1
	}

	rng := rand.New(rand.NewSource(seed)) //nolint:gosec // not security-sensitive

	fadeSamples := int(math.Round(cfg.CrossfadeSeconds * float64(sampleRate)))
	fadeStart := max(crossoverSample-fadeSamples/2, 0)
	fadeEnd := min(crossoverSample+(fadeSamples-fadeSamples/2), totalLen)

	for i := fadeStart; i < totalLen; i++ {
		t := float64(i) / float64(sampleRate)
		timeSinceCrossover := t - cfg.CrossoverSeconds
		envelope := math.Exp(-gamma * timeSinceCrossover / 2)
		noise := rng.NormFloat64() * envelope * referenceAmp

		if i < fadeEnd && fadeEnd > fadeStart {
			// Crossfade from early (stat = out.Samples[i]) to tail noise.
			weight := crossfadeWeight(i-fadeStart, fadeEnd-fadeStart)
			out.Samples[i] = out.Samples[i]*(1-weight) + noise*weight
		} else {
			out.Samples[i] = noise
		}
	}

	return out, nil
}

// measureReferenceEnergy returns the mean-square energy of the last ~20 ms
// of samples up to `end`. This gives a reasonable "just before crossover"
// level to scale the statistical tail to.
func measureReferenceEnergy(samples []float64, end, sampleRate int) float64 {
	if end <= 0 || len(samples) == 0 {
		return 0
	}

	windowSeconds := 0.02
	windowSamples := int(math.Round(windowSeconds * float64(sampleRate)))

	end = min(end, len(samples))
	start := max(end-windowSamples, 0)

	if start >= end {
		return 0
	}

	var sum float64

	for i := start; i < end; i++ {
		sum += samples[i] * samples[i]
	}

	mean := sum / float64(end-start)

	return mean
}

// crossfadeWeight returns a smooth cosine crossfade weight in [0, 1] for
// index `i` within a fade region of length `length`. Weight=0 at i=0,
// weight=1 at i=length.
func crossfadeWeight(i, length int) float64 {
	if length <= 0 {
		return 1
	}

	phase := math.Pi * float64(i) / float64(length)

	return 0.5 * (1 - math.Cos(phase))
}

func sampleOrZero(samples []float64, i int) float64 {
	if i >= 0 && i < len(samples) {
		return samples[i]
	}

	return 0
}
