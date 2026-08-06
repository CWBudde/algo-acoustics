package ir

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
)

const binauralSpectralSampleRate = 8000

type directionalDataset struct{}

func (directionalDataset) SampleRate() int { return 100 }

func (directionalDataset) Lookup(dir geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	if dir.X >= 0 {
		return []float64{1}, []float64{0.5}, 0, nil
	}

	return []float64{0.5}, []float64{1}, 0, nil
}

type longHRIRDataset struct {
	lookups int
}

func (*longHRIRDataset) SampleRate() int { return 100 }

func (d *longHRIRDataset) Lookup(_ geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	d.lookups++

	return make([]float64, 32), make([]float64, 32), 0, nil
}

type batchedBinauralDataset struct{}

func (batchedBinauralDataset) SampleRate() int { return binauralSpectralSampleRate }

func (batchedBinauralDataset) Lookup(dir geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	if dir.X >= 0 {
		return []float64{0.8, -0.2, 0.1}, []float64{0.2, 0.4}, 3.0 / binauralSpectralSampleRate, nil
	}

	return []float64{-0.1, 0.3}, []float64{0.7, 0.1, -0.2}, -2.0 / binauralSpectralSampleRate, nil
}

type allocationBinauralDataset struct {
	left  []float64
	right []float64
}

func (allocationBinauralDataset) SampleRate() int { return binauralSpectralSampleRate }

func (d allocationBinauralDataset) Lookup(_ geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	return d.left, d.right, 0, nil
}

func TestRenderBinauralWithNoopDatasetMatchesMono(t *testing.T) {
	t.Parallel()

	events := []Event{{TimeSeconds: 0.01, Amplitude: 0.5, Direction: geometry.Vec3{X: 1}}, {TimeSeconds: 0.02, Amplitude: 0.25, Direction: geometry.Vec3{Y: 1}}}

	mono, err := RenderMono(events, RenderConfig{SampleRate: 100, DurationSeconds: 0.1})
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	left, right, err := RenderBinaural(events, hrtf.NoopDataset{SampleRateHz: 100}, RenderConfig{SampleRate: 100, DurationSeconds: 0.1})
	if err != nil {
		t.Fatalf("RenderBinaural() error = %v", err)
	}

	if left.Len() != mono.Len() || right.Len() != mono.Len() {
		t.Fatalf("stereo lengths = %d/%d, want %d", left.Len(), right.Len(), mono.Len())
	}

	for i := range mono.Samples {
		if math.Abs(left.Samples[i]-mono.Samples[i]) > 1e-12 {
			t.Fatalf("left[%d] = %v, want %v", i, left.Samples[i], mono.Samples[i])
		}

		if math.Abs(right.Samples[i]-mono.Samples[i]) > 1e-12 {
			t.Fatalf("right[%d] = %v, want %v", i, right.Samples[i], mono.Samples[i])
		}
	}
}

func TestRenderBinauralUsesDirectionalHRTF(t *testing.T) {
	t.Parallel()

	left, right, err := RenderBinaural([]Event{{TimeSeconds: 0.01, Amplitude: 1, Direction: geometry.Vec3{X: 1}}}, directionalDataset{}, RenderConfig{SampleRate: 100, DurationSeconds: 0.1, BandSpec: acoustics.BandSpec{CenterFreqs: []float64{125}, LowerEdges: []float64{88}, UpperEdges: []float64{177}}})
	if err != nil {
		t.Fatalf("RenderBinaural() error = %v", err)
	}

	if math.Abs(left.Samples[1]-1) > 1e-12 {
		t.Fatalf("left sample = %v, want 1", left.Samples[1])
	}

	if math.Abs(right.Samples[1]-0.5) > 1e-12 {
		t.Fatalf("right sample = %v, want 0.5", right.Samples[1])
	}
}

