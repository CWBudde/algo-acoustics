package pde

import "testing"

func TestSplitTFRejectsMismatchedShape(t *testing.T) {
	t.Parallel()

	inputs := []*TransferFunction{
		{Freqs: []float64{100, 200}, H: []complex128{1}},
		{Freqs: []float64{100}, H: []complex128{1, 2}},
	}
	for _, input := range inputs {
		low, high := SplitTF(input, CrossoverConfig{FreqHz: 150})
		if low != nil || high != nil {
			t.Fatalf("SplitTF() = (%v, %v), want (nil, nil) for mismatched Freqs/H", low, high)
		}
	}
}

func TestBlendTFRejectsMismatchedShape(t *testing.T) {
	t.Parallel()

	valid := &TransferFunction{Freqs: []float64{100}, H: []complex128{1}}

	invalidInputs := []*TransferFunction{
		{Freqs: []float64{100, 200}, H: []complex128{1}},
		{Freqs: []float64{100}, H: []complex128{1, 2}},
	}
	for _, invalid := range invalidInputs {
		if got := BlendTF(valid, invalid, CrossoverConfig{FreqHz: 150}); got != nil {
			t.Fatalf("BlendTF() = %#v, want nil for mismatched Freqs/H", got)
		}

		if got := BlendTF(invalid, valid, CrossoverConfig{FreqHz: 150}); got != nil {
			t.Fatalf("BlendTF() = %#v, want nil for mismatched Freqs/H", got)
		}
	}
}
