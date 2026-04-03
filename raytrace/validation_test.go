package raytrace

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/scene"
)

type metricWindow struct {
	min float64
	max float64
}

type measuredMetrics struct {
	edt float64
	t30 float64
	c80 float64
	d50 float64
}

func TestRayTracerScatteringChangesLateDecay(t *testing.T) {
	t.Parallel()

	specular := validationRoomScene(0)
	diffuse := validationRoomScene(1)

	config := LaunchConfig{
		NumRays:        100000,
		MaxBounces:     40,
		MaxTimeSeconds: 6,
		SpeedOfSound:   acoustics.SpeedOfSound,
	}

	specMetrics := traceSceneMetrics(t, specular, config, 4.0)
	diffMetrics := traceSceneMetrics(t, diffuse, config, 4.0)

	if diffMetrics.edt <= specMetrics.edt {
		t.Fatalf("EDT with scattering = %v, want greater than specular EDT %v", diffMetrics.edt, specMetrics.edt)
	}

	specTail := tailEnergyAfter(specular, config, 4.0, 0.08)
	diffTail := tailEnergyAfter(diffuse, config, 4.0, 0.08)
	if math.Abs(diffTail-specTail) <= 1e-6 {
		t.Fatalf("late-tail energy unchanged between specular and diffuse cases: spec=%g diffuse=%g", specTail, diffTail)
	}
}

func TestRayTracerT30ConvergesWithMoreRays(t *testing.T) {
	t.Parallel()

	sc := validationRoomScene(0)
	counts := []int{1000, 10000, 100000}
	values := make([]float64, len(counts))

	for i, rays := range counts {
		values[i] = traceSceneMetrics(t, sc, LaunchConfig{
			NumRays:        rays,
			MaxBounces:     40,
			MaxTimeSeconds: 6,
			SpeedOfSound:   acoustics.SpeedOfSound,
		}, 1.0).t30
	}

	baseline := values[len(values)-1]
	if baseline <= 0 || math.IsNaN(baseline) || math.IsInf(baseline, 0) {
		t.Fatalf("baseline T30 = %v, want positive finite", baseline)
	}

	for i, got := range values[:len(values)-1] {
		relative := math.Abs(got-baseline) / baseline
		if relative > 0.02 {
			t.Fatalf("T30 at %d rays = %v, baseline = %v, rel diff = %.3f > 0.02", counts[i], got, baseline, relative)
		}
	}
}

func TestCorpusMetricsStayInExpectedRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fixture string
		t30     metricWindow
		c80     metricWindow
		d50     metricWindow
	}{
		{name: "control room", fixture: "control_room.json", t30: metricWindow{0.20, 0.36}, c80: metricWindow{14, 22}, d50: metricWindow{0.90, 0.97}},
		{name: "lecture room", fixture: "lecture_room.json", t30: metricWindow{0.45, 0.62}, c80: metricWindow{6, 10}, d50: metricWindow{0.60, 0.72}},
		{name: "pa room", fixture: "pa_room.json", t30: metricWindow{0.36, 0.48}, c80: metricWindow{9, 14}, d50: metricWindow{0.78, 0.88}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fixture := filepath.Join("..", "testdata", "rooms", tt.fixture)
			sc, err := scene.LoadSceneFile(fixture)
			if err != nil {
				t.Fatalf("LoadSceneFile(%q) error = %v", fixture, err)
			}

			metrics := traceSceneMetrics(t, sc, LaunchConfig{
				NumRays:        100000,
				MaxBounces:     40,
				MaxTimeSeconds: 6,
				SpeedOfSound:   acoustics.SpeedOfSound,
			}, 3.0)

			assertWithinWindow(t, "T30", metrics.t30, tt.t30)
			assertWithinWindow(t, "C80", metrics.c80, tt.c80)
			assertWithinWindow(t, "D50", metrics.d50, tt.d50)
		})
	}
}

func validationRoomScene(scattering float64) *scene.Scene {
	material := scene.Material{Name: "m", AbsorptionByBand: []float64{0.05, 0.05, 0.05, 0.05, 0.05, 0.05}}
	for i := range material.Scattering {
		material.Scattering[i] = scattering
	}

	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  40,
				Depth:  30,
				Height: 10,
				WallMaterials: [6]string{
					"m", "m", "m", "m", "m", "m",
				},
			},
		},
		Materials:  map[string]scene.Material{"m": material},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 3, Y: 3, Z: 2}, GainDB: 0}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 37, Y: 27, Z: 2}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}

func traceSceneMetrics(t *testing.T, sc *scene.Scene, cfg LaunchConfig, receiverRadius float64) measuredMetrics {
	t.Helper()

	tracer := &RayTracer{Config: cfg, Scene: sc, ReceiverRadius: receiverRadius}
	hist, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}

	buf := hist.ToLateMono(sc.SampleRate)

	edt, err := metrics.EDT(buf)
	if err != nil {
		t.Fatalf("EDT() error = %v", err)
	}

	t30, err := metrics.T30(buf)
	if err != nil {
		t.Fatalf("T30() error = %v", err)
	}

	c80, err := metrics.C80(buf)
	if err != nil {
		t.Fatalf("C80() error = %v", err)
	}

	d50, err := metrics.D50(buf)
	if err != nil {
		t.Fatalf("D50() error = %v", err)
	}

	return measuredMetrics{edt: edt, t30: t30, c80: c80, d50: d50}
}

func tailEnergyAfter(sc *scene.Scene, cfg LaunchConfig, receiverRadius float64, earlySeconds float64) float64 {
	tracer := &RayTracer{Config: cfg, Scene: sc, ReceiverRadius: receiverRadius}
	hist, err := tracer.Trace()
	if err != nil {
		return math.NaN()
	}

	var tail float64
	for binIndex, bin := range hist.Bins {
		binTime := float64(binIndex) * hist.BinDuration
		if binTime <= earlySeconds {
			continue
		}

		for _, energy := range bin.BandEnergy {
			tail += energy
		}
	}

	return tail
}

func assertWithinWindow(t *testing.T, name string, value float64, window metricWindow) {
	t.Helper()

	if math.IsNaN(value) || math.IsInf(value, 0) {
		t.Fatalf("%s = %v, want finite", name, value)
	}

	if value < window.min || value > window.max {
		t.Fatalf("%s = %v, want within [%.3f, %.3f]", name, value, window.min, window.max)
	}
}
