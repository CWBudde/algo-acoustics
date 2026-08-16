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
//
// The caller specifies the simulated duration in seconds; the step count is
// derived from the time step the solver actually uses, which is
// 0.95·CFLLimit(c) and can be several times smaller than the nominal
// h/(c·√3) bound when the grid contains small cut cells. Deriving nSteps
// from a nominal dt (as this test used to) silently shortens the run and
// coarsens the achievable FFT resolution.
func runFDTD(
	room *ConvexRoom,
	h float64,
	bc WallBC,
	srcPos, rcvPos geometry.Vec3,
	duration float64,
) ([]float64, float64) {
	g := ClassifyGrid(room, h)
	stencil := NewIBMStencil(g, bc)

	c := 343.0
	dt := 0.95 * stencil.CFLLimit(c)
	nSteps := int(math.Ceil(duration / dt))

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

	// Remove the mean and taper before transforming.
	//
	// A rigid enclosure is sealed, so the volume the soft source injects has
	// nowhere to go: the pressure settles on a non-zero mean and stays there.
	// That step is by far the largest feature of the record — for the 3x2.5x2
	// shoebox the DC bin comes out ~49x the strongest acoustic mode — and its
	// 1/f leakage tail leans on the bottom of the analysis band. Neither is
	// acoustic content. Subtracting the mean removes the bin, and the Hann
	// taper removes the record-edge discontinuity that a non-decaying signal
	// otherwise leaves under a rectangular window.
	//
	// This was masked before Phase 26.1: hundreds of wall directions were
	// silently pressure-release, which vented the enclosure and suppressed the
	// drift. With the walls correctly rigid it dominates, so detrending is not
	// cosmetic — without it the peak threshold below rejects nearly every mode.
	mean := 0.0
	for _, v := range ts {
		mean += v
	}

	mean /= float64(n)

	padded := make([]float64, nfft)

	for i, v := range ts {
		w := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1))
		padded[i] = (v - mean) * w
	}

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
	//
	// The threshold is taken from the strongest magnitude *inside* the analysis
	// band. Anchoring it to the global maximum lets a feature the band excludes
	// set the bar for everything in it, which is what a DC step does.
	threshold := 0.0

	for i, m := range mag {
		f := float64(i) * freqBin
		if f >= minFreq && f <= maxFreq && m > threshold {
			threshold = m
		}
	}

	threshold *= 0.01 // 1% of the in-band max

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

// binTolerance is the number of true FFT bins a detected peak is allowed to
// sit away from an analytical mode before the match is rejected.
//
// The FFT in extractPeakFreqs zero-pads to the next power of two, so it reports
// frequencies on a finer grid than the run can actually resolve; the true
// resolution is df = 1/(nSteps·dt). A damped, un-windowed mode smears over
// roughly one true bin, and the interpolated peak location of such a lobe is
// only good to a fraction of it, so one bin is the floor for a meaningful
// tolerance. 1.5 adds margin for modal broadening from wall losses and for the
// slight downward frequency bias of the dispersive FDTD stencil, without
// letting the window grow into the neighbouring mode.
const binTolerance = 1.5

// matchToleranceHz returns the absolute Hz tolerance used to decide whether a
// detected peak identifies an analytical mode: the larger of a relative
// accuracy target and the physically achievable frequency resolution.
func matchToleranceHz(fAnalytical, relTol, df float64) float64 {
	return math.Max(relTol*fAnalytical, binTolerance*df)
}

