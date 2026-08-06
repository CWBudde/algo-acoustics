package hybrid

import (
	"fmt"
	"math"
	"strings"

	"github.com/cwbudde/algo-acoustics/ir"
	window "github.com/cwbudde/algo-dsp/dsp/window"
)

const (
	defaultFadeWindowName = "hann"
	fadeWindowLinear      = "linear"
	fadeWindowBlackman    = "blackman"
	fadeWindowTukey       = "tukey"
)

var supportedFadeWindowNames = []string{
	fadeWindowLinear,
	defaultFadeWindowName,
	"hamming",
	fadeWindowBlackman,
	"exact-blackman",
	"blackman-harris",
	"blackman-harris-3t",
	"blackman-nuttall",
	"nuttall-ctd",
	"nuttall-cfd",
	"flat-top",
	"kaiser",
	fadeWindowTukey,
	"triangle",
	"cosine",
	"welch",
	"lanczos",
	"gauss",
	"lawrey-5t",
	"lawrey-6t",
	"burgess-59db",
	"burgess-71db",
	"albrecht-2t",
	"albrecht-3t",
	"albrecht-4t",
	"albrecht-5t",
	"albrecht-6t",
	"albrecht-7t",
	"albrecht-8t",
	"albrecht-9t",
	"albrecht-10t",
	"albrecht-11t",
}

var fadeWindowTypes = map[string]window.Type{
	defaultFadeWindowName: window.TypeHann,
	"hamming":             window.TypeHamming,
	fadeWindowBlackman:    window.TypeBlackman,
	"exact-blackman":      window.TypeExactBlackman,
	"blackman-harris":     window.TypeBlackmanHarris4Term,
	"blackmanharris":      window.TypeBlackmanHarris4Term,
	"blackman-harris-4t":  window.TypeBlackmanHarris4Term,
	"blackman-harris-3t":  window.TypeBlackmanHarris3Term,
	"blackman-nuttall":    window.TypeBlackmanNuttall,
	"nuttall-ctd":         window.TypeNuttallCTD,
	"nuttall-cfd":         window.TypeNuttallCFD,
	"flat-top":            window.TypeFlatTop,
	"flattop":             window.TypeFlatTop,
	"kaiser":              window.TypeKaiser,
	fadeWindowTukey:       window.TypeTukey,
	"triangle":            window.TypeTriangle,
	"cosine":              window.TypeCosine,
	"welch":               window.TypeWelch,
	"lanczos":             window.TypeLanczos,
	"gauss":               window.TypeGauss,
	"gaussian":            window.TypeGauss,
	"lawrey-5t":           window.TypeLawrey5Term,
	"lawrey-6t":           window.TypeLawrey6Term,
	"burgess-59db":        window.TypeBurgessOptimized59dB,
	"burgess-71db":        window.TypeBurgessOptimized71dB,
	"albrecht-2t":         window.TypeAlbrecht2Term,
	"albrecht-3t":         window.TypeAlbrecht3Term,
	"albrecht-4t":         window.TypeAlbrecht4Term,
	"albrecht-5t":         window.TypeAlbrecht5Term,
	"albrecht-6t":         window.TypeAlbrecht6Term,
	"albrecht-7t":         window.TypeAlbrecht7Term,
	"albrecht-8t":         window.TypeAlbrecht8Term,
	"albrecht-9t":         window.TypeAlbrecht9Term,
	"albrecht-10t":        window.TypeAlbrecht10Term,
	"albrecht-11t":        window.TypeAlbrecht11Term,
}

// FadeWindowConfig selects the fade law used across the crossover window.
type FadeWindowConfig struct {
	// Name selects the window family. Empty defaults to "hann".
	// Supported values match algo-dsp window names plus "linear".
	Name string
	// Alpha is forwarded to parametric algo-dsp windows such as kaiser,
	// tukey, gauss, and lanczos when non-zero.
	Alpha float64
}

// SupportedFadeWindows returns the canonical window names accepted by FadeWindowConfig.
func SupportedFadeWindows() []string {
	return append([]string(nil), supportedFadeWindowNames...)
}

// ValidateFadeWindowConfig reports whether a fade window config is supported.
func ValidateFadeWindowConfig(cfg FadeWindowConfig) error {
	name := normalizedFadeWindowName(cfg.Name)
	if name == fadeWindowLinear {
		return nil
	}

	if _, ok := resolveFadeWindowType(name); !ok {
		return fmt.Errorf("unsupported fade window %q", cfg.Name)
	}

	return nil
}

// LinearFade returns a linear ramp with n points.
func LinearFade(start, end int, n int) []float64 {
	if n <= 0 {
		if end > start {
			n = end - start
		} else {
			return nil
		}
	}

	if n == 1 {
		return []float64{1}
	}

	out := make([]float64, n)
	for i := range n {
		out[i] = float64(i) / float64(n-1)
	}

	return out
}

// HannFade returns a Hann window with n points.
func HannFade(n int) []float64 {
	return window.Generate(window.TypeHann, n)
}

// ApplyFade applies a crossover fade to a buffer copy.
func ApplyFade(buf *ir.Buffer, startSample, endSample int, fadeIn bool) *ir.Buffer {
	return ApplyFadeWithWindow(buf, startSample, endSample, fadeIn, FadeWindowConfig{})
}

// ApplyFadeWithWindow applies a parameterized crossover fade to a buffer copy.
func ApplyFadeWithWindow(buf *ir.Buffer, startSample, endSample int, fadeIn bool, cfg FadeWindowConfig) *ir.Buffer {
	if buf == nil {
		return nil
	}

	out := cloneBuffer(buf)

	if startSample < 0 {
		startSample = 0
	}

	if endSample > len(out.Samples) {
		endSample = len(out.Samples)
	}

	if startSample >= endSample {
		return out
	}

	weights := fadeWeights(endSample-startSample, fadeIn, cfg)
	for i := startSample; i < endSample; i++ {
		out.Samples[i] *= weights[i-startSample]
	}

	return out
}

func fadeWeights(n int, fadeIn bool, cfg FadeWindowConfig) []float64 {
	if n <= 0 {
		return nil
	}

	if n == 1 {
		if fadeIn {
			return []float64{1}
		}

		return []float64{0}
	}

	increasing := increasingFadeWeights(n, cfg)
	if fadeIn {
		return increasing
	}

	decreasing := make([]float64, len(increasing))
	for i, weight := range increasing {
		decreasing[i] = 1 - weight
	}

	return decreasing
}

func increasingFadeWeights(n int, cfg FadeWindowConfig) []float64 {
	name := normalizedFadeWindowName(cfg.Name)

	if name == fadeWindowLinear {
		return LinearFade(0, n-1, n)
	}

	windowType, ok := resolveFadeWindowType(name)
	if !ok {
		windowType = window.TypeHann
	}

	opts := make([]window.Option, 0, 1)
	if cfg.Alpha != 0 {
		opts = append(opts, window.WithAlpha(cfg.Alpha))
	}

	half := window.Generate(windowType, 2*n-1, opts...)[:n]
	start := half[0]
	end := half[len(half)-1]

	span := end - start
	if math.Abs(span) < 1e-12 {
		return LinearFade(0, n-1, n)
	}

	out := make([]float64, len(half))
	for i, weight := range half {
		out[i] = (weight - start) / span
	}

	return out
}

func normalizedFadeWindowName(name string) string {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return defaultFadeWindowName
	}

	return trimmed
}

func resolveFadeWindowType(name string) (window.Type, bool) {
	windowType, ok := fadeWindowTypes[name]

	return windowType, ok
}
