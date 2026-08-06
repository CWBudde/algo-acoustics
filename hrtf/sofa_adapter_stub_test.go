//go:build !sofa

package hrtf

import (
	"strings"
	"testing"
)

func TestLoadSOFAWithoutBuildTagReportsUnavailable(t *testing.T) {
	t.Parallel()

	_, err := LoadSOFA("example.sofa")
	if err == nil || !strings.Contains(err.Error(), "requires the sofa build tag") {
		t.Fatalf("LoadSOFA() error = %v, want build-tag requirement", err)
	}
}