// windowCoverage returns the fraction of [lo, hi] covered by the union of the
// tolerance windows around the analytical modes. It is the probability that a
// peak placed uniformly at random in the analysis band would be counted as a
// match, i.e. the chance-level match rate the pass criterion must beat.
func windowCoverage(analytical []float64, relTol, df, lo, hi float64) float64 {
	if hi <= lo {
		return 0
	}

	type span struct{ a, b float64 }

	spans := make([]span, 0, len(analytical))

	for _, f := range analytical {
		tol := matchToleranceHz(f, relTol, df)
		a, b := math.Max(f-tol, lo), math.Min(f+tol, hi)

		if b > a {
			spans = append(spans, span{a, b})
		}
	}

	sort.Slice(spans, func(i, j int) bool { return spans[i].a < spans[j].a })

	covered := 0.0
	curA, curB := math.Inf(1), math.Inf(-1)

	for _, s := range spans {
		switch {
		case math.IsInf(curA, 1):
			curA, curB = s.a, s.b
		case s.a <= curB:
			curB = math.Max(curB, s.b)
		default:
			covered += curB - curA
			curA, curB = s.a, s.b
		}
	}

	if !math.IsInf(curA, 1) {
		covered += curB - curA
	}

	return covered / (hi - lo)
}

// logChanceLevel reports how much of the analysis band the tolerance windows
// occupy — the rate at which peaks placed at random would be "matched".
//
// It also splits the band in three, because mode density grows with frequency:
// where the analytical set is sparse a match is real evidence, where it is
// dense (windows covering most of the sub-band) the match statistic carries no
// information and any pass threshold derived from it is meaningless.
func logChanceLevel(t *testing.T, analytical []float64, relTol, df, lo, hi float64) {
	t.Helper()

	t.Logf("chance-level match rate over %.0f–%.0f Hz: %.1f%% of the band lies inside a tolerance window",
		lo, hi, windowCoverage(analytical, relTol, df, lo, hi)*100)

	third := (hi - lo) / 3
	for i := range 3 {
		a, b := lo+float64(i)*third, lo+float64(i+1)*third
		t.Logf("  sub-band %.0f–%.0f Hz: %d analytical modes, chance level %.1f%%",
			a, b, countInBand(analytical, a, b), windowCoverage(analytical, relTol, df, a, b)*100)
	}
}

// modeRecall counts how many analytical modes in [lo, hi] were actually found
// by the solver, i.e. have at least one detected peak within tolerance.
//
// This runs the comparison the opposite way from "every peak sits near some
// mode", and it is the direction that carries information. A peak-first count
// gets easier as the analytical set grows denser — with the triangular prism's
// 953 modes below 500 Hz the tolerance windows tile the band and every possible
// peak matches something, so 40/40 says nothing about the solver. Recall gets
// *harder* with a denser set, because each mode has to be produced, not merely
// approached.
//
// chance reports the probability of a single mode being covered by luck, given
// the number of detected peaks in the band: 1 − (1 − 2·tol/W)^P. Comparing the
// recall rate against it is what stops a pass threshold from encoding a lottery.
// It scores *clusters* of modes, not individual modes, and consumes each peak
// at most once. Both corrections matter, and neither is cosmetic:
//
//   - Counting each analytical mode separately let a single peak satisfy an
//     unbounded number of modes. With the prism's 953 modes below 500 Hz and
//     only ~40 detected peaks, the reported 58 % came largely from reuse: a
//     solver producing a handful of resonances could clear the threshold.
//     Peaks are therefore matched bijectively.
//
//   - Demanding a distinct peak per mode is equally wrong in the other
//     direction, because modes closer together than the FFT resolution are not
//     separable *even in principle*. The prism's 3D modes below 187 Hz sit
//     ~1.7 Hz apart against a 2 Hz resolution. Modes within a tolerance window
//     of each other are therefore collapsed into one cluster, and the cluster
//     counts as recalled if any peak lands in it. That is the strongest claim
//     the measurement can actually support.
//
// chance is computed over clusters for the same reason.
func modeRecall(analytical, peaks []float64, relTol, df, lo, hi float64) (found, total int, chance float64) {
	// Only in-band peaks may match. The chance model already counts just these,
	// and letting the matching loop use out-of-band peaks made recall and chance
	// disagree — an out-of-band peak could lift recall without lifting chance.
	inBand := make([]float64, 0, len(peaks))

	for _, p := range peaks {
		if p >= lo && p <= hi {
			inBand = append(inBand, p)
		}
	}

	clusters := clusterModes(analytical, relTol, df, lo, hi)
	total = len(clusters)

	if total == 0 {
		return 0, 0, 0
	}

	used := make([]bool, len(inBand))
	avgTol := 0.0

	for _, c := range clusters {
		avgTol += c.tol

		// Take the nearest still-unused peak inside the cluster window. Nearest
		// rather than first, so an early cluster cannot steal a peak that sits
		// far closer to a later one.
		best, bestDist := -1, math.Inf(1)

		for i, p := range inBand {
			if used[i] {
				continue
			}

			if d := math.Abs(p - c.center); d < c.tol && d < bestDist {
				best, bestDist = i, d
			}
		}

		if best >= 0 {
			used[best] = true
			found++
		}
	}

	if hi > lo {
		avgTol /= float64(total)
		chance = 1 - math.Pow(math.Max(0, 1-2*avgTol/(hi-lo)), float64(len(inBand)))
	}

	return found, total, chance
}

