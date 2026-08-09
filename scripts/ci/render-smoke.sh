#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SMOKE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/algo-acoustics-render-smoke.XXXXXX")"
ROOMIR_BIN="$SMOKE_DIR/roomir"

cleanup() {
	rm -rf "$SMOKE_DIR"
}
trap cleanup EXIT

if [[ $(go env GOOS) == windows ]]; then
	ROOMIR_BIN+=".exe"
fi

cd "$ROOT_DIR"
go build -o "$ROOMIR_BIN" ./cmd/roomir

"$ROOMIR_BIN" render testdata/rooms/shoebox_simple.json \
	--output "$SMOKE_DIR/mono.wav" \
	--mode early \
	--duration 0.1 \
	--max-order 1

"$ROOMIR_BIN" render-stereo testdata/rooms/shoebox_simple.json \
	--output "$SMOKE_DIR/stereo.wav" \
	--duration 0.1 \
	--crossover-time 0.02 \
	--max-order 1 \
	--num-rays 128

"$ROOMIR_BIN" render testdata/rooms/shoebox_simple.json \
	--output "$SMOKE_DIR/low-frequency.wav" \
	--mode early \
	--duration 0.1 \
	--max-order 1 \
	--enable-lowfreq \
	--lowfreq-min 40 \
	--lowfreq-max 120 \
	--lowfreq-points 4 \
	--lowfreq-crossover 100

for output in mono.wav stereo.wav low-frequency.wav; do
	if [[ ! -s $SMOKE_DIR/$output ]]; then
		echo "render smoke output is missing or empty: $output" >&2
		exit 1
	fi
done

echo "Mono, stereo, and low-frequency render smoke tests passed"
