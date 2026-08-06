//go:build !sofa

package hrtf

import "errors"

// SOFAAdapter is unavailable without the sofa build tag.
type SOFAAdapter struct{}

// LoadSOFA returns an error unless the sofa build tag is enabled.
func LoadSOFA(_ string) (*SOFAAdapter, error) {
	return nil, errors.New("SOFA support requires the sofa build tag")
}