// modeCluster is a group of analytical modes that no measurement at this
// resolution could tell apart, scored as a single target.
type modeCluster struct {
	center float64
	tol    float64
}

// clusterModes merges modes in [lo, hi] whose separation is below the match
// tolerance. The cluster's window is widened to span its members, so a cluster
// is never harder to hit than the individual modes it replaced.
func clusterModes(analytical []float64, relTol, df, lo, hi float64) []modeCluster {
	inBand := make([]float64, 0, len(analytical))

	for _, af := range analytical {
		if af >= lo && af <= hi {
			inBand = append(inBand, af)
		}
	}

	if len(inBand) == 0 {
		return nil
	}

	sort.Float64s(inBand)

	var (
		clusters   []modeCluster
		start, end = inBand[0], inBand[0]
	)

	flush := func() {
		center := (start + end) / 2
		tol := matchToleranceHz(center, relTol, df) + (end-start)/2
		clusters = append(clusters, modeCluster{center: center, tol: tol})
	}

	for _, f := range inBand[1:] {
		if f-end < matchToleranceHz(f, relTol, df) {
			end = f

			continue
		}

		flush()

		start, end = f, f
	}

	flush()

	return clusters
}

// requireModeRecall asserts that the solver reproduces the modes in [lo, hi]
// and that the result is clear of the chance level, so the threshold cannot be
// satisfied by a dense analytical set alone.
func requireModeRecall(t *testing.T, name string, analytical, peaks []float64, relTol, df, lo, hi float64, minRate float64) {
	t.Helper()

	found, total, chance := modeRecall(analytical, peaks, relTol, df, lo, hi)
	if total == 0 {
		t.Fatalf("%s: no analytical modes in %.0f–%.0f Hz to score against", name, lo, hi)
	}

	rate := float64(found) / float64(total)
	t.Logf("%s: recalled %d/%d analytical modes in %.0f–%.0f Hz (%.0f%%), chance level %.0f%%",
		name, found, total, lo, hi, rate*100, chance*100)

	if rate < minRate {
		t.Errorf("%s: recalled %d/%d modes in %.0f–%.0f Hz (%.0f%%), want ≥ %.0f%%",
			name, found, total, lo, hi, rate*100, minRate*100)
	}

	if rate <= chance {
		t.Errorf("%s: recall %.0f%% is at or below the %.0f%% chance level, so it demonstrates nothing",
			name, rate*100, chance*100)
	}
}

// countInBand counts analytical modes falling in [lo, hi].
func countInBand(freqs []float64, lo, hi float64) int {
	n := 0

	for _, f := range freqs {
		if f >= lo && f <= hi {
			n++
		}
	}

	return n
}

