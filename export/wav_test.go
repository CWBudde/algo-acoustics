package export

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/wav"
)

func TestFloat64ToInt16(t *testing.T) {
	t.Parallel()

	got := Float64ToInt16([]float64{-1.2, -0.5, 0, 0.5, 1.2})

	want := []int16{-32768, -16384, 0, 16384, 32767}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}

	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("got[%d] = %d, want %d", index, got[index], want[index])
		}
	}
}

func TestWriteMonoWAVRoundTrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mono.wav")

	input := &ir.Buffer{SampleRate: 48000, Samples: make([]float64, 100)}
	for index := range input.Samples {
		input.Samples[index] = math.Sin(float64(index) * 0.05)
	}

	err := WriteMonoWAV(path, input)
	if err != nil {
		t.Fatalf("WriteMonoWAV() error = %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open wav: %v", err)
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)

	decoded, err := decoder.FullPCMBuffer()
	if err != nil {
		t.Fatalf("FullPCMBuffer() error = %v", err)
	}

	if got, want := int(decoder.SampleRate), input.SampleRate; got != want {
		t.Fatalf("SampleRate = %d, want %d", got, want)
	}

	if got, want := int(decoder.NumChans), 1; got != want {
		t.Fatalf("NumChans = %d, want %d", got, want)
	}

	if got, want := len(decoded.Data), len(input.Samples); got != want {
		t.Fatalf("decoded length = %d, want %d", got, want)
	}

	for index, sample := range decoded.Data {
		want := input.Samples[index]
		if math.Abs(float64(sample)-want) > 1.0/32768.0 {
			t.Fatalf("decoded[%d] = %v, want %v", index, sample, want)
		}
	}
}

func TestWriteStereoWAVRoundTrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "stereo.wav")
	left := &ir.Buffer{SampleRate: 44100, Samples: []float64{0, 0.25, 0.5, 0.75}}
	right := &ir.Buffer{SampleRate: 44100, Samples: []float64{1, 0.5, 0, -0.5}}

	err := WriteStereoWAV(path, left, right)
	if err != nil {
		t.Fatalf("WriteStereoWAV() error = %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open wav: %v", err)
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)

	decoded, err := decoder.FullPCMBuffer()
	if err != nil {
		t.Fatalf("FullPCMBuffer() error = %v", err)
	}

	if got, want := int(decoder.NumChans), 2; got != want {
		t.Fatalf("NumChans = %d, want %d", got, want)
	}

	if got, want := int(decoder.SampleRate), left.SampleRate; got != want {
		t.Fatalf("SampleRate = %d, want %d", got, want)
	}

	if got, want := len(decoded.Data), len(left.Samples)*2; got != want {
		t.Fatalf("decoded length = %d, want %d", got, want)
	}

	for index := range left.Samples {
		leftSample := float64(decoded.Data[index*2])

		rightSample := float64(decoded.Data[index*2+1])
		if math.Abs(leftSample-left.Samples[index]) > 1.0/32768.0 {
			t.Fatalf("left[%d] = %v, want %v", index, leftSample, left.Samples[index])
		}

		if math.Abs(rightSample-right.Samples[index]) > 1.0/32768.0 {
			t.Fatalf("right[%d] = %v, want %v", index, rightSample, right.Samples[index])
		}
	}
}

func TestEncodeStereoWAVBytes(t *testing.T) {
	t.Parallel()

	left := &ir.Buffer{SampleRate: 8000, Samples: []float64{0.5, -0.25}}
	right := &ir.Buffer{SampleRate: 8000, Samples: []float64{-0.5, 0.25}}

	encoded, err := EncodeStereoWAVBytes(left, right)
	if err != nil {
		t.Fatalf("EncodeStereoWAVBytes() error = %v", err)
	}

	if len(encoded) <= 44 {
		t.Fatalf("encoded length = %d, want WAV header and samples", len(encoded))
	}

	if got := string(encoded[:4]); got != "RIFF" {
		t.Fatalf("header = %q, want RIFF", got)
	}
}
