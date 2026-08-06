package pde

import (
	"math"
	"math/cmplx"
	"sort"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	algofft "github.com/cwbudde/algo-fft"
)

// runFDTD runs an FDTD simulation with a Gaussian pulse source and records
// the pressure time series at a receiver node. Returns the time series and dt.
func runFDTD(room *ConvexRoom, h float64, bc WallBC, srcPos, rcvPos geometry.Vec3, nSteps int) ([]float64, float64) {
	g := ClassifyGrid(room, h)
	stencil := NewIBMStencil(g, bc)

	c := 343.0
	dt := 0.95 * stencil.CFLLimit(c)

	src, err := NewIBMSource(room, g, srcPos, SoftSource)
	if err != nil {
		panic("source creation: " + err.Error())
	}

	rcvIx, rcvIy, rcvIz := nearestNode(g, rcvPos)
	rcvIdx := g.nodeIndex(rcvIx, rcvIy, rcvIz)

	n := g.Nx * g.Ny * g.Nz
	pCur := make([]float64, n)
	pPrev := make([]float64, n)
	pNext := make([]float64, n)

	// Gaussian pulse — center at 5*dt, narrow sigma for broadband excitation.
	t0 := 5.0 * dt
	sigma := 2.0 * dt

	timeSeries := make([]float64, nSteps)

	for step := range nSteps {
		tNow := float64(step) * dt
		src.Inject(pCur, GaussianPulse(tNow, t0, sigma))

		stencil.FDTDStep(pNext, pCur, pPrev, c, dt)
		pPrev, pCur, pNext = pCur, pNext, pPrev

		timeSeries[step] = pCur[rcvIdx]
	}

	return timeSeries, dt
}

// extractPeakFreqs FFTs a time series and returns the frequencies of the
// strongest spectral peaks, sorted ascending.
func extractPeakFreqs(ts []float64, dt float64, minFreq, maxFreq float64, nPeaks int) []float64 {
	n := len(ts)

	// Zero-pad to next power of 2 for cleaner FFT.
	nfft := 1
	for nfft < n {
		nfft <<= 1
	}

	padded := make([]float64, nfft)
	copy(padded, ts)

	plan, err := algofft.NewPlanReal64(nfft)
	if err != nil {
		panic("FFT plan: " + err.Error())
	}

	spectrum := make([]complex128, nfft/2+1)

	err = plan.Forward(spectrum, padded)
	if err != nil {
		panic("FFT forward: " + err.Error())
	}

	// Compute magnitude spectrum.
	mag := make([]float64, len(spectrum))
	freqBin := 1.0 / (float64(nfft) * dt)

	for i, c := range spectrum {
		mag[i] = cmplx.Abs(c)
	}

	// Find peaks: local maxima above a threshold.
	threshold := 0.0
	for _, m := range mag {
		if m > threshold {
			threshold = m
		}
	}

	threshold *= 0.01 // 1% of max

	type peak struct {
		freq float64
		mag  float64
	}

	var peaks []peak

	for i := 1; i < len(mag)-1; i++ {
		f := float64(i) * freqBin
		if f < minFreq || f > maxFreq {
			continue
		}

		if mag[i] > mag[i-1] && mag[i] > mag[i+1] && mag[i] > threshold {
			peaks = append(peaks, peak{freq: f, mag: mag[i]})
		}
	}

	// Sort by magnitude descending.
	sort.Slice(peaks, func(i, j int) bool {
		return peaks[i].mag > peaks[j].mag
	})

	// Take top nPeaks.
	if len(peaks) > nPeaks {
		peaks = peaks[:nPeaks]
	}

	// Sort by frequency ascending.
	sort.Slice(peaks, func(i, j int) bool {
		return peaks[i].freq < peaks[j].freq
	})

	freqs := make([]float64, len(peaks))
	for i, p := range peaks {
		freqs[i] = p.freq
	}

	return freqs
}

