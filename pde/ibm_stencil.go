package pde

import (
	"math"
)

// WallBCType selects the boundary condition model at wall surfaces.
type WallBCType int

const (
	// WallRigid is a perfectly rigid wall (Neumann: ∂p/∂n = 0).
	// The ghost pressure mirrors the interior node: p_ghost = p_node.
	WallRigid WallBCType = iota

	// WallImpedance uses a frequency-independent real-valued pressure
	// reflection coefficient R ∈ [−1, 1].  R = 1 is rigid, R = −1 is
	// pressure-release, R = 0 is perfectly absorbing.
	WallImpedance

	// WallADE uses an auxiliary differential equation for frequency-dependent
	// impedance modelling at each boundary node.
	WallADE
)

// WallBC configures the boundary condition for all walls.
type WallBC struct {
	Type WallBCType

	// R is the pressure reflection coefficient for WallImpedance.
	R float64

	// ADE coefficients for frequency-dependent impedance.
	// The surface impedance is modelled as a rational function in s (Laplace):
	//   Z(s) = Z_inf + Σ_k  c_k / (s + λ_k)
	// where Λ are the poles and C are the residues.
	ADEPoles    []float64 // λ_k  (positive reals, decay rates)
	ADEResidues []float64 // c_k  (real residues)
}

// RigidWallBC returns a rigid-wall boundary condition (Neumann).
func RigidWallBC() WallBC { return WallBC{Type: WallRigid, R: 1} }

// ImpedanceWallBC returns a wall BC with a constant reflection coefficient.
func ImpedanceWallBC(r float64) WallBC { return WallBC{Type: WallImpedance, R: r} }

// adeState holds auxiliary variables for a single ADE boundary node.
type adeState struct {
	// Phi[k] is the auxiliary variable for pole k.
	Phi []float64
}

// IBMStencil applies the 3D Laplacian operator on an IBMGrid with
// modified coefficients at boundary nodes (Hamilton–Bilbao method).
type IBMStencil struct {
	Grid *IBMGrid
	BC   WallBC

	// ADE auxiliary state per boundary node (only when BC.Type == WallADE).
	adeStates map[int]*adeState
}

// NewIBMStencil creates a stencil operator for the given grid and wall BC.
func NewIBMStencil(g *IBMGrid, bc WallBC) *IBMStencil {
	s := &IBMStencil{Grid: g, BC: bc}

	if bc.Type == WallADE && len(bc.ADEPoles) > 0 {
		s.adeStates = make(map[int]*adeState, len(g.Boundary))

		for idx := range g.Boundary {
			s.adeStates[idx] = &adeState{
				Phi: make([]float64, len(bc.ADEPoles)),
			}
		}
	}

	return s
}

// ApplyLaplacian computes dst = −∇²(src) on the IBM grid.
//
// Interior nodes use the standard 7-point stencil.
// Boundary nodes use the Hamilton–Bilbao modification with sub-cell
// wall positions from the grid's BoundaryInfo.
// Exterior nodes are set to zero.
//
// The sign convention matches algo-pde/fd.Apply3D: positive-definite
// negative Laplacian, so eigenvalues are positive.
func (s *IBMStencil) ApplyLaplacian(dst, src []float64) {
	g := s.Grid
	invH2 := 1.0 / (g.H * g.H)
	nyNz := g.Ny * g.Nz

	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				idx := ix*nyNz + iy*g.Nz + iz

				switch g.Class[idx] {
				case Exterior:
					dst[idx] = 0

				case Interior:
					u := src[idx]
					// Standard 7-point stencil. All 6 neighbors are guaranteed
					// interior or boundary (never exterior or out-of-bounds)
					// because interior nodes have no exterior neighbors.
					xm := src[idx-nyNz]
					xp := src[idx+nyNz]
					ym := src[idx-g.Nz]
					yp := src[idx+g.Nz]
					zm := src[idx-1]
					zp := src[idx+1]

					dst[idx] = (6*u - xm - xp - ym - yp - zm - zp) * invH2

				case Boundary:
					s.applyBoundaryStencil(dst, src, idx, ix, iy, iz)
				}
			}
		}
	}
}

