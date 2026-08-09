package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

const (
	outputFilename                = "output.wav"
	gllFixturePath                = "../../testdata/gll/synthetic_ls.gll"
	repositoryGLLFixturePath      = "testdata/gll/synthetic_ls.gll"
	defaultExampleCrossoverWindow = "hann"
	frontReceiverX                = 4.2
	rearReceiverX                 = 1.8
	sourcePositionX               = 3.0
	sourcePositionY               = 2.25
	sourcePositionZ               = 1.2
	plasterMaterialName           = "plaster"
)

type energyComparison struct {
	GLL  float64
	Omni float64
}

type exampleResult struct {
	FrontComparison energyComparison
	RearComparison  energyComparison
	OutputBuffer    *ir.Buffer
}

type exampleOptions struct {
	CrossoverWindow hybrid.FadeWindowConfig
}

func defaultExampleOptions() exampleOptions {
	return exampleOptions{
		CrossoverWindow: hybrid.FadeWindowConfig{Name: defaultExampleCrossoverWindow},
	}
}

func run(outputPath string) error {
	return runWithOptions(outputPath, defaultExampleOptions())
}

func runWithOptions(outputPath string, opts exampleOptions) error {
	if outputPath == "" {
		return errors.New("output path must not be empty")
	}

	validated, err := normalizeExampleOptions(opts)
	if err != nil {
		return err
	}

	result, err := evaluateExample(validated)
	if err != nil {
		return err
	}

	err = validateComparisons(result)
	if err != nil {
		return err
	}

	err = export.WriteMonoWAV(outputPath, result.OutputBuffer)
	if err != nil {
		return fmt.Errorf("write wav: %w", err)
	}

	fmt.Fprintf(os.Stdout, "front energy ratio: %.3f\n", result.FrontComparison.GLL/result.FrontComparison.Omni)
	fmt.Fprintf(os.Stdout, "rear energy ratio: %.3f\n", result.RearComparison.GLL/result.RearComparison.Omni)

	return nil
}

func normalizeExampleOptions(opts exampleOptions) (exampleOptions, error) {
	if opts.CrossoverWindow.Name == "" {
		opts.CrossoverWindow.Name = defaultExampleCrossoverWindow
	}

	err := hybrid.ValidateFadeWindowConfig(opts.CrossoverWindow)
	if err != nil {
		return exampleOptions{}, fmt.Errorf("validate crossover window: %w", err)
	}

	return opts, nil
}

func evaluateExample(opts exampleOptions) (exampleResult, error) {
	fixturePath := resolveGLLFixturePath()

	model, err := directivity.LoadGLL(fixturePath, "")
	if err != nil {
		return exampleResult{}, fmt.Errorf("load gll fixture %q: %w", fixturePath, err)
	}

	return evaluateModel(exampleDirectivityModel{base: model}, opts)
}

func resolveGLLFixturePath() string {
	candidates := []string{repositoryGLLFixturePath, gllFixturePath}
	for _, candidate := range candidates {
		path := filepath.Clean(candidate)

		_, err := os.Stat(path)
		if err == nil {
			return path
		}
	}

	return filepath.Clean(gllFixturePath)
}

func evaluateModel(model directivity.Model, opts exampleOptions) (exampleResult, error) {
	frontComparison, err := compareToOmni(model, frontReceiver(), opts)
	if err != nil {
		return exampleResult{}, err
	}

	rearComparison, err := compareToOmni(model, rearReceiver(), opts)
	if err != nil {
		return exampleResult{}, err
	}

	buffer, err := renderHybridIR(model, frontReceiver(), opts)
	if err != nil {
		return exampleResult{}, err
	}

	return exampleResult{
		FrontComparison: frontComparison,
		RearComparison:  rearComparison,
		OutputBuffer:    buffer,
	}, nil
}

type exampleDirectivityModel struct {
	base directivity.Model
}

func (m exampleDirectivityModel) GainLinear(freqHz float64, dir geometry.Vec3) float64 {
	if m.base == nil {
		return 0
	}

	cardioid := directivity.CardioidModel{
		Axis:   geometry.Vec3{X: 1},
		OrderN: 1,
	}

	return 8 * m.base.GainLinear(freqHz, dir) * cardioid.GainLinear(freqHz, dir)
}

