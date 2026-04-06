package worker

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestProbeWithoutBinaryReturnsUnavailable(t *testing.T) {
	// Use a nonexistent path to trigger binary-not-found.
	err := Probe(context.Background(), "/nonexistent/algo-acoustics-gpu")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrGPUUnavailable) {
		t.Errorf("expected ErrGPUUnavailable, got: %v", err)
	}
}

func TestStartIfAvailableWithoutBinary(t *testing.T) {
	w, err := StartIfAvailable(context.Background(), "/nonexistent/algo-acoustics-gpu")
	if w != nil {
		t.Fatal("expected nil worker")
	}

	if !errors.Is(err, ErrGPUUnavailable) {
		t.Errorf("expected ErrGPUUnavailable, got: %v", err)
	}
}

func TestStartIfAvailableWithRealServer(t *testing.T) {
	bin := os.Getenv("ALGO_GPU_SERVER")
	if bin == "" {
		t.Skip("ALGO_GPU_SERVER not set")
	}

	w, err := StartIfAvailable(context.Background(), bin)
	if err != nil {
		t.Fatalf("StartIfAvailable: %v", err)
	}

	defer w.Close()

	// Should be able to ping.
	err = w.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestIsGPUError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"generic", errors.New("foo"), false},
		{"OOM", &ServerError{Code: StatusOOM, MsgType: MsgRunFDTD}, true},
		{"CUDA", &ServerError{Code: StatusCUDA, MsgType: MsgRunFDTD}, true},
		{"bad_handle", &ServerError{Code: StatusBadHandle, MsgType: MsgRunFDTD}, true},
		{"bad_msg", &ServerError{Code: StatusBadMsg, MsgType: MsgRunFDTD}, false},
		{"not_impl", &ServerError{Code: StatusNotImpl, MsgType: MsgRunFDTD}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGPUError(tt.err)
			if got != tt.want {
				t.Errorf("IsGPUError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestFindServerBinaryNotInPath(t *testing.T) {
	// Save PATH and set it to empty to ensure binary is not found.
	origPath := os.Getenv("PATH")

	t.Setenv("PATH", "/nonexistent")

	defer func() {
		t.Setenv("PATH", origPath)
	}()

	_, err := findServerBinary()
	if err == nil {
		t.Fatal("expected error when binary not in PATH")
	}
}
