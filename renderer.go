package algoacoustics

import (
	"errors"
	"fmt"

	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/pde"
	"github.com/cwbudde/algo-acoustics/scene"
)

// TransferFunction carries a low-frequency transfer function.
type TransferFunction = pde.TransferFunction

// EventEngine generates a sparse event stream for a scene.
type EventEngine interface {
	Generate(sc *scene.Scene, cfg ir.RenderConfig) ([]ir.Event, error)
}

// LowFreqEngine generates a transfer function for low-frequency rendering.
type LowFreqEngine interface {
	Transfer(sc *scene.Scene, cfg ir.RenderConfig) (*TransferFunction, error)
}

// Renderer orchestrates early, late, and future low-frequency engines.
type Renderer struct {
	Early   EventEngine
	Late    EventEngine
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
			return nil, fmt.Errorf("generate early events: %w", err)
		}
	}

	if r.Late != nil {
		lateEvents, err = r.Late.Generate(sc, cfg)
		if err != nil {
			return nil, fmt.Errorf("generate late events: %w", err)
		}
	}

	combined := hybrid.Combine(earlyEvents, lateEvents, r.Hybrid)

	buffer, err := ir.RenderMono(combined, cfg)
	if err != nil {
		return nil, fmt.Errorf("render mono buffer: %w", err)
	}

	buffer, err = r.applyLowFreq(sc, cfg, buffer)
	if err != nil {
		return nil, err
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
			return nil, nil, fmt.Errorf("generate early events: %w", err)
		}
	}

	var lateEvents []ir.Event
	if r.Late != nil {
		lateEvents, err = r.Late.Generate(sc, cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("generate late events: %w", err)
		}
	}

	combined := hybrid.Combine(earlyEvents, lateEvents, r.Hybrid)

	leftBuf, rightBuf, err := ir.RenderBinaural(combined, receiver.HRTF, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("render binaural buffer: %w", err)
	}

	return append([]float64(nil), leftBuf.Samples...), append([]float64(nil), rightBuf.Samples...), nil
}

func (r Renderer) applyLowFreq(sc *scene.Scene, cfg ir.RenderConfig, buffer *ir.Buffer) (*ir.Buffer, error) {
	if r.LowFreq == nil {
		return buffer, nil
	}

	provider, ok := r.LowFreq.(interface{ CrossoverHz() float64 })
	if !ok {
		return buffer, nil
	}

	transfer, err := r.LowFreq.Transfer(sc, cfg)
	if err != nil {
		return nil, fmt.Errorf("generate low-frequency transfer: %w", err)
	}

	if transfer == nil {
		return buffer, nil
	}

	lowIR := transfer.ToTimeDomain(cfg.SampleRate, len(buffer.Samples))

	return hybrid.BlendLowFreq(lowIR, buffer, provider.CrossoverHz(), cfg.SampleRate), nil
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
