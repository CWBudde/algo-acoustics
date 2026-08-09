package scene

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LoadMaterialFile loads a material definition from JSON or CSV.
func LoadMaterialFile(path string) (Material, error) {
	file, err := os.Open(path)
	if err != nil {
		return Material{}, fmt.Errorf("open material file %q: %w", path, err)
	}
	defer file.Close()

	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return LoadMaterialJSON(file)
	case ".csv":
		material, err := LoadMaterialCSV(file)
		if err != nil {
			return Material{}, err
		}

		if material.Name == "" {
			material.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}

		return material, nil
	default:
		return Material{}, fmt.Errorf("unsupported material file extension %q", filepath.Ext(path))
	}
}

// LoadMaterialJSON loads a material definition from JSON.
func LoadMaterialJSON(r io.Reader) (Material, error) {
	var material Material

	err := json.NewDecoder(r).Decode(&material)
	if err != nil {
		return Material{}, fmt.Errorf("decode material JSON: %w", err)
	}

	return material, nil
}

// LoadMaterialCSV loads a single material table from CSV.
func LoadMaterialCSV(r io.Reader) (Material, error) {
	reader := csv.NewReader(r)

	records, err := reader.ReadAll()
	if err != nil {
		return Material{}, fmt.Errorf("read material CSV: %w", err)
	}

	if len(records) == 0 {
		return Material{}, errors.New("material CSV is empty")
	}

	header := make(map[string]int, len(records[0]))
	for i, column := range records[0] {
		header[strings.ToLower(strings.TrimSpace(column))] = i
	}

	columns := materialCSVColumns{}
	var okBand, okAbsorption, okScattering bool
	columns.band, okBand = header["band"]
	columns.absorption, okAbsorption = header["absorption"]

	columns.scattering, okScattering = header["scattering"]
	if !okBand || !okAbsorption || !okScattering {
		return Material{}, errors.New("material CSV requires band, absorption, and scattering columns")
	}

	columns.transmission, columns.hasTransmission = header["transmission"]
	columns.reduction, columns.hasReduction = header["sound_reduction_index"]

	material := Material{
		AbsorptionByBand: make([]float64, 0, len(records)-1),
		ScatteringByBand: make([]float64, 0, len(records)-1),
	}

	if columns.hasTransmission {
		material.TransmissionByBand = make([]float64, 0, len(records)-1)
	}

	if columns.hasReduction {
		material.SoundReductionIndex = make([]float64, 0, len(records)-1)
	}

	for rowIndex, record := range records[1:] {
		err := appendMaterialCSVRow(&material, record, rowIndex, columns)
		if err != nil {
			return Material{}, err
		}
	}

	copy(material.Scattering[:], material.ScatteringByBand)

	return material, nil
}

type materialCSVColumns struct {
	band, absorption, scattering  int
	transmission, reduction       int
	hasTransmission, hasReduction bool
}

func appendMaterialCSVRow(material *Material, record []string, rowIndex int, columns materialCSVColumns) error {
	required := max(columns.band, columns.absorption, columns.scattering)
	if required >= len(record) {
		return fmt.Errorf("material CSV row %d is too short", rowIndex+1)
	}

	band, err := strconv.Atoi(strings.TrimSpace(record[columns.band]))
	if err != nil {
		return fmt.Errorf("parse band index on row %d: %w", rowIndex+1, err)
	}

	if band != rowIndex {
		return fmt.Errorf("material CSV row %d has band %d, want %d", rowIndex+1, band, rowIndex)
	}

	absorption, err := parseMaterialCSVFloat(record, columns.absorption, "absorption", rowIndex)
	if err != nil {
		return err
	}

	scattering, err := parseMaterialCSVFloat(record, columns.scattering, "scattering", rowIndex)
	if err != nil {
		return err
	}

	material.AbsorptionByBand = append(material.AbsorptionByBand, absorption)
	material.ScatteringByBand = append(material.ScatteringByBand, scattering)

	if columns.hasTransmission {
		transmission, parseErr := parseMaterialCSVFloat(record, columns.transmission, "transmission", rowIndex)
		if parseErr != nil {
			return parseErr
		}

		material.TransmissionByBand = append(material.TransmissionByBand, transmission)
	}

	if columns.hasReduction {
		reduction, parseErr := parseMaterialCSVFloat(record, columns.reduction, "sound reduction index", rowIndex)
		if parseErr != nil {
			return parseErr
		}

		material.SoundReductionIndex = append(material.SoundReductionIndex, reduction)
	}

	return nil
}

func parseMaterialCSVFloat(record []string, column int, property string, rowIndex int) (float64, error) {
	if column >= len(record) {
		return 0, fmt.Errorf("material CSV row %d is too short", rowIndex+1)
	}

	value, err := strconv.ParseFloat(strings.TrimSpace(record[column]), 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s on row %d: %w", property, rowIndex+1, err)
	}

	return value, nil
}
