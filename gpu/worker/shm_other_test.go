//go:build !linux

package worker

import (
	"context"
	"errors"
	"testing"
)

// The tests in this file only build off Linux, which is the point: they pin the
// behaviour of the shm_other.go stubs on the platforms that use them.
//
// Nothing else reaches that code at runtime. Probe and StartIfAvailable both go
// through resolveServerBinary first and return early when no server binary is
// present, and the integration tests skip without ALGO_GPU_SERVER, so on a CI
// runner without a GPU the platform guard would otherwise only ever be
// compiled, never executed — and deleting it would still pass.

func TestStartOnUnsupportedPlatform(t *testing.T) {
	// Start is called directly rather than through StartIfAvailable: the latter
	// resolves the binary first and would fail there instead, never reaching the
	// platform guard this test exists to cover.
	w, err := Start(context.Background(), "/nonexistent/algo-acoustics-gpu")
	if w != nil {
		w.Close()
		t.Fatal("expected nil worker")
	}

	if !errors.Is(err, ErrGPUUnavailable) {
		t.Errorf("expected ErrGPUUnavailable, got: %v", err)
	}
}

func TestCreateShmOnUnsupportedPlatform(t *testing.T) {
	name, data, err := createShm(64)
	if name != "" || data != nil {
		t.Errorf("expected no segment, got name %q and %d bytes", name, len(data))
	}

	if !errors.Is(err, ErrGPUUnavailable) {
		t.Errorf("expected ErrGPUUnavailable, got: %v", err)
	}
}

func TestCloseShmOnUnsupportedPlatform(t *testing.T) {
	err := closeShm("algo_gpu_deadbeef", nil)
	if !errors.Is(err, ErrGPUUnavailable) {
		t.Errorf("expected ErrGPUUnavailable, got: %v", err)
	}
}
