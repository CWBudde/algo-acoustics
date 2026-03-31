package ir

import "testing"

func TestNewBuffer(t *testing.T) {
	buf := NewBuffer(8, 1.25)
	if got, want := buf.SampleRate, 8; got != want {
		t.Fatalf("SampleRate = %d, want %d", got, want)
	}
	if got, want := buf.Len(), 10; got != want {
		t.Fatalf("Len() = %d, want %d", got, want)
	}
	if got, want := buf.Duration(), 1.25; got != want {
		t.Fatalf("Duration() = %v, want %v", got, want)
	}
}

func TestBufferNilAndEmptyCases(t *testing.T) {
	var buf *Buffer
	if got := buf.Len(); got != 0 {
		t.Fatalf("nil Len() = %d, want 0", got)
	}
	if got := buf.Duration(); got != 0 {
		t.Fatalf("nil Duration() = %v, want 0", got)
	}

	zero := NewBuffer(0, 1)
	if got, want := zero.Len(), 0; got != want {
		t.Fatalf("zero-rate Len() = %d, want %d", got, want)
	}
	if got, want := zero.Duration(), 0.0; got != want {
		t.Fatalf("zero-rate Duration() = %v, want %v", got, want)
	}
}
