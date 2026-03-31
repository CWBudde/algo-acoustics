package acoustics_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
)

func TestSpeedOfSoundAt20C(t *testing.T) {
	// At 20 °C the Cramer approximation gives ≈343.2 m/s; allow ±0.5 m/s.
	got := acoustics.SpeedOfSoundAt(20)
	if math.Abs(got-343.0) > 0.5 {
		t.Errorf("SpeedOfSoundAt(20) = %.3f, want ~343.0 m/s", got)
	}
}

func TestSpeedOfSoundAt0C(t *testing.T) {
	// At 0 °C the standard value is 331.3 m/s.
	got := acoustics.SpeedOfSoundAt(0)
	if math.Abs(got-331.3) > 0.1 {
		t.Errorf("SpeedOfSoundAt(0) = %.3f, want ~331.3 m/s", got)
	}
}

func TestSpeedOfSoundIncreaseWithTemp(t *testing.T) {
	if acoustics.SpeedOfSoundAt(40) <= acoustics.SpeedOfSoundAt(20) {
		t.Error("speed of sound should increase with temperature")
	}
}

func TestCharacteristicImpedanceAt20C(t *testing.T) {
	// Standard value ≈ 413 Pa·s/m; allow ±5.
	got := acoustics.CharacteristicImpedance(20)
	if math.Abs(got-413.0) > 5 {
		t.Errorf("CharacteristicImpedance(20) = %.2f, want ~413 rayl", got)
	}
}

func TestCharacteristicImpedanceDecreaseWithTemp(t *testing.T) {
	// Impedance drops as temperature rises (density falls faster than speed rises).
	if acoustics.CharacteristicImpedance(40) >= acoustics.CharacteristicImpedance(20) {
		t.Error("characteristic impedance should decrease with temperature")
	}
}
