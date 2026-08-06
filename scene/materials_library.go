package scene

const materialGlass = "glass"

// MaterialLibrary provides a collection of pre-configured materials with published
// absorption and scattering data for common surfaces.
//
// Data sources:
// - ISO 17497-1/2: Measurement of scattering properties of materials
// - Cox & D'Antonio (2017): Acoustic Absorbers and Diffusers (3rd ed.)
// - Vorländer (2020): Auralization: Fundamentals of Acoustics, Modelling, Simulation
// - Bläse et al. (1999): Sound scattering on random-incidence absorption materials
//
// Octave bands (bottom frequency in Hz):
// [0]: 125 Hz.
// [1]: 250 Hz.
// [2]: 500 Hz.
// [3]: 1000 Hz.
// [4]: 2000 Hz.
// [5]: 4000 Hz.
var MaterialLibrary = map[string]Material{
	// Hard, reflective surfaces (low absorption, low scattering)

	materialGlass: {
		Name: materialGlass,
		AbsorptionByBand: []float64{
			0.03, // 125 Hz
			0.03, // 250 Hz
			0.04, // 500 Hz
			0.04, // 1000 Hz
			0.05, // 2000 Hz
			0.05, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.05, // 125 Hz: low, smooth surface
			0.05, // 250 Hz
			0.06, // 500 Hz
			0.08, // 1000 Hz
			0.10, // 2000 Hz
			0.12, // 4000 Hz: slightly higher at higher frequencies
		},
	},

	"painted_concrete": {
		Name: "painted_concrete",
		AbsorptionByBand: []float64{
			0.01, // 125 Hz
			0.02, // 250 Hz
			0.02, // 500 Hz
			0.03, // 1000 Hz
			0.04, // 2000 Hz
			0.05, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.02, // 125 Hz: very smooth when painted
			0.02, // 250 Hz
			0.03, // 500 Hz
			0.04, // 1000 Hz
			0.05, // 2000 Hz
			0.06, // 4000 Hz
		},
	},

	"plasterboard": {
		Name: "plasterboard",
		AbsorptionByBand: []float64{
			0.08, // 125 Hz
			0.09, // 250 Hz
			0.12, // 500 Hz
			0.15, // 1000 Hz
			0.12, // 2000 Hz
			0.10, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.04, // 125 Hz
			0.05, // 250 Hz
			0.06, // 500 Hz
			0.08, // 1000 Hz
			0.10, // 2000 Hz
			0.12, // 4000 Hz
		},
	},

	// Masonry and stone

	"exposed_brick": {
		Name: "exposed_brick",
		AbsorptionByBand: []float64{
			0.03, // 125 Hz
			0.03, // 250 Hz
			0.04, // 500 Hz
			0.05, // 1000 Hz
			0.07, // 2000 Hz
			0.09, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.10, // 125 Hz: rough texture
			0.12, // 250 Hz
			0.15, // 500 Hz
			0.18, // 1000 Hz
			0.20, // 2000 Hz
			0.22, // 4000 Hz
		},
	},

	"concrete_block": {
		Name: "concrete_block",
		AbsorptionByBand: []float64{
			0.04, // 125 Hz
			0.05, // 250 Hz
			0.06, // 500 Hz
			0.07, // 1000 Hz
			0.09, // 2000 Hz
			0.10, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.08, // 125 Hz
			0.10, // 250 Hz
			0.12, // 500 Hz
			0.15, // 1000 Hz
			0.18, // 2000 Hz
			0.20, // 4000 Hz
		},
	},

	// Floors

	"wooden_floor": {
		Name: "wooden_floor",
		AbsorptionByBand: []float64{
			0.08, // 125 Hz
			0.07, // 250 Hz
			0.06, // 500 Hz
			0.06, // 1000 Hz
			0.07, // 2000 Hz
			0.07, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.04, // 125 Hz
			0.05, // 250 Hz
			0.06, // 500 Hz
			0.07, // 1000 Hz
			0.08, // 2000 Hz
			0.10, // 4000 Hz
		},
	},

	"carpet_on_concrete": {
		Name: "carpet_on_concrete",
		AbsorptionByBand: []float64{
			0.08, // 125 Hz
			0.20, // 250 Hz
			0.35, // 500 Hz
			0.45, // 1000 Hz
			0.52, // 2000 Hz
			0.55, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.10, // 125 Hz
			0.12, // 250 Hz
			0.15, // 500 Hz
			0.18, // 1000 Hz
			0.20, // 2000 Hz
			0.22, // 4000 Hz
		},
	},

	// Textiles and soft furnishings

	"stage_curtain": {
		Name: "stage_curtain",
		AbsorptionByBand: []float64{
			0.20, // 125 Hz
			0.35, // 250 Hz
			0.50, // 500 Hz
			0.65, // 1000 Hz
			0.70, // 2000 Hz
			0.65, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.15, // 125 Hz
			0.18, // 250 Hz
			0.20, // 500 Hz
			0.22, // 1000 Hz
			0.25, // 2000 Hz
			0.28, // 4000 Hz
		},
	},

	"audience_seated": {
		Name: "audience_seated",
		AbsorptionByBand: []float64{
			0.50, // 125 Hz
			0.55, // 250 Hz
			0.60, // 500 Hz
			0.65, // 1000 Hz
			0.62, // 2000 Hz
			0.58, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.20, // 125 Hz
			0.25, // 250 Hz
			0.30, // 500 Hz
			0.32, // 1000 Hz
			0.35, // 2000 Hz
			0.38, // 4000 Hz
		},
	},

	"audience_empty": {
		Name: "audience_empty",
		AbsorptionByBand: []float64{
			0.08, // 125 Hz
			0.10, // 250 Hz
			0.12, // 500 Hz
			0.15, // 1000 Hz
			0.18, // 2000 Hz
			0.20, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.05, // 125 Hz
			0.06, // 250 Hz
			0.08, // 500 Hz
			0.10, // 1000 Hz
			0.12, // 2000 Hz
			0.15, // 4000 Hz
		},
	},

	// Diffusing and scattering surfaces

	"qrd_diffuser": {
		Name: "qrd_diffuser",
		AbsorptionByBand: []float64{
			0.05, // 125 Hz
			0.05, // 250 Hz
			0.05, // 500 Hz
			0.08, // 1000 Hz
			0.10, // 2000 Hz
			0.12, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.60, // 125 Hz: high scattering design
			0.65, // 250 Hz
			0.70, // 500 Hz
			0.75, // 1000 Hz
			0.78, // 2000 Hz
			0.80, // 4000 Hz: maximum scattering effect
		},
	},

	"bookshelf_sparse": {
		Name: "bookshelf_sparse",
		AbsorptionByBand: []float64{
			0.10, // 125 Hz
			0.12, // 250 Hz
			0.15, // 500 Hz
			0.18, // 1000 Hz
			0.18, // 2000 Hz
			0.15, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.35, // 125 Hz
			0.40, // 250 Hz
			0.45, // 500 Hz
			0.50, // 1000 Hz
			0.52, // 2000 Hz
			0.52, // 4000 Hz
		},
	},

	"bookshelf_dense": {
		Name: "bookshelf_dense",
		AbsorptionByBand: []float64{
			0.15, // 125 Hz
			0.20, // 250 Hz
			0.25, // 500 Hz
			0.30, // 1000 Hz
			0.28, // 2000 Hz
			0.24, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.50, // 125 Hz: more scattering due to density
			0.55, // 250 Hz
			0.60, // 500 Hz
			0.62, // 1000 Hz
			0.62, // 2000 Hz
			0.62, // 4000 Hz
		},
	},

	// Additional reference materials

	"gypsum_board": {
		Name: "gypsum_board",
		AbsorptionByBand: []float64{
			0.12, // 125 Hz
			0.09, // 250 Hz
			0.07, // 500 Hz
			0.08, // 1000 Hz
			0.09, // 2000 Hz
			0.12, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.04, // 125 Hz
			0.05, // 250 Hz
			0.06, // 500 Hz
			0.08, // 1000 Hz
			0.10, // 2000 Hz
			0.12, // 4000 Hz
		},
	},

	"linoleum_floor": {
		Name: "linoleum_floor",
		AbsorptionByBand: []float64{
			0.02, // 125 Hz
			0.03, // 250 Hz
			0.03, // 500 Hz
			0.04, // 1000 Hz
			0.04, // 2000 Hz
			0.05, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.02, // 125 Hz
			0.03, // 250 Hz
			0.04, // 500 Hz
			0.05, // 1000 Hz
			0.06, // 2000 Hz
			0.07, // 4000 Hz
		},
	},

	"marble_wall": {
		Name: "marble_wall",
		AbsorptionByBand: []float64{
			0.01, // 125 Hz
			0.01, // 250 Hz
			0.01, // 500 Hz
			0.02, // 1000 Hz
			0.02, // 2000 Hz
			0.03, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.04, // 125 Hz: high polish, low scattering
			0.05, // 250 Hz
			0.06, // 500 Hz
			0.08, // 1000 Hz
			0.10, // 2000 Hz
			0.12, // 4000 Hz
		},
	},

	"plaster_rough": {
		Name: "plaster_rough",
		AbsorptionByBand: []float64{
			0.12, // 125 Hz
			0.10, // 250 Hz
			0.09, // 500 Hz
			0.09, // 1000 Hz
			0.10, // 2000 Hz
			0.12, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.08, // 125 Hz
			0.10, // 250 Hz
			0.12, // 500 Hz
			0.15, // 1000 Hz
			0.18, // 2000 Hz
			0.20, // 4000 Hz
		},
	},

	"acoustic_foam_wedge": {
		Name: "acoustic_foam_wedge",
		AbsorptionByBand: []float64{
			0.15, // 125 Hz
			0.35, // 250 Hz
			0.75, // 500 Hz
			0.85, // 1000 Hz
			0.80, // 2000 Hz
			0.75, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.10, // 125 Hz: foam absorbs rather than scatters
			0.12, // 250 Hz
			0.15, // 500 Hz
			0.18, // 1000 Hz
			0.20, // 2000 Hz
			0.22, // 4000 Hz
		},
	},

	"curtain_light": {
		Name: "curtain_light",
		AbsorptionByBand: []float64{
			0.10, // 125 Hz
			0.15, // 250 Hz
			0.20, // 500 Hz
			0.25, // 1000 Hz
			0.28, // 2000 Hz
			0.25, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.12, // 125 Hz
			0.15, // 250 Hz
			0.18, // 500 Hz
			0.20, // 1000 Hz
			0.22, // 2000 Hz
			0.25, // 4000 Hz
		},
	},

	"curtain_heavy": {
		Name: "curtain_heavy",
		AbsorptionByBand: []float64{
			0.25, // 125 Hz
			0.40, // 250 Hz
			0.55, // 500 Hz
			0.65, // 1000 Hz
			0.68, // 2000 Hz
			0.60, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.15, // 125 Hz
			0.18, // 250 Hz
			0.22, // 500 Hz
			0.25, // 1000 Hz
			0.28, // 2000 Hz
			0.30, // 4000 Hz
		},
	},

	"tile_floor": {
		Name: "tile_floor",
		AbsorptionByBand: []float64{
			0.01, // 125 Hz
			0.01, // 250 Hz
			0.02, // 500 Hz
			0.02, // 1000 Hz
			0.03, // 2000 Hz
			0.03, // 4000 Hz
		},
		Scattering: [NumBands]float64{
			0.06, // 125 Hz
			0.08, // 250 Hz
			0.10, // 500 Hz
			0.12, // 1000 Hz
			0.15, // 2000 Hz
			0.18, // 4000 Hz
		},
	},
}

// FromLibrary returns a copy of the named material from the library.
// If the name is not found, returns an empty Material with the given name.
func (m *Material) FromLibrary(name string) (Material, bool) {
	lib, ok := MaterialLibrary[name]
	if !ok {
		return Material{Name: name}, false
	}

	lib.AbsorptionByBand = append([]float64(nil), lib.AbsorptionByBand...)
	lib.ScatteringByBand = append([]float64(nil), lib.ScatteringByBand...)

	return lib, true
}
