# Compare Against Another Tool

Cross-tool comparison is meaningful only after the two renderers describe the
same experiment. Treat `roomir compare` as a numerical report, not as proof that
two different solver models should produce identical samples.

## 1. Match the Scene

Record these values alongside both exports:

- right-handed coordinates, `+Z` up, and dimensions/positions in meters;
- speed of sound and any air-absorption settings;
- material absorption and scattering for the same octave bands;
- source gain, directivity, position, and orientation;
- receiver position, orientation, and HRTF choice;
- reflection order, render duration, late-ray count/seed, crossover settings,
  and low-frequency boundary condition.

Use a shoebox, one omni source, one omni receiver, and early-only output for the
first comparison. Add directivity, stochastic late energy, HRTF, mesh import,
and modal blending one at a time. The interchange conventions are detailed in
[external-tool compatibility](external-tool-compatibility.md).

## 2. Export Compatible WAV Files

Both WAVs must have the same sample rate. `roomir compare` averages channels of
a multichannel input to mono, pads the shorter input with zeros for correlation,
and does not resample, time-align, trim, or normalize. Do those operations
explicitly in the producing tools and retain the unprocessed exports.

For an algo-acoustics reference:

```bash
go run ./cmd/roomir render scene.json \
  --output /tmp/algo-acoustics.wav \
  --mode early \
  --duration 1.5 \
  --max-order 3
```

Check that direct-arrival sample/time, polarity, length, channel convention, and
sample rate agree before interpreting later reflections.

## 3. Generate a Report

The first input is labeled `Expected`; the second is `Actual`:

```bash
go run ./cmd/roomir compare \
  /tmp/other-tool.wav \
  /tmp/algo-acoustics.wav \
  --format markdown \
  --output /tmp/comparison.md
```

`--format` accepts `table`, `csv`, or `markdown`. The report contains:

- peak and RMS amplitude in linear units;
- full-buffer correlation, where timing and polarity errors strongly reduce
  the coefficient; and
- energy levels and deltas for the built-in six octave bands from 125 Hz to
  4 kHz.

There is no built-in pass threshold. Choose tolerances before inspecting the
result and base them on the intended equivalence: a same-engine regression can
be strict, while different ray tracers should usually be compared on decay and
band energy rather than sample correlation.

## 4. Diagnose Differences

1. Compare direct-arrival time to catch units, speed-of-sound, or position
   mismatches.
2. Compare direct amplitude to catch gain, normalization, and directivity-axis
   mismatches.
3. Render first-order early output to isolate wall ordering and material
   reflectance.
4. Compare late-field RMS and band deltas with fixed ray settings; do not expect
   matching stochastic samples.
5. Add HRTF only after mono agreement and confirm both tools use the same head
   orientation and HRIR sample rate.

Keep the source scenes, raw WAVs or reproducible download references, exact
commands, tool versions, and comparison report together. Do not label an
algo-acoustics-generated golden as third-party ground truth.
