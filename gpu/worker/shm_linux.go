//go:build linux

package worker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// On Linux, POSIX shared memory is implemented as regular files under /dev/shm/.
// shm_open("/name", ...) is equivalent to open("/dev/shm/name", ...).
// We implement the same semantics directly to avoid depending on cgo for the
// shm_open(3) glibc wrapper.

const shmDir = "/dev/shm/"
const shmPrefix = "algo_gpu_"

// shmPath converts a logical name ("algo_gpu_deadbeef") to its filesystem path.
func shmPath(name string) string { return shmDir + name }

// createShm creates a new POSIX-style shared memory segment of size bytes
// under /dev/shm/.  Returns the logical name (e.g. "algo_gpu_deadbeef01234567")
// and a writable []byte mapping.  Call closeShm when done.
func createShm(size int) (name string, data []byte, err error) {
	var raw [8]byte

	_, err = rand.Read(raw[:])
	if err != nil {
		return "", nil, fmt.Errorf("rand: %w", err)
	}

	name = shmPrefix + hex.EncodeToString(raw[:])
	path := shmPath(name)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return "", nil, fmt.Errorf("create shm %s: %w", path, err)
	}
	defer f.Close()

	err = f.Truncate(int64(size))
	if err != nil {
		os.Remove(path)
		return "", nil, fmt.Errorf("truncate shm %s: %w", path, err)
	}

	data, err = unix.Mmap(int(f.Fd()), 0, size,
		unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		os.Remove(path)
		return "", nil, fmt.Errorf("mmap shm %s: %w", path, err)
	}

	return name, data, nil
}

// closeShm unmaps the mapping.  If name is non-empty the file is removed.
func closeShm(name string, data []byte) error {
	err := unix.Munmap(data)
	if err != nil {
		return fmt.Errorf("munmap shm %s: %w", name, err)
	}

	if name != "" {
		err = os.Remove(shmPath(name))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove shm %s: %w", name, err)
		}
	}

	return nil
}

// float32sToBytes reinterprets []float32 as []byte without copying.
func float32sToBytes(f []float32) []byte {
	if len(f) == 0 {
		return nil
	}

	return unsafe.Slice((*byte)(unsafe.Pointer(&f[0])), len(f)*4)
}