// logRunResolution reports the timing and resolution actually achieved, plus
// the tolerance those numbers imply, so the numbers behind a pass/fail are
// visible without re-deriving them.
func logRunResolution(t *testing.T, dt float64, nSteps int, relTol, loFreq float64) float64 {
	t.Helper()

	df := 1.0 / (float64(nSteps) * dt)
	tolLo := matchToleranceHz(loFreq, relTol, df)

	t.Logf("dt=%.6g s, nSteps=%d, simulated duration=%.4g s, true FFT resolution df=%.3f Hz",
		dt, nSteps, float64(nSteps)*dt, df)
	t.Logf("match tolerance = max(%.1f%%·f, %.1f·df) → %.2f Hz at %.0f Hz (= %.2f%% relative)",
		relTol*100, binTolerance, tolLo, loFreq, tolLo/loFreq*100)

	return df
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

	// Run long enough for modes to develop and ring; frequency resolution
	// df = 1/T, so 0.5 s of simulated time gives df ≈ 2 Hz.
	const (
		duration = 0.5
		minFreq  = 30.0
		relTol   = 0.005
	)

	ts, dtActual := runFDTD(room, h, RigidWallBC(), srcPos, rcvPos, duration)

	maxFreq := 400.0 // well below Nyquist, well-resolved by grid
	analytical := shoeboxEigenfreqs(lx, ly, lz, c, maxFreq)

	// Extract spectral peaks from FDTD.
	fdtdPeaks := extractPeakFreqs(ts, dtActual, minFreq, maxFreq, 40)

	df := logRunResolution(t, dtActual, len(ts), relTol, minFreq)
	t.Logf("analytical modes (up to %.0f Hz): %d", maxFreq, len(analytical))
	logChanceLevel(t, analytical, relTol, df, minFreq, maxFreq)
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

		tol := matchToleranceHz(bestAnalytical, relTol, df)
		if bestDist < tol {
			matched++
		}

		t.Logf("FDTD peak %.1f Hz → analytical %.1f Hz (error %.3f%%, %.2f Hz; tol %.2f Hz)",
			fp, bestAnalytical, relErr*100, bestDist, tol)
	}

	t.Logf("matched %d/%d FDTD peaks, max error: %.4f%%", matched, len(fdtdPeaks), maxError*100)

	// Scored on recall, not on the peak-first count above, which is kept only
	// as a diagnostic. The band stops at 153 Hz: past that the analytical set
	// packs modes closer than the tolerance window (chance level 92% in the
	// middle third, 98% in the top), so a peak-first count there is free.
	//
	// Measured 8/12 = 67% against a 39% chance level, identically on amd64 and
	// arm64. See docs/validation.md, which carries the full table.
	// The threshold sits below that with room for solver changes, but well
	// clear of chance — and requireModeRecall fails outright if a future change
	// ever drags it down to chance.
	requireModeRecall(t, "shoebox", analytical, fdtdPeaks, relTol, df, minFreq, 153, 0.60)
}

