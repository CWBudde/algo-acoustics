# Maintenance

This runbook turns the maintenance budget into repeatable, reviewable work.

## Quarterly Private-Dependency Audit

The [Private Dependency Audit](../.github/workflows/dependency-audit.yml) runs
on the first day of January, April, July, and October and can be started with
`workflow_dispatch`. It records current and available versions for
`algo-dsp`, `algo-fft`, `algo-pde`, `gll-tools`, and `wav`, verifies downloaded
modules, runs tests against the pinned graph, and uploads the report.

The workflow requires an Actions secret named `PRIVATE_MODULES_TOKEN`. Use a
fine-grained personal access token with read-only Contents access to all five
private dependency repositories. The default `GITHUB_TOKEN` is scoped to this
repository and is not assumed to read sibling private repositories. Rotate the
secret according to the organization policy; a missing or expired token
intentionally fails at the credential preflight.

For each quarterly run:

1. Download `private-dependency-audit` from the workflow run and record the run
   URL in the maintenance issue or release notes.
2. Review upstream changelogs, Go-version changes, transitive graph changes,
   licenses, and security advisories for each candidate version.
3. Update one direct dependency at a time with `go get module@version`, then run
   `go mod tidy`, `just test`, `just lint`, and `just check-formatted`.
4. For DSP, FFT, PDE, GLL, or WAV behavior changes, also run the regression
   commands and relevant audio/export tests:

   ```bash
   go run ./cmd/roombench run
   go run ./cmd/roombench report --format table
   ```

5. Open separate reviewable changes when upgrades affect different solver
   paths. Do not automatically commit versions merely because an update exists.

To reproduce the inventory locally with credentials already configured:

```bash
GOPRIVATE=github.com/cwbudde/* GONOSUMDB=github.com/cwbudde/* \
  go list -m -u \
  github.com/cwbudde/algo-dsp \
  github.com/cwbudde/algo-fft \
  github.com/cwbudde/algo-pde \
  github.com/cwbudde/gll-tools \
  github.com/cwbudde/wav
go mod verify
```

## Benchmark Baseline Updates

Baseline updates are a consequence of an understood change, not a maintenance
goal by themselves. Follow [the regression workflow](regression-workflow.md)
for event JSON, hybrid metric envelopes, and audio goldens. Capture before/after
reports, run twice to establish determinism, and explain the acoustic effect in
review. Keep performance baselines separate from acoustic acceptance ranges;
the profiling procedure lives in [profiling-baseline.md](profiling-baseline.md).

## Fixture-First Feature Workflow

Every new scene format or solver feature starts with the smallest fixture that
can prove its behavior:

1. Add a compact scene or geometry asset under the relevant `testdata`
   directory. Prefer analytic dimensions and a few triangles/samples over a
   production asset.
2. Add a focused white-box test for parsing/validation and one solver behavior.
   Make the expected physical relationship explicit, such as earlier arrival,
   lower rear energy, or reduced band energy.
3. If JSON changes, update `docs/scene-schema.json`, scene marshal/unmarshal
   tests, and [scene authoring](scene-authoring.md).
4. If the CLI changes, update a file under `examples/scenes`, its documented
   command, and validate it in the same change.
5. Run the focused test, `just test`, `just lint`, and `just check-formatted`.
6. Add large manufacturer data, complex meshes, or broad corpus cases only
   after the small fixture makes failures easy to diagnose.

Keep generated or licensed data out of the small-fixture layer unless its
provenance and redistribution terms are recorded.

## Cross-Platform Floating-Point Determinism (FMA)

Go permits the compiler to fuse `x*y + z` into a single fused multiply-add
instruction. It does so on arm64, ppc64, s390x, and riscv64, but not on amd64.
A fused instruction does not round the intermediate product, so the same
expression evaluates to slightly different values on the two architectures —
typically a handful of ulps. Nothing is broken on either side; the arithmetic is
simply not bit-identical.

The consequence for this codebase is a hard rule: **geometric predicates must
never be decided by an exact tie.** A wall plane that lands exactly on a grid
node plane, a point exactly on a half-space boundary, a fraction exactly equal
to `1` — each of these resolves one way on amd64 and the other way on arm64,
because the deciding comparison sits within a few ulps of the branch point. This
is what made `TestIBMValidation_EquilateralTriangle` fail on the macOS CI runner
(Apple Silicon) while it passed on every amd64 runner: a room dimension that was
an exact multiple of the grid spacing put hundreds of boundary nodes on the
wrong side of a `frac > 1` test, giving them the opposite boundary condition.

### Suppressing contraction

The Go spec guarantees that an explicit conversion forces the intermediate value
to be rounded to its type, which defeats the fusion. Wrap the product wherever
bit-identical arithmetic actually matters:

```go
// Contracted into one FMA on arm64, two rounded operations on amd64:
origin.Z = center.Z - float64(halfNz)*h

// Rounded after the multiply on every architecture:
origin.Z = center.Z - float64(float64(halfNz)*h)
```

Use this deliberately and locally. It is a determinism tool, not a correctness
tool; the fused form is usually the more accurate one. The better fix for a
classifier is almost always to remove the tie, not to pin the rounding.

### Reproducing an arm64 divergence locally

No Apple hardware is needed — `qemu-aarch64-static` (Debian/Ubuntu package
`qemu-user-static`) reproduces the macOS result exactly:

1. Cross-compile the test binary for arm64:

   ```bash
   GOOS=linux GOARCH=arm64 go test -c -o /tmp/pde.arm64.test ./pde/
   ```

2. Run it under emulation:

   ```bash
   qemu-aarch64-static /tmp/pde.arm64.test -test.run TestIBMValidation -test.v
   ```

3. Compare against the same test on the host to confirm the divergence, then
   confirm FMA contraction is the cause by rebuilding the arm64 binary with a
   contraction hash filter and bisecting the pattern until the divergence
   disappears:

   ```bash
   GOOS=linux GOARCH=arm64 go test -c -gcflags=all=-d=fmahash=1010 \
     -o /tmp/pde.arm64.test ./pde/
   ```

   `-d=fmahash=<pattern>` enables FMA contraction only at call sites whose
   position hash matches the given bit pattern. Narrowing the pattern until the
   output flips identifies the exact contraction site responsible. If some
   pattern makes the arm64 output byte-identical to amd64, FMA is confirmed as
   the source and the matching site is the one to fix.

Emulation is roughly ten times slower than native execution; budget accordingly
when running a whole package.

### For reviewers

- Prefer tolerance-based comparisons over exact equality or bare `>` / `<` in
  any code that classifies geometry (inside/outside, on-plane, cut-cell
  fractions). State the tolerance and why it has the magnitude it has.
- Any new geometric classifier needs a golden or determinism test — hash its
  output for a fixture whose dimensions are an exact multiple of the grid
  spacing, so a per-architecture flip fails loudly instead of drifting.
- Cross-platform CI covers this class of defect: the emulated arm64 job in
  [the unit test workflow](../.github/workflows/test-unit.yml) catches it before
  it reaches the macOS job.
