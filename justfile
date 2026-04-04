set shell := ["bash", "-uc"]

export GOPRIVATE := "github.com/cwbudde"

# Default recipe - show available commands
default:
    @just --list

# Format all code using treefmt
fmt:
    treefmt --allow-missing-formatter

# Check if code is formatted correctly
check-formatted:
    treefmt --allow-missing-formatter --fail-on-change

# Run linters
lint:
    GOCACHE="${GOCACHE:-/tmp/gocache}" GOMODCACHE="${GOMODCACHE:-/tmp/gomodcache}" GOLANGCI_LINT_CACHE="${GOLANGCI_LINT_CACHE:-/tmp/golangci-lint-cache}" golangci-lint run --timeout=2m ./...

# Run linters with auto-fix
lint-fix:
    GOCACHE="${GOCACHE:-/tmp/gocache}" GOMODCACHE="${GOMODCACHE:-/tmp/gomodcache}" GOLANGCI_LINT_CACHE="${GOLANGCI_LINT_CACHE:-/tmp/golangci-lint-cache}" golangci-lint run --fix --timeout=2m ./...

# Ensure go.mod is tidy
check-tidy:
    go mod tidy
    git diff --exit-code go.mod go.sum

# Run all tests
test:
    go test -v ./...

# Run tests with race detector
test-race:
    go test -race ./...

# Run tests with coverage
test-coverage:
    go test -v -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Run benchmarks
bench:
    go test -run=^$ -bench=. -benchmem ./...

# Build all CLI tools
build:
    go build ./cmd/roomir/...
    go build ./cmd/roomplot/...
    go build ./cmd/roombench/...

# Build the browser WebAssembly demo
build-web-demo:
    ./web/build-wasm.sh

# Serve the browser demo locally
web-demo: build-web-demo
    python3 -m http.server 8080 -d web

# Run all checks (formatting, linting, tests, tidiness)
ci: check-formatted test lint check-tidy

# Phase 14.1: CPU profiling baseline — record CPU profile from pipeline benchmarks
# Usage: just profile-cpu [bench=<pattern>]
# Output: cpu.prof  (view: go tool pprof -http=:8080 cpu.prof)
bench_pattern := env_var_or_default("bench", "BenchmarkHybridPipeline_4K|BenchmarkRayTrace_16K|BenchmarkPDEShoebox|BenchmarkISM|BenchmarkIBM_FDTDStep")
profile-cpu:
    go test -run=^$ -bench='{{bench_pattern}}' -benchtime=10s -cpuprofile=cpu.prof \
        github.com/cwbudde/algo-acoustics \
        github.com/cwbudde/algo-acoustics/pde
    @echo "CPU profile written to cpu.prof — view with: go tool pprof -http=:8080 cpu.prof"

# Phase 14.1: memory profile from pipeline benchmarks
# Output: mem.prof  (view: go tool pprof -http=:8080 mem.prof)
profile-mem:
    go test -run=^$ -bench='{{bench_pattern}}' -benchtime=10s -memprofile=mem.prof \
        github.com/cwbudde/algo-acoustics \
        github.com/cwbudde/algo-acoustics/pde
    @echo "Memory profile written to mem.prof — view with: go tool pprof -http=:8080 mem.prof"

# Phase 14.1: goroutine block profile (mutex/channel contention)
# Output: block.prof  (view: go tool pprof -http=:8080 block.prof)
profile-block:
    go test -run=^$ -bench='{{bench_pattern}}' -benchtime=10s -blockprofile=block.prof \
        github.com/cwbudde/algo-acoustics \
        github.com/cwbudde/algo-acoustics/pde
    @echo "Block profile written to block.prof — view with: go tool pprof -http=:8080 block.prof"

# Phase 14.1: GOMAXPROCS scaling sweep — measures single-core vs multi-core throughput
# Runs BenchmarkRayTrace_16K and BenchmarkIBM_FDTDStep_RectRoom at GOMAXPROCS=1,2,4,8
# Output: gomaxprocs_sweep.txt
gomaxprocs-sweep:
    @echo "GOMAXPROCS sweep — $(date)" | tee gomaxprocs_sweep.txt
    @for N in 1 2 4 8; do \
        echo "" | tee -a gomaxprocs_sweep.txt; \
        echo "=== GOMAXPROCS=$N ===" | tee -a gomaxprocs_sweep.txt; \
        GOMAXPROCS=$$N go test -run=^$ \
            -bench='BenchmarkRayTrace_16K|BenchmarkIBM_FDTDStep_RectRoom' \
            -benchtime=5s -count=3 \
            github.com/cwbudde/algo-acoustics \
            github.com/cwbudde/algo-acoustics/pde 2>&1 | tee -a gomaxprocs_sweep.txt; \
    done
    @echo ""
    @echo "Sweep results written to gomaxprocs_sweep.txt"

# Clean build artifacts
clean:
    rm -f coverage.out coverage.html
    rm -f bin/roomir bin/roomplot bin/roombench
    rm -f cpu.prof mem.prof block.prof gomaxprocs_sweep.txt
