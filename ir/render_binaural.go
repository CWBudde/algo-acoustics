package ir

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	algofft "github.com/cwbudde/algo-fft"
)

type binauralBandedEvent struct {
	event        Event
	sampleIndex  int
	delaySeconds float64
	leftHRIR     []float64
	rightHRIR    []float64
}

type binauralBandWorkspace struct {
	plan            *algofft.Plan[complex128]
	weights         [][]float64
	leftCombined    []complex128
	rightCombined   []complex128
	leftTime        []complex128
	rightTime       []complex128
	leftSpectrum    []complex128
	rightSpectrum   []complex128
	leftCorrection  []float64
	rightCorrection []float64
}

// RenderBinaural renders sparse events into dense left/right IR buffers.
func RenderBinaural(events []Event, hrtfDataset hrtf.Dataset, cfg RenderConfig) (left, right *Buffer, err error) {
	if hrtfDataset == nil {
		return nil, nil, errors.New("hrtf dataset must not be nil")
	}

	err = validateRenderConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	if sampleRate := hrtfDataset.SampleRate(); sampleRate > 0 && sampleRate != cfg.SampleRate {
		return nil, nil, fmt.Errorf("hrtf sample rate %d does not match render sample rate %d", sampleRate, cfg.SampleRate)
	}

	left = NewBuffer(cfg.SampleRate, cfg.DurationSeconds)
	right = NewBuffer(cfg.SampleRate, cfg.DurationSeconds)

	bandedEvents, err := collectBinauralEvents(events, hrtfDataset, cfg, left.Samples, right.Samples)
	if err != nil {
		return nil, nil, err
	}

	if len(bandedEvents) > 0 {
		err = renderBinauralBandedInto(left.Samples, right.Samples, bandedEvents, cfg)
		if err != nil {
			return nil, nil, err
		}
	}

	return left, right, nil
}

func collectBinauralEvents(events []Event, dataset hrtf.Dataset, cfg RenderConfig, left, right []float64) ([]binauralBandedEvent, error) {
	bandedEvents := make([]binauralBandedEvent, 0, len(events))

	for index, event := range events {
		bandedEvent, isBanded, err := prepareBinauralEvent(index, event, dataset, cfg, left, right)
		if err != nil {
			return nil, err
		}

		if isBanded {
			bandedEvents = append(bandedEvents, bandedEvent)
		}
	}

	return bandedEvents, nil
}

func prepareBinauralEvent(index int, event Event, dataset hrtf.Dataset, cfg RenderConfig, left, right []float64) (binauralBandedEvent, bool, error) {
	if event.TimeSeconds < 0 {
		return binauralBandedEvent{}, false, fmt.Errorf("event %d has negative time %g", index, event.TimeSeconds)
	}

	if event.TimeSeconds >= cfg.DurationSeconds {
		return binauralBandedEvent{}, false, nil
	}

	leftHRIR, rightHRIR, delaySeconds, err := dataset.Lookup(eventDirectionForHRTF(event.Direction))
	if err != nil {
		return binauralBandedEvent{}, false, fmt.Errorf("event %d hrtf lookup: %w", index, err)
	}

	bandCount := cfg.BandSpec.BandCount()
	sampleIndex := int(math.Round(event.TimeSeconds * float64(cfg.SampleRate)))

	if bandCount > 0 && len(event.BandGain) > 0 {
		if len(event.BandGain) != bandCount {
			return binauralBandedEvent{}, false, fmt.Errorf("event %d: band gain length %d does not match band count %d",
				index, len(event.BandGain), bandCount)
		}

		if sampleIndex < 0 || sampleIndex >= len(left) {
			return binauralBandedEvent{}, false, nil
		}

		return binauralBandedEvent{
			event:        event,
			sampleIndex:  sampleIndex,
			delaySeconds: delaySeconds,
			leftHRIR:     leftHRIR,
			rightHRIR:    rightHRIR,
		}, true, nil
	}

	if sampleIndex < 0 || sampleIndex >= len(left) {
		return binauralBandedEvent{}, false, nil
	}

	gain, err := monoEventGain(event, bandCount)
	if err != nil {
		return binauralBandedEvent{}, false, fmt.Errorf("event %d: %w", index, err)
	}

	convolveImpulseInto(left, sampleIndex, gain, delaySeconds, cfg.SampleRate, leftHRIR)
	convolveImpulseInto(right, sampleIndex, gain, delaySeconds, cfg.SampleRate, rightHRIR)

	return binauralBandedEvent{}, false, nil
}

