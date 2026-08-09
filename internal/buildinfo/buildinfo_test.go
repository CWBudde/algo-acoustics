package buildinfo

import "testing"

func TestString(t *testing.T) {
	oldVersion, oldCommit, oldBuildDate := Version, Commit, BuildDate

	t.Cleanup(func() {
		Version, Commit, BuildDate = oldVersion, oldCommit, oldBuildDate
	})

	Version = "v1.2.3"
	Commit = "abcdef0"
	BuildDate = "2026-08-08T12:34:56Z"

	const want = "v1.2.3 (commit abcdef0, built 2026-08-08T12:34:56Z)"
	if got := String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
