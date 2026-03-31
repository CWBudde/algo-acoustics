package hybrid

import (
	"fmt"
	"math"
	"strings"

	"github.com/cwbudde/algo-acoustics/ir"
	window "github.com/cwbudde/algo-dsp/dsp/window"
)

const defaultFadeWindowName = "hann"

var supportedFadeWindowNames = []string{
	"linear",
	"hann",
	"hamming",
	"blackman",
	"exact-blackman",
	"blackman-harris",
	"blackman-harris-3t",
	"blackman-nuttall",
	"nuttall-ctd",
	"nuttall-cfd",
	"flat-top",
	"kaiser",
	"tukey",
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
	if name == "linear" {
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
	for i := 0; i < n; i++ {
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

	if name == "linear" {
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
	switch name {
	case "hann":
		return window.TypeHann, true
	case "hamming":
		return window.TypeHamming, true
	case "blackman":
		return window.TypeBlackman, true
	case "exact-blackman":
		return window.TypeExactBlackman, true
	case "blackman-harris", "blackmanharris", "blackman-harris-4t":
		return window.TypeBlackmanHarris4Term, true
	case "blackman-harris-3t":
		return window.TypeBlackmanHarris3Term, true
	case "blackman-nuttall":
		return window.TypeBlackmanNuttall, true
	case "nuttall-ctd":
		return window.TypeNuttallCTD, true
	case "nuttall-cfd":
		return window.TypeNuttallCFD, true
	case "flat-top", "flattop":
		return window.TypeFlatTop, true
	case "kaiser":
		return window.TypeKaiser, true
	case "tukey":
		return window.TypeTukey, true
	case "triangle":
		return window.TypeTriangle, true
	case "cosine":
		return window.TypeCosine, true
	case "welch":
		return window.TypeWelch, true
	case "lanczos":
		return window.TypeLanczos, true
	case "gauss", "gaussian":
		return window.TypeGauss, true
	case "lawrey-5t":
		return window.TypeLawrey5Term, true
	case "lawrey-6t":
		return window.TypeLawrey6Term, true
	case "burgess-59db":
		return window.TypeBurgessOptimized59dB, true
	case "burgess-71db":
		return window.TypeBurgessOptimized71dB, true
	case "albrecht-2t":
		return window.TypeAlbrecht2Term, true
	case "albrecht-3t":
		return window.TypeAlbrecht3Term, true
	case "albrecht-4t":
		return window.TypeAlbrecht4Term, true
	case "albrecht-5t":
		return window.TypeAlbrecht5Term, true
	case "albrecht-6t":
		return window.TypeAlbrecht6Term, true
	case "albrecht-7t":
		return window.TypeAlbrecht7Term, true
	case "albrecht-8t":
		return window.TypeAlbrecht8Term, true
	case "albrecht-9t":
		return window.TypeAlbrecht9Term, true
	case "albrecht-10t":
		return window.TypeAlbrecht10Term, true
	case "albrecht-11t":
		return window.TypeAlbrecht11Term, true
	default:
		return 0, false
	}
}
