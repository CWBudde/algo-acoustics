package export

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/wav"
	"github.com/go-audio/audio"
)

const (
	pcm16Max = 32767
	pcm16Min = -32768
)

// WriteMonoWAV writes a mono 16-bit PCM WAV file from a dense IR buffer.
func WriteMonoWAV(path string, buf *ir.Buffer) error {
	if buf == nil {
		return errors.New("buffer must not be nil")
	}

	if buf.SampleRate <= 0 {
		return errors.New("buffer sample rate must be positive")
	}

	return writeWAV(path, buf.SampleRate, 1, monoData(buf.Samples))
}

// WriteStereoWAV writes a stereo 16-bit PCM WAV file from two dense IR buffers.
func WriteStereoWAV(path string, left, right *ir.Buffer) error {
	if left == nil || right == nil {
		return errors.New("left and right buffers must not be nil")
	}

	if left.SampleRate <= 0 || right.SampleRate <= 0 {
		return errors.New("buffer sample rates must be positive")
	}

	if left.SampleRate != right.SampleRate {
		return errors.New("buffer sample rates must match")
	}

	if len(left.Samples) != len(right.Samples) {
		return errors.New("buffer lengths must match")
	}

	data := make([]float32, 0, len(left.Samples)*2)
	for index := range left.Samples {
		data = append(data, float32(left.Samples[index]), float32(right.Samples[index]))
	}

	return writeWAV(path, left.SampleRate, 2, data)
}

// Float64ToInt16 converts normalized floating-point samples to signed 16-bit PCM values.
func Float64ToInt16(samples []float64) []int16 {
	if len(samples) == 0 {
		return nil
	}

	out := make([]int16, len(samples))
	for index, sample := range samples {
		if sample > 1 {
			sample = 1
		} else if sample < -1 {
			sample = -1
		}

		value := int64(math.Round(sample * 32768))
		if value > pcm16Max {
			value = pcm16Max
		} else if value < pcm16Min {
			value = pcm16Min
		}

		out[index] = int16(value)
	}

	return out
}

// EncodeMonoWAVBytes encodes a mono 16-bit PCM WAV into an in-memory byte slice.
func EncodeMonoWAVBytes(buf *ir.Buffer) ([]byte, error) {
	if buf == nil {
		return nil, errors.New("buffer must not be nil")
	}

	if buf.SampleRate <= 0 {
		return nil, errors.New("buffer sample rate must be positive")
	}

	var output memBuffer

	err := encodeWAV(&output, buf.SampleRate, 1, monoData(buf.Samples))
	if err != nil {
		return nil, err
	}

	return output.bytes(), nil
}

func writeWAV(path string, sampleRate int, numChans int, data []float32) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create wav file: %w", err)
	}

	defer func() {
		closeErr := file.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	return encodeWAV(file, sampleRate, numChans, data)
}

func encodeWAV(w io.WriteSeeker, sampleRate int, numChans int, data []float32) error {
	encoder := wav.NewEncoder(w, sampleRate, 16, numChans, 1)

	err := encoder.Write(&audio.Float32Buffer{
		Format: &audio.Format{
			NumChannels: numChans,
			SampleRate:  sampleRate,
		},
		Data: data,
	})
	if err != nil {
		return fmt.Errorf("write wav data: %w", err)
	}

	err = encoder.Close()
	if err != nil {
		return fmt.Errorf("close wav encoder: %w", err)
	}

	return nil
}

// memBuffer is an in-memory io.WriteSeeker for WAV encoding.
type memBuffer struct {
	data []byte
	pos  int64
}

func (m *memBuffer) Write(p []byte) (int, error) {
	end := m.pos + int64(len(p))
	if end > int64(len(m.data)) {
		grown := make([]byte, end)
		copy(grown, m.data)
		m.data = grown
	}

	copy(m.data[m.pos:end], p)
	m.pos = end

	return len(p), nil
}

func (m *memBuffer) Seek(offset int64, whence int) (int64, error) {
	var next int64

	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = m.pos + offset
	case io.SeekEnd:
		next = int64(len(m.data)) + offset
	default:
		return 0, fmt.Errorf("unsupported seek whence %d", whence)
	}

	if next < 0 {
		return 0, fmt.Errorf("negative seek position %d", next)
	}

	m.pos = next

	return next, nil
}

func (m *memBuffer) bytes() []byte {
	return append([]byte(nil), m.data...)
}

func monoData(samples []float64) []float32 {
	if len(samples) == 0 {
		return nil
	}

	out := make([]float32, len(samples))
	for index, sample := range samples {
		out[index] = float32(sample)
	}

	return out
}
