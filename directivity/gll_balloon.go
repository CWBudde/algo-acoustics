package directivity

import (
	"errors"
	"fmt"

	ggll "github.com/cwbudde/gll-tools/pkg/gll"
)

// BandCenterFrequencies returns the native frequency grid of the loaded
// balloon, ascending, in Hz. GLL files are typically measured at 1/24 octave,
// which is far finer than the octave bands the renderer works in — use it to
// discover the measured range, not as an extraction target.
func (m *GLLModel) BandCenterFrequencies() []float64 {
	responses := m.loadedResponses()
	if len(responses) == 0 {
		return nil
	}

	definition := responses[0].Definition

	count := int(definition.PointCount)
	if count <= 0 || count > len(responses[0].Level) {
		count = len(responses[0].Level)
	}

	frequencies := make([]float64, 0, count)

	for index := range count {
		freq := definition.GetFrequency(index)
		if freq <= 0 {
			continue
		}

		frequencies = append(frequencies, freq)
	}

	return frequencies
}

// ExtractBalloon reduces the loaded GLL balloon to a BalloonDirectivity
// tabulated at the supplied band centres and grid resolution, normalised
// on-axis exactly as GainLinear is. Passing no bands extracts the native
// frequency grid.
//
// The result no longer references the GLL file, so it can be held, compared,
// or serialised without keeping megabytes of measurement data alive — pass the
// renderer's band centres (acoustics.Octave8.CenterFreqs) to get a table sized
// for the bands the pipeline actually evaluates.
func (m *GLLModel) ExtractBalloon(
	bands []float64,
	grid SphericalGrid,
	mode InterpolationMode,
) (*BalloonDirectivity, error) {
	if m == nil || m.balloon == nil {
		return nil, errors.New("gll model has no balloon data")
	}

	if m.SourceDefinition != nil && m.SourceDefinition.BalloonData != nil && len(m.loadedResponses()) == 0 {
		return nil, fmt.Errorf(
			"balloon for source %q has no loaded responses; load the file with LoadGLL or LoadGLLReader",
			m.SourceKey,
		)
	}

	if len(bands) == 0 {
		bands = m.BandCenterFrequencies()
		if len(bands) == 0 {
			return nil, fmt.Errorf("balloon for source %q exposes no frequency grid", m.SourceKey)
		}
	}

	balloon, err := SampleBalloon(m, bands, grid, mode)
	if err != nil {
		return nil, fmt.Errorf("extract balloon for source %q: %w", m.SourceKey, err)
	}

	return balloon, nil
}

// loadedResponses returns the hydrated measurements, or nil when the balloon
// was never backed by a real GLL file.
func (m *GLLModel) loadedResponses() []ggll.TransferFunction {
	if m == nil || m.SourceDefinition == nil || m.SourceDefinition.BalloonData == nil {
		return nil
	}

	return m.SourceDefinition.BalloonData.Responses
}
