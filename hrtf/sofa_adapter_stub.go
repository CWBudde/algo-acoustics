package hrtf

import "fmt"

// SOFAAdapter is unavailable without the sofa build tag.
type SOFAAdapter struct{}

// LoadSOFA returns an error unless the sofa build tag is enabled.
func LoadSOFA(path string) (*SOFAAdapter, error) {
	return nil, fmt.Errorf("SOFA support requires the sofa build tag")
}