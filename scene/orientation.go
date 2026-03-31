package scene

import "github.com/cwbudde/algo-acoustics/geometry"

func effectiveOrientation(q geometry.Quaternion) geometry.Quaternion {
	if q == (geometry.Quaternion{}) {
		return geometry.QuatIdentity()
	}

	return q
}
