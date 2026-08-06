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

	rows := passingCorpusRows()
	command := newReportCommandWithBuilder(func(string, int) ([]corpusRow, error) {
		return rows, nil
	})

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "bench_report.md")
	command.SetArgs([]string{"--format", "markdown", "--output", outputPath})

	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stdout)

	err := command.Execute()
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

	if strings.Contains(text, "FAIL") {
		t.Fatalf("report contains a failing row: %q", text)
	}

	if !strings.Contains(stdout.String(), "wrote "+outputPath) {
		t.Fatalf("stdout = %q, want write confirmation", stdout.String())
	}
}

func TestReportCommandFailsClosedForIncompleteRows(t *testing.T) {
	t.Parallel()

	command := newReportCommandWithBuilder(func(string, int) ([]corpusRow, error) {
		return passingCorpusRows()[:len(defaultCorpusCases)-1], nil
	})
	command.SetArgs([]string{"--format", "table"})
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	err := command.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want incomplete corpus rows to fail")
	}
}

func TestCorpusRowsPassFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func([]corpusRow) []corpusRow
	}{
		{name: "no rows", edit: func([]corpusRow) []corpusRow { return nil }},
		{name: "missing row", edit: func(rows []corpusRow) []corpusRow { return rows[:len(rows)-1] }},
		{name: "missing range", edit: func(rows []corpusRow) []corpusRow {
			rows[0].rangeStr = ""
			return rows
		}},
		{name: "failed range", edit: func(rows []corpusRow) []corpusRow {
			rows[0].pass = false
			return rows
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if corpusRowsPass(test.edit(passingCorpusRows())) {
				t.Fatal("corpusRowsPass() = true, want false")
			}
		})
	}
}

func passingCorpusRows() []corpusRow {
	rows := make([]corpusRow, 0, len(defaultCorpusCases))
	for _, corpusCase := range defaultCorpusCases {
		rows = append(rows, corpusRow{
			name:     corpusCase.name,
			rangeStr: formatRange(corpusCase.t60Range, corpusCase.edtRange, corpusCase.c80Range),
			pass:     true,
		})
	}

	return rows
}
