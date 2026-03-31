package ir

import "github.com/cwbudde/algo-acoustics/geometry"

// EventKind classifies the origin of an IR event.
type EventKind int

const (
	EventDirect EventKind = iota
	EventSpecular
	EventDiffuse
	EventPDE
)

// Event describes a sparse contribution that can later be rendered into a dense IR.
type Event struct {
	TimeSeconds    float64       `json:"timeSeconds"`
	Amplitude      float64       `json:"amplitude"`
	Direction      geometry.Vec3 `json:"direction"`
	DistanceMeters float64       `json:"distanceMeters"`
	BandGain       []float64     `json:"bandGain,omitempty"`
	PhaseRadians   float64       `json:"phaseRadians"`
	Kind           EventKind     `json:"kind"`
}
