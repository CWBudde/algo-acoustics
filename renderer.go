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

// LateBufferEngine renders a ray- or energy-based late field without reducing
// it to sparse pressure events. Hybrid renders with an early engine use the
// explicit time-based crossover in HybridConfig.
type LateBufferEngine interface {
	RenderMono(sc *scene.Scene, cfg ir.RenderConfig) (*ir.Buffer, error)
}

// BinauralLateBufferEngine is a LateBufferEngine that preserves directional
// late-field information for HRTF rendering.
type BinauralLateBufferEngine interface {
	LateBufferEngine
	RenderBinaural(sc *scene.Scene, receiver scene.Receiver, cfg ir.RenderConfig) (left, right *ir.Buffer, err error)
}

// LowFreqEngine generates a transfer function for low-frequency rendering.
type LowFreqEngine interface {
	Transfer(sc *scene.Scene, cfg ir.RenderConfig) (*TransferFunction, error)
}

// LowFreqCrossoverProvider optionally overrides the default 200 Hz crossover
// used when blending a LowFreqEngine into a mono render.
type LowFreqCrossoverProvider interface {
	CrossoverHz() float64
}

const defaultLowFreqCrossoverHz = 200.0

// Renderer orchestrates early, late, and low-frequency engines.
type Renderer struct {
	Early EventEngine
	// Late accepts sparse pressure-event engines for compatibility. Configure
	// either Late or LateBuffer, never both.
	Late EventEngine
	// LateBuffer is the canonical path for energy-histogram late fields such as
	// RaytraceEngine. Configure either LateBuffer or Late, never both.
	LateBuffer LateBufferEngine
	LowFreq    LowFreqEngine
	Hybrid     hybrid.HybridConfig
}

// RenderMono renders a scene into a mono buffer using the configured engines.
func (r Renderer) RenderMono(sc *scene.Scene, cfg ir.RenderConfig) ([]float64, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	earlyEvents, lateEvents, err := r.generateEvents(sc, cfg)
	if err != nil {
		return nil, err
	}

	combined := hybrid.Combine(earlyEvents, lateEvents, r.Hybrid)

	buffer, err := ir.RenderMono(combined, cfg)
	if err != nil {
		return nil, fmt.Errorf("render mono buffer: %w", err)
	}

	if r.LateBuffer != nil {
		lateBuffer, renderErr := r.LateBuffer.RenderMono(sc, cfg)
		if renderErr != nil {
			return nil, fmt.Errorf("render late buffer: %w", renderErr)
		}

		buffer, err = combineLateBuffer(buffer, lateBuffer, earlyEvents, r.Hybrid, r.Early != nil)
		if err != nil {
			return nil, err
		}
	}

	buffer, err = r.applyLowFreq(sc, cfg, buffer)
	if err != nil {
		return nil, err
	}

	return append([]float64(nil), buffer.Samples...), nil
}

// RenderStereo renders binaural output using the first binaural receiver.
// LowFreq is intentionally not applied: LowFreqEngine produces one monaural
// transfer function, which cannot preserve ear-specific HRTF information.
func (r Renderer) RenderStereo(sc *scene.Scene, cfg ir.RenderConfig) (left, right []float64, err error) {
	if sc == nil {
		return nil, nil, errors.New("scene is nil")
	}

	receiver, err := firstBinauralReceiver(sc)
	if err != nil {
		return nil, nil, err
	}

	earlyEvents, lateEvents, err := r.generateEvents(sc, cfg)
	if err != nil {
		return nil, nil, err
	}

	combined := hybrid.Combine(earlyEvents, lateEvents, r.Hybrid)
	headEvents := make([]ir.Event, len(combined))
	copy(headEvents, combined)

	for index := range headEvents {
		headEvents[index].Direction = receiver.WorldToHeadDir(headEvents[index].Direction)
	}

	leftBuf, rightBuf, err := ir.RenderBinaural(headEvents, receiver.HRTF, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("render binaural buffer: %w", err)
	}

	if r.LateBuffer != nil {
		binauralEngine, ok := r.LateBuffer.(BinauralLateBufferEngine)
		if !ok {
			return nil, nil, errors.New("configured late buffer engine does not support binaural rendering")
		}

		lateLeft, lateRight, renderErr := binauralEngine.RenderBinaural(sc, receiver, cfg)
		if renderErr != nil {
			return nil, nil, fmt.Errorf("render binaural late buffer: %w", renderErr)
		}

		leftBuf, err = combineLateBuffer(leftBuf, lateLeft, earlyEvents, r.Hybrid, r.Early != nil)
		if err != nil {
			return nil, nil, fmt.Errorf("combine left late buffer: %w", err)
		}

		rightBuf, err = combineLateBuffer(rightBuf, lateRight, earlyEvents, r.Hybrid, r.Early != nil)
		if err != nil {
			return nil, nil, fmt.Errorf("combine right late buffer: %w", err)
		}
	}

	return append([]float64(nil), leftBuf.Samples...), append([]float64(nil), rightBuf.Samples...), nil
}

func (r Renderer) generateEvents(sc *scene.Scene, cfg ir.RenderConfig) (early, late []ir.Event, err error) {
	if r.Late != nil && r.LateBuffer != nil {
		return nil, nil, errors.New("configure either sparse Late or dense LateBuffer, not both")
	}

	if r.Early != nil {
		early, err = r.Early.Generate(sc, cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("generate early events: %w", err)
		}
	}

	if r.Late != nil {
		late, err = r.Late.Generate(sc, cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("generate late events: %w", err)
		}
	}

	return early, late, nil
}

func combineLateBuffer(
	early, late *ir.Buffer,
	earlyEvents []ir.Event,
	cfg hybrid.HybridConfig,
	hasEarlyEngine bool,
) (*ir.Buffer, error) {
	if late == nil {
		return nil, errors.New("late buffer engine returned nil")
	}

	if !hasEarlyEngine {
		return late, nil
	}

	if cfg.CrossoverMode != hybrid.TimeBased {
		return nil, errors.New("dense late buffers require a time-based crossover")
	}

	aligned := hybrid.AlignLateTail(late, earlyEvents, cfg)

	combined := hybrid.CombineBuffers(early, aligned, cfg)
	if combined == nil {
		return nil, errors.New("combine hybrid buffers returned nil")
	}

	return combined, nil
}

func (r Renderer) applyLowFreq(sc *scene.Scene, cfg ir.RenderConfig, buffer *ir.Buffer) (*ir.Buffer, error) {
	if r.LowFreq == nil {
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
	crossoverHz := defaultLowFreqCrossoverHz

	if provider, ok := r.LowFreq.(LowFreqCrossoverProvider); ok {
		if providedHz := provider.CrossoverHz(); providedHz > 0 {
			crossoverHz = providedHz
		}
	}

	return hybrid.BlendLowFreq(lowIR, buffer, crossoverHz, cfg.SampleRate), nil
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
