package metrics

import (
	"math"
	"testing"
)

func TestApparentSoundReductionIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		sourceLevel            float64
		receiverLevel          float64
		partitionArea          float64
		receiverAbsorptionArea float64
		want                   float64
		wantNaN                bool
	}{
		{
			name:                   "reference formula",
			sourceLevel:            80,
			receiverLevel:          45,
			partitionArea:          16,
			receiverAbsorptionArea: 10,
			want:                   35 + 10*math.Log10(1.6),
		},
		{
			name:                   "equal areas",
			sourceLevel:            72,
			receiverLevel:          41,
			partitionArea:          16,
			receiverAbsorptionArea: 16,
			want:                   31,
		},
		{
			name:                   "zero partition area",
			sourceLevel:            80,
			receiverLevel:          45,
			partitionArea:          0,
			receiverAbsorptionArea: 10,
			wantNaN:                true,
		},
		{
			name:                   "negative absorption area",
			sourceLevel:            80,
			receiverLevel:          45,
			partitionArea:          16,
			receiverAbsorptionArea: -1,
			wantNaN:                true,
		},
		{
			name:                   "non-finite level",
			sourceLevel:            math.Inf(1),
			receiverLevel:          45,
			partitionArea:          16,
			receiverAbsorptionArea: 10,
			wantNaN:                true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := ApparentSoundReductionIndex(
				test.sourceLevel,
				test.receiverLevel,
				test.partitionArea,
				test.receiverAbsorptionArea,
			)
			if test.wantNaN {
				if !math.IsNaN(got) {
					t.Fatalf("ApparentSoundReductionIndex() = %v, want NaN", got)
				}

				return
			}

			if math.Abs(got-test.want) > 1e-12 {
				t.Fatalf("ApparentSoundReductionIndex() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestApparentSoundReductionIndexPartitionAreaScaling(t *testing.T) {
	t.Parallel()

	base := ApparentSoundReductionIndex(80, 45, 8, 10)
	doubled := ApparentSoundReductionIndex(80, 45, 16, 10)
	wantIncrease := 10 * math.Log10(2)

	if math.Abs((doubled-base)-wantIncrease) > 1e-12 {
		t.Fatalf("doubling partition area changed result by %v dB, want %v dB", doubled-base, wantIncrease)
	}
}

func TestFlankingApparentSoundReductionIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		paths      []float64
		want       float64
		wantNaN    bool
		wantPosInf bool
	}{
		{name: "single 30 dB path", paths: []float64{0.001}, want: 30},
		{name: "two equal paths", paths: []float64{0.001, 0.001}, want: -10 * math.Log10(0.002)},
		{name: "parallel paths may exceed unity", paths: []float64{0.75, 0.75}, want: -10 * math.Log10(1.5)},
		{name: "empty", wantPosInf: true},
		{name: "all zero", paths: []float64{0, 0}, wantPosInf: true},
		{name: "negative coefficient", paths: []float64{-0.1}, wantNaN: true},
		{name: "coefficient above unity", paths: []float64{1.1}, wantNaN: true},
		{name: "non-finite coefficient", paths: []float64{math.NaN()}, wantNaN: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := FlankingApparentSoundReductionIndex(test.paths)
			switch {
			case test.wantNaN:
				if !math.IsNaN(got) {
					t.Fatalf("FlankingApparentSoundReductionIndex() = %v, want NaN", got)
				}
			case test.wantPosInf:
				if !math.IsInf(got, 1) {
					t.Fatalf("FlankingApparentSoundReductionIndex() = %v, want +Inf", got)
				}
			case math.Abs(got-test.want) > 1e-12:
				t.Fatalf("FlankingApparentSoundReductionIndex() = %v, want %v", got, test.want)
			}
		})
	}
}
