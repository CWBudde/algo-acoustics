package geometry

import (
	"math"
	"math/cmplx"
)

// WedgeDiffraction evaluates the Kouyoumjian-Pathak UTD diffraction
// coefficient for a rigid scalar wedge.
//
// The implementation follows the four-term form used for the incident shadow
// boundary and the two reflection shadow boundaries. The caller supplies the
// path-dependent distance parameter L; the helper functions below provide the
// companion spreading factor and geometric L term used by later integration
// stages.
func WedgeDiffraction(phi, phiPrime, betaZero, n, k, l float64) complex128 {
	if n <= 0 || k <= 0 || l < 0 {
		return 0
	}

	sinBetaZero := math.Sin(betaZero)
	if math.Abs(sinBetaZero) < 1e-15 {
		return 0
	}

	prefactor := -cmplx.Exp(complex(0, -math.Pi/4)) / complex(2*n*math.Sqrt(2*math.Pi*k)*sinBetaZero, 0)

	betaMinus := phi - phiPrime
	betaPlus := phi + phiPrime

	return prefactor * (cot((math.Pi+betaMinus)/(2*n))*FresnelTransition(k*l*wedgeTransitionArgumentPlus(betaMinus, n)) +
		cot((math.Pi-betaMinus)/(2*n))*FresnelTransition(k*l*wedgeTransitionArgumentMinus(betaMinus, n)) +
		cot((math.Pi+betaPlus)/(2*n))*FresnelTransition(k*l*wedgeTransitionArgumentPlus(betaPlus, n)) +
		cot((math.Pi-betaPlus)/(2*n))*FresnelTransition(k*l*wedgeTransitionArgumentMinus(betaPlus, n)))
}

func wedgeSpreadingFactor(s, sPrime float64) float64 {
	if s <= 0 || sPrime <= 0 {
		return 0
	}

	return math.Sqrt(1 / (s * sPrime * (s + sPrime)))
}

func wedgeDistanceParameter(s, sPrime, betaZero float64) float64 {
	if s <= 0 || sPrime <= 0 {
		return 0
	}

	sinBetaZero := math.Sin(betaZero)
	if math.Abs(sinBetaZero) < 1e-15 {
		return 0
	}

	return (s * sPrime / (s + sPrime)) * sinBetaZero * sinBetaZero
}

// WedgeSpreadingFactor returns the geometric spreading factor used by the UTD
// diffraction contribution.
func WedgeSpreadingFactor(s, sPrime float64) float64 {
	return wedgeSpreadingFactor(s, sPrime)
}

// WedgeDistanceParameter returns the path-dependent distance parameter used by
// the UTD transition functions.
func WedgeDistanceParameter(s, sPrime, betaZero float64) float64 {
	return wedgeDistanceParameter(s, sPrime, betaZero)
}

func wedgeTransitionArgumentPlus(beta, n float64) float64 {
	if n <= 0 {
		return 0
	}

	period := 2 * n * math.Pi
	target := (beta + math.Pi) / period
	N := math.Round(target)

	c := math.Cos((period*N - beta) / 2)

	return 2 * c * c
}

func wedgeTransitionArgumentMinus(beta, n float64) float64 {
	if n <= 0 {
		return 0
	}

	period := 2 * n * math.Pi
	target := (beta - math.Pi) / period
	N := math.Round(target)

	c := math.Cos((period*N - beta) / 2)

	return 2 * c * c
}

func cot(x float64) complex128 {
	sinX := math.Sin(x)
	if math.Abs(sinX) < 1e-15 {
		return complex(math.Copysign(math.Inf(1), math.Cos(x)), 0)
	}

	return complex(math.Cos(x)/sinX, 0)
}
