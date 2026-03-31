package algoacoustics

import (
	"errors"

	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/pde"
	"github.com/cwbudde/algo-acoustics/scene"
)

// TransferFunction carries a low-frequency transfer function.
type TransferFunction = pde.TransferFunction

// EarlyEngine generates the early sparse event stream for a scene.
type EarlyEngine interface {
	Generate(sc *scene.Scene, cfg ir.RenderConfig) ([]ir.Event, error)
}

// LateEngine generates the late sparse event stream for a scene.
type LateEngine interface {
	Generate(sc *scene.Scene, cfg ir.RenderConfig) ([]ir.Event, error)
}

// LowFreqEngine generates a transfer function for low-frequency rendering.
type LowFreqEngine interface {
	Transfer(sc *scene.Scene, cfg ir.RenderConfig) (*TransferFunction, error)
}

// Renderer orchestrates early, late, and future low-frequency engines.
type Renderer struct {
	Early   EarlyEngine
	Late    LateEngine
	LowFreq LowFreqEngine
	Hybrid  hybrid.HybridConfig
}

// RenderMono renders a scene into a mono buffer using the configured engines.
func (r Renderer) RenderMono(sc *scene.Scene, cfg ir.RenderConfig) ([]float64, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}
	var earlyEvents []ir.Event
	var lateEvents []ir.Event
	var err error

	if r.Early != nil {
		earlyEvents, err = r.Early.Generate(sc, cfg)
		if err != nil {
			return nil, err
		}
	}

	if r.Late != nil {
		lateEvents, err = r.Late.Generate(sc, cfg)
		if err != nil {
			return nil, err
		}
	}

	combined := hybrid.Combine(earlyEvents, lateEvents, r.Hybrid)

	buffer, err := ir.RenderMono(combined, cfg)
	if err != nil {
		return nil, err
	}

	if r.LowFreq != nil {
		if provider, ok := r.LowFreq.(interface{ CrossoverHz() float64 }); ok {
			transfer, err := r.LowFreq.Transfer(sc, cfg)
			if err != nil {
				return nil, err
			}

			if transfer != nil {
				lowIR := transfer.ToTimeDomain(cfg.SampleRate, len(buffer.Samples))
				buffer = hybrid.BlendLowFreq(lowIR, buffer, provider.CrossoverHz(), cfg.SampleRate)
			}
		}
	}

	return append([]float64(nil), buffer.Samples...), nil
}

// RenderStereo is reserved for Phase 7 binaural output.
func (r Renderer) RenderStereo(sc *scene.Scene, cfg ir.RenderConfig) (left, right []float64, err error) {
	if sc == nil {
		return nil, nil, errors.New("scene is nil")
	}

	receiver, err := firstBinauralReceiver(sc)
	if err != nil {
		return nil, nil, err
	}

	var earlyEvents []ir.Event
	if r.Early != nil {
		earlyEvents, err = r.Early.Generate(sc, cfg)
		if err != nil {
			return nil, nil, err
		}
	}

	var lateEvents []ir.Event
	if r.Late != nil {
		lateEvents, err = r.Late.Generate(sc, cfg)
		if err != nil {
			return nil, nil, err
		}
	}

	combined := hybrid.Combine(earlyEvents, lateEvents, r.Hybrid)

	leftBuf, rightBuf, err := ir.RenderBinaural(combined, receiver.HRTF, cfg)
	if err != nil {
		return nil, nil, err
	}

	return append([]float64(nil), leftBuf.Samples...), append([]float64(nil), rightBuf.Samples...), nil
}

func firstBinauralReceiver(sc *scene.Scene) (scene.Receiver, error) {
	for _, receiver := range sc.Receivers {
		if receiver.Type == scene.ReceiverBinaural {
			if receiver.HRTF == nil {
				return scene.Receiver{}, errors.New("binaural receiver is missing an HRTF")
			}

			return receiver, nil
		}
	}

	return scene.Receiver{}, errors.New("scene does not contain a binaural receiver")
}
