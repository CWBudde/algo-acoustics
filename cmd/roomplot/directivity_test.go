package main

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

type constantDirectivityModel struct {
	gain float64
}

func (m constantDirectivityModel) GainLinear(_ float64, _ geometry.Vec3) float64 {
	return m.gain
}

func TestBuildSourceDirectivityRows(t *testing.T) {
	model := constantDirectivityModel{gain: 2}
	rows := buildSourceDirectivityRows(model, 1000, 0, 90)

	if len(rows) != 4 {
		t.Fatalf("len(rows) = %d, want 4", len(rows))
	}

	if rows[0].AzimuthDeg != 0 || rows[1].AzimuthDeg != 90 || rows[2].AzimuthDeg != 180 || rows[3].AzimuthDeg != 270 {
		t.Fatalf("unexpected azimuth sequence: %#v", rows)
	}

	if got, want := rows[0].GainLinear, 2.0; got != want {
		t.Fatalf("GainLinear() = %v, want %v", got, want)
	}

	if got, want := rows[0].GainDB, 20*math.Log10(2.0); got != want {
		t.Fatalf("GainDB() = %v, want %v", got, want)
	}
}

func TestWriteSourceDirectivityCSV(t *testing.T) {
	rows := []sourceDirectivityRow{{AzimuthDeg: 0, GainLinear: 1, GainDB: 0}}

	var buf bytes.Buffer

	err := writeSourceDirectivityCSV(&buf, rows)
	if err != nil {
		t.Fatalf("writeSourceDirectivityCSV() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "azimuth_deg,gain_linear,gain_db") {
		t.Fatalf("CSV output missing header: %q", output)
	}

	if !strings.Contains(output, "0.0,1,0") {
		t.Fatalf("CSV output missing row: %q", output)
	}
}
