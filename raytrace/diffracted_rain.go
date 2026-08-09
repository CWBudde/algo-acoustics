package raytrace

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

type dapdfDeposit struct {
	SurfaceIndex int
	BandIndex    int
	Energy       float64
	ArrivalTime  float64
	ArrivalDir   geometry.Vec3
}

type dapdfTarget struct {
	center  geometry.Vec3
	radius  float64
	polygon []geometry.Vec3
	surface int
}

// dapdfRainForSegment deposits diffracted energy into receiver targets and
// recursively forwards it through visible deflection cylinders. receiver may
// be nil; surface indices are preserved in the returned deposits.
func dapdfRainForSegment(
	state RayState,
	ray geometry.Ray,
	segmentLength float64,
	index *DiffractionEdgeIndex,
	tracer Tracer,
	receiver *SphereReceiver,
	surfaces []SurfaceReceiver,
	config LaunchConfig,
	centerFreqs []float64,
	launchEnergy float64,
) []dapdfDeposit {
	if index == nil || config.effectiveDiffractionMode() != DiffractionDAPDFRain || segmentLength <= 0 {
		return nil
	}

	maxRadius := maximumDeflectionRadius(config.SpeedOfSound, centerFreqs)
	if maxRadius <= 0 {
		return nil
	}

	candidates := index.candidateRefs(ray.Origin, ray.At(segmentLength), maxRadius)
	if len(candidates) == 0 {
		return nil
	}

	deposits := make([]dapdfDeposit, 0)

	cullThreshold := launchEnergy * math.Pow(10, diffractionCullingDB/10)
	if cullThreshold < config.EnergyTerminationThreshold {
		cullThreshold = config.EnergyTerminationThreshold
	}

	for bandIndex, energy := range state.Energy {
		frequency := bandFrequency(centerFreqs, bandIndex)
		if frequency <= 0 {
			continue
		}

		wavelength := config.SpeedOfSound / frequency
		var nearestRef diffractionEdgeRef
		var nearestHit geometry.RayCylinderHit
		found := false

		for _, ref := range candidates {
			cylinder := NewDeflectionCylinder(ref.index, ref.edge, wavelength)

			hit, ok := geometry.RayOpenFiniteCylinder(ray, ref.edge.Start, ref.edge.End, cylinder.Radius, wallEpsilon, segmentLength-wallEpsilon)
			if !ok || (found && hit.RayDistance >= nearestHit.RayDistance) {
				continue
			}

			nearestRef = ref
			nearestHit = hit
			found = true
		}

		if !found {
			continue
		}

		incoming := attenuateEnergyByAir([]float64{energy}, []float64{frequency}, nearestHit.RayDistance)[0]
		if incoming <= cullThreshold {
			continue
		}

		visited := map[int]bool{nearestRef.index: true}
		pathLength := state.PathLength + nearestHit.RayDistance
		deposits = append(deposits, dapdfForward(
			nearestRef.index,
			nearestHit.EdgePoint,
			nearestRef.edge.Direction,
			ray.Direction,
			nearestHit.FlyByDistance,
			bandIndex,
			frequency,
			incoming,
			pathLength,
			state.DelaySeconds,
			1,
			visited,
			index,
			tracer,
			receiver,
			surfaces,
			config,
			cullThreshold,
		)...)
	}

	return deposits
}