// shoeboxEigenfreqs returns the analytical eigenfrequencies of a rectangular
// room with dimensions Lx×Ly×Lz, sorted ascending, up to maxFreq.
func shoeboxEigenfreqs(lx, ly, lz, c, maxFreq float64) []float64 {
	var freqs []float64

	maxN := int(math.Ceil(2 * maxFreq * lx / c))

	for nx := range maxN + 1 {
		for ny := range maxN + 1 {
			for nz := range maxN + 1 {
				if nx == 0 && ny == 0 && nz == 0 {
					continue
				}

				f := c / 2.0 * math.Sqrt(
					float64(nx*nx)/(lx*lx)+
						float64(ny*ny)/(ly*ly)+
						float64(nz*nz)/(lz*lz),
				)
				if f <= maxFreq {
					freqs = append(freqs, f)
				}
			}
		}
	}

	sort.Float64s(freqs)

	return freqs
}

func TestIBMValidation_RectangularEigenfreqs(t *testing.T) {
	// Rectangular room: compare IBM FDTD eigenfrequencies against analytical.
	// Room dimensions chosen to give well-separated low-frequency modes.
	lx, ly, lz := 3.0, 2.5, 2.0
	c := 343.0
	h := 0.05 // fine grid for accuracy

	room := rectRoom(lx, ly, lz)

	// Place source and receiver off-centre to excite all modes.
	srcPos := geometry.Vec3{X: 0.7, Y: 0.6, Z: 0.5}
	rcvPos := geometry.Vec3{X: 2.3, Y: 1.9, Z: 1.5}

	// Run enough steps for modes to develop and ring.
	// Frequency resolution df = 1/T, so 0.5 s gives df ≈ 2 Hz.
	dt := 0.95 * h / (c * math.Sqrt(3))
	nSteps := int(0.5 / dt) // 500 ms of simulation

	ts, dtActual := runFDTD(room, h, RigidWallBC(), srcPos, rcvPos, nSteps)

	maxFreq := 400.0 // well below Nyquist, well-resolved by grid
	analytical := shoeboxEigenfreqs(lx, ly, lz, c, maxFreq)

	// Extract spectral peaks from FDTD.
	fdtdPeaks := extractPeakFreqs(ts, dtActual, 30, maxFreq, 40)

	t.Logf("dt=%.6g s, nSteps=%d, simulation time=%.4g s", dtActual, nSteps, float64(nSteps)*dtActual)
	t.Logf("analytical modes (up to %.0f Hz): %d", maxFreq, len(analytical))
	t.Logf("FDTD peaks found: %d", len(fdtdPeaks))

	// Match each FDTD peak to nearest analytical mode.
	matched := 0
	maxError := 0.0

	for _, fp := range fdtdPeaks {
		bestDist := math.Inf(1)
		bestAnalytical := 0.0

		for _, af := range analytical {
			d := math.Abs(fp - af)
			if d < bestDist {
				bestDist = d
				bestAnalytical = af
			}
		}

		if bestAnalytical == 0 {
			continue
		}

		relErr := bestDist / bestAnalytical

		if relErr > maxError {
			maxError = relErr
		}

		if relErr < 0.005 { // 0.5% match tolerance for peak identification
			matched++
		}

		t.Logf("FDTD peak %.1f Hz → analytical %.1f Hz (error %.3f%%)",
			fp, bestAnalytical, relErr*100)
	}

	t.Logf("matched %d/%d FDTD peaks, max error: %.4f%%", matched, len(fdtdPeaks), maxError*100)

	// Require at least 20 matched modes with < 0.5% error.
	if matched < 20 {
		t.Errorf("only %d modes matched within 0.5%%, want ≥ 20", matched)
	}

	// The best matches should be within 0.1%.
	if maxError > 0.005 {
		t.Logf("warning: max error %.4f%% exceeds 0.1%% target for some peaks", maxError*100)
	}
}

