// Package preview holds the interactive preview machinery that sits above the
// solvers: progressive level-of-detail rendering, named quality presets,
// synthetic statistical tails for the fast tiers, and a debouncer for
// coalescing rapid parameter changes.
//
// It is internal because none of it is part of the library's rendering
// contract — it exists to serve the WASM demo and any future editor UI, and
// its shape should stay free to follow those callers.
package preview
