package ir

import "math"

// Buffer holds a dense impulse response sample buffer.
type Buffer struct {
	SampleRate int       `json:"sampleRate"`
	Samples    []float64 `json:"samples"`
}

// Len returns the number of samples in the buffer.
func (b *Buffer) Len() int {
	if b == nil {
		return 0
	}

	return len(b.Samples)
}

// Duration returns the buffer duration in seconds.
func (b *Buffer) Duration() float64 {
	if b == nil || b.SampleRate <= 0 {
		return 0
	}

	return float64(len(b.Samples)) / float64(b.SampleRate)
}

// NewBuffer allocates a buffer long enough to hold durationSeconds at sampleRate.
func NewBuffer(sampleRate int, durationSeconds float64) *Buffer {
	length := 0
	if sampleRate > 0 && durationSeconds > 0 {
		length = int(math.Ceil(durationSeconds * float64(sampleRate)))
	}

	return &Buffer{
		SampleRate: sampleRate,
		Samples:    make([]float64, length),
	}
}
