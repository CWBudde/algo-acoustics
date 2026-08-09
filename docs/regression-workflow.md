# Regression Workflow

The repository has three distinct baseline types. Update only the one affected
by an intentional change, and keep the evidence for that change in the review.

## Run the Baselines

```bash
# Deterministic ISM event JSON in testdata/regression.
go run ./cmd/roombench run

# Deterministic hybrid metric envelopes for the named corpus rooms.
go run ./cmd/roombench report --format table

# Full unit and integration coverage, including audio goldens.
just test
```

`roombench run` pairs every `testdata/rooms/shoebox_*.json` fixture with an
equal-named event array in `testdata/regression`. It uses maximum order 2 unless
`--max-order` is supplied. Event comparison tolerates 50 microseconds of time
drift, 0.5 dB of nonzero amplitude drift, and `1e-9` for distance, direction,
and per-band gains.

`roombench report` renders the small named corpus with 4096 rays, maximum order
3, a 2.5 second buffer, and a 0.08 second hybrid crossover. Its T60, EDT, and
C80 ranges are internal regression envelopes in `cmd/roombench/report.go`, not
measured-room tolerances.

## Updating Event Baselines

Only refresh event JSON after reviewing the solver change. From the repository
root, generate candidates outside the working tree first:

```bash
mkdir -p /tmp/algo-acoustics-regression

for scene_path in testdata/rooms/shoebox_*.json; do
  fixture_name="${scene_path##*/}"
  go run ./cmd/roomir dump-events "$scene_path" \
    --max-order 2 \
    --format json \
    --output "/tmp/algo-acoustics-regression/$fixture_name"
done

for candidate_path in /tmp/algo-acoustics-regression/shoebox_*.json; do
  fixture_name="${candidate_path##*/}"
  diff -u "testdata/regression/$fixture_name" "$candidate_path"
done
```

Review event count, kind, arrival time, direction, distance, amplitude, and
per-band gain changes. Copy only approved candidates into
`testdata/regression`, then run `roombench run` twice. Do not loosen tolerances
or refresh baselines merely to make unexplained drift pass.

## Updating Metric Envelopes

Capture a Markdown report before changing ranges:

```bash
go run ./cmd/roombench report \
  --format markdown \
  --output /tmp/bench-report.md
```

Run it repeatedly on a clean tree. If the values are stable and the algorithmic
change explains the shift, update the narrowest justified ranges in
`defaultCorpusCases`. Preserve useful ordering between treated/small and
live/large rooms. Include old values, new values, and the acoustic reason in the
change description.

## Audio Goldens

Audio goldens have their own producer settings and acceptance tests. Follow the
procedure next to the fixture rather than treating WAV files as event
baselines. The external mesh golden is documented in
[external-tool compatibility](external-tool-compatibility.md).

Before committing any baseline update, run `just test`, `just lint`, and
`just check-formatted`, and inspect `git diff --stat` for accidental large
fixtures.