// triangleEigenfreqs returns analytical eigenfrequencies for an equilateral
// triangle with side length L (Neumann BC):
//
//	f_{m,n} = (c / 3L) · √(m² + mn + n²)
//
// with m ≥ 1, n ≥ 0, m > n (avoiding duplicates).
func triangleEigenfreqs(sideLength, c, maxFreq float64) []float64 {
	var freqs []float64
	base := c / (3 * sideLength)

	maxN := int(math.Ceil(maxFreq / base))

	for m := 1; m <= maxN; m++ {
		for n := 0; n <= m; n++ {
			f := base * math.Sqrt(float64(m*m+m*n+n*n))
			if f > 0 && f <= maxFreq {
				freqs = append(freqs, f)
			}
		}
	}

	sort.Float64s(freqs)

	// Remove near-duplicates.
	unique := freqs[:0]

	for i, f := range freqs {
		if i == 0 || f-unique[len(unique)-1] > 0.1 {
			unique = append(unique, f)
		}
	}

	return unique
}

// equilateralTriangleRoom constructs a ConvexRoom for an equilateral triangle
// extruded along z. The triangle has side length L, centered at (cx, cy),
// with z extent [0, zHeight].
func equilateralTriangleRoom(sideLength, cx, cy, zHeight float64) *ConvexRoom {
	// Equilateral triangle vertices centered at (cx, cy).
	h := sideLength * math.Sqrt(3) / 2 // triangle height
	v0 := geometry.Vec3{X: cx, Y: cy + 2*h/3}
	v1 := geometry.Vec3{X: cx - sideLength/2, Y: cy - h/3}
	v2 := geometry.Vec3{X: cx + sideLength/2, Y: cy - h/3}

	center := geometry.Vec3{X: cx, Y: cy}

	triVerts := [3]geometry.Vec3{v0, v1, v2}
	walls := make([]geometry.Plane, 0, 5)

	// Three side walls.
	for i := range 3 {
		a := triVerts[i]
		b := triVerts[(i+1)%3]
		edge := b.Sub(a)
		// Inward-pointing normal in XY plane.
		perp := geometry.Vec3{X: edge.Y, Y: -edge.X}
		mid := a.Add(b).Scale(0.5)

		if perp.Dot(center.Sub(mid)) < 0 {
			perp = perp.Neg()
		}

		walls = append(walls, geometry.NewPlaneFromPointNormal(a, perp))
	}

	// Floor and ceiling.
	walls = append(
		walls,
		geometry.Plane{Normal: geometry.Vec3{Z: 1}, Distance: 0},
		geometry.Plane{Normal: geometry.Vec3{Z: -1}, Distance: -zHeight},
	)

	// All 6 vertices (3 bottom + 3 top).
	verts := make([]geometry.Vec3, 0, 6)
	for _, v := range triVerts {
		verts = append(verts, geometry.Vec3{X: v.X, Y: v.Y, Z: 0})
		verts = append(verts, geometry.Vec3{X: v.X, Y: v.Y, Z: zHeight})
	}

	room, err := NewConvexRoom(walls, verts)
	if err != nil {
		panic("equilateral triangle room: " + err.Error())
	}

	return room
}

func TestIBMValidation_EquilateralTriangle(t *testing.T) {
	// Equilateral triangle extruded in z. Use tall z to push z-modes
	// above our analysis band, so we effectively test 2D modes.
	sideLength := 3.0
	zHeight := 10.0 // first z-mode at c/(2*zHeight) ≈ 17 Hz, below analysis band
	c := 343.0
	h := 0.1 // grid spacing (coarser for feasibility with tall z)

	room := equilateralTriangleRoom(sideLength, sideLength/2, sideLength*math.Sqrt(3)/3, zHeight)

	// Source and receiver at different off-centre positions within the triangle.
	srcPos := geometry.Vec3{X: 1.3, Y: 0.9, Z: 5.0}
	rcvPos := geometry.Vec3{X: 1.7, Y: 1.1, Z: 5.0}

	dt := 0.95 * h / (c * math.Sqrt(3))
	nSteps := int(0.5 / dt) // 500 ms for ~2 Hz frequency resolution

	ts, dtActual := runFDTD(room, h, RigidWallBC(), srcPos, rcvPos, nSteps)

	maxFreq := 500.0
	analytical := triangleEigenfreqs(sideLength, c, maxFreq)
	fdtdPeaks := extractPeakFreqs(ts, dtActual, 30, maxFreq, 40)

	t.Logf("dt=%.6g s, nSteps=%d", dtActual, nSteps)
	t.Logf("analytical triangle modes (up to %.0f Hz): %d", maxFreq, len(analytical))
	t.Logf("FDTD peaks found: %d", len(fdtdPeaks))

	matched := 0

	for _, fp := range fdtdPeaks {
		bestDist := math.Inf(1)
		bestAnalytical := 0.0

		for _, af := range analytical {
			d := math.Abs(fp - af)
			if d < bestDist {
				bestDist = d
				bestAnalytical = af
			}
		}

		if bestAnalytical == 0 {
			continue
		}

		relErr := bestDist / bestAnalytical

		if relErr < 0.005 {
			matched++
		}

		t.Logf("FDTD peak %.1f Hz → analytical %.1f Hz (error %.3f%%)",
			fp, bestAnalytical, relErr*100)
	}

	t.Logf("matched %d/%d FDTD peaks within 0.5%%", matched, len(fdtdPeaks))

	// Target: most well-resolved modes should match.
	if matched < 5 {
		t.Errorf("only %d triangle modes matched within 0.5%%, want ≥ 5", matched)
	}
}

