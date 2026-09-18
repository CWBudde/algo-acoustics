//go:build !linux

package worker

import (
	"fmt"
	"runtime"
)

// The shared memory transport used to hand bulk grid and histogram data to the
// GPU server is Linux-only: it relies on POSIX shared memory being exposed as
// regular files under /dev/shm/.  This stub supplies the same symbols on every
// other platform so the package still compiles and can be imported anywhere.
// Each entry point fails with an error wrapping ErrGPUUnavailable, which is
// exactly the condition callers already handle by falling back to CPU-only
// execution (see the contract documented on StartIfAvailable).

// errUnsupportedPlatform builds the ErrGPUUnavailable-wrapped error reported by
// every shared memory entry point on non-Linux platforms.
func errUnsupportedPlatform() error {
	return fmt.Errorf("%w: shared memory transport requires Linux, not %s/%s",
		ErrGPUUnavailable, runtime.GOOS, runtime.GOARCH)
}

// createShm is unavailable on this platform.
func createShm(_ int) (name string, data []byte, err error) {
	return "", nil, errUnsupportedPlatform()
}

// closeShm is unavailable on this platform.
func closeShm(_ string, _ []byte) error {
	return errUnsupportedPlatform()
}

// platformSupported reports whether this build has a working shared memory
// transport.  On non-Linux platforms it never does.
func platformSupported() error { return errUnsupportedPlatform() }