func dapdfForward(
	edgeID int,
	point, edgeDirection, incidentDirection geometry.Vec3,
	flyByDistance float64,
	bandIndex int,
	frequency, incomingEnergy, pathLength, delaySeconds float64,
	depth int,
	visited map[int]bool,
	index *DiffractionEdgeIndex,
	tracer Tracer,
	receiver *SphereReceiver,
	surfaces []SurfaceReceiver,
	config LaunchConfig,
	cullThreshold float64,
) []dapdfDeposit {
	if incomingEnergy <= cullThreshold || depth > config.effectiveMaxDiffractionDepth() {
		return nil
	}

	wavelength := config.SpeedOfSound / frequency
	b := 6 * math.Max(flyByDistance, wavelength*1e-6) / wavelength
	targets := make([]dapdfTarget, 0, len(surfaces)+1)

	if receiver != nil && receiver.Radius > 0 {
		distance := point.Distance(receiver.Center)
		if distance > 0 && segmentVisibleToTarget(point, receiver.Center, distance-receiver.Radius, tracer) {
			targets = append(targets, dapdfTarget{
				center:  receiver.Center,
				radius:  receiver.Radius,
				surface: -1,
			})
		}
	}

	for surfaceIndex, surface := range surfaces {
		distance := point.Distance(surface.Center)
		if distance <= 0 || !segmentVisibleToTarget(point, surface.Center, distance, tracer) {
			continue
		}

		targets = append(targets, dapdfTarget{
			center:  surface.Center,
			polygon: surface.Polygon,
			surface: surfaceIndex,
		})
	}

	deposits := make([]dapdfDeposit, 0, len(targets))
	for _, target := range targets {
		distance := point.Distance(target.center)

		angleMin, angleMax, ok := targetDeflectionInterval(point, edgeDirection, incidentDirection, target)
		if !ok || angleMax <= angleMin {
			continue
		}

		weight := DAPDFIntegral(angleMin, angleMax, b)
		weight = math.Max(0, math.Min(1, weight))

		energy := attenuateEnergyByAir([]float64{incomingEnergy * weight}, []float64{frequency}, distance)[0]
		if energy <= cullThreshold {
			continue
		}

		arrivalTime := delaySeconds + (pathLength+distance)/config.SpeedOfSound
		if arrivalTime > config.MaxTimeSeconds {
			continue
		}

		deposits = append(deposits, dapdfDeposit{
			SurfaceIndex: target.surface,
			BandIndex:    bandIndex,
			Energy:       energy,
			ArrivalTime:  arrivalTime,
			ArrivalDir:   point.Sub(target.center).Normalize(),
		})
	}

	if depth >= config.effectiveMaxDiffractionDepth() {
		return deposits
	}

	type forwardingTarget struct {
		ref      diffractionEdgeRef
		point    geometry.Vec3
		distance float64
		weight   float64
	}

	forwarding := make([]forwardingTarget, 0)
	var totalWeight float64

	for nextID, nextEdge := range index.edges {
		if nextID == edgeID || visited[nextID] {
			continue
		}

		nextPoint := nextEdge.Start.Add(nextEdge.End.Sub(nextEdge.Start).Scale(0.5))

		distance := point.Distance(nextPoint)
		if distance <= wallEpsilon || !segmentVisibleToTarget(point, nextPoint, distance, tracer) {
			continue
		}

		radius := 7 * wavelength

		angleMin, angleMax, ok := cylinderDeflectionInterval(point, edgeDirection, incidentDirection, nextEdge, radius)
		if !ok || angleMax <= angleMin {
			continue
		}

		weight := math.Max(0, DAPDFIntegral(angleMin, angleMax, b))
		if weight <= 0 {
			continue
		}

		forwarding = append(forwarding, forwardingTarget{
			ref:      diffractionEdgeRef{index: nextID, edge: nextEdge},
			point:    nextPoint,
			distance: distance,
			weight:   weight,
		})
		totalWeight += weight
	}

	weightScale := 1.0
	if totalWeight > 1 {
		weightScale = 1 / totalWeight
	}

	for _, next := range forwarding {
		forwardedEnergy := incomingEnergy * next.weight * weightScale

		forwardedEnergy = attenuateEnergyByAir([]float64{forwardedEnergy}, []float64{frequency}, next.distance)[0]
		if forwardedEnergy <= cullThreshold {
			continue
		}

		nextVisited := make(map[int]bool, len(visited)+1)
		for id := range visited {
			nextVisited[id] = true
		}

		nextVisited[next.ref.index] = true

		deposits = append(deposits, dapdfForward(
			next.ref.index,
			next.point,
			next.ref.edge.Direction,
			next.point.Sub(point).Normalize(),
			0,
			bandIndex,
			frequency,
			forwardedEnergy,
			pathLength+next.distance,
			delaySeconds,
			depth+1,
			nextVisited,
			index,
			tracer,
			receiver,
			surfaces,
			config,
			cullThreshold,
		)...)
	}

	return deposits
}