// triangleEigenfreqs returns analytical eigenfrequencies for an equilateral
// triangle with side length L (Neumann BC). From Lamé's closed form, the
// eigenvalues are λ_{m,n} = (16π²/9L²)·(m² + mn + n²), so
//
//	f_{m,n} = c·√λ/(2π) = (2c / 3L) · √(m² + mn + n²)
//
// with m ≥ 1, n ≥ 0, m > n (avoiding duplicates).
//
// The factor of 2 was missing until 2026-08-16: the base was c/(3L), which put
// every transverse mode at half its true frequency and meant the triangular
// prism test was scoring the solver against a mode set the room cannot have.
// The lowest mode is a quick sanity check — for L = 3 m this gives 2c/3L =
// 76.2 Hz, consistent with a room whose largest inscribed dimension is 3 m,
// whereas the old 38.1 Hz implied a ~4.5 m cavity.
func triangleEigenfreqs(sideLength, c, maxFreq float64) []float64 {
	var freqs []float64
	base := 2 * c / (3 * sideLength)

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

// triangularPrismEigenfreqs returns the analytical eigenfrequencies of an
// equilateral-triangle prism of side length L extruded to zHeight, with rigid
// (Neumann) walls:
//
//	f_{m,n,q} = √( f2D(m,n)² + (q·c/(2·zHeight))² ),  q = 0, 1, 2, …
//
// The 2D cross-section modes come from triangleEigenfreqs; q = 0 reproduces
// them. A tall extrusion does not push the axial modes out of the analysis
// band — it packs them in at a spacing of c/(2·zHeight) — so the full 3D set
// is what an FDTD run of this room actually contains.
func triangularPrismEigenfreqs(sideLength, zHeight, c, maxFreq float64) []float64 {
	return prismEigenfreqs(triangleEigenfreqs(sideLength, c, maxFreq), zHeight, c, maxFreq)
}

// prismEigenfreqs combines a cross-section's 2D mode set with the axial modes
// of an extrusion of height zHeight:
//
//	f_{2D,q} = hypot(f_2D, q·c/(2·zHeight)),  q = 0,1,2,...
//
// plus the purely axial family (q ≥ 1 with no transverse component).
//
// Every extruded fixture in this file needs this. A 2D-only mode set silently
// assumes the extrusion contributes nothing, which holds only when the first
// axial mode sits above the analysis band — and at zHeight = 10 m that mode is
// at 17.15 Hz, so the assumption is inverted: c/(2·zHeight) is the mode
// *spacing*, and axial harmonics fill the band rather than avoiding it.
func prismEigenfreqs(modes2D []float64, zHeight, c, maxFreq float64) []float64 {
	fz := c / (2 * zHeight) // axial mode spacing

	// The purely axial family (m = n = 0) is a valid mode set too.
	freqs := make([]float64, 0, len(modes2D))

	for q := 1; float64(q)*fz <= maxFreq; q++ {
		freqs = append(freqs, float64(q)*fz)
	}

	for _, f2D := range modes2D {
		for q := 0; ; q++ {
			fq := float64(q) * fz

			f := math.Hypot(f2D, fq)
			if f > maxFreq {
				break
			}

			freqs = append(freqs, f)
		}
	}

	sort.Float64s(freqs)

	// Remove near-duplicates, as triangleEigenfreqs does.
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
// The side length is parameterised even though both call sites currently pass
// 3 m: it is what the analytical mode set is derived from, so hard-coding it
// here would hide the coupling between the fixture and triangleEigenfreqs.
//
//nolint:unparam // deliberate: keeps the geometry/analytics coupling explicit.
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
	// Equilateral triangle extruded in z. The extrusion does NOT isolate the
	// 2D cross-section modes: c/(2·zHeight) ≈ 17.15 Hz is the axial mode
	// *spacing*, so z-harmonics densely populate the whole analysis band.
	// We therefore compare against the full 3D prism mode set. See the
	// chance-level log line below for what that density costs in test power.
	sideLength := 3.0
	zHeight := 10.0 // axial modes every c/(2*zHeight) ≈ 17 Hz
	c := 343.0
	h := 0.1 // grid spacing (coarser for feasibility with tall z)

	room := equilateralTriangleRoom(sideLength, sideLength/2, sideLength*math.Sqrt(3)/3, zHeight)

	// Source and receiver at different off-centre positions within the triangle.
	srcPos := geometry.Vec3{X: 1.3, Y: 0.9, Z: 5.0}
	rcvPos := geometry.Vec3{X: 1.7, Y: 1.1, Z: 5.0}

	const (
		duration = 0.5 // 500 ms of simulated time
		minFreq  = 30.0
		relTol   = 0.005
	)

	ts, dtActual := runFDTD(room, h, RigidWallBC(), srcPos, rcvPos, duration)

	maxFreq := 500.0
	analytical := triangularPrismEigenfreqs(sideLength, zHeight, c, maxFreq)
	fdtdPeaks := extractPeakFreqs(ts, dtActual, minFreq, maxFreq, 40)

	df := logRunResolution(t, dtActual, len(ts), relTol, minFreq)
	t.Logf("analytical 2D cross-section modes (up to %.0f Hz): %d",
		maxFreq, len(triangleEigenfreqs(sideLength, c, maxFreq)))
	t.Logf("analytical 3D prism modes (up to %.0f Hz): %d", maxFreq, len(analytical))
	logChanceLevel(t, analytical, relTol, df, minFreq, maxFreq)
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

		tol := matchToleranceHz(bestAnalytical, relTol, df)
		if bestDist < tol {
			matched++
		}

		t.Logf("FDTD peak %.1f Hz → analytical %.1f Hz (error %.3f%%, %.2f Hz; tol %.2f Hz)",
			fp, bestAnalytical, relErr*100, bestDist, tol)
	}

	t.Logf("matched %d/%d FDTD peaks", matched, len(fdtdPeaks))

	// Scored over 30–187 Hz, not the whole band, and against a much lower
	// threshold than this test carried until 2026-08-16. Both changes follow
	// from fixing how recall is counted, and neither is a concession:
	//
	// The old figure — 58% against a 40% chance level over 30–500 Hz — was
	// inflated, because each analytical mode scanned the whole peak list
	// independently and so a single peak could satisfy dozens of modes. With
	// ~40 detected peaks and 953 analytical modes, most of that 58% was one
	// peak counted many times. Consuming each peak once (see modeRecall) drops
	// the same run to 50% against a 67% chance level, i.e. below chance: over
	// the full band this fixture demonstrates nothing, and no threshold could
	// change that.
	//
	// Lengthening the run does not rescue it. At 2 s the resolution improves to
	// 0.5 Hz and the analytical set resolves into 73 clusters, but the solver
	// still yields only 40 peaks, so recall is capped at 55% by construction.
	//
	// The low band is where the measurement carries information: the chance
	// level drops to 27% there rather than the 67% above, and the solver
	// recalls 35%. (27% is modeRecall's chance level, not the band-coverage
	// figure logChanceLevel prints — that is 75.3% for this sub-band.)
	// That margin is modest but real, and requireModeRecall fails outright if
	// recall ever falls back to the chance level, so the +8 pp is the actual
	// assertion. The threshold below is the secondary guard.
	requireModeRecall(t, "triangular prism", analytical, fdtdPeaks, relTol, df, minFreq, 187, 0.30)
}

