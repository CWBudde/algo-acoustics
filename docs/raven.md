# RAVEN -- Condensed Reference

Condensed from: Dirk Schroeder, *Physically Based Real-Time Auralization of Interactive Virtual Environments*, RWTH Aachen, 2011.

This document distills the key formulas, models, and design decisions from the RAVEN dissertation. It focuses on what goes beyond textbook acoustics -- the specific simulation models, their combination into a hybrid system, the energy calibration between methods, and the real-time implementation strategies. Basic wave acoustics is included only where the specific formulation matters for implementation.

---

## 1. Sound Propagation Fundamentals

### 1.1 Wave Equation and Point Sources

All of room acoustics simulation starts from the scalar wave equation for sound pressure $p$ in a homogeneous medium:

$$\Delta p = \frac{1}{c^2} \frac{\partial^2 p}{\partial t^2}$$

The simplest analytic solution is the spherical wave radiated by a point source with volume velocity $Q$:

$$p(r,t) = \frac{\rho_0}{4\pi r} \frac{\partial Q}{\partial t}\left(t - \frac{r}{c}\right)$$

where $r$ is the distance from the source, $\rho_0$ the static air density, and $c$ the speed of sound. The retardation $r/c$ encodes the finite propagation speed.

Sound intensity describes the energy flow through a reference area per unit time:

$$\vec{I} = \overline{p \cdot \vec{v}} = \frac{1}{T} \int_0^T p \cdot \vec{v}\, dt$$

For spherical waves, the total radiated power and the intensity at distance $r$ are:

$$P = \frac{\rho_0 \omega^2 \hat{Q}^2}{8\pi c} \qquad I = \frac{P}{4\pi r^2}$$

The $1/r^2$ distance law is the fundamental decay of intensity with distance for a point source in free field.

**Air absorption** introduces an additional exponential decay: $I(r) = I_0 e^{-mr}$, where the absorption constant $m$ depends on frequency, temperature, and humidity in a complex way. Air absorption is negligible below ~4 kHz but dominates at high frequencies, particularly over long propagation paths.

**Directivity** scales the intensity by an angle-dependent factor: $I_\beta(\varphi, \Theta) = \beta(\varphi, \Theta) \cdot I$, with $0 \le \beta \le 1$. The point source remains a good approximation as long as the source dimensions are small compared to the wavelength.

### 1.2 Reflection and Absorption