// applyBoundaryStencil computes the modified Laplacian at a boundary node
// using the Shortley–Weller / Hamilton–Bilbao scheme.
//
// For each axis we determine the effective distances to the two neighbors
// (or walls).  The general non-uniform second derivative is:
//
//	d²p/dx² ≈ 2/(h²) · [ p_neg/(α(α+β)) − p/(αβ) + p_pos/(β(α+β)) ]
//
// where α is the distance in grid units to the negative-side value and β
// to the positive-side value.  For interior neighbors α or β = 1; for a
// wall at fractional distance θ the distance is θ and the value comes
// from the boundary condition (ghost value).
//
// Corner nodes (walls on multiple axes) are handled naturally: each axis
// is treated independently with its own α/β.
func (s *IBMStencil) applyBoundaryStencil(dst, src []float64, idx, ix, iy, iz int) {
	g := s.Grid
	bi := g.Boundary[idx]
	u := src[idx]
	invH2 := 1.0 / (g.H * g.H)

	deltas := [3][2]int{
		{-g.Ny * g.Nz, g.Ny * g.Nz},
		{-g.Nz, g.Nz},
		{-1, 1},
	}

	coords := [3]int{ix, iy, iz}
	sizes := [3]int{g.Nx, g.Ny, g.Nz}

	var lap float64

	for a := range 3 {
		// Determine distance and value for negative (d=0) and positive (d=1) sides.
		var dist [2]float64
		var pVal [2]float64

		for d := range 2 {
			theta := bi.Frac[a][d]

			if theta > 0 {
				// Wall at fractional distance θ.
				dist[d] = theta
				pVal[d] = s.ghostValue(u, idx, a, d)
			} else {
				// Interior/boundary neighbor at distance 1.
				dist[d] = 1.0
				pVal[d] = s.neighborValue(src, idx, deltas[a][d], coords[a], sizes[a], d)
			}
		}

		alpha := dist[0] // negative side
		beta := dist[1]  // positive side

		// Negative Laplacian (−∇²) contribution for this axis via Shortley–Weller.
		// Standard form of second derivative: 2/(h²) · [p_neg/(α(α+β)) − p/(αβ) + p_pos/(β(α+β))]
		// Negated to match the positive-definite sign convention of the interior stencil.
		lap += 2.0 * invH2 * (u/(alpha*beta) - pVal[0]/(alpha*(alpha+beta)) - pVal[1]/(beta*(alpha+beta)))
	}

	dst[idx] = lap
}

// neighborValue reads the pressure at the neighbor of idx along the given
// delta, returning 0 if the neighbor is exterior or out-of-bounds.
func (s *IBMStencil) neighborValue(src []float64, idx, delta, coord, size, dir int) float64 {
	nc := coord
	if dir == 0 {
		nc--
	} else {
		nc++
	}

	if nc < 0 || nc >= size {
		return 0
	}

	nIdx := idx + delta

	if s.Grid.Class[nIdx] == Exterior {
		return 0
	}

	return src[nIdx]
}

// ghostValue returns the ghost pressure at the wall for a given boundary
// node, axis, and direction.
func (s *IBMStencil) ghostValue(pNode float64, idx, axis, dir int) float64 {
	switch s.BC.Type {
	case WallRigid:
		// Neumann: ∂p/∂n = 0 → ghost mirrors the node.
		return pNode

	case WallImpedance:
		// p_ghost = R · p_node.
		return s.BC.R * pNode

	case WallADE:
		return s.adeGhostValue(pNode, idx, axis, dir)

	default:
		return pNode
	}
}

// adeGhostValue computes the ghost value using the ADE auxiliary variables.
// The ADE models frequency-dependent impedance as:
//
//	Z(s) = Z_inf + Σ_k c_k / (s + λ_k)
//
// Each auxiliary variable ψ_k satisfies: dψ_k/dt = −λ_k·ψ_k + c_k·∂p/∂n
// The ghost value incorporates the accumulated impedance response.
func (s *IBMStencil) adeGhostValue(pNode float64, idx, _, _ int) float64 {
	st, ok := s.adeStates[idx]
	if !ok || len(st.Phi) == 0 {
		return pNode // fallback to rigid
	}

	// Sum auxiliary variable contributions to get the impedance correction.
	correction := 0.0
	for k := range st.Phi {
		correction += st.Phi[k]
	}

	// Ghost value: rigid component + impedance correction.
	return pNode - correction
}

