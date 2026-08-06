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
