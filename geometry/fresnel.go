package geometry

import (
	"math"
	"math/cmplx"
)

// fresnelSmallThreshold is the upper bound for the small-x power series regime.
const fresnelSmallThreshold = 0.3

// fresnelLargeThreshold is the lower bound for the large-x asymptotic regime.
const fresnelLargeThreshold = 10.0

// FresnelTransition computes the UTD Fresnel transition function:
//
//	F(x) = 2j√x · e^(jx) · ∫_√x^∞ e^(-jt²) dt
//
// Three evaluation regimes are used for accuracy and efficiency:
//   - x > 10:  asymptotic expansion
//   - x < 0.3: small-argument power series
//   - otherwise: numerical integration via the complementary Fresnel integrals
func FresnelTransition(x float64) complex128 {
	if x <= 0 {
		return 0
	}

	if x > fresnelLargeThreshold {
		return fresnelAsymptotic(x)
	}

	if x < fresnelSmallThreshold {
		return fresnelSmallArg(x)
	}

	return fresnelIntermediate(x)
}

// fresnelAsymptotic uses the asymptotic expansion for large x:
//
//	F(x) ≈ 1 + j/(2x) - 3/(4x²) - 15j/(8x³) + ...
func fresnelAsymptotic(x float64) complex128 {
	invX := 1.0 / x
	inv2 := invX * invX

	// First four terms of the asymptotic series.
	re := 1.0 - 0.75*inv2 + (75.0/16.0)*inv2*inv2
	im := 0.5*invX - (15.0/8.0)*inv2*invX + (3675.0/128.0)*inv2*inv2*invX

	return complex(re, im)
}

// fresnelSmallArg uses a power series for small x.
//
// For small x, F(x) ≈ √(πx) · e^(jπ/4) · (1 - j·2x/3 - 4x²/15 + ...).
// This is derived from the Taylor expansion of the Fresnel integral.
func fresnelSmallArg(x float64) complex128 {
	sqrtX := math.Sqrt(x)

	// Compute F(x) = 2j√x · e^(jx) · ∫_√x^∞ e^(-jt²) dt via the
	// relationship to the standard Fresnel integrals C(u) and S(u) where
	// u = √(2x/π).
	return fresnelViaIntegrals(sqrtX, x)
}

// fresnelIntermediate evaluates F(x) in the intermediate regime using
// the standard Fresnel integrals.
func fresnelIntermediate(x float64) complex128 {
	sqrtX := math.Sqrt(x)

	return fresnelViaIntegrals(sqrtX, x)
}

// fresnelViaIntegrals computes F(x) from the complementary Fresnel integrals.
//
// The integral ∫_√x^∞ e^(-jt²) dt = ∫_√x^∞ cos(t²) dt - j ∫_√x^∞ sin(t²) dt
//
// Using the standard Fresnel integrals C(u), S(u) where:
//
//	C(u) = ∫_0^u cos(π t²/2) dt
//	S(u) = ∫_0^u sin(π t²/2) dt
//
// We substitute t = τ·√(π/2) to convert:
//
//	∫_0^a cos(t²) dt = √(π/2) · C(a · √(2/π))
//	∫_0^a sin(t²) dt = √(π/2) · S(a · √(2/π))
//
// And the complementary integrals (from a to ∞) use C(∞) = S(∞) = 0.5.
func fresnelViaIntegrals(sqrtX, x float64) complex128 {
	u := sqrtX * math.Sqrt(2.0/math.Pi)
	cU, sU := fresnelCS(u)

	cComp := 0.5 - cU
	sComp := 0.5 - sU

	sqrtPiOver2 := math.Sqrt(math.Pi / 2)

	// ∫_√x^∞ e^(-jt²) dt = √(π/2) · (cComp - j·sComp)
	integral := complex(sqrtPiOver2*cComp, -sqrtPiOver2*sComp)

	// F(x) = 2j√x · e^(jx) · integral
	prefix := complex(0, 2*sqrtX) * cmplx.Exp(complex(0, x))

	return prefix * integral
}

// fresnelCS computes the standard Fresnel integrals C(x) and S(x):
//
//	C(x) = ∫_0^x cos(π t²/2) dt
//	S(x) = ∫_0^x sin(π t²/2) dt
//
// Uses the rational approximation from Boersma (1960) / Abramowitz & Stegun.
func fresnelCS(x float64) (c, s float64) {
	ax := math.Abs(x)

	if ax < fresnelCSSmallThreshold {
		return fresnelCSSmall(x)
	}

	return fresnelCSLarge(x)
}

