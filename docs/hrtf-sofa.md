# HRTF and SOFA

Binaural rendering depends on the narrow `hrtf.Dataset` interface: a sample
rate plus left/right HRIR lookup for a direction. `NearestNeighborDataset` is
the default measured-grid implementation and is the dataset supported by scene
JSON (`"type": "nearestNeighbor"`). Missing grids fall back to a centered
identity impulse.

Interpolation is explicit opt-in through `InterpolatingDataset` or
`InterpolateHRIR`. It uses only triangles supplied in
`MeasurementGrid.Triangles`, falls back to the nearest measurement when no
valid containing triangle exists, and is not currently a scene-JSON dataset
type.

SOFA file loading is not implemented. Without the `sofa` build tag, `LoadSOFA`
reports that the tag is required; tagged builds expose the adapter but still
return an error because no concrete SOFA reader is wired in. Do not treat the
tagged adapter as full SOFA ingestion.