When a plane sound wave hits an infinitely large smooth surface, it reflects specularly (Snell's law: angle of incidence equals angle of reflection). The complex, angle-dependent reflection factor relates the reflected pressure to the incident pressure:

$$\underline{R} = \frac{p_r}{p_i} = \frac{\underline{Z}\cos(\Theta) - Z_0}{\underline{Z}\cos(\Theta) + Z_0}$$

where $Z_0 = \rho_0 c$ is the characteristic impedance of air and $\underline{Z}$ is the surface impedance (ratio of total sound pressure to surface-normal particle velocity at the point of incidence). For a locally reacting surface, $\underline{Z}$ is independent of the angle of incidence.

The absorption coefficient $\alpha$ gives the fraction of incident energy that is absorbed:

$$\alpha = 1 - |\underline{R}|^2 = \frac{4 Re(\varsigma)\cos(\Theta)}{1 + 2Re(\varsigma)\cos(\Theta) + |\varsigma|^2\cos^2(\Theta)}, \quad \varsigma = \underline{Z}/Z_0$$

In practice, absorption coefficients are measured in reverberation chambers (DIN EN ISO 354) and provided as frequency-band averages, which means the angle dependence is already averaged out. The reflected intensity is simply:

$$I_r = (1 - \alpha) I_i$$

**Important limitation:** The IS method uses $|R| = \sqrt{1-\alpha}$ as the magnitude of the reflection factor. This is exact only for $R = 1$ and $R = -1$, but remains a good approximation for low absorption coefficients. The plane wave assumption is only valid when the source-wall distance is large and the angle of incidence is less than about 60 degrees from the surface normal.

### 1.3 Scattering

Real surfaces are not perfectly smooth. When the surface irregularity dimensions (height $h$, length $a$) are on the order of half a wavelength ($f \approx c/2a$), a scattered wave occurs in addition to the specular reflection. The scattering coefficient $s$ (measurable per ISO 17497) partitions the reflected energy:

$$E_{r,specular} = (1-s)(1-\alpha) E_i$$
$$E_{r,diffuse} = s(1-\alpha) E_i$$
$$E_r = (1-\alpha) E_i$$

At low frequencies ($f \ll c/2a$), the surface acts smooth and the scattering coefficient is effectively zero. At high frequencies ($f \gg c/2a$), the texture elements themselves cause specular reflections from individual faces. The transition region is where scattering is most significant.

In the temporal development of a room impulse response, scattering is already a dominant effect from reflection order 2-3 onward, even in rooms with relatively smooth surfaces. This is why pure image source modeling fails for the late part of the RIR.

### 1.4 Sound Transmission

When sound hits a wall separating two rooms, part of the energy is transmitted through. The sound reduction index $R$ quantifies the insulation performance:

$$R = -10\log\tau = 10\log\frac{I_i}{I_t}$$

where $\tau = I_r/I_t$ is the transmission coefficient. Solid structures transmit higher frequencies less efficiently (low-pass characteristic).

For diffuse sound fields in both the source room (level $L_S$) and receiving room (level $L_R$), the sound reduction index can be measured as:

$$R = L_S - L_R + 10\log\frac{S}{A_R}$$

where $S$ is the partition area and $A_R$ is the equivalent absorption area of the receiving room. The latter term compensates for the influence of reverberation in the receiving room.

In real buildings, sound also travels via flanking paths (floor, ceiling, side walls). The apparent sound reduction index $R'$ accounts for all $N \times M$ transmission paths:

$$R' = -10\log\left(\sum_{i=1}^{N}\sum_{j=1}^{M} 10^{-R_{ij}/10}\right)$$

The transmission coefficient used for filter construction is recovered as:

$$\tau_{x,y} = 10^{-R_{x,y}/10}$$

### 1.5 Edge Diffraction (Semi-infinite Rigid Edge)

Diffraction is a wave phenomenon that bends sound around obstacles into shadow zones. It cannot be ignored in auralization because it prevents abrupt, unnatural jumps in the sound field when sources or receivers cross geometric shadow boundaries.

The exact solution for sound diffraction at a semi-infinite rigid edge gives the total sound field as:

$$p(r,\varphi) = p_S(0) \cdot \frac{1+j}{2} \cdot \left\{e^{j \cdot k \cdot r \cdot \cos(\varphi-\varphi_0)} \cdot \Phi_+ + e^{j \cdot k \cdot r \cdot \cos(\varphi+\varphi_0)} \cdot \Phi_-\right\}$$

with the Fresnel integral terms $\Phi_{\pm}$ encoding the transition behavior:

$$\Phi_+ = \frac{1-j}{2} + C(u) - j \cdot S(u), \quad u = \sqrt{2 \cdot k \cdot r} \cdot \cos\frac{\varphi - \varphi_0}{2}$$
$$\Phi_- = \frac{1-j}{2} + C(v) - j \cdot S(v), \quad v = \sqrt{2 \cdot k \cdot r} \cdot \cos\frac{\varphi + \varphi_0}{2}$$

where $\varphi_0$ is the angle of the source from the edge, $\varphi$ the observation angle, and $C(x)$, $S(x)$ are the Fresnel integrals:

$$C(x) = \sqrt{\frac{2}{\pi}}\int_0^x \cos(t^2)dt \qquad S(x) = \sqrt{\frac{2}{\pi}}\int_0^x \sin(t^2)dt$$

The sound field divides into three zones around the edge:

**Reflection zone** ($\varphi < \pi - \varphi_0$): Both incident and reflected waves are present. The approximation gives the exact free-field solution for a rigid wall. Diffraction has minimal influence here.

$$p(r,\varphi) \approx p_S(0) \cdot \left\{e^{j \cdot k \cdot r \cdot \cos(\varphi-\varphi_0)} + e^{j \cdot k \cdot r \cdot \cos(\varphi+\varphi_0)}\right\}$$

**View zone** ($\pi - \varphi_0 < \varphi < \pi + \varphi_0$): Only the incident wave is present (no reflected wave). Diffraction has little influence.

$$p(r,\varphi) \approx p_S(0) \cdot e^{j \cdot k \cdot r \cdot \cos(\varphi-\varphi_0)}$$

**Shadow zone** ($\varphi > \pi + \varphi_0$): No direct or reflected wave reaches the receiver. The sound field is entirely due to diffraction and decays as $1/\sqrt{kr}$:

$$p(r,\varphi) \approx p_Q(0) \cdot \frac{j-1}{2\sqrt{2}\pi\sqrt{2 \cdot k \cdot r}} \cdot e^{-j \cdot k \cdot r} \left\{\frac{1}{\left\|\cos\frac{\varphi-\varphi_0}{2}\right\|} + \frac{1}{\left\|\cos\frac{\varphi+\varphi_0}{2}\right\|}\right\}$$

The key takeaway: diffraction has significant influence only in the shadow zone, but the smooth transition at the boundary is critical for perceptual plausibility.

### 1.6 Room Impulse Response Structure

The room impulse response (RIR) is the "acoustical fingerprint" of a room. Human hearing processes it in three perceptually distinct parts:

| Part                | Delay        | Character                    | Perception                                                            |
| ------------------- | ------------ | ---------------------------- | --------------------------------------------------------------------- |
| Direct sound        | $d/c$        | Single impulse               | Source localization (precedence effect)                                |
| Early reflections   | ~0--80 ms    | Discrete specular arrivals   | Source width, distance, loudness (Haas effect: up to +10 dB fused)    |
| Late reverberation  | >50--80 ms   | Diffuse, exponential decay   | Room character, spaciousness                                          |

This division has profound implications for simulation: the direct sound and early reflections must be computed with high precision in timing and spectral content (they affect source localization), while the late reverberation requires only energetically correct behavior over time slots and angular fields (the human hearing integrates it coarsely). This motivates the hybrid approach of combining deterministic IS with stochastic RT.

### 1.7 Auralization (LTI Convolution)

Assuming a room behaves as a linear time-invariant (LTI) system, the output signal $g(t)$ at the listener is the convolution of the dry source signal $s(t)$ with the room impulse response $h(t)$:

$$g(t) = s(t) * h(t) = \int_{-\infty}^{\infty} s(\tau) h(t-\tau) d\tau$$

In the frequency domain, this simplifies to multiplication: $\underline{G}(f) = \underline{S}(f) \cdot \underline{H}(f)$

The Binaural Room Transfer Function (BRTF) $\underline{H}(f)$ can be decomposed as a sum of transfer function chains, one for each reflection path. Each chain comprises:

$$H_{Reflection} = H_D \cdot H_{Air} \cdot H_{Head} \cdot \prod_{HitWalls} H_{E_i}$$

where:

- $H_D$ = source directivity transfer function for the emission angle
- $H_{E_i}$ = wall or edge transfer functions for each hit constructional element
- $H_{Air}$ = air absorption over the complete propagation path
- $H_{Head}$ = HRTF pair (left/right ear) for the angle of incidence at the listener

The sum of all these contributions gives the complete BRIR. If the BRIRs are exact, convolving them with the dry source signal produces an output identical to the real listening experience, including spatial perception.

---

## 2. Image Source Method (ISM)

The ISM (Allen & Berkley, 1979; extended by Borish, 1984) is a deterministic method for finding all specular reflection paths between a point source $S$ and a point receiver $R$ inside a polyhedral room. It is the workhorse for computing the early, specular part of the RIR.

### 2.1 Image Source Construction

An image source is constructed by mirroring the point source across a wall plane. Given a source $\vec{S}$, a plane with unit normal $\vec{n}$ through a point $\vec{P}$, and $\vec{r} = \vec{S} - \vec{P}$:

$$\vec{I} = \vec{S} - 2 \cdot \vec{n} \cdot |\vec{r}| \cdot \cos(\alpha)$$

where $\alpha$ is the angle between $\vec{n}$ and $\vec{r}$. This is the standard point-across-plane reflection.

The method proceeds recursively: mirror $S$ across all $N$ walls to get first-order image sources, then mirror each of those across all walls to get second-order sources, and so on. The source chain notation tracks the wall sequence:

$$S \to I_{n_1} \to I_{n_1 n_2} \to \ldots \to I_{n_1 n_2 \ldots n_i}$$

The number of image sources grows exponentially: roughly $N \cdot (N-1)^{i-1}$ at order $i$. For a room with 12 walls at 4th order, this produces over 17,000 image sources.

### 2.2 Audibility Test

Only a small fraction of generated image sources represent physically valid (audible) reflections. An audibility test traces backwards from the receiver through the source chain:

1. Draw a line from receiver $R$ to the last image source $I_{n_1...n_i}$
2. Check that this line intersects the wall on which $I_{n_k}$ was mirrored (within the polygon boundary)
3. Check that no other wall blocks this line segment
4. Step back to the parent IS and repeat with the intersection point as the new endpoint
5. Continue until the primary source is reached (path is audible) or a test fails (path is blocked)

This test is the primary computational bottleneck for real-time IS simulation. BSP-tree acceleration reduces the intersection test complexity from $\mathcal{O}(N)$ to $\mathcal{O}(\log N)$.

### 2.3 IS Impulse Response Construction

Each audible image source contributes a filtered Dirac delta to the RIR. The spectrum of the $j$-th reflection path is:

$$\underline{H}_j = \frac{e^{-j\omega t_j}}{ct_j} \cdot H_{air}(ct_j) \cdot \underline{H}_{Source}(\theta,\phi) \cdot \underline{H}_{Receiver}(\vartheta,\varphi) \cdot \prod_{i=1}^{n_j} \underline{R}_i$$

The factors are:

- $e^{-j\omega t_j}$: phase shift corresponding to the path delay $t_j$
- $1/(ct_j)$: inverse-distance law for spherical wave amplitude
- $H_{air}(ct_j)$: air absorption over the full path length
- $\underline{H}_{Source}(\theta,\phi)$: source directivity at the emission angle
- $\underline{H}_{Receiver}(\vartheta,\varphi)$: receiver directivity -- for binaural rendering this is replaced by the left/right HRTF pair for the angle of incidence, producing two impulse responses $\underline{H}_{j,L}$ and $\underline{H}_{j,R}$
- $\prod \underline{R}_i$: product of wall reflection factors for all $n_j$ surfaces hit

### 2.4 Plane-Polygon Map Optimization

A critical optimization for real-time IS generation: instead of mirroring the source across every polygon, mirror only across the distinct *planes* defined by those polygons. Many polygons share the same plane (e.g., coplanar wall segments). A Plane-Polygon Map (PPM) tracks which polygons belong to which plane.

The IS count reduction is substantial:

$$N_{IS_{poly}} = \sum_{j=1}^{i} n_{poly} \cdot (n_{poly}-1)^{j-1} \qquad N_{IS_{plane}} = \sum_{j=1}^{i} n_{plane} \cdot (n_{plane}-1)^{j-1}$$

Concrete example: a room with 12 polygons on 8 distinct planes produces 4,096 ISs at 4th order instead of 17,568 -- a 4x reduction. The information about which specific polygon was hit is recovered during the audibility test via point-in-polygon tests on the coplanar polygon set.

### 2.5 Diffraction Extension (Biot-Tolstoy-Medwin Expression)

The IS method ignores energy bent into shadow zones. To model edge diffraction, secondary sources called Diffraction Sources (DS) are placed on room edges. A DS becomes active only when the direct path between a source (PS or IS) and the receiver is blocked by geometry, avoiding discontinuities at the view/shadow zone boundary.

The edge diffraction impulse response is derived from the Biot-Tolstoy-Medwin Expression (BTME), which treats every point on the edge as a secondary source:

$$h_{n,diff}(\tau) = \frac{-c\nu}{2\pi} \cdot \frac{\beta(\tau)}{r_S \cdot r_R \cdot \sinh(\eta(\tau))} \cdot H(\tau - \tau_0)$$

where $H(\tau - \tau_0)$ is the Heaviside step function and:

$$\beta(\tau) = \beta_{++}(\tau) + \beta_{+-}(\tau) + \beta_{-+}(\tau) + \beta_{--}(\tau)$$

$$\beta_{\pm\pm}(\tau) = \frac{\sin[\nu \cdot (\pi \pm \theta_S \pm \theta_R)]}{\cosh[\nu \cdot \eta(\tau)] - \cos[\nu \cdot (\pi \pm \theta_S \pm \theta_R)]}$$

$$\eta(\tau) = \cosh^{-1}\frac{c^2 \cdot \tau^2 - (r_S^2 + r_R^2 + z_R^2)}{2 \cdot r_S \cdot r_R}$$

$$\nu = \frac{\pi}{\theta_W}, \quad \tau_0 = \frac{L_0}{c}$$

All coordinates are cylindrical relative to the edge axis:

- $r_S, \theta_S, z_S$: source position in edge-relative cylindrical coords
- $r_R, \theta_R, z_R$: receiver position in edge-relative cylindrical coords
- $\theta_W$: wedge open angle
- $\nu$: edge index (determines how strongly the edge diffracts)
- $L_0$: shortest path length (source to nearest edge point to receiver)

The four $\beta$ terms account for scattering on both sides of the edge. The impulse response starts at $\tau_0 = L_0/c$ (shortest path) and energy is halved at $\tau_{min}$ since secondary source pairs cease to exist beyond the edge endpoints:

$$\tau_{min} = \frac{L_1 + L_2}{c}, \quad \tau_{max} = \frac{L_3 + L_4}{c}$$

For multi-edge diffraction paths, the sound path is decomposed into subpath types: S2D (source-to-diffraction-edge), D2D (edge-to-edge), D2R (edge-to-receiver). The total transfer function concatenates all subpath contributions:

$$H_{AudibleSoundPath_i} = H_{PS} \cdot H_R \cdot \prod_n H_n$$

Second-order diffraction (paths via two edges) is approximated by multiplying two first-order diffraction transfer functions with an additional receiver at the midpoint between the edges. This is an approximation -- exact second-order diffraction would require integrating contributions from every element on Edge 1 via every element on Edge 2, which is computationally prohibitive.

---

## 3. Stochastic Ray Tracing

Ray tracing computes the late, diffuse part of the RIR. It is a Monte Carlo method: the result converges statistically with increasing particle count, unlike the deterministic IS method which finds exact solutions.

### 3.1 Energy Particle Model

The source emits a finite number of energy particles uniformly distributed over its surface. Each particle travels in a straight line at the speed of sound $c$ and carries energy determined by the source's directional pattern for the emission direction.

When a particle hits a wall, it loses energy according to the absorption coefficient $\alpha$. The reflection type is decided stochastically using the scattering coefficient $s$:

- With probability $(1-s)$: **specular reflection** -- angle of incidence equals angle of reflection
- With probability $s$: **diffuse scattering** -- reflected in a random direction following Lambert's cosine law (intensity proportional to $\cos\theta$ relative to surface normal)

Total energy after reflection: $(1-\alpha) E_{incident}$, split as $(1-s)(1-\alpha)$ specular + $s(1-\alpha)$ diffuse.

**Detector model:** Receivers are volume detectors (spheres), not point receivers, since the probability of a particle hitting an exact point is zero. Each particle that enters the detector sphere has its energy, arrival time, and direction logged into a frequency-dependent energy histogram with time slots of size $\Delta t$ (typically a few milliseconds). A complete RT simulation runs separate cycles for each octave or 1/3-octave band center frequency (10 or 31 cycles for the audible range).

Particle tracing terminates when:

- Particle energy drops below a threshold
- Propagation time exceeds the histogram length
- Reflection order exceeds a maximum (prevents trapping between close polygons)

### 3.2 Diffuse Rain (Secondary Radiation)

Diffuse rain is a variance-reduction technique that avoids relying solely on direct particle-detector hits. At each diffuse reflection, a secondary "rain" of energy is sent toward the detector, regardless of whether the particle itself would hit it. This dramatically reduces the number of particles needed for convergence.

The hit probability combines a uniform solid-angle term with Lambert's cosine law:

$$P_{Hit} = P_{Hit,equal} \cdot P_{Lambert}(\Theta)$$

**For spherical detectors** (the standard receiver), the compensation factor for equal distribution over the half-sphere is $a = 1/(2\pi)$, and for Lambert distribution $b = 2$. The detected scattered energy becomes:

$$E_s = E_P \cdot (1-\alpha) \cdot s \cdot (1 - \cos\frac{\gamma}{2}) \cdot 2 \cdot \cos(\Theta) \cdot e^{-m \cdot r}$$

where $E_P$ is the particle energy before hitting the wall, $\alpha$ and $s$ are the wall's absorption and scattering coefficients, $\gamma$ is the detector's opening angle, $\Theta$ is the angle between the surface normal and the connection vector $\vec{r}$ to the detector center, and $e^{-m \cdot r}$ is the air absorption over the rain path of length $r$.

**For surface detectors** (used at portals for inter-room transmission), the solid angle is approximated by a pyramid volume ratio, and an additional $\cos(\Psi)$ factor accounts for the orientation of the detector surface:

$$E_s = E_P \cdot (1-\alpha) \cdot s \cdot \frac{A}{2\pi \cdot r^2} \cdot \cos(\Psi) \cdot \cos(\Theta) \cdot e^{-m \cdot r}$$

where $A$ is the detector area and $\Psi$ the angle between $\vec{r}$ and the detector's normal vector.

### 3.3 Diffracted Rain (DAPDF)

To incorporate edge diffraction into the stochastic ray tracer, finite cylindrical detectors called deflection cylinders (DC) are placed around each edge. The cylinder radius is frequency-dependent: $r = 7\lambda$, ensuring that particles passing within a few wavelengths of the edge are captured.

When a particle passes through a DC, the edge's influence on the particle is described by the 2D Deflection Angle Probability Density Function (DAPDF), derived from the Fraunhofer single-slit diffraction expression:

$$D(v) = D_0 \cdot \begin{cases} 1 - v^2 & \text{if } |v| \le v_0 \\ \frac{1/2}{\sqrt{2-1+v^2}} & \text{if } |v| > v_0 \end{cases}$$

with $v_0 = \sqrt{1 - 1/\sqrt{2}} \approx 0.5412$ and $v = 2 \cdot b \cdot \epsilon$, where $b = 6a$ is the apparent slit width ($a$ = shortest distance from particle path to edge) and $\epsilon$ is the deflection angle. $D_0$ normalizes the integral to unity.

The outgoing energy from an edge toward a visible detector is computed by integrating the DAPDF over the angular range subtended by the detector:

$$E_{out} = E_{in} \cdot e^{-m \cdot h} \cdot \int_{\epsilon_{min}}^{\epsilon_{max}} D(\epsilon, b)\, d\epsilon$$

where $h$ is the fly-by distance and $\epsilon_{min}, \epsilon_{max}$ are the angles to the detector edges. The DAPDF integral has six piecewise closed-form solutions depending on where $v_{low}$ and $v_{high}$ fall relative to $\pm v_0$.

Diffracted rain can be applied recursively: any DC can forward energy to other visible detectors (spheres, portals, or other DCs), enabling multi-order diffraction paths.

### 3.4 Poisson Noise Process for RIR Synthesis

The energy histogram from ray tracing gives only the temporal envelope of the RIR, not its fine structure. For auralization, the temporal microstructure must be synthesized. This is done using a Poisson-distributed noise process.

Sound reflections are modeled as random events. The probability of $n$ events in a time interval $\Delta t$ follows the Poisson distribution:

$$w_n(\Delta t) = \frac{(\mu\Delta t)^n}{n!} e^{-\mu\Delta t}$$

The mean event occurrence rate $\mu$ increases with time as reflections accumulate:

$$\mu = \frac{4\pi c^3 t^2}{V}$$

where $V$ is the room volume. The time interval between consecutive events is exponentially distributed and can be drawn from a uniform random number $z \in (0,1]$:

$$\Delta t_A(z) = \frac{1}{\mu}\ln\left(\frac{1}{z}\right)$$

The process is valid only after a minimum time when reflections become dense enough:

$$t_0 = \sqrt[3]{\frac{2V\ln 2}{4\pi c^3}} \approx 0.0014\sqrt[3]{V}$$

The rate $\mu$ should be capped at $10{,}000\,s^{-1}$ to avoid audible artifacts (rattling). Dirac deltas are assigned positive or negative signs by counting them within each sample's temporal half (first half positive, second half negative), and restricting to at most one delta per sample.

### 3.5 Bandpass-Filtered RIR Construction

The Poisson sequence of Dirac deltas is filtered per frequency band using a Raised Cosine Filter (RCF) with asymmetric slopes:

$$RCF(n) = \begin{cases} 0 & f < f_{low} \\ \frac{1}{2}(1 + \cos(\frac{2\pi f}{f_n})) & f_{low} \le f < f_n \\ \frac{1}{2}(1 - \cos(\frac{2\pi f}{f_{n+1}})) & f_n \le f < f_{high} \\ 0 & f \ge f_{high} \end{cases}$$

where $f_n$ is the center frequency of the $n$-th band and $f_{low}$, $f_{high}$ its lower and upper limits.

Each filtered sequence is then weighted sample-by-sample to match the energy histogram:

$$s_i = v_i \cdot \sqrt{\frac{E_n(k)}{\sum_{i=g(k-1)+1}^{g(k)} v_i^2}} \cdot \sqrt{\frac{BW}{f_s/2}}$$

where $g(k) = \lfloor k \cdot f_s \cdot \Delta t \rfloor$ maps histogram time slots to sample indices, $v_i$ is the $i$-th sample of the filtered Dirac delta sequence, $E_n(k)$ is the energy of the $k$-th time slot for band $n$, $BW$ is the bandwidth of the band, and $f_s$ is the sampling frequency. The $\sqrt{BW/(f_s/2)}$ factor normalizes for the band's spectral width. The final monaural RIR is the sum of all weighted band sequences.

**Binaural construction** is more complex: the detector sphere is subdivided into $m$ directivity groups (DGs) by azimuth and elevation. Each DG accumulates a separate energy histogram. For each time slot, HRIRs are selected based on the DG hit probabilities and convolved with the Poisson sequence. The weighted, HRIR-convolved noise fragments are overlap-added (50% Hanning window) to produce the left and right ear channels of the BRIR.

---

## 4. Hybrid Simulation (IS + RT)

The hybrid approach combines the deterministic IS method (accurate early specular reflections) with stochastic RT (efficient late diffuse energy). Two critical issues must be handled: avoiding double-counting of energy and calibrating the energy scales between the two methods.

### 4.1 Energy Calibration

Both methods must agree on the energy of the direct sound. In free-field conditions, the IS method gives the direct sound energy at distance $r$ as:

$$E_{IS} = E_{source} \cdot \frac{1}{r^2} \cdot e^{-mr}$$

The RT method detects the direct sound through the spherical detector's solid angle coverage:

$$E_{RT} = E_p \cdot N \cdot \frac{1}{2}(1 - \cos\frac{\gamma}{2}) \cdot e^{-mr}$$

where $E_p$ is the energy per particle, $N$ the number of launched particles, and $\gamma$ the detector's opening angle. Setting $E_{IS} = E_{RT}$ and solving for $E_p$:

$$E_p = \frac{2 \cdot E_{source}}{N \cdot r^2 \cdot (1 - \cos\frac{\gamma}{2})}$$

Since the IS method carries energy for all frequency bands simultaneously while each RT particle belongs to one band, the total IS energy is $E_{IS} = n \cdot E_{source}$ and the ray tracer must launch $N_{RT} = n \cdot N$ particles total ($N$ per band).

### 4.2 Combining IS and RT

The two methods produce separate impulse responses that are superposed: $h_{hybrid}(t) = h_{IS}(t) + h_{RT}(t)$

To avoid double-counting, the RT detector must reject particles whose propagation path is already covered by the IS method. The detection logic tracks each particle's reflection history:

```text
DETECTION_ALLOWED_HYBRID:
  if EDOrder != 0:
    // Particle has been diffracted at least once
    return HasDiffuseHistory OR
           PreEDreflectionOrder > MaxPreEDISOrder OR
           EDreflectionOrder > MaxEDISOrder OR
           EDOrder > MaxEDOrder
  else:
    // Pure reflection path -- only detect if beyond IS coverage
    return reflectionOrder > MaxISOrder
```

Key rules:

- RT only detects particles that have at least one diffuse (scattered) reflection in their history, or that exceed the maximum IS reflection order
- Direct hits of scattered or diffracted rain particles on detectors that cross the current propagation path must not be counted (the rain energy was already accounted for)
- After a specular reflection, the particle's `allowDetection` flag is set to true; after a scatter or diffraction event, it is reset to false (to prevent double-counting with the rain)

---

## 5. Sound Transmission Between Rooms

### 5.1 Acoustic Scene Graph and Room Groups

The multi-room scene is organized as an Acoustic Scene Graph (ASG): a graph where nodes represent rooms and edges represent portals (doors, windows, walls). Each portal has a state (open/closed). Rooms connected by open portals form a *room group* -- a single acoustic space simulated as one unit.

Sound propagation paths across rooms are found by depth-first search through the ASG. The search prunes branches where the accumulated sound reduction $R_w$ exceeds the source's sound level, since the transmitted signal would be inaudible.

### 5.2 Secondary Source Model

For sound transmission through structural elements, the source room is acoustically substituted by secondary point sources placed at the center of each radiating structural element in the receiver room:

$$SS_y = S \cdot \sum_{x=0}^{X} H_{S,x} \cdot H_{x,y}$$

where $H_{S,x}$ is the room transfer function from the source to structural element $x$ in the source room, and $H_{x,y}$ is the transmission path transfer function between elements $x$ and $y$ (derived from interpolated sound reduction indices).

The total transfer function for a complete propagation path through multiple rooms is:

$$\underline{H}_{PP}\big|_{left,right} = \underline{H}_{PS} \cdot \prod_{p=0}^{P} H_{Portal}(p) \cdot \prod_{r=0}^{R} H_{RoomGroup}(r) \cdot \underline{H}_R\big|_{left,right}$$

Each factor is a separately simulated or measured transfer function. Portal transfer functions use interpolated transmission coefficients; room group transfer functions come from IS+RT simulations within that group.

### 5.3 Portal Crossfading

When a door opens or closes, a smooth transition between the two acoustic states is needed. RAVEN caches BRIRs for both portal states and crossfades between them using an $n$-th root function:

$$y(x) = \sqrt[n]{x}, \quad x \in [0,1]$$

where $x$ maps the relative aperture angle of the portal. As $x$ increases from 0 to 1 (door opening), the portal's sound reduction indices are continuously lowered until the portal filter $h_{Portal}$ becomes an all-pass (door fully open). At that point, a hard switch to the pre-cached BRIR of the merged room group can occur without audible artifacts.

---

## 6. Spatial Data Structures

### 6.1 BSP Tree (Binary Space Partitioning)

BSP trees recursively subdivide 3D space using planes as partitioners, producing a binary tree of convex subspaces. They are used in RAVEN for fast ray-polygon intersection tests during IS audibility checks and RT simulations.

The hyperplane equation in Hessian normal form: $(\vec{n} \cdot \vec{x}) - d = u$, where $u > 0$ means the point is in front, $u < 0$ behind, and $u = 0$ on the plane.

RAVEN uses planes spanned by the scene polygons themselves as partitioners. The selection criterion balances tree balance against polygon splits:

$$r(p) = \min_{p \in P}\left\{\frac{\sum_{p' \ne p}\delta_{in\,front}(p',p)}{\sum_{p' \ne p}\delta_{behind}(p',p)}, \frac{\sum_{p' \ne p}\delta_{behind}(p',p)}{\sum_{p' \ne p}\delta_{in\,front}(p',p)}\right\}$$

$$s(p) = \sum_{p' \ne p}\delta_{crosses}(p',p)$$

A polygon $p$ is chosen as partitioner if its ratio $r(p)$ exceeds a threshold $t$ (acceptable balance) and it produces the fewest splits $s(p)$. Higher $t$ values yield more balanced trees but more polygon splits; the optimal $t$ is scene-dependent.

**Ray intersection using BSP:** classify the ray's start and end points relative to each partitioner plane. If both are on the same side, only that subspace needs searching. If they straddle the plane, test the partitioner polygon first, then search both subspaces in order (front-to-back). This reduces intersection complexity from $\mathcal{O}(N)$ to $\mathcal{O}(\log N)$.

**Limitation:** BSP trees are expensive to rebuild when geometry changes. This makes them unsuitable as the sole spatial structure for dynamic scenes.

### 6.2 Spatial Hashing

Spatial hashing maps 3D space into a flat hash table for $\mathcal{O}(1)$ polygon lookups. The space is divided into axis-aligned cubical voxels with edge length $a$.

Voxel coordinates from world position:

$$u = \left\lfloor\frac{x}{a}\right\rfloor, \quad v = \left\lfloor\frac{y}{a}\right\rfloor, \quad w = \left\lfloor\frac{z}{a}\right\rfloor$$

Two hash functions are considered:

$$h_1(u,v,w) = (u \cdot p_1 \oplus v \cdot p_2 \oplus w \cdot p_3) \bmod n$$
$$h_2(u,v,w) = (u \cdot p_1 + v \cdot p_2 + w \cdot p_3) \bmod n$$

where $p_1, p_2, p_3$ are large primes, $\oplus$ is XOR, and $n$ is the hash table size.

Three parameters must be optimized: (1) hash table size $n$ for minimal collisions without excessive memory, (2) voxel edge length $a$ relative to scene dimensions, and (3) the hash function itself for uniform distribution.

**Key advantage over BSP:** Insertion and deletion of $m$ polygons takes only $\mathcal{O}(m)$, independent of scene complexity. RAVEN uses spatial hashing for the *dynamic mode* -- handling geometry modifications in real-time -- while using BSP for the faster *static mode*.

---

## 7. Collision Tests

During RT simulation, millions of ray-polygon intersection tests are performed. In addition, detectors (spheres and cylinders) must be checked for hits separately since they are not part of the polygonal scene geometry.

### 7.1 Ray-Sphere (Receiver Detection)

The test reduces to a 2D closest-approach problem. Given ray from $S$ in direction $\vec{b} = E - S$ and sphere center $C$:

$$P = \vec{S} + u \cdot \vec{b}, \quad u = \frac{\vec{a} \circ \vec{b}}{\vec{b} \circ \vec{b}}, \quad \vec{d} = C - P$$

where $\vec{a} = C - S$. If $|\vec{d}| \le r$ (sphere radius), the ray collides with the sphere. The constraint $0 \le u \le 1$ is automatically satisfied since rays travel from surface to surface.

### 7.2 Ray-Cylinder (Edge Diffraction Detection)

For cylindrical deflection cylinders aligned with edges, the test computes the closest distance between the ray and the cylinder axis:

$$\vec{n} = \vec{e} \times \vec{b}, \quad \hat{n} = \frac{\vec{n}}{|\vec{n}|}, \quad d = |\hat{n} \circ \vec{s_E}|$$

where $\vec{e}$ is the edge direction, $\vec{b}$ the ray direction, and $\vec{s_E} = S - S_E$ the vector from the edge start to the ray start. If $d \le r$ (cylinder radius $= 7\lambda$), the ray is close enough for diffraction to apply.

---

## 8. Real-Time Constraints and Update Strategy

The total system latency budget for a VR auralization system is about 50 ms. Hardware overhead (tracking, audio I/O) already consumes ~35 ms, leaving only ~35 ms for the acoustic simulation in the worst case.

Perceptual research shows that different interaction events have vastly different update requirements:

| Event                  | Update interval  | What to update                     |
| ---------------------- | ---------------- | ---------------------------------- |
| Head/source rotation   | 35 ms            | Specular BRIR (directivity/HRTFs)  |
| Translation >0.25 m    | 550 ms           | Specular BRIR (full IS retest)     |
| Translation >1.0 m     | 2 s              | Diffuse BRIR (full ray tracing)    |
| Geometry change        | 550 ms / 2 s     | Both specular and diffuse parts    |
| Portal interaction     | crossfade        | Both cached BRIRs                  |

Key perceptual thresholds from user studies:

- Audio-visual synchronization tolerance: up to 35 ms additional delay is unnoticed
- Head tracking lag tolerance: up to 70 ms before localization is affected
- Peak head rotation speed: ~45 deg/s
- Typical translation speed: 30--50 cm/s
- 25 cm trigger threshold (half a concert seat width) catches 70% of translational movements at 750 ms intervals, 90% at 550 ms

**IS method separation:** The IS simulation is decomposed into three independent steps (Translation, Audibility Test, Filter Construction) with three entry points depending on the event type. Rotational events only require filter reconstruction (cheapest). Translational events require audibility re-testing. Source movement additionally requires IS position updates.

**RT method:** No meaningful decomposition is possible since the energy histogram depends on the full particle propagation. However, rotational events can skip ray tracing entirely and just recompute the BRIR from cached energy histograms by rotating the detector's coordinate system.

---

## 9. Limits of Geometrical Acoustics

- **Large rooms only**: GA assumes wavelengths are small compared to geometry. Room modes dominate at low frequencies and are not modeled. Valid above the Schroeder frequency.
- **Broadband signals required**: narrowband/tonal sources reveal phase effects (interference, flutter echoes) that GA does not capture.
- **Low absorption approximation**: Using $|R| = \sqrt{1-\alpha}$ is only accurate for low $\alpha$. The plane-wave reflection model is valid only for large source-wall distances and incidence angles below ~60 deg.
- **Model detail**: Acoustic models are simpler than visual models. The minimum element size is $a \ge c/(2f)$ (e.g., 1.72 m for 100 Hz). Details smaller than this wavelength-dependent threshold do not produce meaningful specular reflections.
- **Frequency range**: The audible range spans three orders of magnitude (20 Hz to 20 kHz, wavelengths from 20 m to 2 cm). Neither small-wavelength nor large-wavelength approximations hold universally -- hence the need for hybrid wave/GA approaches.

---

## 10. Key Implementation Data Structures

### Image Source Tree (IST)

All image sources are organized in a tree structure where nodes represent ISs and edges link them by their reflection path. The tree depth corresponds to the IS reflection order. The root node is the primary source (or secondary source/diffraction source).

The IST supports dynamic insertion (adding new reflection planes when objects are created), deletion (removing subtrees when objects are destroyed), and position updates (traversing affected subtrees when objects move). This is essential for the dynamic simulation mode.

For diffraction, each IST node additionally stores edge visibility information and, in the case of a DS-IST, receiver visibility and the associated genuine edge instance.

### Acoustic Scene Graph (ASG)

A graph where nodes represent individual rooms and edges represent portals (physical separators like doors, walls, windows). Each portal stores two room IDs, a state (open/closed), and a pointer to its counter-portal in the adjacent room.

Rooms connected by open portals are merged into *room groups* -- single acoustic spaces where IS and RT simulations are performed. The ASG enables depth-first search across rooms to find all sound propagation paths, producing a Path Search Tree (PST).

### Path Search Tree (PST) and Propagation Path Graph (PPG)

The PST is the hierarchical result of depth-first search across the ASG. It is transformed into a PPG (directed acyclic graph) that serves as a construction plan for the filter network:

1. PST root node becomes the source node $S$
2. PST edges become portal filter nodes
3. PST nodes become room transfer function edges (RIR/BRIR)
4. All PST leaf nodes merge into a single receiver node $R$

The PPG enables source elimination: frequency bands that are completely attenuated by portal transmission can be skipped in the RT simulation, reducing computational load significantly.

### Energy Particle (RT)

Each ray-traced particle carries its complete propagation state:

```c
struct EnergyParticle {
    Point  StartPoint, EndPoint;   // Current segment endpoints
    Vec    Direction;              // Current travel direction
    float  CurrentEnergy;          // Remaining energy
    float  CurrentTime;            // Accumulated travel time
    float  CurrentSpeed;           // Speed of sound (may vary)
    bool   HasDiffuseHistory;      // Has been scattered at least once?
    bool   AllowDetection;         // May be counted by detector?
    int    LastHitDC_ID;           // Last deflection cylinder hit (-1 = none)
    int    PreEDreflectionOrder;   // Reflections before first edge diffraction
    int    EDreflectionOrder;      // Reflections after first edge diffraction
    int    ReflectionOrder;        // Total number of reflections
    int    EDOrder;                // Number of edge diffractions
};
```

The status variables and counters enable the hybrid detection logic (see Section 4.2) and ensure correct energy partitioning between the IS and RT methods.

---

## 11. Validation Results

### 11.1 Hybrid Model Validation

The hybrid IS+RT model was validated against analytical predictions in a 1344 m^3 shoebox room. The reverberation time T30 was computed for scattering coefficients ranging from 0 to 1 in steps of 0.05.

Key results:

- At scattering coefficient $s = 1$ (fully diffuse field), the simulated T30 exactly matches the Eyring prediction. This confirms that the RT method correctly models diffuse energy decay.
- At $s = 0$ (purely specular), the T30 is highest, matching the known behavior that specular reflections in non-diffuse rooms produce longer reverberation.
- The transition between these extremes falls exponentially, confirming that the IS and RT energy models are correctly combined.
- Simulation parameters: IS order 4, 25000 particles/band, octave-band resolution, 36 receivers, detector radius 0.25 m.

### 11.2 Sound Transmission Validation

A virtual two-room test facility (each room 90 m^3, partition area 16 m^2) was modeled with IS order 3, 20000 particles/band at 1/3-octave resolution. The simulated sound reduction indices were compared against the input portal transfer function:

- Best agreement at 2--16 kHz: average error <1.5 dB
- Below 2 kHz: similar slope but ~2.5 dB offset (expected, as frequencies approach Schroeder frequency)
- Very low frequencies (<125 Hz): errors increase to ~8 dB (well below Schroeder frequency, GA is not valid here)
- At 20 kHz: 8 dB error due to insufficient particle count for the massive air absorption at this frequency

### 11.3 Edge Diffraction Validation

Validated against Svensson's Edge Diffraction Toolbox (an exact analytical solution) using a 10-degree wedge with 100 m edge length:

**IS-based diffraction (BTME):**

- Time-domain impulse responses match the reference with correct onset and cutoff
- Frequency-domain transmission levels: mean error <0.5 dB below 12 kHz
- Above 12 kHz: errors grow to ~2 dB due to less accurate numerical integration in the BTME
- With overall transmission levels of -35 dB at these high frequencies, the deviation is imperceptible

**RT-based diffraction (DAPDF):**

- At 50 Hz: good match across all receiver angles, with known errors in the deep shadow zone for low frequencies
- At 10 kHz: simulation results closely match the reference across view and shadow zones
- The stochastic DAPDF model reproduces diffraction effects with ~1 dB accuracy for typical geometries

### 11.4 Perceptual Thresholds (Listening Tests)

3-AFC (three-alternative forced-choice) experiments determined the minimum simulation parameters for perceptually indistinguishable auralization:

| Parameter            | Lab (audio only) | CAVE (audio+visual) |
| -------------------- | ---------------- | ------------------- |
| Particles per band   | ~7,900           | ~6,300              |
| IS order             | 3 sufficient     | 3 sufficient        |
| Histogram bin width  | 5 ms adequate    | 5 ms adequate       |

Key finding: **visual stimuli reduce auditory sensitivity** -- fewer particles are needed when visual immersion is present (multimodal masking effect). This is relevant for our web demo where visual context is always present.

Detection thresholds for acoustic parameters (from ISO 3382 and listening studies):

- T60 JND (just noticeable difference): ~5%
- EDT JND: ~5%
- C80 JND: ~1 dB
- IACC JND: ~0.075

---

## 12. Implementation Notes Relevant to algo-acoustics

This section maps the RAVEN dissertation's concepts to our codebase, identifies what we already implement, what differs, and what could be added. Each subsection references both the RAVEN formulas above and the specific Go packages in our repo.

### 12.1 What We Already Implement

The following RAVEN techniques have direct counterparts in our codebase:

**Image Source Method** (`ism/` package)

- Mirror source construction across wall planes -- both shoebox (unfolded enumeration with `WallMask` bit tracking) and arbitrary mesh rooms (`solveMesh()` + `GenerateMeshImageSources()` with BVH-accelerated visibility)
- Audibility test via back-tracing through source chains with BVH intersection (our equivalent of RAVEN's BSP-accelerated test)
- IS reflection spectrum construction with per-band wall reflection factors, source/receiver directivity, and air absorption
- Pressure reflectance model from absorption coefficients using `wayverbPressureReflectance()` (Vorlaender impedance-based, comparable to RAVEN's Eq. 2.7--2.8)

**Scattering** (`scene/`, `raytrace/` packages)

- Per-band scattering coefficients in material definitions (`Material.Scattering[6]` and `ScatteringByBand[]`)
- Specular/diffuse energy splitting at each reflection in the ray tracer, with three strategies: Probabilistic (random decision), DeterministicBlend (weighted combination), and RussianRoulette
- Lambert cosine-weighted hemisphere sampling for diffuse reflections (`LambertDirection()`)

**Ray Tracing** (`raytrace/` package)

- Monte Carlo energy particle model with configurable bounce depth
- Fibonacci sphere + stratified direction sampling for uniform initial ray distribution
- Spherical receiver with configurable radius (default 0.25 m)
- Energy histogram accumulation in time-and-frequency bins
- ISO 9613-1 air absorption (`AlphaAirISO9613_1()`) -- frequency, temperature, humidity dependent

**Band-Filtered Rendering** (`ir/` package)

- Half-cosine crossover filters between adjacent bands (`buildBandpassWeights()`) -- functionally equivalent to RAVEN's Raised Cosine Filter (Eq. 5.46), ensuring a smooth partition of unity across the spectrum
- Per-band FFT filtering with frequency-domain weighting (`RenderMonoBanded()`)
- Both scalar (delta) and banded rendering paths

**Hybrid IS+RT Combination** (`hybrid/` package)

- Three crossover modes: TimeBased (explicit threshold), OrderBased (IS reflection order), EnergyBased (automatic after early-field decay)
- Smooth crossfade with configurable fade window (`ApplyFadeWithWindow()` with raised-cosine or triangular windows)
- Energy calibration between IS and RT via `calibratedRayLaunchEnergy()` -- our implementation of RAVEN's Eq. 5.54
- Final IR: sum of early (IS) and late (RT) buffers, corresponding to $h_{hybrid}(t) = h_{IS}(t) + h_{RT}(t)$

**Low-Frequency Modal/Wave Blending** (`pde/`, `hybrid/` packages)

- Shoebox modal analysis (`ShoeboxModes()`) for analytical mode frequencies
- Full 3D IBM (Immersed Boundary Method) PDE solver with compressed sparse grid
- Frequency-dependent impedance boundaries via Auxiliary Differential Equations (ADE walls) -- this goes *beyond* RAVEN, which uses only Geometrical Acoustics above the Schroeder frequency
- FFT-based frequency-domain blending of GA and PDE results (`BlendLowFreq()`) with smooth crossover weighting

**First-Order Edge Diffraction** (`ism/`, `geometry/` packages)

- UTD (Uniform Theory of Diffraction) based first-order diffraction via `SolveWithDiffraction()` and `DiffractionEvents()`
- Fresnel transition function with three evaluation regimes (asymptotic for $x > 10$, power series for $x < 0.3$, numerical integration for intermediate values)
- Edge extraction and coplanar merging from mesh geometry (`ExtractDiffractionEdges()`)
- Band-specific diffraction coefficient accumulation with -60 dB culling

**Diffraction in Ray Tracing** (`raytrace/` package)

- `spawnDiffractionBranches()`: spawns diffraction rays near edges using Keller cone sampling
- Proximity weighting and energy attenuation for diffraction events
- Configurable angular threshold and cone sample count

**Binaural Rendering** (`ir/`, `hrtf/` packages)

- HRTF dataset interface with lookup by direction
- Barycentric interpolation within enclosing triangles on the measurement grid (`InterpolateHRIR()`)
- Binaural rendering via per-event HRIR convolution (`RenderBinaural()`)

**Source Directivity** (`directivity/` package)

- `Model` interface: `GainLinear(freqHz, direction)`
- Omni-directional and power-cardioid models

### 12.2 What RAVEN Does Differently

These areas represent meaningful differences in approach between RAVEN and our implementation:

**Spatial Data Structures**

- RAVEN uses BSP trees for static geometry (O(log N) intersection, but expensive rebuild) and Spatial Hashing for dynamic geometry (O(1) lookup, O(m) insert/delete). We use only BVH (Bounding Volume Hierarchy) with midpoint-split construction. BVH provides O(log N) intersection like BSP but is generally faster for ray tracing workloads and easier to update incrementally. The lack of spatial hashing is only a limitation if we need real-time dynamic geometry modification.

**Receiver Model for Ray Tracing**

- RAVEN subdivides the spherical detector into directivity groups (DGs) by azimuth/elevation for binaural RT rendering. Energy particles are binned by direction of arrival, and each DG selects the most probable HRIR for that time slot. We apply HRTFs per-event instead, which is simpler but doesn't provide directional information for the diffuse late field from RT.

**Image Source Organization**

- RAVEN stores all image sources in an Image Source Tree (IST) with dynamic insertion/deletion support for runtime geometry changes. It also uses Plane-Polygon Maps to reduce IS count by mirroring across distinct planes rather than individual polygons. We use flat enumeration for shoeboxes and BVH-based mesh traversal for arbitrary rooms. The IST structure would mainly benefit us if we added real-time dynamic geometry support.

**Poisson Noise Process**

- RAVEN generates the late-field temporal fine structure using a formal Poisson noise process: random Dirac delta times drawn from the exponential distribution, filtered per band, then weighted by the energy histogram. We use a simpler approach: random phase assignment to histogram bins when converting to time-domain samples. The difference is audible mainly in the temporal texture of late reverberation.

**Hybrid Detection Logic**

- RAVEN tracks per-particle state (`HasDiffuseHistory`, `PreEDreflectionOrder`, `EDOrder`, etc.) to precisely avoid double-counting between IS and RT. Our hybrid blending uses time/order/energy-based crossover without per-particle tracking. This is simpler but may produce slight energy overlap or gaps at the IS/RT transition boundary.

### 12.3 What We Could Add (Ordered by Impact)

The following additions would improve our simulation quality, roughly ordered by expected perceptual impact:

#### High Impact

**1. Diffuse Rain from RT Reflection Points**

Currently, our ray tracer only counts energy when a particle physically enters the detector sphere. RAVEN's diffuse rain sends secondary energy toward the detector at every diffuse reflection, dramatically reducing the particle count needed for convergence. The formula (Section 3.2, Eq. 5.20) is straightforward to implement:

$$E_s = E_P \cdot (1-\alpha) \cdot s \cdot (1 - \cos\frac{\gamma}{2}) \cdot 2 \cdot \cos(\Theta) \cdot e^{-m \cdot r}$$

This requires: (a) a visibility check from each diffuse reflection point to the receiver, (b) computing $\Theta$ (angle between surface normal and direction to receiver), and (c) adding the energy to the appropriate histogram bin based on travel time. The visibility check can use our existing BVH. Expected benefit: 4-10x reduction in required particle count for equivalent histogram quality.

**2. Poisson Noise Process for Late-Field Synthesis**

Replace our random-phase histogram-to-time-domain conversion with the formal Poisson process (Section 3.4). The mean event rate $\mu = 4\pi c^3 t^2 / V$ increases with time, producing increasingly dense reflections. The process is valid after $t_0 \approx 0.0014\sqrt[3]{V}$. Key implementation steps:

- Generate Poisson-distributed Dirac delta times using $\Delta t_A(z) = \frac{1}{\mu}\ln(1/z)$
- Cap $\mu$ at 10,000 s$^{-1}$ to avoid artifacts
- Assign random signs (positive/negative) to create noise-like texture
- Filter per band with our existing half-cosine crossover
- Weight by energy histogram envelope (Eq. 5.47)

This produces more realistic temporal microstructure in the late reverberation, particularly audible with headphones.

**3. Sound Transmission Through Walls** (`scene/`, `raytrace/` packages)

Add transmission coefficients to materials and implement the secondary source model (Section 5.1). For the simplest useful case (single wall between two rooms):

- Add `TransmissionByBand[]` to `Material` (or derive from sound reduction index $R$: $\tau = 10^{-R/10}$)
- When a ray hits a wall with a non-zero transmission coefficient, spawn a secondary particle on the other side with energy $E \cdot \tau$
- Model the transmitted source as a point source at the wall center in the receiving room

This would enable basic multi-room scenarios without the full Acoustic Scene Graph infrastructure.

#### Medium Impact

**4. Higher-Order Edge Diffraction**

Our current first-order UTD diffraction handles the most important case (direct sound bending around a single edge). RAVEN additionally models second-order diffraction (sound diffracting around two edges in sequence) using the BTME formulation with an intermediate receiver at the midpoint between edges (Section 2.5). This matters for:

- Double-doorway scenarios (sound propagating through two openings)
- L-shaped corridors
- Receivers deep in the shadow of multiple obstacles

Implementation would extend `SolveWithDiffraction()` to enumerate edge-to-edge paths using our existing edge extraction infrastructure.

**5. DAPDF for Stochastic Diffraction** (`raytrace/` package)

Replace our Keller-cone diffraction sampling with RAVEN's DAPDF model (Section 3.3). The DAPDF is physically motivated (derived from Fraunhofer single-slit diffraction) and provides better energy distribution:

$$D(v) = D_0 \cdot \begin{cases} 1 - v^2 & |v| \le v_0 \\ \frac{1/2}{\sqrt{2-1+v^2}} & |v| > v_0 \end{cases}$$

This requires placing frequency-dependent deflection cylinders ($r = 7\lambda$) around edges and integrating the DAPDF over the angular range subtended by visible detectors. The six piecewise closed-form solutions for the integral (Eqs. 5.29--5.34) make this efficient at runtime.

**6. Directivity Groups for Binaural RT**

Subdivide our spherical detector into angular sectors (directivity groups). Each sector accumulates a separate energy histogram. During BRIR construction, the most probable HRIR is selected per time slot based on the sector hit probabilities (Section 3.5, Fig. 5.19). This provides directional information for the late diffuse field, improving spatial perception of reverberation.

**7. Plane-Polygon Map for Mesh ISM**

When our mesh ISM encounters rooms with many coplanar polygons (common in architectural models), mirroring across distinct planes instead of individual triangles would reduce the IS count significantly. For a room with $n$ polygons on $p$ planes at order $i$: the IS count drops from $n(n-1)^{i-1}$ to $p(p-1)^{i-1}$ (Section 2.4, Eq. 9.1).

#### Lower Impact (Completeness / Future Work)

**8. Frequency-Dependent Source Directivity**

Our cardioid model is frequency-independent. Real instruments and loudspeakers have strongly frequency-dependent radiation patterns (narrow at high frequencies, omnidirectional at low). Adding frequency dependence to the `Model` interface (`GainLinear(freqHz, direction)` already accepts frequency but our models ignore it) would improve high-frequency early reflections.

**9. SOFA File Loading for HRTFs**

Our HRTF infrastructure supports the `Dataset` interface and barycentric interpolation, but SOFA file loading is only a stub. The SOFA format is the standard for exchanging HRTF datasets. Full loading support would allow using individualized or high-resolution HRTF measurements.

**10. Image Source Tree (IST) for Dynamic Geometry**

If we ever need real-time geometry manipulation (adding/removing objects during simulation), RAVEN's IST structure with dynamic insertion/deletion would be more efficient than recomputing all image sources. The IST supports incremental updates: adding a new reflection plane inserts brother nodes at each level, while removing one recursively destroys the affected subtrees (Section 10, Figs. 9.3--9.4).

**11. Per-Particle Hybrid Detection Logic**

For more precise IS/RT energy partitioning, track per-particle state as RAVEN does: `HasDiffuseHistory`, `PreEDreflectionOrder`, `EDreflectionOrder`, `EDOrder` (Section 4.2). This ensures that purely specular paths covered by IS are never double-counted by RT, even for complex reflection sequences involving diffraction.

**12. Multi-Room Acoustic Scene Graph**

The full ASG/PST/PPG infrastructure (Section 5, Section 10) enables simulation of complex multi-room environments with portal-based room coupling, source elimination (skipping inaudible frequency bands per propagation path), and filter network construction for sound transmission through chains of rooms. This is a major architectural addition that would require portal definitions, room group management, and a path search algorithm.

### 12.4 What We Have That RAVEN Does Not

Our codebase includes several features not present in the RAVEN dissertation:

- **IBM PDE solver with ADE boundaries**: RAVEN operates purely in the Geometrical Acoustics domain above the Schroeder frequency. We have a full 3D wave equation solver (Immersed Boundary Method) with frequency-dependent impedance via Auxiliary Differential Equations, enabling accurate low-frequency modal content.
- **GA/PDE frequency-domain blending**: Our `BlendLowFreq()` merges geometric and wave-based results with a smooth crossover, providing physically accurate simulation across the full frequency range.
- **BVH for mesh rooms**: RAVEN uses BSP trees (polygon-aligned partitioning). Our BVH with midpoint-split construction is generally faster for ray tracing and supports mesh-based ISM out of the box.
- **Multiple reflection strategies**: Our ray tracer offers Probabilistic, DeterministicBlend, and RussianRoulette scattering -- RAVEN uses only probabilistic.
- **WASM web demo**: Browser-based auralization not addressed in the dissertation.

---

*Source: Schroeder, D. (2011). Physically Based Real-Time Auralization of Interactive Virtual Environments. RWTH Aachen University. ISBN 978-3-8325-3031-0.*
