package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// ErrGPUUnavailable is returned when no GPU server binary is found or
// the GPU server cannot start (e.g. no CUDA device present).
var ErrGPUUnavailable = errors.New("GPU acceleration unavailable")

// Probe checks whether GPU acceleration is available by locating the
// server binary and starting a short-lived connection.  It returns nil
// if the GPU is usable, or a descriptive error otherwise.
//
// serverBin may be empty, in which case Probe searches for the binary
// in well-known locations (see findServerBinary).
func Probe(ctx context.Context, serverBin string) error {
	bin, err := resolveServerBinary(serverBin)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrGPUUnavailable, err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	w, startErr := Start(ctx, bin)
	if startErr != nil {
		return fmt.Errorf("%w: server start failed: %w", ErrGPUUnavailable, startErr)
	}

	defer w.Close() //nolint:contextcheck // Close uses internal timeout

	pingErr := w.Ping(ctx)
	if pingErr != nil {
		return fmt.Errorf("%w: ping failed: %w", ErrGPUUnavailable, pingErr)
	}

	return nil
}

// resolveServerBinary finds the GPU server binary.  If serverBin is
// non-empty, it is returned directly (after checking existence).
// Otherwise, standard locations are searched.
func resolveServerBinary(serverBin string) (string, error) {
	if serverBin != "" {
		_, err := os.Stat(serverBin)
		if err != nil {
			return "", fmt.Errorf("server binary %q: %w", serverBin, err)
		}

		return serverBin, nil
	}

	return findServerBinary()
}

// findServerBinary searches for the GPU server binary in well-known
// locations relative to the executable directory and PATH.
func findServerBinary() (string, error) {
	candidates := []string{
		"algo-acoustics-gpu",
		"./algo-acoustics-gpu",
		"./bin/algo-acoustics-gpu",
		"./gpu/server/algo-acoustics-gpu",
	}

	for _, c := range candidates {
		path, err := exec.LookPath(c)
		if err == nil {
			return path, nil
		}
	}

	return "", errors.New("algo-acoustics-gpu binary not found in PATH or standard locations")
}

// StartIfAvailable attempts to start a GPU worker.  If the GPU is not
// available (binary missing, no CUDA device, driver error), it returns
// nil and a descriptive error wrapping ErrGPUUnavailable.  The caller
// should fall back to CPU-only execution when errors.Is(err, ErrGPUUnavailable).
//
// On success, the caller must call Worker.Close when done.
func StartIfAvailable(ctx context.Context, serverBin string) (*Worker, error) {
	bin, err := resolveServerBinary(serverBin)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGPUUnavailable, err)
	}

	w, err := Start(ctx, bin)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGPUUnavailable, err)
	}

	// Verify the GPU is responsive before returning.
	pingErr := w.Ping(ctx)
	if pingErr != nil {
		w.Close() //nolint:contextcheck // Close uses internal timeout

		return nil, fmt.Errorf("%w: %w", ErrGPUUnavailable, pingErr)
	}

	return w, nil
}

// IsGPUError reports whether err indicates a GPU-side failure that the
// caller should handle by falling back to CPU.  This includes OOM,
// CUDA errors, and bad handles.
func IsGPUError(err error) bool {
	var se *ServerError
	if !errors.As(err, &se) {
		return false
	}

	switch se.Code {
	case StatusOOM, StatusCUDA, StatusBadHandle:
		return true
	default:
		return false
	}
}
