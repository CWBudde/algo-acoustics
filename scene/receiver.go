package scene

import (
	"encoding/json"
	"fmt"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
)

// ReceiverType describes the supported receiver models.
type ReceiverType string

const (
	// ReceiverOmni is a single-point receiver.
	ReceiverOmni ReceiverType = "omni"
	// ReceiverBinaural is a left/right receiver pair using an HRTF dataset.
	ReceiverBinaural ReceiverType = "binaural"
)

// Receiver is a listening position in the scene.
type Receiver struct {
	Position    geometry.Vec3       `json:"position"`
	Orientation geometry.Quaternion `json:"orientation"`
	Type        ReceiverType        `json:"type"`
	HRTF        hrtf.Dataset        `json:"-"`
}

type receiverJSON struct {
	Position    geometry.Vec3       `json:"position"`
	Orientation geometry.Quaternion `json:"orientation"`
	Type        ReceiverType        `json:"type"`
	HRTF        *hrtfJSON           `json:"hrtf,omitempty"`
}

type hrtfJSON struct {
	Type         string `json:"type"`
	SampleRateHz int    `json:"sampleRate,omitempty"`
}

func hrtfJSONFromDataset(dataset hrtf.Dataset) (*hrtfJSON, error) {
	switch typed := dataset.(type) {
	case nil:
		return nil, nil
	case hrtf.NoopDataset:
		return &hrtfJSON{Type: "noop", SampleRateHz: typed.SampleRateHz}, nil
	case *hrtf.NoopDataset:
		return &hrtfJSON{Type: "noop", SampleRateHz: typed.SampleRateHz}, nil
	case hrtf.NearestNeighborDataset:
		return &hrtfJSON{Type: "nearestNeighbor", SampleRateHz: typed.SampleRateHz}, nil
	case *hrtf.NearestNeighborDataset:
		return &hrtfJSON{Type: "nearestNeighbor", SampleRateHz: typed.SampleRateHz}, nil
	default:
		return nil, fmt.Errorf("unsupported HRTF dataset %T", dataset)
	}
}

func (h hrtfJSON) toDataset() (hrtf.Dataset, error) {
	switch h.Type {
	case "noop":
		return hrtf.NoopDataset{SampleRateHz: h.SampleRateHz}, nil
	case "nearestNeighbor":
		return hrtf.NearestNeighborDataset{SampleRateHz: h.SampleRateHz}, nil
	default:
		return nil, fmt.Errorf("unsupported HRTF type %q", h.Type)
	}
}

// WorldToHeadDir returns a world-space direction in the receiver's head frame.
func (r Receiver) WorldToHeadDir(worldDir geometry.Vec3) geometry.Vec3 {
	return effectiveOrientation(r.Orientation).Conj().Rotate(worldDir).Normalize()
}

// MarshalJSON preserves the interface-backed HRTF field.
func (r Receiver) MarshalJSON() ([]byte, error) {
	encoded := receiverJSON{
		Position:    r.Position,
		Orientation: r.Orientation,
		Type:        r.Type,
	}

	if r.HRTF != nil {
		hrtfValue, err := hrtfJSONFromDataset(r.HRTF)
		if err != nil {
			return nil, err
		}

		encoded.HRTF = hrtfValue
	}

	return json.Marshal(encoded)
}

// UnmarshalJSON restores the interface-backed HRTF field.
func (r *Receiver) UnmarshalJSON(data []byte) error {
	var decoded receiverJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	r.Position = decoded.Position
	r.Orientation = decoded.Orientation
	r.Type = decoded.Type
	r.HRTF = nil

	if decoded.HRTF != nil {
		dataset, err := decoded.HRTF.toDataset()
		if err != nil {
			return err
		}

		r.HRTF = dataset
	}

	return nil
}
