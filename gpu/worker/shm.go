package worker

import "unsafe"

// float32sToBytes reinterprets []float32 as []byte without copying.
//
// This is pure pointer arithmetic with no OS dependency, so it lives here
// rather than in the platform-tagged shm_*.go files: every build of this
// package needs it, including the ones without a shared memory transport.
func float32sToBytes(f []float32) []byte {
	if len(f) == 0 {
		return nil
	}

	return unsafe.Slice((*byte)(unsafe.Pointer(&f[0])), len(f)*4)
}