func renderBinauralBandedInto(left, right []float64, events []binauralBandedEvent, cfg RenderConfig) error {
	fftSize := nextPow2(2 * len(left))

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return fmt.Errorf("create FFT plan: %w", err)
	}

	workspace := binauralBandWorkspace{
		plan:            plan,
		weights:         buildBandpassWeights(cfg.BandSpec, fftSize, cfg.SampleRate),
		leftCombined:    make([]complex128, fftSize),
		rightCombined:   make([]complex128, fftSize),
		leftTime:        make([]complex128, fftSize),
		rightTime:       make([]complex128, fftSize),
		leftSpectrum:    make([]complex128, fftSize),
		rightSpectrum:   make([]complex128, fftSize),
		leftCorrection:  make([]float64, len(left)),
		rightCorrection: make([]float64, len(right)),
	}

	for band := range cfg.BandSpec.BandCount() {
		err = renderBinauralBand(&workspace, events, band, cfg.SampleRate, len(left))
		if err != nil {
			return err
		}
	}

	err = plan.Inverse(workspace.leftTime, workspace.leftCombined)
	if err != nil {
		return fmt.Errorf("IFFT left: %w", err)
	}

	err = plan.Inverse(workspace.rightTime, workspace.rightCombined)
	if err != nil {
		return fmt.Errorf("IFFT right: %w", err)
	}

	for i := range left {
		left[i] += real(workspace.leftTime[i]) + workspace.leftCorrection[i]
		right[i] += real(workspace.rightTime[i]) + workspace.rightCorrection[i]
	}

	return nil
}

func renderBinauralBand(workspace *binauralBandWorkspace, events []binauralBandedEvent, band, sampleRate, outputLen int) error {
	clear(workspace.leftTime)
	clear(workspace.rightTime)

	for _, event := range events {
		gain := event.event.Amplitude * event.event.BandGain[band] * math.Cos(event.event.PhaseRadians)
		accumulateBinauralBandInput(workspace.leftTime, event.sampleIndex, gain, event.delaySeconds, sampleRate, outputLen, event.leftHRIR)
		accumulateBinauralBandInput(workspace.rightTime, event.sampleIndex, gain, event.delaySeconds, sampleRate, outputLen, event.rightHRIR)
	}

	err := workspace.plan.Forward(workspace.leftSpectrum, workspace.leftTime)
	if err != nil {
		return fmt.Errorf("FFT left band %d: %w", band, err)
	}

	err = workspace.plan.Forward(workspace.rightSpectrum, workspace.rightTime)
	if err != nil {
		return fmt.Errorf("FFT right band %d: %w", band, err)
	}

	for k, weight := range workspace.weights[band] {
		workspace.leftCombined[k] += workspace.leftSpectrum[k] * complex(weight, 0)
		workspace.rightCombined[k] += workspace.rightSpectrum[k] * complex(weight, 0)
		workspace.leftSpectrum[k] = complex(weight, 0)
	}

	// The inverse band weights are the zero-phase impulse response used by
	// renderMonoBanded. They let us correct only the render-window edges
	// affected by commuting the short HRIR convolution ahead of filtering.
	err = workspace.plan.Inverse(workspace.leftTime, workspace.leftSpectrum)
	if err != nil {
		return fmt.Errorf("IFFT band %d kernel: %w", band, err)
	}

	for _, event := range events {
		gain := event.event.Amplitude * event.event.BandGain[band] * math.Cos(event.event.PhaseRadians)
		correctBinauralBandEdges(workspace.leftCorrection, workspace.leftTime, event.sampleIndex, gain, event.delaySeconds, sampleRate, event.leftHRIR)
		correctBinauralBandEdges(workspace.rightCorrection, workspace.leftTime, event.sampleIndex, gain, event.delaySeconds, sampleRate, event.rightHRIR)
	}

	return nil
}

