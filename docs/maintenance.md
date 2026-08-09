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
