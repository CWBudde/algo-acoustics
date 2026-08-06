package pde

import "math"

// CrossoverConfig controls the low/high transfer-function split.
type CrossoverConfig struct {
	FreqHz           float64
	BandwidthOctaves float64
}

// SplitTF separates a transfer function into low and high components.
func SplitTF(tf *TransferFunction, cfg CrossoverConfig) (low, high *TransferFunction) {
	if !validTF(tf) {
		return nil, nil
	}

	low = cloneTF(tf)

	high = cloneTF(tf)
	for i, freq := range tf.Freqs {
		weight := lowWeight(freq, cfg)
		low.H[i] = tf.H[i] * complex(weight, 0)
		high.H[i] = tf.H[i] * complex(1-weight, 0)
	}

	return low, high
}

// BlendTF crossfades low and high transfer functions around the crossover band.
func BlendTF(low, high *TransferFunction, cfg CrossoverConfig) *TransferFunction {
	if low != nil && !validTF(low) {
		return nil
	}

	if high != nil && !validTF(high) {
		return nil
	}

	if low == nil {
		return cloneTF(high)
	}

	if high == nil {
		return cloneTF(low)
	}

	out := &TransferFunction{Freqs: append([]float64(nil), low.Freqs...), H: make([]complex128, len(low.H))}
	for i, freq := range out.Freqs {
		lowVal := low.sampleAt(freq)
		highVal := high.sampleAt(freq)
		weight := lowWeight(freq, cfg)
		out.H[i] = lowVal*complex(weight, 0) + highVal*complex(1-weight, 0)
	}

	return out
}

func lowWeight(freq float64, cfg CrossoverConfig) float64 {
	if cfg.FreqHz <= 0 {
		return 1
	}

	if freq <= 0 {
		return 1
	}

	bandwidth := cfg.BandwidthOctaves
	if bandwidth <= 0 {
		bandwidth = 1
	}

	half := bandwidth / 2
	lowEdge := cfg.FreqHz * math.Pow(2, -half)
	highEdge := cfg.FreqHz * math.Pow(2, half)

	if freq <= lowEdge {
		return 1
	}

	if freq >= highEdge {
		return 0
	}

	x := (math.Log2(freq/cfg.FreqHz)/half + 1) / 2

	return 0.5 * (1 + math.Cos(math.Pi*x))
}

func cloneTF(tf *TransferFunction) *TransferFunction {
	if tf == nil {
		return nil
	}

	out := &TransferFunction{Freqs: append([]float64(nil), tf.Freqs...), H: make([]complex128, len(tf.H))}
	copy(out.H, tf.H)

	return out
}

func validTF(tf *TransferFunction) bool {
	return tf != nil && len(tf.Freqs) == len(tf.H)
}
