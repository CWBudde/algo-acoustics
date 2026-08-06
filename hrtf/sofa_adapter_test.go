//go:build sofa

package hrtf

import (
	"strings"
	"testing"
)

func TestLoadSOFAWithBuildTagReportsUnimplementedAdapter(t *testing.T) {
	t.Parallel()

	_, err := LoadSOFA("example.sofa")
	if err == nil || !strings.Contains(err.Error(), "not wired") {
		t.Fatalf("LoadSOFA() error = %v, want unimplemented adapter error", err)
	}
}
