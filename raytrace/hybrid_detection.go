package raytrace

// HybridDetectionConfig gates ray-traced detections against the portion of the
// sound field already covered by the image-source method. It is a direct port
// of RAVEN's DETECTION_ALLOWED_HYBRID logic (see docs/raven.md section 4.2).
//
// The zero value is disabled, which keeps the tracer's historical behaviour of
// detecting every particle that crosses the receiver.
type HybridDetectionConfig struct {
	// Enabled activates the hybrid gate. When false, every particle is detected.
	Enabled bool
	// MaxISOrder is the highest reflection order covered by the image-source
	// solver for purely reflected (non-diffracted) paths.
	MaxISOrder int
	// MaxPreEDISOrder is the highest reflection order covered by the image-source
	// solver before the first edge diffraction.
	MaxPreEDISOrder int
	// MaxEDISOrder is the highest reflection order covered by the image-source
	// solver after the first edge diffraction.
	MaxEDISOrder int
	// MaxEDOrder is the highest edge-diffraction order covered by the
	// image-source solver.
	MaxEDOrder int
}

// DetectionAllowedHybrid reports whether a particle in the given state may be
// counted by the ray-tracing detector without double-counting energy that the
// image-source method already contributes.
//
// Diffracted particles (EDOrder != 0) are detected once any part of their
// history escapes image-source coverage: a diffuse reflection, too many
// reflections before or after the first diffraction, or too many diffractions.
// Purely reflected particles are detected only beyond the image-source order.
func DetectionAllowedHybrid(state RayState, cfg HybridDetectionConfig) bool {
	if !cfg.Enabled {
		return true
	}

	if state.EDOrder != 0 {
		return state.HasDiffuseHistory ||
			state.PreEDReflOrder > cfg.MaxPreEDISOrder ||
			state.EDReflOrder > cfg.MaxEDISOrder ||
			state.EDOrder > cfg.MaxEDOrder
	}

	return state.ReflectionOrder > cfg.MaxISOrder
}

// advanceStateAfterReflection updates the hybrid bookkeeping counters of a
// particle that has just been reflected. Reflections are routed to the pre- or
// post-diffraction counter depending on whether the particle has already been
// diffracted. AllowDetection is set after a specular reflection and reset after
// a scattering event, mirroring RAVEN's rain double-counting protection.
func advanceStateAfterReflection(state *RayState, diffuse bool) {
	if state == nil {
		return
	}

	state.ReflectionOrder++

	if state.EDOrder == 0 {
		state.PreEDReflOrder++
	} else {
		state.EDReflOrder++
	}

	if diffuse {
		state.HasDiffuseHistory = true
	}

	state.AllowDetection = !diffuse
}

// advanceStateAfterDiffraction updates the hybrid bookkeeping counters of a
// particle that has just been diffracted at an edge.
func advanceStateAfterDiffraction(state *RayState) {
	if state == nil {
		return
	}

	state.EDOrder++
	state.AllowDetection = false
}

// reflectedChildState derives a branch state that inherits the parent's hybrid
// counters plus the increment for its own reflection. Ray, energy and path
// bookkeeping are left to the caller.
func reflectedChildState(parent RayState, diffuse bool) RayState {
	child := parent
	child.Energy = nil
	advanceStateAfterReflection(&child, diffuse)

	return child
}

// diffractedChildState derives a branch state that inherits the parent's hybrid
// counters plus one edge diffraction. Ray, energy and path bookkeeping are left
// to the caller.
func diffractedChildState(parent RayState) RayState {
	child := parent
	child.Energy = nil
	advanceStateAfterDiffraction(&child)

	return child
}
