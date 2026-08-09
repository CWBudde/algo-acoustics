# Diffraction Reference Fixtures

This directory reserves the machine-readable external-validation corpus for
Phase 22. It currently contains a reproducible manifest, but no reference CSV:
external validation is **not complete**.

## Current Blocker

The official NTNU EDB2 archive was inspected on 2026-08-09. It contains MATLAB
source for time- and frequency-domain diffraction and a small setup example, but
no machine-readable output for the RAVEN finite-wedge case. Neither MATLAB nor
GNU Octave is installed in the validation environment. The RAVEN dissertation
publishes plots and summary error statements, not the underlying numeric series.
Consequently, numeric values cannot be produced here without an unverified
reimplementation and must not be treated as goldens.

The inspected artifacts were:

| Artifact | SHA-256 |
| --- | --- |
| `EDB2toolbox_25April2015.zip` | `fd8dfcd13848817e116fe059fc25d3f07d462e9e070fd927dc6a2e1238b3af3d` |
| `EDBtoolbox_examples.zip` | `c3558bd7dbd52e0e2fdeac7aa39b1be5c1b3a47d743ab3b68999873b62bef3f0` |

Both archives are linked from Peter Svensson's official
[software page](https://ulfps.folk.ntnu.no/software/index.html) and are licensed
under GPL-3.0 according to that page. They are intentionally not vendored.

## Reproduction Contract

[`reference-manifest.json`](reference-manifest.json) fixes the repository's
coordinate embedding, source/receiver sweep, frequencies, output columns, and
source artifacts. It canonicalizes the rotation-invariant geometry described in
Section 10.3 of the
[RAVEN dissertation](https://publications.rwth-aachen.de/record/50580/files/3875.pdf):
a 10-degree, 100 m finite wedge; source and 15 receivers at 10 m radius in the
edge-normal plane; receiver angles from -84 to +84 degrees in 12-degree steps.

To complete the fixture with a licensed MATLAB installation:

1. Download EDB2 from the official page and verify its SHA-256.
2. Check in the MATLAB generator beside the manifest. It must construct the
   manifest coordinates, request the isolated diffraction transfer `tfdiff`, and
   sample exactly the four listed frequencies.
3. Export `svensson_edb2_single_wedge.csv` with the manifest column order and
   decimal values at full MATLAB precision. Do not digitize the thesis plots.
4. Run the generator twice and require byte-identical CSV output.
5. Add the generator SHA-256, MATLAB release, platform, and generated CSV
   SHA-256 to the manifest; then add a Go test that consumes the CSV.
6. Compare complex transfer magnitudes using the same free-field normalization
   on both sides. Keep phase available for diagnosis, but apply the Phase 22
   acceptance threshold to magnitude in dB.

The repository implementation should be compared at 50 Hz, 500 Hz, 5 kHz, and
10 kHz. BTME's target is at most 0.5 dB error below 12 kHz. DAPDF is stochastic:
compare repeated seeded means, target at most 1 dB mean error, and separately
report the documented low-frequency deep-shadow exception rather than hiding it
in the mean.