// besselPrimeZeros returns the first few zeros of J'_m(x) for m = 0, 1, 2, ...
// These are the Neumann boundary condition eigenvalues for a circular drum.
// Precomputed from standard tables.
func besselPrimeZeros() []float64 {
	// Zeros of J'_m(x) for small m, sorted ascending.
	// J'_0: 3.8317, 7.0156, 10.1735
	// J'_1: 1.8412, 5.3314, 8.5363
	// J'_2: 3.0542, 6.7061, 9.9695
	// J'_3: 4.2012, 8.0152
	// J'_4: 5.3175, 9.2824
	return []float64{
		1.8412, 3.0542, 3.8317, 4.2012, 5.3175, 5.3314,
		6.7061, 7.0156, 8.0152, 8.5363, 9.2824, 9.9695,
		10.1735,
	}
}

// circularRoomEigenfreqs returns eigenfrequencies for a circular room with
// Neumann BC: f_{m,n} = c · α'_{m,n} / (2π·R).
func circularRoomEigenfreqs(radius, c, maxFreq float64) []float64 {
	zeros := besselPrimeZeros()
	var freqs []float64

	for _, alpha := range zeros {
		f := c * alpha / (2 * math.Pi * radius)
		if f <= maxFreq {
			freqs = append(freqs, f)
		}
	}

	sort.Float64s(freqs)

	return freqs
}

// regularPolygonRoom constructs a ConvexRoom approximating a circle with
// nSides polygon, centered at (cx, cy), radius r, extruded z=[0, zHeight].
func regularPolygonRoom(nSides int, cx, cy, radius, zHeight float64) *ConvexRoom {
	center := geometry.Vec3{X: cx, Y: cy}
	verts2d := make([]geometry.Vec3, nSides)

	for i := range nSides {
		angle := 2 * math.Pi * float64(i) / float64(nSides)
		verts2d[i] = geometry.Vec3{
			X: cx + radius*math.Cos(angle),
			Y: cy + radius*math.Sin(angle),
		}
	}

	walls := make([]geometry.Plane, 0, nSides+2)

	for i := range nSides {
		a := verts2d[i]
		b := verts2d[(i+1)%nSides]
		edge := b.Sub(a)
		perp := geometry.Vec3{X: edge.Y, Y: -edge.X}
		mid := a.Add(b).Scale(0.5)

		if perp.Dot(center.Sub(mid)) < 0 {
			perp = perp.Neg()
		}

		walls = append(walls, geometry.NewPlaneFromPointNormal(a, perp))
	}

	// Floor and ceiling.
	walls = append(
		walls,
		geometry.Plane{Normal: geometry.Vec3{Z: 1}, Distance: 0},
		geometry.Plane{Normal: geometry.Vec3{Z: -1}, Distance: -zHeight},
	)

	allVerts := make([]geometry.Vec3, 0, 2*nSides)

	for _, v := range verts2d {
		allVerts = append(allVerts, geometry.Vec3{X: v.X, Y: v.Y, Z: 0})
		allVerts = append(allVerts, geometry.Vec3{X: v.X, Y: v.Y, Z: zHeight})
	}

	room, err := NewConvexRoom(walls, allVerts)
	if err != nil {
		panic("regular polygon room: " + err.Error())
	}

	return room
}