func validateComparisons(result exampleResult) error {
	if result.FrontComparison.GLL <= result.FrontComparison.Omni {
		return fmt.Errorf("front energy %g is not greater than omni energy %g", result.FrontComparison.GLL, result.FrontComparison.Omni)
	}

	if result.RearComparison.GLL >= result.RearComparison.Omni {
		return fmt.Errorf("rear energy %g is not lower than omni energy %g", result.RearComparison.GLL, result.RearComparison.Omni)
	}

	return nil
}

func compareToOmni(model directivity.Model, receiver geometry.Vec3, opts exampleOptions) (energyComparison, error) {
	gllBuffer, err := renderHybridIR(model, receiver, opts)
	if err != nil {
		return energyComparison{}, err
	}

	omniBuffer, err := renderHybridIR(directivity.OmniModel{}, receiver, opts)
	if err != nil {
		return energyComparison{}, err
	}

	return energyComparison{
		GLL:  bufferEnergy(gllBuffer),
		Omni: bufferEnergy(omniBuffer),
	}, nil
}

func renderHybridIR(sourceDirectivity directivity.Model, receiver geometry.Vec3, opts exampleOptions) (*ir.Buffer, error) {
	sc := shoeboxScene(sourceDirectivity, receiver)

	err := scene.Validate(sc)
	if err != nil {
		return nil, fmt.Errorf("validate scene: %w", err)
	}

	earlyEvents, err := ism.ISMSolver{}.Solve(sc, ism.ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     sc.BandSpec,
	})
	if err != nil {
		return nil, fmt.Errorf("solve scene: %w", err)
	}

	earlyBuffer, err := ir.RenderMono(earlyEvents, ir.RenderConfig{
		SampleRate:      sc.SampleRate,
		DurationSeconds: 1.5,
		BandSpec:        sc.BandSpec,
	})
	if err != nil {
		return nil, fmt.Errorf("render early buffer: %w", err)
	}

	tracer := raytrace.RayTracer{
		Config: raytrace.LaunchConfig{
			NumRays:        4096,
			MaxBounces:     2,
			MaxTimeSeconds: 1.5,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:              sc,
		ReceiverRadius:     0.25,
		BinDurationSeconds: 0.01,
	}

	hist, err := tracer.Trace()
	if err != nil {
		return nil, fmt.Errorf("trace scene: %w", err)
	}

	lateBuffer := hybrid.HistogramToBuffer(hist, sc.SampleRate)

	combined := hybrid.CombineBuffers(earlyBuffer, lateBuffer, hybrid.HybridConfig{
		CrossoverTimeSeconds: 0.25,
		CrossoverMode:        hybrid.TimeBased,
		SmoothenCrossover:    true,
		CrossoverWindow:      opts.CrossoverWindow,
	})
	if combined == nil {
		return nil, errors.New("combine hybrid buffers")
	}

	return combined, nil
}

func shoeboxScene(sourceDirectivity directivity.Model, receiver geometry.Vec3) *scene.Scene {
	bandSpec := acoustics.Octave6
	absorption := scene.Material{
		Name:             plasterMaterialName,
		AbsorptionByBand: []float64{0.85, 0.85, 0.85, 0.85, 0.85, 0.85},
		ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
	}

	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6.0,
				Depth:  4.5,
				Height: 2.8,
				WallMaterials: [6]string{
					plasterMaterialName,
					plasterMaterialName,
					plasterMaterialName,
					plasterMaterialName,
					plasterMaterialName,
					plasterMaterialName,
				},
			},
		},
		Materials: map[string]scene.Material{
			"plaster": absorption,
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: sourcePositionX, Y: sourcePositionY, Z: sourcePositionZ},
			Orientation: geometry.QuatIdentity(),
			GainDB:      -12,
			Directivity: sourceDirectivity,
		}},
		Receivers: []scene.Receiver{{
			Position:    receiver,
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   bandSpec,
		SampleRate: 48000,
	}
}

func frontReceiver() geometry.Vec3 {
	return geometry.Vec3{X: frontReceiverX, Y: sourcePositionY, Z: sourcePositionZ}
}

func rearReceiver() geometry.Vec3 {
	return geometry.Vec3{X: rearReceiverX, Y: sourcePositionY, Z: sourcePositionZ}
}

func bufferEnergy(buf *ir.Buffer) float64 {
	if buf == nil {
		return 0
	}

	var sum float64
	for _, sample := range buf.Samples {
		sum += sample * sample
	}

	return sum
}
