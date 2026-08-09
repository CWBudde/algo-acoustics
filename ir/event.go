package ir

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// EventKind classifies the origin of an IR event.
type EventKind int

const (
	EventDirect EventKind = iota
	EventSpecular
	EventDiffuse
	EventDiffraction
	EventPDE
	// EventTransmission is emitted by a secondary source created at a
	// transmitting surface between rooms.
	EventTransmission
)

// Event describes a sparse contribution that can later be rendered into a dense IR.
//
//nolint:tagliatelle // Camel-case tags are part of the established exported event schema.
type Event struct {
	TimeSeconds    float64       `json:"timeSeconds"`
	Amplitude      float64       `json:"amplitude"`
	Direction      geometry.Vec3 `json:"direction"`
	DistanceMeters float64       `json:"distanceMeters"`
	BandGain       []float64     `json:"bandGain,omitempty"`
	PhaseRadians   float64       `json:"phaseRadians"`
	Kind           EventKind     `json:"kind"`
}

// EnergyEmission describes a delayed point-source emission in the energy
// domain. BandEnergy contains absolute linear energy per frequency band.
type EnergyEmission struct {
	Position    geometry.Vec3
	TimeSeconds float64
	BandEnergy  []float64
}

// PressureEmission describes a delayed point-source emission in the pressure
// domain. BandPressure contains absolute linear pressure per frequency band.
type PressureEmission struct {
	Position     geometry.Vec3
	TimeSeconds  float64
	BandPressure []float64
	PhaseRadians float64
}

// ToPressure converts energy to pressure using p = sqrt(E). Negative and
// non-finite energy is represented as NaN so callers can reject invalid input.
func (e EnergyEmission) ToPressure() PressureEmission {
	pressure := make([]float64, len(e.BandEnergy))
	for index, energy := range e.BandEnergy {
		pressure[index] = math.Sqrt(energy)
	}

	return PressureEmission{
		Position:     e.Position,
		TimeSeconds:  e.TimeSeconds,
		BandPressure: pressure,
	}
}
