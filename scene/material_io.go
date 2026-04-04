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

	idxBand, okBand := header["band"]
	idxAbsorption, okAbsorption := header["absorption"]

	idxScattering, okScattering := header["scattering"]
	if !okBand || !okAbsorption || !okScattering {
		return Material{}, errors.New("material CSV requires band, absorption, and scattering columns")
	}

	material := Material{
		AbsorptionByBand: make([]float64, 0, len(records)-1),
		ScatteringByBand: make([]float64, 0, len(records)-1),
	}

	for rowIndex, record := range records[1:] {
		if idxBand >= len(record) || idxAbsorption >= len(record) || idxScattering >= len(record) {
			return Material{}, fmt.Errorf("material CSV row %d is too short", rowIndex+1)
		}

		band, err := strconv.Atoi(strings.TrimSpace(record[idxBand]))
		if err != nil {
			return Material{}, fmt.Errorf("parse band index on row %d: %w", rowIndex+1, err)
		}

		if band != rowIndex {
			return Material{}, fmt.Errorf("material CSV row %d has band %d, want %d", rowIndex+1, band, rowIndex)
		}

		absorption, err := strconv.ParseFloat(strings.TrimSpace(record[idxAbsorption]), 64)
		if err != nil {
			return Material{}, fmt.Errorf("parse absorption on row %d: %w", rowIndex+1, err)
		}

		scattering, err := strconv.ParseFloat(strings.TrimSpace(record[idxScattering]), 64)
		if err != nil {
			return Material{}, fmt.Errorf("parse scattering on row %d: %w", rowIndex+1, err)
		}

		material.AbsorptionByBand = append(material.AbsorptionByBand, absorption)
		material.ScatteringByBand = append(material.ScatteringByBand, scattering)
	}

	copy(material.Scattering[:], material.ScatteringByBand)

	return material, nil
}
