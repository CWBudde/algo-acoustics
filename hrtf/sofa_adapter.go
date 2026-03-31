//go:build sofa

package hrtf

import "fmt"

// SOFAAdapter is the tagged SOFA-backed HRTF implementation.
type SOFAAdapter struct{}

// LoadSOFA loads a SOFA dataset.
func LoadSOFA(path string) (*SOFAAdapter, error) {
	return nil, fmt.Errorf("SOFA support is not wired to a concrete go-sofa API in this build: %s", path)
}