func maximumDeflectionRadius(speedOfSound float64, centerFreqs []float64) float64 {
	if speedOfSound <= 0 {
		return 0
	}

	var radius float64

	if len(centerFreqs) == 0 {
		return 7 * speedOfSound
	}

	for _, frequency := range centerFreqs {
		if frequency > 0 {
			radius = math.Max(radius, 7*speedOfSound/frequency)
		}
	}

	return radius
}

func bandFrequency(centerFreqs []float64, bandIndex int) float64 {
	if bandIndex >= 0 && bandIndex < len(centerFreqs) {
		return centerFreqs[bandIndex]
	}

	return float64(bandIndex + 1)
}

func segmentVisibleToTarget(start, target geometry.Vec3, targetDistance float64, tracer Tracer) bool {
	if tracer == nil || targetDistance <= wallEpsilon {
		return true
	}

	direction := target.Sub(start).Normalize()
	if direction == geometry.Vec3Zero {
		return false
	}

	origin := start.Add(direction.Scale(10 * wallEpsilon))
	hitPoint, _, _, hit := tracer.NextHit(geometry.NewRay(origin, direction))

	return !hit || origin.Distance(hitPoint) >= targetDistance-10*wallEpsilon
}

func deflectionAngle(edgeDirection, incidentDirection, targetDirection geometry.Vec3) (float64, bool) {
	axis := edgeDirection.Normalize()
	reference := incidentDirection.Sub(axis.Scale(incidentDirection.Dot(axis))).Normalize()

	target := targetDirection.Sub(axis.Scale(targetDirection.Dot(axis))).Normalize()
	if axis == geometry.Vec3Zero || reference == geometry.Vec3Zero || target == geometry.Vec3Zero {
		return 0, false
	}

	return math.Atan2(axis.Dot(reference.Cross(target)), reference.Dot(target)), true
}

func targetDeflectionInterval(
	origin, edgeDirection, incidentDirection geometry.Vec3,
	target dapdfTarget,
) (float64, float64, bool) {
	centerAngle, ok := deflectionAngle(edgeDirection, incidentDirection, target.center.Sub(origin))
	if !ok {
		return 0, 0, false
	}

	if target.radius > 0 {
		distance := origin.Distance(target.center)
		if distance <= 0 {
			return 0, 0, false
		}

		halfSpan := math.Asin(math.Min(1, target.radius/distance))

		return centerAngle - halfSpan, centerAngle + halfSpan, halfSpan > 0
	}

	return projectedDeflectionInterval(origin, edgeDirection, incidentDirection, centerAngle, target.polygon, 0)
}

func cylinderDeflectionInterval(
	origin, edgeDirection, incidentDirection geometry.Vec3,
	edge geometry.DiffractionEdge,
	radius float64,
) (float64, float64, bool) {
	center := edge.Start.Add(edge.End.Sub(edge.Start).Scale(0.5))

	centerAngle, ok := deflectionAngle(edgeDirection, incidentDirection, center.Sub(origin))
	if !ok {
		return 0, 0, false
	}

	distance := origin.Distance(center)

	padding := 0.0
	if distance > 0 {
		padding = math.Asin(math.Min(1, radius/distance))
	}

	return projectedDeflectionInterval(origin, edgeDirection, incidentDirection, centerAngle, []geometry.Vec3{edge.Start, edge.End}, padding)
}

func projectedDeflectionInterval(
	origin, edgeDirection, incidentDirection geometry.Vec3,
	centerAngle float64,
	points []geometry.Vec3,
	padding float64,
) (float64, float64, bool) {
	if len(points) == 0 {
		return 0, 0, false
	}

	minimum, maximum := 0.0, 0.0
	valid := false

	for _, point := range points {
		angle, ok := deflectionAngle(edgeDirection, incidentDirection, point.Sub(origin))
		if !ok {
			continue
		}

		relative := wrapAngle(angle - centerAngle)
		minimum = math.Min(minimum, relative)
		maximum = math.Max(maximum, relative)
		valid = true
	}

	return centerAngle + minimum - padding, centerAngle + maximum + padding, valid
}

func wrapAngle(angle float64) float64 {
	for angle > math.Pi {
		angle -= 2 * math.Pi
	}

	for angle < -math.Pi {
		angle += 2 * math.Pi
	}

	return angle
}
