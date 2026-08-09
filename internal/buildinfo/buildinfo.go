// Package buildinfo exposes metadata injected into command binaries at build time.
package buildinfo

import "fmt"

// These defaults keep local `go build` output useful. Release builds replace them
// with -ldflags -X values; the names and types must therefore remain variables.
//
//nolint:gochecknoglobals // linker-injected build metadata is process-global by design
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// String returns a concise, human-readable build identifier.
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildDate)
}
