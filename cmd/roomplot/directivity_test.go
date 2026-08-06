package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
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

func TestSourceDirectivityInvalidFormatDoesNotTruncateOutput(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "directivity.txt")
	const sentinel = "keep this content"

	err := os.WriteFile(outputPath, []byte(sentinel), 0o600)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := newRootCommand()
	stderr := &bytes.Buffer{}
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"source-directivity", "unused.gll",
		"--format", "invalid",
		"--output", outputPath,
	})

	if exitCode := run(cmd); exitCode == 0 {
		t.Fatalf("run() = 0, want error; stderr=%q", stderr.String())
	}

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(got) != sentinel {
		t.Fatalf("output content = %q, want sentinel %q", got, sentinel)
	}
}
