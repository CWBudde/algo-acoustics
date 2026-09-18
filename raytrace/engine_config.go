package raytrace

const (
	// DefaultDirectionGroupAzimuth is the azimuth resolution used when an
	// EngineConfig leaves DirectionGroupAzimuth at zero.
	DefaultDirectionGroupAzimuth = 12
	// DefaultDirectionGroupElevation is the elevation resolution used when an
	// EngineConfig leaves DirectionGroupElevation at zero.
	DefaultDirectionGroupElevation = 6
)

// EngineConfig configures a dense late-field render driven by RayTracer. Its
// fields map one-to-one onto the tracer's own settings, which is why it lives
// here rather than beside any single engine that builds one: the single-room
// adapter and the cross-room engines configure the same tracer.
//
// Launch.MaxTimeSeconds defaults to the render duration and Launch.SpeedOfSound
// defaults to acoustics.SpeedOfSound; both are filled in by the caller, which
// is the only party that knows the render config.
type EngineConfig struct {
	Launch                  LaunchConfig
	ReceiverRadius          float64
	BinDurationSeconds      float64
	DirectionGroupAzimuth   int
	DirectionGroupElevation int
}

// DirectionGroupCounts returns the directivity-group resolution, substituting
// the package defaults for non-positive values.
func (c EngineConfig) DirectionGroupCounts() (azimuth, elevation int) {
	azimuth = c.DirectionGroupAzimuth
	if azimuth <= 0 {
		azimuth = DefaultDirectionGroupAzimuth
	}

	elevation = c.DirectionGroupElevation
	if elevation <= 0 {
		elevation = DefaultDirectionGroupElevation
	}

	return azimuth, elevation
}