// UpdateADE advances the ADE auxiliary variables by one time step dt.
// Must be called once per FDTD time step, after ApplyLaplacian.
// For each boundary node, updates ψ_k using the pressure gradient.
func (s *IBMStencil) UpdateADE(src []float64, dt float64) {
	if s.BC.Type != WallADE || s.adeStates == nil {
		return
	}

	g := s.Grid

	for idx, bi := range g.Boundary {
		st := s.adeStates[idx]
		if st == nil {
			continue
		}

		// Estimate ∂p/∂n at the wall using the nearest-wall normal.
		// Use one-sided difference toward the wall.
		pNode := src[idx]

		// Find the axis/direction with the smallest wall fraction (closest wall).
		minFrac := math.Inf(1)

		for a := range 3 {
			for d := range 2 {
				if f := bi.Frac[a][d]; f > 0 && f < minFrac {
					minFrac = f
				}
			}
		}

		// Approximate ∂p/∂n ≈ 0 for nodes very close to rigid-like walls.
		dpdn := 0.0
		if minFrac > 0 && !math.IsInf(minFrac, 1) {
			// Simple estimate: pressure drops to ~0 at wall over θ·h.
			dpdn = -pNode / (minFrac * g.H)
		}

		// Update each auxiliary variable: ψ_k^{n+1} = ψ_k^n · e^{−λ_k·dt} + c_k · dpdn · dt
		for k := range st.Phi {
			decay := math.Exp(-s.BC.ADEPoles[k] * dt)
			st.Phi[k] = st.Phi[k]*decay + s.BC.ADEResidues[k]*dpdn*dt
		}
	}
}

// CFLLimit returns the maximum stable time step for the FDTD scheme
// on this grid with sound speed c.
//
// For the standard 3D Cartesian stencil: dt_max = h / (c · √3).
// The Hamilton–Bilbao modification can reduce this; we apply a safety
// factor based on the minimum fractional distance in the grid.
func (s *IBMStencil) CFLLimit(soundSpeed float64) float64 {
	dtStandard := s.Grid.H / (soundSpeed * math.Sqrt(3))

	// Find minimum fractional distance across all boundary nodes.
	minTheta := 1.0

	for _, bi := range s.Grid.Boundary {
		for a := range 3 {
			for d := range 2 {
				if f := bi.Frac[a][d]; f > 0 && f < minTheta {
					minTheta = f
				}
			}
		}
	}

	// The Shortley–Weller stencil coefficient scales as 1/θ, which
	// effectively reduces the stable time step.  Conservative bound:
	// dt_max ≈ dt_standard · √(θ_min) for θ_min < 1.
	if minTheta < 1 {
		return dtStandard * math.Sqrt(minTheta)
	}

	return dtStandard
}

// FDTDStep performs one second-order FDTD time step:
//
//	p^{n+1} = 2·p^n − p^{n-1} − (c·dt)² · L(p^n)
//
// where L is the negative Laplacian (positive-definite).
// pCur, pPrev, and pNext are all sized Nx·Ny·Nz.
// The caller is responsible for ensuring dt ≤ CFLLimit(c).
func (s *IBMStencil) FDTDStep(pNext, pCur, pPrev []float64, c, dt float64) {
	n := s.Grid.Nx * s.Grid.Ny * s.Grid.Nz

	// Compute −∇²(pCur) into pNext (used as scratch).
	s.ApplyLaplacian(pNext, pCur)

	cdt2 := c * c * dt * dt

	for i := range n {
		if s.Grid.Class[i] == Exterior {
			pNext[i] = 0

			continue
		}

		pNext[i] = 2*pCur[i] - pPrev[i] - cdt2*pNext[i]
	}

	// Update ADE state if applicable.
	if s.BC.Type == WallADE {
		s.UpdateADE(pCur, dt)
	}
}
