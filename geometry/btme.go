package geometry

import (
	"errors"
	"math"
	"math/cmplx"
)

const (
	btmeMinimumSteps = 512
	btmeMaximumSteps = 131072
)

// BTMETransfer evaluates the finite-edge Biot-Tolstoy-Medwin impulse
// response at one frequency. The edge is treated as open at its endpoints;
// source and receiver must have a finite interior Fermat point on the edge.
//
//nolint:cyclop,funlen // Validation and quadrature stay together to keep the public calculation auditable.
func BTMETransfer(source, receiver Vec3, edge DiffractionEdge, frequencyHz, speedOfSound float64) (complex128, error) {
	if frequencyHz <= 0 || math.IsNaN(frequencyHz) || math.IsInf(frequencyHz, 0) {
		return 0, errors.New("frequency must be finite and positive")
	}

	if speedOfSound <= 0 || math.IsNaN(speedOfSound) || math.IsInf(speedOfSound, 0) {
		return 0, errors.New("speed of sound must be finite and positive")
	}

	if edge.Length <= diffractionPathEpsilon {
		return 0, errors.New("diffraction edge must have positive length")
	}

	axis := edge.Direction.Normalize()
	if axis == Vec3Zero || edge.WedgeIndex <= 0 {
		return 0, errors.New("diffraction edge has invalid geometry")
	}

	point, _, ok := FindDiffractionPoint(source, receiver, edge)
	if !ok {
		return 0, errors.New("source and receiver have no interior finite-edge Fermat point")
	}

	sourceProjection := edge.Start.Add(axis.Scale(source.Sub(edge.Start).Dot(axis)))
	receiverProjection := edge.Start.Add(axis.Scale(receiver.Sub(edge.Start).Dot(axis)))
	rSource := source.Distance(sourceProjection)
	rReceiver := receiver.Distance(receiverProjection)

	if rSource <= diffractionPathEpsilon || rReceiver <= diffractionPathEpsilon {
		return 0, errors.New("source and receiver must not lie on the diffraction edge axis")
	}

	z := receiverProjection.Sub(sourceProjection).Dot(axis)
	onsetDistance := source.Distance(point) + receiver.Distance(point)
	endpointDistances := [2]float64{
		source.Distance(edge.Start) + receiver.Distance(edge.Start),
		source.Distance(edge.End) + receiver.Distance(edge.End),
	}
	minimumEndpointDistance := math.Min(endpointDistances[0], endpointDistances[1])
	maximumEndpointDistance := math.Max(endpointDistances[0], endpointDistances[1])

	if maximumEndpointDistance <= onsetDistance+diffractionPathEpsilon {
		return 0, errors.New("finite diffraction edge has no non-zero response interval")
	}

	etaAtDistance := func(distance float64) float64 {
		argument := (distance*distance - (rSource*rSource + rReceiver*rReceiver + z*z)) / (2 * rSource * rReceiver)
		if argument < 1 {
			argument = 1
		}

		return math.Acosh(argument)
	}
	etaMinimum := etaAtDistance(minimumEndpointDistance)
	etaMaximum := etaAtDistance(maximumEndpointDistance)

	if etaMaximum <= 0 || math.IsNaN(etaMaximum) || math.IsInf(etaMaximum, 0) {
		return 0, errors.New("finite diffraction edge produced an invalid response interval")
	}

	thetaSource := btmeEdgeAngle(edge, source.Sub(sourceProjection))
	thetaReceiver := btmeEdgeAngle(edge, receiver.Sub(receiverProjection))
	nu := 1 / edge.WedgeIndex
	angularFrequency := 2 * math.Pi * frequencyHz
	duration := (maximumEndpointDistance - onsetDistance) / speedOfSound
	steps := max(btmeMinimumSteps, int(math.Ceil(frequencyHz*duration*16)))
	steps = min(steps, btmeMaximumSteps)
	step := etaMaximum / float64(steps)

	var transfer complex128

	for index := range steps {
		eta := (float64(index) + 0.5) * step
		distance := math.Sqrt(rSource*rSource + rReceiver*rReceiver + z*z + 2*rSource*rReceiver*math.Cosh(eta))
		timeSeconds := distance / speedOfSound
		beta := btmeBeta(nu, eta, thetaSource, thetaReceiver)
		window := 1.0

		if eta > etaMinimum {
			window = 0.5
		}

		impulseDEta := -nu * beta * window / (2 * math.Pi * speedOfSound * timeSeconds)
		transfer += complex(impulseDEta*step, 0) * cmplx.Exp(complex(0, -angularFrequency*timeSeconds))
	}

	if math.IsNaN(real(transfer)) || math.IsNaN(imag(transfer)) || math.IsInf(real(transfer), 0) || math.IsInf(imag(transfer), 0) {
		return 0, errors.New("BTME integration produced a non-finite transfer")
	}

	return transfer, nil
}

func btmeBeta(nu, eta, thetaSource, thetaReceiver float64) float64 {
	cosh := math.Cosh(nu * eta)
	beta := 0.0

	for _, sourceSign := range [...]float64{-1, 1} {
		for _, receiverSign := range [...]float64{-1, 1} {
			angle := nu * (math.Pi + sourceSign*thetaSource + receiverSign*thetaReceiver)
			denominator := cosh - math.Cos(angle)

			if math.Abs(denominator) <= diffractionPathEpsilon {
				continue
			}

			beta += math.Sin(angle) / denominator
		}
	}

	return beta
}

func btmeEdgeAngle(edge DiffractionEdge, transverse Vec3) float64 {
	axis := edge.Direction.Normalize()
	reference := edge.FaceONormal.Normalize()
	basis := axis.Cross(reference).Normalize()

	if reference == Vec3Zero || basis == Vec3Zero || transverse.Norm() <= diffractionPathEpsilon {
		return 0
	}

	angle := math.Atan2(transverse.Dot(basis), transverse.Dot(reference))
	if angle < 0 {
		angle += 2 * math.Pi
	}

	return angle
}