func TestRenderBinauralPreservesBandSpectrum(t *testing.T) {
	t.Parallel()

	const duration = 0.128

	spec := acoustics.BandSpec{
		CenterFreqs: []float64{125, 2000},
		LowerEdges:  []float64{88, 1000},
		UpperEdges:  []float64{500, 2828},
	}
	cfg := RenderConfig{SampleRate: binauralSpectralSampleRate, DurationSeconds: duration, BandSpec: spec}
	event := Event{TimeSeconds: 0.02, Amplitude: 1, BandGain: []float64{1, 0}}

	mono, err := RenderMono([]Event{event}, cfg)
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	left, right, err := RenderBinaural([]Event{event}, hrtf.NoopDataset{SampleRateHz: binauralSpectralSampleRate}, cfg)
	if err != nil {
		t.Fatalf("RenderBinaural() error = %v", err)
	}

	for index := range mono.Samples {
		if math.Abs(left.Samples[index]-mono.Samples[index]) > 1e-10 ||
			math.Abs(right.Samples[index]-mono.Samples[index]) > 1e-10 {
			t.Fatalf("sample %d: stereo = %g/%g, mono = %g", index, left.Samples[index], right.Samples[index], mono.Samples[index])
		}
	}

	highBand, _, err := RenderBinaural([]Event{{
		TimeSeconds: 0.02,
		Amplitude:   1,
		BandGain:    []float64{0, 1},
	}}, hrtf.NoopDataset{SampleRateHz: binauralSpectralSampleRate}, cfg)
	if err != nil {
		t.Fatalf("RenderBinaural() high-band error = %v", err)
	}

	if spectralMagnitude(left.Samples, 125) <= spectralMagnitude(highBand.Samples, 125) {
		t.Fatal("low-band event did not retain more 125 Hz energy than high-band event")
	}

	if spectralMagnitude(highBand.Samples, 2000) <= spectralMagnitude(left.Samples, 2000) {
		t.Fatal("high-band event did not retain more 2000 Hz energy than low-band event")
	}
}

func TestRenderBinauralHonorsConfiguredDuration(t *testing.T) {
	t.Parallel()

	dataset := &longHRIRDataset{}
	events := []Event{
		{TimeSeconds: 0.09, Amplitude: 1, Direction: geometry.Vec3{X: 1}},
		{TimeSeconds: 1e6, Amplitude: 1, Direction: geometry.Vec3{X: 1}},
	}

	left, right, err := RenderBinaural(events, dataset, RenderConfig{SampleRate: 100, DurationSeconds: 0.1})
	if err != nil {
		t.Fatalf("RenderBinaural() error = %v", err)
	}

	if left.Len() != 10 || right.Len() != 10 {
		t.Fatalf("stereo lengths = %d/%d, want configured length 10", left.Len(), right.Len())
	}

	if dataset.lookups != 1 {
		t.Fatalf("HRTF lookup count = %d, want 1 (far-future event skipped)", dataset.lookups)
	}
}

func TestRenderBinauralBatchedMatchesPerEventReference(t *testing.T) {
	t.Parallel()

	const duration = 0.032

	cfg := RenderConfig{
		SampleRate:      binauralSpectralSampleRate,
		DurationSeconds: duration,
		BandSpec: acoustics.BandSpec{
			CenterFreqs: []float64{250, 2000},
			LowerEdges:  []float64{177, 1000},
			UpperEdges:  []float64{700, 2828},
		},
	}
	events := []Event{
		{TimeSeconds: 1.0 / binauralSpectralSampleRate, Amplitude: 0.7, PhaseRadians: 0.2, Direction: geometry.Vec3{X: 1}, BandGain: []float64{1, 0.2}},
		{TimeSeconds: 0.012, Amplitude: -0.4, PhaseRadians: -0.3, Direction: geometry.Vec3{X: -1}, BandGain: []float64{0.1, 0.9}},
		{TimeSeconds: duration - 2.0/binauralSpectralSampleRate, Amplitude: 0.3, Direction: geometry.Vec3{X: 1}, BandGain: []float64{0.6, -0.2}},
		{TimeSeconds: 0.02, Amplitude: 0.25, Direction: geometry.Vec3{Y: 1}},
	}
	dataset := batchedBinauralDataset{}

	wantLeft, wantRight, err := renderBinauralPerEventReference(events, dataset, cfg)
	if err != nil {
		t.Fatalf("per-event reference error = %v", err)
	}

	gotLeft, gotRight, err := RenderBinaural(events, dataset, cfg)
	if err != nil {
		t.Fatalf("RenderBinaural() error = %v", err)
	}

	for i := range gotLeft.Samples {
		if math.Abs(gotLeft.Samples[i]-wantLeft.Samples[i]) > 1e-10 {
			t.Fatalf("left[%d] = %g, per-event reference %g", i, gotLeft.Samples[i], wantLeft.Samples[i])
		}

		if math.Abs(gotRight.Samples[i]-wantRight.Samples[i]) > 1e-10 {
			t.Fatalf("right[%d] = %g, per-event reference %g", i, gotRight.Samples[i], wantRight.Samples[i])
		}
	}
}

