# Architecture

The canonical renderer keeps early pressure events separate from late energy
histograms:

```text
scene.Validate
  ├─ NewISMEngine → []ir.Event (direct/specular, sparse)
  │                    └─ ir.RenderMono / ir.RenderBinaural
  ├─ NewRaytraceEngine → band-energy histogram (late, dense)
  │                    ├─ HistogramToBuffer (mono)
  │                    └─ directional groups → binaural Poisson + HRTF
  └─ PDELowFreqEngine → transfer function (optional, mono only)
                         ↓
             time/frequency crossover → WAV, metrics, or samples
```

`NewISMEngine(ism.ISMConfig)` is the shipped `EventEngine` adapter. It accepts
one or more sources and requires exactly one receiver. `NewRaytraceEngine`
implements `LateBufferEngine` and `BinauralLateBufferEngine`, not
`EventEngine`: converting its band-energy bins into sparse pressure events
would lose information. Ray tracing requires exactly one source and exactly one
receiver. A renderer may configure the compatibility `Late EventEngine` or the
canonical `LateBuffer`, but not both; combining sparse early output with a dense
late buffer requires a time-based crossover.

For stereo, early event directions and late direction groups are transformed
into the receiver's head frame before HRTF lookup. A configured
`LowFreqEngine` is blended only by `RenderMono`; `RenderStereo` deliberately
skips it because the transfer function is monaural and cannot preserve
ear-specific spatial information.

`RenderProgressive` validates before publishing an update, requires exactly one
source and one receiver, and runs statistical, preview, refined-batch, then
final tiers. The scene is authoritative for sample rate and band specification:
those two fields overwrite `ProgressiveConfig.Render` so every tier has the
same format.
