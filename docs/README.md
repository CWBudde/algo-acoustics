# Documentation

Start with [scene authoring](scene-authoring.md), then choose a rendering guide
for the output you need.

## User Guides

- [Scene authoring](scene-authoring.md): build, validate, inspect, and render JSON
  scenes.
- [HRTF usage](hrtf-sofa.md): configure binaural receivers and provide measured
  grids from Go.
- [Directivity usage](directivity-gll.md): use omni, cardioid, and GLL source
  models.
- [Hybrid rendering](hybrid-rendering.md): choose early, late, hybrid, and
  low-frequency options.
- [Diffraction](diffraction.md): enable finite-edge BTME paths or stochastic
  DAPDF rain and understand their validation status.
- [Sound transmission](sound-transmission.md): author adjacent rooms and
  portals, render secondary-source propagation, and interpret isolation
  metrics.
- [Comparing against another tool](compare-another-tool.md): prepare equivalent
  inputs and interpret `roomir compare` output.
- [External-tool compatibility](external-tool-compatibility.md): interchange
  conventions and the external-authoring import golden.

## Contributor Guides

- [Regression workflow](regression-workflow.md): run and intentionally update
  event, metric-envelope, and audio baselines.
- [Maintenance](maintenance.md): quarterly private-dependency audit, benchmark
  upkeep, and fixture-first feature workflow.
- [Validation](validation.md): validation boundaries and engine cardinality.
- [Architecture](architecture.md): rendering pipeline and package boundaries.
- [Profiling baseline](profiling-baseline.md) and
  [GPU profiling](profiling-gpu-kernels.md): performance measurement.
- [WASM memory budget](wasm-memory-budget.md): the browser demo's 512 MiB peak
  target, what enforces it, and the measurements behind the request envelope.
- [Web demo limits](web-demo-limits.md): the browser demo's request envelope,
  its progressive tiers, and what happens when a render overruns its budget.

## Reference

- [Scene format](scene-format.md) and canonical
  [JSON Schema](scene-schema.json).
- [GPU deployment](gpu-deployment.md).
- [Web demo deployment](web-deployment.md): the MIME type, cache headers, and
  COOP/COEP policy the browser demo needs from its host.
- [RAVEN notes](raven.md).
