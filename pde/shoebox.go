package pde

import (
	"errors"
	"math"
	"sort"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
	algofft "github.com/cwbudde/algo-fft"
)

// TransferFunction stores a sampled complex transfer function.
type TransferFunction struct {
	Freqs []float64
	H     []complex128
}

// Magnitude returns the magnitude of the transfer function at index i.
func (tf *TransferFunction) Magnitude(i int) float64 {
	if tf == nil || i < 0 || i >= len(tf.H) {
		return 0
	}

	return math.Hypot(real(tf.H[i]), imag(tf.H[i]))
}

// PhaseRad returns the phase of the transfer function at index i.
func (tf *TransferFunction) PhaseRad(i int) float64 {
	if tf == nil || i < 0 || i >= len(tf.H) {
		return 0
	}

	return math.Atan2(imag(tf.H[i]), real(tf.H[i]))
}

// ToTimeDomain converts the transfer function to a real time-domain signal.
func (tf *TransferFunction) ToTimeDomain(sampleRate int, nFFT int) []float64 {
	if tf == nil || sampleRate <= 0 || len(tf.H) == 0 {
		return nil
	}

	if nFFT < 2*(len(tf.H)-1) {
		nFFT = 2 * (len(tf.H) - 1)
	}

	if nFFT < 2 {
		return nil
	}

	spectrum := make([]complex128, nFFT)

	half := nFFT / 2
	for k := 0; k <= half; k++ {
		freq := float64(k) * float64(sampleRate) / float64(nFFT)
		spectrum[k] = tf.sampleAt(freq)
	}

	spectrum[0] = complex(real(spectrum[0]), 0)
	if nFFT%2 == 0 {
		spectrum[half] = complex(real(spectrum[half]), 0)

		for k := 1; k < half; k++ {
			spectrum[nFFT-k] = complex(real(spectrum[k]), -imag(spectrum[k]))
		}
	} else {
		for k := 1; k <= half; k++ {
			spectrum[nFFT-k] = complex(real(spectrum[k]), -imag(spectrum[k]))
		}
	}

	plan, err := algofft.NewPlan64(nFFT)
	if err != nil {
		return nil
	}

	out := make([]complex128, nFFT)

	err = plan.Inverse(out, spectrum)
	if err != nil {
		return nil
	}

	result := make([]float64, nFFT)
	for i := range result {
		result[i] = real(out[i])
	}

	return result
}

func (tf *TransferFunction) sampleAt(freq float64) complex128 {
	if tf == nil || len(tf.H) == 0 {
		return 0
	}

	type sample struct {
		freq float64
		val  complex128
	}

	samples := make([]sample, 0, len(tf.H))
	for i := range tf.H {
		f := 0.0
		if i < len(tf.Freqs) {
			f = tf.Freqs[i]
		}

		samples = append(samples, sample{freq: f, val: tf.H[i]})
	}

	if len(samples) == 0 {
		return 0
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i].freq < samples[j].freq })

	if freq <= samples[0].freq {
		return samples[0].val
	}

	last := samples[len(samples)-1]
	if freq >= last.freq {
		return last.val
	}

	for i := range len(samples) - 1 {
		lo := samples[i]

		hi := samples[i+1]
		if freq < lo.freq || freq > hi.freq {
			continue
		}

		span := hi.freq - lo.freq
		if span <= 0 {
			return hi.val
		}

		w := (freq - lo.freq) / span

		return complex(
			real(lo.val)*(1-w)+real(hi.val)*w,
			imag(lo.val)*(1-w)+imag(hi.val)*w,
		)
	}

	return last.val
}

// SweepConfig controls the low-frequency frequency sweep.
type SweepConfig struct {
	FreqMin           float64
	FreqMax           float64
	NumPoints         int
	BoundaryCondition string
}

// PDELowFreqEngine generates a low-frequency transfer function using Helmholtz sweeps.
type PDELowFreqEngine struct {
	Sweep           SweepConfig
	CrossoverFreqHz float64
}

// CrossoverHz returns the blend crossover frequency.
func (e PDELowFreqEngine) CrossoverHz() float64 {
	if e.CrossoverFreqHz > 0 {
		return e.CrossoverFreqHz
	}

	return 200
}

// Transfer generates a low-frequency transfer function for the first source/receiver pair.
func (e PDELowFreqEngine) Transfer(sc *scene.Scene, _ ir.RenderConfig) (*TransferFunction, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	if sc.Room.Kind != scene.RoomKindShoebox || sc.Room.Shoebox == nil {
		return nil, errors.New("PDE low-frequency solver requires a shoebox room")
	}

	if len(sc.Sources) == 0 {
		return nil, errors.New("scene has no sources")
	}

	if len(sc.Receivers) == 0 {
		return nil, errors.New("scene has no receivers")
	}

	sweep := e.Sweep
	if sweep.FreqMin <= 0 {
		sweep.FreqMin = 20
	}

	if sweep.FreqMax <= sweep.FreqMin {
		sweep.FreqMax = 300
	}

	if sweep.NumPoints <= 0 {
		sweep.NumPoints = 48
	}

	if sweep.BoundaryCondition == "" {
		sweep.BoundaryCondition = boundaryNeumann
	}

	tf, err := SweepShoebox(sc.Room.Shoebox, sc.Sources[0].Position, sc.Receivers[0].Position, sweep)
	if err != nil {
		return nil, err
	}

	return tf, nil
}

func cellIndex(ix, iy, iz, ny, nz int) int {
	return ix*ny*nz + iy*nz + iz
}

func nearestCell(pos geometry.Vec3, nx, ny, nz int, hx, hy, hz float64) (int, int, int) {
	ix := int(math.Round(pos.X / hx))
	iy := int(math.Round(pos.Y / hy))
	iz := int(math.Round(pos.Z / hz))

	if ix < 0 {
		ix = 0
	} else if ix >= nx {
		ix = nx - 1
	}

	if iy < 0 {
		iy = 0
	} else if iy >= ny {
		iy = ny - 1
	}

	if iz < 0 {
		iz = 0
	} else if iz >= nz {
		iz = nz - 1
	}

	return ix, iy, iz
}