// besselJPrime evaluates J'_m(x) via the standard recurrence
// J'_m = (J_{m-1} − J_{m+1})/2, with J'_0 = −J_1.
func besselJPrime(m int, x float64) float64 {
	if m == 0 {
		return -math.J1(x)
	}

	return (math.Jn(m-1, x) - math.Jn(m+1, x)) / 2
}

// besselPrimeZeros returns every zero of J'_m(x) in (0, maxAlpha], across all
// angular orders m — the Neumann eigenvalues of a circular cross-section.
//
// These are computed rather than tabulated. The previous version was a
// hard-coded list of 13 zeros ending at 10.1735, which silently truncated the
// analytical mode set: a 2 m radius scored to 400 Hz needs every root out to
// α = 2π·R·f/c ≈ 14.66, so whole families of radial and angular modes were
// missing and the recall test simply never asked about them. A table cannot
// track a change of radius or band, so it should not be a table.
//
// Orders are scanned until the first zero of J'_m exceeds maxAlpha; since
// j'_{m,1} grows monotonically with m (roughly m + 0.81·m^⅓), nothing is
// missed after that.
func besselPrimeZeros(maxAlpha float64) []float64 {
	const (
		step   = 1e-3
		bisect = 60
	)

	var zeros []float64

	for m := 0; ; m++ {
		var found []float64

		prev := besselJPrime(m, step)

		for x := 2 * step; x <= maxAlpha; x += step {
			cur := besselJPrime(m, x)

			if (prev < 0) != (cur < 0) {
				lo, hi := x-step, x

				for range bisect {
					mid := (lo + hi) / 2
					if (besselJPrime(m, lo) < 0) != (besselJPrime(m, mid) < 0) {
						hi = mid
					} else {
						lo = mid
					}
				}

				// x = 0 is a root of J'_m for every m ≥ 1 and corresponds to no
				// acoustic mode, so only strictly positive roots count.
				if z := (lo + hi) / 2; z > 1e-6 {
					found = append(found, z)
				}
			}

			prev = cur
		}

		// No root of J'_m below the cap means no higher order has one either.
		if len(found) == 0 {
			break
		}

		zeros = append(zeros, found...)
	}

	sort.Float64s(zeros)

	return zeros
}

