package scene

import "github.com/cwbudde/algo-acoustics/acoustics"

// Scene ties together the room, material palette, emitters, and receivers.
type Scene struct {
	Room       Room                `json:"room"`
	Materials  map[string]Material `json:"materials,omitempty"`
	Sources    []Source            `json:"sources,omitempty"`
	Receivers  []Receiver          `json:"receivers,omitempty"`
	BandSpec   acoustics.BandSpec  `json:"bandSpec"`
	SampleRate int                 `json:"sampleRate"`
}