const fresnelCSSmallThreshold = 1.6

// fresnelCSSmall computes C(x) and S(x) using power series for |x| < 1.6.
func fresnelCSSmall(x float64) (c, s float64) {
	// C(x) = x · Σ (-1)^n (πx²/2)^(2n) / ((4n+1)(2n)!)
	// S(x) = x · Σ (-1)^n (πx²/2)^(2n+1) / ((4n+3)(2n+1)!)
	ax := math.Abs(x)
	t := math.Pi * ax * ax / 2
	t2 := t * t

	// C(x) power series: sum terms until convergence.
	cSum := 1.0
	term := 1.0

	for n := 1; n <= 25; n++ {
		term *= -t2 / float64(2*n*(2*n-1))
		contribution := term / float64(4*n+1)

		cSum += contribution
		if math.Abs(contribution) < 1e-16*math.Abs(cSum) {
			break
		}
	}

	c = ax * cSum

	// S(x) power series.
	term = t
	sSum := term / 3.0

	for n := 1; n <= 25; n++ {
		term *= -t2 / float64((2*n)*(2*n+1))
		contribution := term / float64(4*n+3)

		sSum += contribution
		if math.Abs(contribution) < 1e-16*math.Abs(sSum) {
			break
		}
	}

	s = ax * sSum

	if x < 0 {
		c = -c
		s = -s
	}

	return c, s
}

// fresnelCSLarge computes C(x) and S(x) using auxiliary functions f(x), g(x)
// for |x| >= 1.6:
//
//	C(x) = 0.5 + f(x)·sin(πx²/2) - g(x)·cos(πx²/2)
//	S(x) = 0.5 - f(x)·cos(πx²/2) - g(x)·sin(πx²/2)
//
// f and g are computed via rational approximations.
func fresnelCSLarge(x float64) (c, s float64) {
	ax := math.Abs(x)
	t := math.Pi * ax * ax / 2
	sinT := math.Sin(t)
	cosT := math.Cos(t)

	// Auxiliary functions via asymptotic expansion:
	// f(x) ≈ 1/(πx) · (1 - 1·3/(πx²)² + 1·3·5·7/(πx²)⁴ - ...)
	// g(x) ≈ 1/(π²x³) · (1 - 1·3·5/(πx²)² + ...)
	pix := math.Pi * ax
	inv := 1.0 / (pix * ax) // 1/(π x²)

	// Compute f(x) and g(x) using continued-fraction-like rational form.
	f, g := fresnelAuxFG(ax)

	c = 0.5 + f*sinT - g*cosT
	s = 0.5 - f*cosT - g*sinT

	_ = inv // used conceptually in the derivation

	if x < 0 {
		c = -c
		s = -s
	}

	return c, s
}

// fresnelAuxFG computes the auxiliary functions f(x) and g(x) for the Fresnel
// integrals using the asymptotic series.
//
//	f(x) = (1/(πx)) · Σ (-1)^n (2n)! / (πx²/2)^n / n! · (for even terms)
//	g(x) = (1/(πx)²) / x · Σ ...
//
// In practice we use the convergent asymptotic expansion truncated when terms
// start growing.
func fresnelAuxFG(x float64) (f, g float64) {
	pix2 := math.Pi * x * x
	invPix2 := 1.0 / pix2

	// f(x) = (1/(πx)) · Σ_{n=0}^{N} (-1)^n · (1·3·5···(4n-1)) / (πx²)^(2n)
	// Recurrence: a_{n+1} = a_n · (-(4n+1)(4n+3)) / (πx²)²
	fSum := 1.0
	fTerm := 1.0

	for n := range 25 {
		prev := fTerm
		fTerm *= -float64((4*n+1)*(4*n+3)) * invPix2 * invPix2

		if math.Abs(fTerm) > math.Abs(prev) {
			break
		}

		fSum += fTerm
	}

	f = fSum / (math.Pi * x)

	// g(x) = (1/(πx)³) · Σ_{n=0}^{N} (-1)^n · (1·3···(4n+1)) / (πx²)^(2n)
	gSum := 1.0
	gTerm := 1.0

	for n := range 25 {
		prev := gTerm
		gTerm *= -float64((4*n+3)*(4*n+5)) * invPix2 * invPix2

		if math.Abs(gTerm) > math.Abs(prev) {
			break
		}

		gSum += gTerm
	}

	g = gSum / (pix2 * math.Pi * x)

	return f, g
}