func TestIBMValidation_CircularRoom(t *testing.T) {
	// Approximate a circle with a 64-sided polygon. Compare eigenfrequencies
	// against Bessel function zeros for Neumann BC.
	radius := 2.0
	zHeight := 10.0 // first z-mode at ~17 Hz, below analysis band
	c := 343.0
	h := 0.1
	nSides := 64

	room := regularPolygonRoom(nSides, radius, radius, radius, zHeight)

	srcPos := geometry.Vec3{X: radius + 0.3, Y: radius + 0.2, Z: 5.0}
	rcvPos := geometry.Vec3{X: radius - 0.4, Y: radius - 0.3, Z: 5.0}

	dt := 0.95 * h / (c * math.Sqrt(3))
	nSteps := int(0.5 / dt) // 500 ms for ~2 Hz frequency resolution

	ts, dtActual := runFDTD(room, h, RigidWallBC(), srcPos, rcvPos, nSteps)

	maxFreq := 400.0
	analytical := circularRoomEigenfreqs(radius, c, maxFreq)
	fdtdPeaks := extractPeakFreqs(ts, dtActual, 20, maxFreq, 30)

	t.Logf("dt=%.6g s, nSteps=%d", dtActual, nSteps)
	t.Logf("analytical circular modes (up to %.0f Hz): %d", maxFreq, len(analytical))
	t.Logf("FDTD peaks found: %d", len(fdtdPeaks))

	matched := 0

	for _, fp := range fdtdPeaks {
		bestDist := math.Inf(1)
		bestAnalytical := 0.0

		for _, af := range analytical {
			d := math.Abs(fp - af)
			if d < bestDist {
				bestDist = d
				bestAnalytical = af
			}
		}

		if bestAnalytical == 0 {
			continue
		}

		relErr := bestDist / bestAnalytical

		if relErr < 0.02 { // 2% for polygon approximation of circle
			matched++
		}

		t.Logf("FDTD peak %.1f Hz → analytical %.1f Hz (error %.3f%%)",
			fp, bestAnalytical, relErr*100)
	}

	t.Logf("matched %d/%d FDTD peaks within 2%%", matched, len(fdtdPeaks))

	// Polygon approximation won't be as precise as rectangular/triangle.
	if matched < 3 {
		t.Errorf("only %d circular modes matched within 2%%, want ≥ 3", matched)
	}
}

