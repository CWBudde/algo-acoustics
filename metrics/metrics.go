package metrics

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/ir"
)

const (
	decayRangeUpperDB     = 0
	decayRangeLowerDB     = -60
	decayTimeTargetDB     = -60
	decayToleranceEpsilon = 1e-12
)

// T60FromDecaySlope estimates the reverberation time from a linear regression over the decay curve.
func T60FromDecaySlope(buf *ir.Buffer) (float64, error) {
	return estimateDecayTime(buf, decayRangeUpperDB, decayRangeLowerDB)
}

// EDT estimates the early decay time from 0 to -10 dB.
func EDT(buf *ir.Buffer) (float64, error) {
	return estimateDecayTime(buf, 0, -10)
}

// T20 estimates the reverberation time from -5 to -25 dB.
func T20(buf *ir.Buffer) (float64, error) {
	return estimateDecayTime(buf, -5, -25)
}

// T30 estimates the reverberation time from -5 to -35 dB.
func T30(buf *ir.Buffer) (float64, error) {
	return estimateDecayTime(buf, -5, -35)
}

// C50 returns the clarity index over the first 50 ms.
func C50(buf *ir.Buffer) (float64, error) {
	return clarityIndex(buf, 0.05)
}

// C80 returns the clarity index over the first 80 ms.
func C80(buf *ir.Buffer) (float64, error) {
	return clarityIndex(buf, 0.08)
}

// D50 returns the definition metric over the first 50 ms.
func D50(buf *ir.Buffer) (float64, error) {
	early, late, err := splitEnergy(buf, 0.05)
	if err != nil {
		return 0, err
	}

	total := early + late
	if total == 0 {
		return 0, errors.New("buffer contains no energy")
	}

	return early / total, nil
}

type decayPoint struct {
	timeSeconds float64
	decayDB     float64
}

func estimateDecayTime(buf *ir.Buffer, upperDB, lowerDB float64) (float64, error) {
	points, err := decayPoints(buf)
	if err != nil {
		return 0, err
	}

	filtered := make([]decayPoint, 0, len(points))
	for _, point := range points {
		if point.decayDB <= upperDB+decayToleranceEpsilon && point.decayDB >= lowerDB-decayToleranceEpsilon {
			filtered = append(filtered, point)
		}
	}

	if len(filtered) < 2 {
		return 0, fmt.Errorf("not enough decay samples in range %g to %g dB", upperDB, lowerDB)
	}

	slope, intercept := linearRegression(filtered)
	if slope >= 0 {
		return 0, errors.New("decay slope must be negative")
	}

	return (decayTimeTargetDB - intercept) / slope, nil
}

func decayPoints(buf *ir.Buffer) ([]decayPoint, error) {
	if buf == nil {
		return nil, errors.New("buffer must not be nil")
	}

	if buf.SampleRate <= 0 {
		return nil, errors.New("buffer sample rate must be positive")
	}

	if len(buf.Samples) == 0 {
		return nil, errors.New("buffer must not be empty")
	}

	totalEnergy := 0.0
	for _, sample := range buf.Samples {
		totalEnergy += sample * sample
	}

	if totalEnergy == 0 {
		return nil, errors.New("buffer contains no energy")
	}

	points := make([]decayPoint, len(buf.Samples))

	remaining := 0.0
	for index := len(buf.Samples) - 1; index >= 0; index-- {
		remaining += buf.Samples[index] * buf.Samples[index]
		points[index] = decayPoint{
			timeSeconds: float64(index) / float64(buf.SampleRate),
			decayDB:     10 * math.Log10(remaining/totalEnergy),
		}
	}

	return points, nil
}

func linearRegression(points []decayPoint) (slope, intercept float64) {
	count := float64(len(points))

	var sumX, sumY, sumXX, sumXY float64
	for _, point := range points {
		sumX += point.timeSeconds
		sumY += point.decayDB
		sumXX += point.timeSeconds * point.timeSeconds
		sumXY += point.timeSeconds * point.decayDB
	}

	denominator := count*sumXX - sumX*sumX
	if math.Abs(denominator) <= decayToleranceEpsilon {
		return 0, sumY / count
	}

	slope = (count*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / count

	return slope, intercept
}

func clarityIndex(buf *ir.Buffer, earlySeconds float64) (float64, error) {
	early, late, err := splitEnergy(buf, earlySeconds)
	if err != nil {
		return 0, err
	}

	if early == 0 && late == 0 {
		return 0, errors.New("buffer contains no energy")
	}

	if late == 0 {
		return math.Inf(1), nil
	}

	if early == 0 {
		return math.Inf(-1), nil
	}

	return 10 * math.Log10(early/late), nil
}

func splitEnergy(buf *ir.Buffer, earlySeconds float64) (early, late float64, err error) {
	if buf == nil {
		return 0, 0, errors.New("buffer must not be nil")
	}

	if buf.SampleRate <= 0 {
		return 0, 0, errors.New("buffer sample rate must be positive")
	}

	if len(buf.Samples) == 0 {
		return 0, 0, errors.New("buffer must not be empty")
	}

	if earlySeconds < 0 {
		return 0, 0, errors.New("early window must be non-negative")
	}

	threshold := int(math.Round(earlySeconds * float64(buf.SampleRate)))
	for index, sample := range buf.Samples {
		energy := sample * sample
		if index <= threshold {
			early += energy
			continue
		}

		late += energy
	}

	return early, late, nil
}
