package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

func TestDumpEventsCommandWritesJSONAndCSV(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"json", "csv"} {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			outputPath := filepath.Join(tmpDir, "events."+format)

			cmd := newRootCommand()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs([]string{
				"dump-events",
				filepath.Join("..", "..", "testdata", "rooms", "shoebox_simple.json"),
				"-o", outputPath,
				"--format", format,
				"--max-order", "1",
			})

			if exitCode := run(cmd); exitCode != 0 {
				t.Fatalf("run() = %d, want 0; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
			}

			if got := stderr.String(); !strings.Contains(got, "dumped") || !strings.Contains(got, format) {
				t.Fatalf("stderr = %q, want dump summary with format", got)
			}

			data, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}

			switch format {
			case "json":
				var events []ir.Event
				if err := json.Unmarshal(data, &events); err != nil {
					t.Fatalf("json.Unmarshal() error = %v", err)
				}
				if len(events) == 0 {
					t.Fatal("json output contains no events")
				}
			case "csv":
				reader := csv.NewReader(bytes.NewReader(data))
				rows, err := reader.ReadAll()
				if err != nil {
					t.Fatalf("csv.ReadAll() error = %v", err)
				}
				if len(rows) < 2 {
					t.Fatalf("csv output rows = %d, want at least 2", len(rows))
				}
				if rows[0][0] != "index" || rows[0][1] != "timeSeconds" {
					t.Fatalf("csv header = %v, want events header", rows[0])
				}
			}
		})
	}
}