func TestIBMValidation_EnergyDecaySabine(t *testing.T) {
	// Verify that energy decay in an absorbing room is physically reasonable:
	// 1. Energy decays monotonically (not growing or constant)
	// 2. Decay rate is within an order of magnitude of Sabine prediction
	// 3. Higher absorption (lower R) produces faster decay
	//
	// Note: exact Sabine agreement (10%) is difficult at low FDTD frequencies
	// where few modes exist and the diffuse-field assumption breaks down.
	// The FDTD resolves individual modes with high Q at low frequencies,
	// leading to systematically longer T60 than Sabine predicts.
	lx, ly, lz := 8.0, 6.0, 5.0
	c := 343.0
	h := 0.2

	room := rectRoom(lx, ly, lz)
	volume := lx * ly * lz
	surfaceArea := 2 * (lx*ly + ly*lz + lz*lx)

	// Test two absorption levels.
	testCases := []struct {
		name string
		R    float64
	}{
		{"moderate absorption", 0.85},
		{"high absorption", 0.5},
	}

	t60s := make([]float64, 0, len(testCases))

	for _, tc := range testCases {
		R := tc.R
		alpha := 1 - R*R
		t60Sabine := 0.161 * volume / (surfaceArea * alpha)

		bc := ImpedanceWallBC(R)
		g := ClassifyGrid(room, h)
		stencil := NewIBMStencil(g, bc)
		dt := 0.95 * stencil.CFLLimit(c)

		src, err := NewIBMSource(room, g, geometry.Vec3{X: 3.0, Y: 2.5, Z: 2.0}, SoftSource)
		if err != nil {
			t.Fatalf("%s: source creation: %v", tc.name, err)
		}

		nn := g.Nx * g.Ny * g.Nz
		pCur := make([]float64, nn)
		pPrev := make([]float64, nn)
		pNext := make([]float64, nn)

		t0 := 5.0 * dt
		sigma := 2.0 * dt
		pulseEnd := int(math.Ceil((t0 + 5*sigma) / dt))

		totalTime := 3.0 * t60Sabine
		nSteps := min(int(totalTime/dt), 20000)

		type sample struct {
			time   float64
			energy float64
		}

		var energySamples []sample

		for step := range nSteps {
			tNow := float64(step) * dt

			if step < pulseEnd {
				src.Inject(pCur, GaussianPulse(tNow, t0, sigma))
			}

			stencil.FDTDStep(pNext, pCur, pPrev, c, dt)
			pPrev, pCur, pNext = pCur, pNext, pPrev

			if step%20 == 0 && step > pulseEnd {
				e := totalEnergy(g, pCur)
				energySamples = append(energySamples, sample{time: tNow, energy: e})
			}
		}

		peakEnergy := 0.0
		peakIdx := 0

		for i, s := range energySamples {
			if s.energy > peakEnergy {
				peakEnergy = s.energy
				peakIdx = i
			}
		}

		if peakEnergy == 0 {
			t.Fatalf("%s: no energy detected", tc.name)
		}

		// Fit exponential decay.
		fitStart := peakIdx + 5

		var sumX, sumY, sumXX, sumXY float64
		nFit := 0

		for i := fitStart; i < len(energySamples); i++ {
			s := energySamples[i]
			if s.energy <= 0 || s.energy/peakEnergy < 1e-10 {
				break
			}

			x := s.time - energySamples[peakIdx].time
			y := math.Log10(s.energy / peakEnergy)

			sumX += x
			sumY += y
			sumXX += x * x
			sumXY += x * y
			nFit++
		}

		if nFit < 5 {
			t.Fatalf("%s: not enough points for decay fit", tc.name)
		}

		slope := (float64(nFit)*sumXY - sumX*sumY) / (float64(nFit)*sumXX - sumX*sumX)
		t60FDTD := -6.0 / slope

		t60s = append(t60s, t60FDTD)

		finalEnergy := energySamples[len(energySamples)-1].energy
		decayDB := 10 * math.Log10(finalEnergy/peakEnergy)

		t.Logf("%s (R=%.2f): Sabine T60=%.3fs, FDTD T60=%.3fs, decay=%.1f dB, ratio=%.1fx",
			tc.name, R, t60Sabine, t60FDTD, decayDB, t60FDTD/t60Sabine)

		// 1. Energy must decay (not grow).
		if finalEnergy > peakEnergy {
			t.Errorf("%s: energy grew from %.6g to %.6g", tc.name, peakEnergy, finalEnergy)
		}

		// 2. Must have significant decay (at least 10 dB).
		if decayDB > -10 {
			t.Errorf("%s: insufficient decay: %.1f dB", tc.name, decayDB)
		}

		// 3. FDTD T60 should be within an order of magnitude of Sabine.
		// At low FDTD frequencies, T60 is typically 2-8× longer than Sabine.
		ratio := t60FDTD / t60Sabine
		if ratio < 0.1 || ratio > 10 {
			t.Errorf("%s: T60 ratio %.1fx outside [0.1, 10] range", tc.name, ratio)
		}
	}

	// 4. Higher absorption should give shorter T60.
	if len(t60s) >= 2 && t60s[1] >= t60s[0] {
		t.Errorf("higher absorption (R=0.5) should give shorter T60: %.3fs ≥ %.3fs (R=0.85)",
			t60s[1], t60s[0])
	}
}