// circularRoomEigenfreqs returns eigenfrequencies for a circular room with
// Neumann BC: f_{m,n} = c · α'_{m,n} / (2π·R).
func circularRoomEigenfreqs(radius, c, maxFreq float64) []float64 {
	// f = c·α/(2πR), so the band edge fixes exactly which roots are needed.
	zeros := besselPrimeZeros(2 * math.Pi * radius * maxFreq / c)
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

	const (
		duration = 0.5 // 500 ms of simulated time
		minFreq  = 20.0
		relTol   = 0.02 // 2% for the polygon approximation of a circle
	)

	ts, dtActual := runFDTD(room, h, RigidWallBC(), srcPos, rcvPos, duration)

	maxFreq := 400.0
	// The cylinder is extruded to zHeight like the triangular prism, so its
	// mode set needs the same axial family — the Bessel zeros alone describe a
	// 2D disc, not this room. Measurably so: against the 2D-only set the
	// solver's recall comes out 15 points *below* chance, i.e. the peaks avoid
	// those frequencies, because most of the real modes have a z component.
	analytical := prismEigenfreqs(circularRoomEigenfreqs(radius, c, maxFreq), zHeight, c, maxFreq)
	fdtdPeaks := extractPeakFreqs(ts, dtActual, minFreq, maxFreq, 30)

	df := logRunResolution(t, dtActual, len(ts), relTol, minFreq)
	t.Logf("analytical circular modes (up to %.0f Hz): %d", maxFreq, len(analytical))
	logChanceLevel(t, analytical, relTol, df, minFreq, maxFreq)
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

		tol := matchToleranceHz(bestAnalytical, relTol, df)
		if bestDist < tol {
			matched++
		}

		t.Logf("FDTD peak %.1f Hz → analytical %.1f Hz (error %.3f%%, %.2f Hz; tol %.2f Hz)",
			fp, bestAnalytical, relErr*100, bestDist, tol)
	}

	t.Logf("matched %d/%d FDTD peaks", matched, len(fdtdPeaks))

	// Scored over 20–147 Hz against a 30% threshold, revised 2026-08-16 for two
	// independent reasons, both of which made the previous 73%/57% figure wrong:
	//
	//   * Recall double-counted. Each mode scanned the entire peak list, so one
	//     peak could satisfy many modes; peaks are now consumed once.
	//   * The analytical set was incomplete. besselPrimeZeros was a table of 13
	//     roots ending at 10.1735 that stopped at angular order m = 4, so it
	//     omitted whole mode families — including four roots inside its own
	//     range — and every omitted mode was one the solver was never asked to
	//     produce. Computing the roots to the band edge (α = 2πRf/c ≈ 14.66 for
	//     this fixture) yields 32 rather than 13.
	//
	// Together those give 36% recall against a 28% chance level over the sparse
	// low band. The chance level stays highish because relTol is 2% here, for
	// the 64-gon's departure from a true circle, which widens every window; the
	// +8 pp margin is what the claim rests on, and requireModeRecall fails
	// outright if recall ever falls back to chance.
	requireModeRecall(t, "cylinder", analytical, fdtdPeaks, relTol, df, minFreq, 147, 0.30)
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
