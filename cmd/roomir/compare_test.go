package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/export"
	"github.com/cwbudde/algo-acoustics/ir"
)

func TestCompareCommandWritesTableReport(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	leftPath := filepath.Join(tmpDir, "left.wav")
	rightPath := filepath.Join(tmpDir, "right.wav")

	left := &ir.Buffer{SampleRate: 48000, Samples: make([]float64, 2048)}
	right := &ir.Buffer{SampleRate: 48000, Samples: make([]float64, 2048)}
	left.Samples[0] = 1
	right.Samples[0] = 0.5

	err := export.WriteMonoWAV(leftPath, left)
	if err != nil {
		t.Fatalf("WriteMonoWAV(left) error = %v", err)
	}

	err = export.WriteMonoWAV(rightPath, right)
	if err != nil {
		t.Fatalf("WriteMonoWAV(right) error = %v", err)
	}

	cmd := newRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"compare", leftPath, rightPath})

	if exitCode := run(cmd); exitCode != 0 {
		t.Fatalf("run() = %d, want 0; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Metric") || !strings.Contains(output, "peak amplitude") || !strings.Contains(output, "correlation") {
		t.Fatalf("stdout = %q, want comparison table", output)
	}

	if !strings.Contains(output, "125 Hz band") {
		t.Fatalf("stdout = %q, want band rows", output)
	}
}
