package directivity

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/cwbudde/algo-acoustics/geometry"
	ggll "github.com/cwbudde/gll-tools/pkg/gll"
)

type gllBalloon interface {
	GetResponseAtAngle(theta, phi float64) *ggll.TransferFunction
}

// GLLModel adapts gll-tools balloon data to the acoustics directivity interface.
type GLLModel struct {
	File             *ggll.File
	Preset           string
	SourceKey        string
	SourceDefinition *ggll.SourceDefinition
	balloon          gllBalloon
}

// LoadGLL loads a GLL file and adapts the selected source definition.
//
// The preset selector currently matches source definition keys or labels.
func LoadGLL(path, preset string) (*GLLModel, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("gll path is empty")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open gll file %q: %w", path, err)
	}
	defer f.Close()

	model, err := LoadGLLReader(f, preset)
	if err != nil {
		return nil, fmt.Errorf("load gll file %q: %w", path, err)
	}

	return model, nil
}

// LoadGLLReader loads a GLL file from an already-open reader and adapts the selected source definition.
func LoadGLLReader(r io.ReadSeeker, preset string) (*GLLModel, error) {
	if r == nil {
		return nil, errors.New("gll reader is nil")
	}

	file, err := ggll.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse gll reader: %w", err)
	}

	model, err := LoadGLLFile(file, preset)
	if err != nil {
		return nil, err
	}

	// Parse records only the offset of the balloon responses, so the balloon
	// has to be hydrated while the reader is still open. Without this the
	// model has no measurements and radiates omnidirectionally.
	err = model.loadBalloonResponses(r)
	if err != nil {
		return nil, err
	}

	return model, nil
}

// loadBalloonResponses reads the deferred balloon measurements from r.
func (m *GLLModel) loadBalloonResponses(r io.ReadSeeker) error {
	balloon := m.SourceDefinition.BalloonData
	if balloon == nil || len(balloon.Responses) > 0 {
		return nil
	}

	err := ggll.LoadBalloonResponses(r, balloon)
	if err != nil {
		return fmt.Errorf("load balloon responses for source %q: %w", m.SourceKey, err)
	}

	return nil
}

// LoadGLLFile adapts a parsed GLL file to the directivity interface.
//
// The balloon measurements of a file parsed this way are loaded lazily by
// gll-tools and cannot be recovered without the original reader, so prefer
// LoadGLL or LoadGLLReader unless the responses are already populated.
func LoadGLLFile(file *ggll.File, preset string) (*GLLModel, error) {
	if file == nil {
		return nil, errors.New("gll file is nil")
	}

	source, sourceKey, err := selectSourceDefinition(file, preset)
	if err != nil {
		return nil, err
	}

	return &GLLModel{
		File:             file,
		Preset:           preset,
		SourceKey:        sourceKey,
		SourceDefinition: source,
		balloon:          source.BalloonData,
	}, nil
}

// GainLinear returns the interpolated directivity gain for the supplied direction.
func (m *GLLModel) GainLinear(freqHz float64, dir geometry.Vec3) float64 {
	if m == nil || m.balloon == nil {
		return 1
	}

	unitDir := dir.Normalize()
	if unitDir == geometry.Vec3Zero {
		return 0
	}

	theta, phi := directionToAngles(unitDir)

	response := m.balloon.GetResponseAtAngle(theta, phi)
	if response == nil {
		return 1
	}

	onAxis := m.balloon.GetResponseAtAngle(0, 0)

	bandIndex := nearestFrequencyBandIndex(response, freqHz)
	if bandIndex < 0 || bandIndex >= len(response.Level) {
		return 1
	}

	gainDB := response.Level[bandIndex]
	if onAxis != nil && bandIndex < len(onAxis.Level) {
		gainDB -= onAxis.Level[bandIndex]
	}

	return math.Pow(10, gainDB/20)
}

func selectSourceDefinition(file *ggll.File, preset string) (*ggll.SourceDefinition, string, error) {
	if file == nil || file.Database == nil {
		return nil, "", errors.New("gll file does not contain a database")
	}

	sourceDefinitions := file.Database.SourceDefinitions
	if len(sourceDefinitions) == 0 {
		return nil, "", errors.New("gll file does not contain source definitions")
	}

	if preset != "" {
		for _, item := range sourceDefinitions {
			if item.Definition == nil {
				continue
			}

			if strings.EqualFold(item.Key, preset) || strings.EqualFold(item.Definition.Label, preset) {
				if item.Definition.BalloonData == nil {
					return nil, "", fmt.Errorf("source definition %q has no balloon data", item.Key)
				}

				return item.Definition, item.Key, nil
			}
		}

		return nil, "", fmt.Errorf("gll preset %q does not match a source definition key or label", preset)
	}

	for _, item := range sourceDefinitions {
		if item.Definition == nil || item.Definition.BalloonData == nil {
			continue
		}

		return item.Definition, item.Key, nil
	}

	return nil, "", errors.New("no source definition with balloon data found")
}

func directionToAngles(dir geometry.Vec3) (theta, phi float64) {
	theta = math.Asin(dir.Z)
	phi = math.Atan2(dir.Y, dir.X)

	return theta, phi
}

func nearestFrequencyBandIndex(response *ggll.TransferFunction, freqHz float64) int {
	if response == nil || len(response.Level) == 0 {
		return -1
	}

	if freqHz <= 0 {
		return 0
	}

	bandCount := len(response.Level)
	if response.Definition.PointCount > 0 && int(response.Definition.PointCount) < bandCount {
		bandCount = int(response.Definition.PointCount)
	}

	bestIndex := 0
	bestDistance := math.Inf(1)

	for index := range bandCount {
		bandFreq := response.Definition.GetFrequency(index)
		if bandFreq <= 0 {
			continue
		}

		distance := math.Abs(math.Log(freqHz / bandFreq))
		if distance < bestDistance {
			bestDistance = distance
			bestIndex = index
		}
	}

	return bestIndex
}
