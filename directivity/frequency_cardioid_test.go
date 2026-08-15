package directivity

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestNewFrequencyDependentCardioidValidation(t *testing.T) {
	t.Parallel()

	axis := geometry.Vec3{X: 1, Y: 0, Z: 0}

	tests := []struct {
		name   string
		bands  []float64
		orders []float64
		wantOK bool
	}{
		{name: "valid", bands: []float64{125, 1000, 8000}, orders: []float64{0, 1, 4}, wantOK: true},
		{name: "single band", bands: []float64{1000}, orders: []float64{2}, wantOK: true},
		{name: "no bands", bands: nil, orders: nil},
		{name: "length mismatch", bands: []float64{125, 250}, orders: []float64{1}},
		{name: "non-positive band", bands: []float64{0, 250}, orders: []float64{1, 2}},
		{name: "descending bands", bands: []float64{250, 125}, orders: []float64{1, 2}},
		{name: "repeated band", bands: []float64{250, 250}, orders: []float64{1, 2}},
		{name: "negative order", bands: []float64{125, 250}, orders: []float64{1, -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewFrequencyDependentCardioid(axis, tt.bands, tt.orders)
			if (err == nil) != tt.wantOK {
				t.Fatalf("NewFrequencyDependentCardioid() error = %v, want ok = %v", err, tt.wantOK)
			}
		})
	}
}

func TestFrequencyDependentCardioidCopiesInput(t *testing.T) {
	t.Parallel()

	bands := []float64{125, 4000}
	orders := []float64{0, 4}

	model, err := NewFrequencyDependentCardioid(geometry.Vec3{X: 1}, bands, orders)
	if err != nil {
		t.Fatalf("NewFrequencyDependentCardioid() error = %v", err)
	}

	orders[1] = 99

	if got := model.OrderAt(4000); got != 4 {
		t.Fatalf("OrderAt(4000) = %v after mutating the caller's slice, want 4", got)
	}
}

func TestFrequencyDependentCardioidOrderAt(t *testing.T) {
	t.Parallel()

	model, err := NewFrequencyDependentCardioid(
		geometry.Vec3{X: 1}, []float64{125, 4000}, []float64{0, 4})
	if err != nil {
		t.Fatalf("NewFrequencyDependentCardioid() error = %v", err)
	}

	tests := []struct {
		name   string
		freqHz float64
		want   float64
	}{
		{name: "below table holds first", freqHz: 20, want: 0},
		{name: "first band", freqHz: 125, want: 0},
		// 707.1 Hz is the geometric mean of 125 and 4000, so log-frequency
		// interpolation must land exactly halfway between the orders.
		{name: "geometric mean is halfway", freqHz: math.Sqrt(125 * 4000), want: 2},
		{name: "last band", freqHz: 4000, want: 4},
		{name: "above table holds last", freqHz: 16000, want: 4},
		{name: "non-positive frequency holds first", freqHz: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := model.OrderAt(tt.freqHz)
			if math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("OrderAt(%v) = %v, want %v", tt.freqHz, got, tt.want)
			}
		})
	}
}

func TestFrequencyDependentCardioidNarrowsWithFrequency(t *testing.T) {
	t.Parallel()

	model, err := NewFrequencyDependentCardioid(
		geometry.Vec3{X: 1}, []float64{125, 500, 2000, 8000}, []float64{0, 1, 2, 4})
	if err != nil {
		t.Fatalf("NewFrequencyDependentCardioid() error = %v", err)
	}

	// 60 degrees off-axis: the pattern must tighten monotonically as frequency
	// rises, which is the whole point of the model.
	offAxis := geometry.Vec3{X: math.Cos(math.Pi / 3), Y: math.Sin(math.Pi / 3)}

	previous := math.Inf(1)

	for _, freq := range []float64{125, 500, 2000, 8000} {
		gain := model.GainLinear(freq, offAxis)
		if gain >= previous {
			t.Fatalf("gain at %v Hz = %v did not fall below %v", freq, gain, previous)
		}

		previous = gain
	}

	// On-axis gain is unity at every frequency regardless of order.
	for _, freq := range []float64{125, 500, 2000, 8000} {
		if got := model.GainLinear(freq, geometry.Vec3{X: 1}); math.Abs(got-1) > 1e-12 {
			t.Fatalf("on-axis gain at %v Hz = %v, want 1", freq, got)
		}
	}
}

func TestFrequencyDependentCardioidMatchesFixedCardioid(t *testing.T) {
	t.Parallel()

	axis := geometry.Vec3{X: 0, Y: 1, Z: 0}

	model, err := NewFrequencyDependentCardioid(axis, []float64{125, 8000}, []float64{2, 2})
	if err != nil {
		t.Fatalf("NewFrequencyDependentCardioid() error = %v", err)
	}

	fixed := CardioidModel{Axis: axis, OrderN: 2}

	for _, dir := range []geometry.Vec3{
		{X: 1}, {Y: 1}, {Z: 1}, {X: -1, Y: -1}, {X: 1, Y: 2, Z: -3},
	} {
		for _, freq := range []float64{125, 1000, 8000} {
			want := fixed.GainLinear(freq, dir)
			if got := model.GainLinear(freq, dir); math.Abs(got-want) > 1e-12 {
				t.Fatalf("GainLinear(%v, %v) = %v, want %v from the fixed cardioid", freq, dir, got, want)
			}
		}
	}
}

func TestFrequencyDependentCardioidDegenerate(t *testing.T) {
	t.Parallel()

	// No bands means no directional shaping: order 0 is omnidirectional.
	empty := FrequencyDependentCardioid{Axis: geometry.Vec3{X: 1}}
	if got := empty.GainLinear(1000, geometry.Vec3{X: -1}); got != 1 {
		t.Fatalf("bandless GainLinear() = %v, want 1", got)
	}

	model := FrequencyDependentCardioid{
		Axis:   geometry.Vec3Zero,
		Bands:  []float64{1000},
		Orders: []float64{1},
	}
	if got := model.GainLinear(1000, geometry.Vec3{X: 1}); got != 0 {
		t.Fatalf("zero-axis GainLinear() = %v, want 0", got)
	}

	model.Axis = geometry.Vec3{X: 1}
	if got := model.GainLinear(1000, geometry.Vec3Zero); got != 0 {
		t.Fatalf("zero-direction GainLinear() = %v, want 0", got)
	}
}