func TestRenderBinauralBandedAllocationsDoNotScalePerEvent(t *testing.T) {
	spec := acoustics.BandSpec{
		CenterFreqs: []float64{250, 2000},
		LowerEdges:  []float64{177, 1000},
		UpperEdges:  []float64{700, 2828},
	}
	cfg := RenderConfig{SampleRate: binauralSpectralSampleRate, DurationSeconds: 0.032, BandSpec: spec}
	dataset := allocationBinauralDataset{left: []float64{0.8, 0.2}, right: []float64{0.3, 0.7}}

	events := make([]Event, 64)
	for i := range events {
		events[i] = Event{
			TimeSeconds: float64(i+1) / binauralSpectralSampleRate,
			Amplitude:   1,
			Direction:   geometry.Vec3{X: 1},
			BandGain:    []float64{1, 0.5},
		}
	}

	allocations := func(input []Event) float64 {
		return testing.AllocsPerRun(5, func() {
			_, _, err := RenderBinaural(input, dataset, cfg)
			if err != nil {
				panic(err)
			}
		})
	}

	oneEvent := allocations(events[:1])
	manyEvents := allocations(events)

	if manyEvents > oneEvent+2 {
		t.Fatalf("allocations grew with event count: one=%g, 64=%g", oneEvent, manyEvents)
	}
}

func renderBinauralPerEventReference(events []Event, dataset hrtf.Dataset, cfg RenderConfig) (left, right *Buffer, err error) {
	left = NewBuffer(cfg.SampleRate, cfg.DurationSeconds)
	right = NewBuffer(cfg.SampleRate, cfg.DurationSeconds)

	for _, event := range events {
		leftHRIR, rightHRIR, delaySeconds, lookupErr := dataset.Lookup(eventDirectionForHRTF(event.Direction))
		if lookupErr != nil {
			return nil, nil, lookupErr
		}

		excitation, renderErr := RenderMono([]Event{event}, cfg)
		if renderErr != nil {
			return nil, nil, renderErr
		}

		convolveReferenceSignalInto(left.Samples, excitation.Samples, delaySeconds, cfg.SampleRate, leftHRIR)
		convolveReferenceSignalInto(right.Samples, excitation.Samples, delaySeconds, cfg.SampleRate, rightHRIR)
	}

	return left, right, nil
}

func convolveReferenceSignalInto(samples, signal []float64, delaySeconds float64, sampleRate int, hrir []float64) {
	if len(hrir) == 0 {
		hrir = []float64{1}
	}

	delaySamples := int(math.Round(delaySeconds * float64(sampleRate)))

	for signalIndex, amplitude := range signal {
		for hrirIndex, coefficient := range hrir {
			outputIndex := signalIndex + delaySamples + hrirIndex
			if outputIndex >= 0 && outputIndex < len(samples) {
				samples[outputIndex] += amplitude * coefficient
			}
		}
	}
}

func spectralMagnitude(samples []float64, frequency float64) float64 {
	var realPart float64
	var imaginaryPart float64

	for index, sample := range samples {
		angle := -2 * math.Pi * frequency * float64(index) / binauralSpectralSampleRate
		realPart += sample * math.Cos(angle)
		imaginaryPart += sample * math.Sin(angle)
	}

	return math.Hypot(realPart, imaginaryPart)
}
