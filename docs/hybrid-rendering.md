# Hybrid Rendering

Hybrid rendering combines geometric early reflections with low-frequency and late-field models so the toolkit can stay accurate without forcing one technique to cover every regime.

The early path is a sparse `[]ir.Event` stream from ISM. The canonical late path
is a dense buffer synthesized from the ray tracer's band-energy histogram; the
binaural variant retains directional-group hit probabilities and applies the
receiver HRTF during Poisson synthesis. Dense late output therefore uses the
explicit time-based crossover and is aligned to the early arrivals before the
buffers are blended.

`Renderer.LowFreq` supplies a monaural transfer function. `RenderMono` converts
and blends it below the engine's crossover (200 Hz by default), while
`RenderStereo` does not apply it. A binaural low-frequency model would need
ear-specific transfer functions and is not part of the current interface.

## CLI Modes

The mono `render` command exposes the three geometric modes:

```bash
# Direct and specular ISM events only.
go run ./cmd/roomir render examples/scenes/shoebox_mono.json \
  --output /tmp/early.wav --mode early --duration 1.5 --max-order 3

# Dense stochastic late field only.
go run ./cmd/roomir render examples/scenes/shoebox_mono.json \
  --output /tmp/late.wav --mode late --duration 1.5 \
  --max-order 3 --num-rays 4096

# Time-domain early/late crossover.
go run ./cmd/roomir render examples/scenes/shoebox_mono.json \
  --output /tmp/hybrid.wav --mode hybrid --duration 1.5 \
  --max-order 3 --num-rays 4096 --crossover-time 0.25 \
  --crossover-window hann
```

The supported window names are reported by `roomir render --help`. Parametric
windows also accept `--crossover-window-alpha`; invalid name/alpha combinations
are rejected before rendering. Keep duration, maximum order, ray count, and
crossover settings in comparison metadata because each changes the result.

## Low-Frequency Blend

Add the PDE path to a mono render with `--enable-lowfreq`:

```bash
go run ./cmd/roomir render examples/scenes/shoebox_mono.json \
  --output /tmp/hybrid_lowfreq.wav \
  --mode hybrid \
  --duration 1.5 \
  --max-order 3 \
  --num-rays 4096 \
  --enable-lowfreq \
  --lowfreq-min 20 \
  --lowfreq-max 300 \
  --lowfreq-points 32 \
  --lowfreq-crossover 200 \
  --lowfreq-boundary neumann
```

The boundary condition is `neumann`, `dirichlet`, or `periodic`. Increase sweep
points only after a small scene is working: PDE cost grows independently of the
geometric ray count. `examples/hybrid_lowfreq` demonstrates direct library use.

## Binaural Hybrid

`render-stereo` always renders binaural early events plus the directional late
field, aligns the late tail, and combines each ear. It requires one binaural
receiver and one source. Its flags are `--output`, `--duration`, `--max-order`,
`--num-rays`, and `--crossover-time`; mono-only flags such as `--mode` and
`--enable-lowfreq` do not apply.

## Reproducible Comparisons

ISM early output is the most direct deterministic diagnostic. Late and hybrid
paths use a deterministic internal seed for regression, but comparisons should
still record all render settings. When a crossover changes, compare early,
late, and hybrid outputs separately to distinguish solver drift from blend
drift. See the [regression workflow](regression-workflow.md).
