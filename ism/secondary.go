package ism

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

// SolveSecondary mirrors delayed pressure-domain secondary point sources in
// the receiving room. The caller is responsible for applying the portal's
// pressure transmission magnitude sqrt(tau) to each band.
func (solver ISMSolver) SolveSecondary(sc *scene.Scene, cfg ISMConfig, emissions []ir.PressureEmission) ([]ir.Event, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	bandCount := cfg.BandSpec.BandCount()
	if bandCount == 0 {
		bandCount = sc.BandSpec.BandCount()
	}

	if bandCount <= 0 {
		return nil, errors.New("secondary source requires at least one frequency band")
	}

	events := make([]ir.Event, 0)

	for index, emission := range emissions {
		err := validateSecondaryEmission(emission, bandCount)
		if err != nil {
			return nil, fmt.Errorf("secondary emission %d: %w", index, err)
		}

		if bandGainSilent(emission.BandPressure) {
			continue
		}

		sceneCopy := *sc
		sceneCopy.Sources = []scene.Source{{Position: emission.Position}}

		emissionEvents, err := solver.Solve(&sceneCopy, cfg)
		if err != nil {
			return nil, fmt.Errorf("solve secondary emission %d: %w", index, err)
		}

		for _, event := range emissionEvents {
			if len(event.BandGain) != bandCount {
				return nil, fmt.Errorf("secondary event band gain count = %d, want %d", len(event.BandGain), bandCount)
			}

			for bandIndex := range event.BandGain {
				event.BandGain[bandIndex] *= emission.BandPressure[bandIndex]
			}

			if bandGainSilent(event.BandGain) {
				continue
			}

			event.TimeSeconds += emission.TimeSeconds
			event.PhaseRadians += emission.PhaseRadians
			event.Kind = ir.EventTransmission
			events = append(events, event)
		}
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].TimeSeconds != events[j].TimeSeconds {
			return events[i].TimeSeconds < events[j].TimeSeconds
		}

		return events[i].DistanceMeters < events[j].DistanceMeters
	})

	return events, nil
}

func validateSecondaryEmission(emission ir.PressureEmission, bandCount int) error {
	if math.IsNaN(emission.TimeSeconds) || math.IsInf(emission.TimeSeconds, 0) || emission.TimeSeconds < 0 {
		return errors.New("time must be finite and non-negative")
	}

	if math.IsNaN(emission.Position.X) || math.IsInf(emission.Position.X, 0) ||
		math.IsNaN(emission.Position.Y) || math.IsInf(emission.Position.Y, 0) ||
		math.IsNaN(emission.Position.Z) || math.IsInf(emission.Position.Z, 0) {
		return errors.New("position must be finite")
	}

	if math.IsNaN(emission.PhaseRadians) || math.IsInf(emission.PhaseRadians, 0) {
		return errors.New("phase must be finite")
	}

	if len(emission.BandPressure) != bandCount {
		return fmt.Errorf("band pressure count = %d, want %d", len(emission.BandPressure), bandCount)
	}

	for index, pressure := range emission.BandPressure {
		if math.IsNaN(pressure) || math.IsInf(pressure, 0) {
			return fmt.Errorf("band pressure[%d] = %v, want finite", index, pressure)
		}
	}

	return nil
}
