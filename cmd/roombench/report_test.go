package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportCommandWritesMarkdownReport(t *testing.T) {
	t.Parallel()

	fixtureDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "rooms"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "bench_report.md")

	command := newReportCommand()
	command.SetArgs([]string{"--format", "markdown", "--output", outputPath, "--fixtures", fixtureDir})

	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stdout)

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	err = command.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "| Room | T60 | EDT | C80 | Expected range | Pass |") {
		t.Fatalf("report header missing from %q", text)
	}

	for _, room := range []string{"tiny room", "control room", "lecture room", "pa room"} {
		if !strings.Contains(text, room) {
			t.Fatalf("report output missing room %q", room)
		}
	}

	if !strings.Contains(stdout.String(), "wrote "+outputPath) {
		t.Fatalf("stdout = %q, want write confirmation", stdout.String())
	}
}
