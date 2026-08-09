# Validation

Call `scene.Validate` before propagation. It aggregates scene errors for:

- a positive sample rate and a non-empty, ordered band specification below
  Nyquist;
- a valid positive-volume shoebox or mesh, defined material references, and
  finite absorption, scattering, and transmission coefficients with
  `absorption + transmission <= 1` in every band;
- either the legacy `room` field or at least two `rooms`, but never both;
- valid portal room indices, material references, planar boundary polygons,
  and winding from the first room index toward the second;
- finite source and receiver positions inside exactly one room; and
- supported receiver types, with a non-nil, sample-rate-matched HRTF for every
  binaural receiver.

Scene validity does not imply that every engine supports every cardinality.
The shipped ISM engine supports one or more sources and exactly one receiver;
the ray-trace engine and `RenderProgressive` require exactly one source and one
receiver. Ray counts, render duration, bounce limits, receiver radius, and
other engine configuration are checked by their owning pipeline. Progressive
rendering validates the complete scene/configuration before its first update
and uses the scene's sample rate and band specification for all tiers.
Cross-room transmission currently requires one source and one receiver in
different, directly connected shoebox rooms.

The `roombench report` corpus in `testdata/rooms` is a deterministic internal
regression corpus. Its T60/EDT/C80 envelopes in `cmd/roombench/report.go` are
codebase baselines with intentionally broad drift tolerances, not published
measurements or third-party ground truth. The external mesh WAV golden has its
separate provenance and refresh procedure in
[external-tool-compatibility.md](external-tool-compatibility.md).

Higher-order diffraction has deterministic unit and regression coverage, but it
does not yet have checked-in Svensson/RAVEN numeric goldens. The official
reference-tool blocker and the exact reproduction contract are documented in
[`testdata/diffraction`](../testdata/diffraction/README.md). Do not describe the
external BTME or DAPDF validation as complete until those fixtures and their
generator are checked in.
