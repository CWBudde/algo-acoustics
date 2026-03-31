package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/wav"
	"github.com/go-audio/audio"
)

type wasmResult struct {
	FrontEnergyRatio float64
	RearEnergyRatio  float64
	WAVBytes         []byte
}

func runWASM(gllBytes []byte) (wasmResult, error) {
	if len(gllBytes) == 0 {
		return wasmResult{}, errors.New("gll bytes must not be empty")
	}

	model, err := directivity.LoadGLLReader(bytes.NewReader(gllBytes), "")
	if err != nil {
		return wasmResult{}, fmt.Errorf("load gll bytes: %w", err)
	}

	result, err := evaluateModel(exampleDirectivityModel{base: model})
	if err != nil {
		return wasmResult{}, err
	}
	if err := validateComparisons(result); err != nil {
		return wasmResult{}, err
	}

	wavBytes, err := encodeMonoWAVBytes(result.OutputBuffer)
	if err != nil {
		return wasmResult{}, err
	}

	return wasmResult{
		FrontEnergyRatio: result.FrontComparison.GLL / result.FrontComparison.Omni,
		RearEnergyRatio:  result.RearComparison.GLL / result.RearComparison.Omni,
		WAVBytes:         wavBytes,
	}, nil
}

func encodeMonoWAVBytes(buf *ir.Buffer) ([]byte, error) {
	if buf == nil {
		return nil, errors.New("buffer must not be nil")
	}
	if buf.SampleRate <= 0 {
		return nil, errors.New("buffer sample rate must be positive")
	}

	var output memoryWAVWriter
	encoder := wav.NewEncoder(&output, buf.SampleRate, 16, 1, 1)
	samples := make([]float32, len(buf.Samples))
	for index, sample := range buf.Samples {
		samples[index] = float32(sample)
	}
	if err := encoder.Write(&audio.Float32Buffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  buf.SampleRate,
		},
		Data: samples,
	}); err != nil {
		return nil, fmt.Errorf("write wav data: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close wav encoder: %w", err)
	}

	return output.Bytes(), nil
}

type memoryWAVWriter struct {
	data []byte
	pos  int64
}

func (w *memoryWAVWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	end := w.pos + int64(len(p))
	if end > int64(len(w.data)) {
		grown := make([]byte, end)
		copy(grown, w.data)
		w.data = grown
	}

	copy(w.data[w.pos:end], p)
	w.pos = end
	return len(p), nil
}

func (w *memoryWAVWriter) Seek(offset int64, whence int) (int64, error) {
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = w.pos + offset
	case io.SeekEnd:
		next = int64(len(w.data)) + offset
	default:
		return 0, fmt.Errorf("unsupported seek whence %d", whence)
	}

	if next < 0 {
		return 0, fmt.Errorf("negative seek position %d", next)
	}

	w.pos = next
	return next, nil
}

func (w *memoryWAVWriter) Bytes() []byte {
	return append([]byte(nil), w.data...)
}
