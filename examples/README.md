# Examples

Run all commands from the repository root.

## JSON Scenes and CLI

- [shoebox_mono.json](scenes/shoebox_mono.json) uses a cardioid source and omni
  receiver for `roomir render`.
- [shoebox_binaural.json](scenes/shoebox_binaural.json) uses a binaural receiver
  with a centered identity HRTF for `roomir render-stereo`. It exercises the
  stereo path but is not a perceptual HRTF dataset.
- [two_room_transmission.json](scenes/two_room_transmission.json) places a
  source and receiver in adjacent shoeboxes separated by a 25 dB door portal.

```bash
go run ./cmd/roomir validate examples/scenes/shoebox_mono.json
go run ./cmd/roomir render examples/scenes/shoebox_mono.json \
  --output /tmp/shoebox_mono.wav --mode hybrid \
  --duration 1.5 --max-order 3 --num-rays 4096

go run ./cmd/roomir validate examples/scenes/shoebox_binaural.json
go run ./cmd/roomir render-stereo examples/scenes/shoebox_binaural.json \
  --output /tmp/shoebox_binaural.wav \
  --duration 1.5 --max-order 3 --num-rays 4096

go run ./cmd/roomir validate examples/scenes/two_room_transmission.json
go run ./cmd/roomir render examples/scenes/two_room_transmission.json \
  --output /tmp/two_room.wav --mode hybrid \
  --duration 1.5 --max-order 3 --num-rays 4096
```

## Go API

- `shoebox_mono` renders sparse ISM events to a mono WAV.
- `shoebox_late` prints a ray-trace energy histogram as CSV.
- `hybrid_lowfreq` blends a ray-traced buffer with a PDE transfer function.
- `gll_source` loads the synthetic GLL fixture, checks front/rear energy, and
  renders a hybrid WAV. Its CLI exposes the current crossover-window options.

```bash
go run ./examples/shoebox_mono
go run ./examples/shoebox_late > /tmp/late.csv
go run ./examples/hybrid_lowfreq
go run ./examples/gll_source --output /tmp/gll.wav \
  --crossover-window hann
```

These programs are executable integration examples. Smaller behavioral
fixtures belong under `testdata`; see [the maintenance guide](../docs/maintenance.md).