func accumulateBinauralBandInput(dst []complex128, sampleIndex int, gain, delaySeconds float64, sampleRate, outputLen int, hrir []float64) {
	hrir = nonEmptyHRIR(hrir)

	delaySamples, ok := boundedDelaySamples(delaySeconds, sampleRate, outputLen, outputLen, len(hrir))
	if !ok {
		return
	}

	for hrirIndex, coefficient := range hrir {
		shift := delaySamples + hrirIndex
		if coefficient == 0 || shift <= -outputLen || shift >= outputLen || useDirectBandEdge(shift, outputLen) {
			continue
		}

		index := sampleIndex + shift
		if index < 0 {
			index += len(dst)
		}

		dst[index] += complex(gain*coefficient, 0)
	}
}

func correctBinauralBandEdges(dst []float64, kernel []complex128, sampleIndex int, gain, delaySeconds float64, sampleRate int, hrir []float64) {
	hrir = nonEmptyHRIR(hrir)

	delaySamples, ok := boundedDelaySamples(delaySeconds, sampleRate, len(dst), len(dst), len(hrir))
	if !ok {
		return
	}

	for hrirIndex, coefficient := range hrir {
		shift := delaySamples + hrirIndex
		if coefficient == 0 || shift <= -len(dst) || shift >= len(dst) {
			continue
		}

		start, end, sign := bandEdgeCorrectionRange(shift, len(dst))
		for outputIndex := start; outputIndex < end; outputIndex++ {
			lag := outputIndex - shift - sampleIndex
			kernelIndex := lag

			if kernelIndex < 0 {
				kernelIndex += len(kernel)
			}

			dst[outputIndex] += sign * gain * coefficient * real(kernel[kernelIndex])
		}
	}
}

func bandEdgeCorrectionRange(shift, outputLen int) (start, end int, sign float64) {
	if useDirectBandEdge(shift, outputLen) {
		if shift >= 0 {
			return shift, outputLen, 1
		}

		return 0, outputLen + shift, 1
	}

	if shift >= 0 {
		return 0, shift, -1
	}

	return outputLen + shift, outputLen, -1
}

func useDirectBandEdge(shift, outputLen int) bool {
	if shift < 0 {
		shift = -shift
	}

	return shift > outputLen/2
}

func eventDirectionForHRTF(dir geometry.Vec3) geometry.Vec3 {
	if dir == geometry.Vec3Zero {
		return geometry.Vec3{X: 1}
	}

	return dir.Normalize()
}

func convolveImpulseInto(samples []float64, sampleIndex int, amplitude, delaySeconds float64, sampleRate int, hrir []float64) {
	hrir = nonEmptyHRIR(hrir)

	delaySamples, ok := boundedDelaySamples(delaySeconds, sampleRate, len(samples), len(samples), len(hrir))
	if !ok {
		return
	}

	for hrirIndex, coefficient := range hrir {
		outputIndex := sampleIndex + delaySamples + hrirIndex
		if outputIndex >= 0 && outputIndex < len(samples) {
			samples[outputIndex] += amplitude * coefficient
		}
	}
}

func boundedDelaySamples(delaySeconds float64, sampleRate, outputLen, signalLen, hrirLen int) (int, bool) {
	maxDelay := float64(outputLen+signalLen+hrirLen) / float64(sampleRate)
	if math.IsNaN(delaySeconds) || delaySeconds >= maxDelay || delaySeconds <= -maxDelay {
		return 0, false
	}

	return int(math.Round(delaySeconds * float64(sampleRate))), true
}

func nonEmptyHRIR(hrir []float64) []float64 {
	if len(hrir) == 0 {
		return []float64{1}
	}

	return hrir
}
