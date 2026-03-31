package pde

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestShoeboxModesMatchLocalSweepPeaksAcrossRepresentativeRooms(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		room scene.Shoebox
		src  geometry.Vec3
		rcv  geometry.Vec3
	}{
		{
			name: "tiny",
			room: scene.Shoebox{Width: 2, Depth: 1.5, Height: 1.2},
			src:  geometry.Vec3{X: 0.45, Y: 0.35, Z: 0.25},
			rcv:  geometry.Vec3{X: 1.55, Y: 1.1, Z: 0.85},
		},
		{
			name: "control",
			room: scene.Shoebox{Width: 5, Depth: 4, Height: 2.5},
			src:  geometry.Vec3{X: 1, Y: 1.1, Z: 0.8},
			rcv:  geometry.Vec3{X: 3.8, Y: 2.9, Z: 1.7},
		},
		{
			name: "lecture",
			room: scene.Shoebox{Width: 12, Depth: 8, Height: 4},
			src:  geometry.Vec3{X: 2, Y: 2.4, Z: 1.2},
			rcv:  geometry.Vec3{X: 8.5, Y: 5.4, Z: 2.8},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			modes := uniqueModesUpToFreq(ShoeboxModes(&tc.room, 20), 300)
			if len(modes) == 0 {
				t.Fatal("ShoeboxModes returned no modes under 300 Hz")
			}

			for _, modeType := range []int{1, 2, 3} {
				mode, ok := lowestModeOfType(modes, modeType)
				if !ok {
					t.Fatalf("no mode of type %d found under 300 Hz", modeType)
				}

				peakFreq, err := localSweepPeak(&tc.room, tc.src, tc.rcv, mode.Freq)
				if err != nil {
					t.Fatalf("localSweepPeak(%v) error = %v", mode.Freq, err)
				}

				if math.Abs(peakFreq-mode.Freq)/mode.Freq > 0.02 {
					t.Fatalf("mode freq = %v, sweep peak = %v (room %s, type %d)", mode.Freq, peakFreq, tc.name, modeType)
				}
			}
		})
	}
}

func uniqueModesUpToFreq(modes []ModalFrequency, maxFreq float64) []ModalFrequency {
	out := make([]ModalFrequency, 0, len(modes))
	for _, mode := range modes {
		if mode.Freq > maxFreq {
			continue
		}
		if len(out) > 0 && math.Abs(out[len(out)-1].Freq-mode.Freq) <= 1e-9 {
			continue
		}
		out = append(out, mode)
	}

	return out
}

func lowestModeOfType(modes []ModalFrequency, modeType int) (ModalFrequency, bool) {
	for _, mode := range modes {
		if nonZeroComponents(mode) == modeType {
			return mode, true
		}
	}

	return ModalFrequency{}, false
}

func nonZeroComponents(mode ModalFrequency) int {
	count := 0
	if mode.Nx != 0 {
		count++
	}
	if mode.Ny != 0 {
		count++
	}
	if mode.Nz != 0 {
		count++
	}

	return count
}

func localSweepPeak(room *scene.Shoebox, src, rcv geometry.Vec3, centerFreq float64) (float64, error) {
	span := math.Max(centerFreq*0.1, 5)
	tf, err := SweepShoebox(room, src, rcv, SweepConfig{
		FreqMin:           math.Max(1, centerFreq-span),
		FreqMax:           centerFreq + span,
		NumPoints:         41,
		BoundaryCondition: "neumann",
	})
	if err != nil {
		return 0, err
	}

	bestIndex := 0
	bestMagnitude := 0.0
	for index, value := range tf.H {
		magnitude := math.Hypot(real(value), imag(value))
		if index == 0 || magnitude > bestMagnitude {
			bestIndex = index
			bestMagnitude = magnitude
		}
	}

	return tf.Freqs[bestIndex], nil
}
