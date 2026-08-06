package main

import (
	"path/filepath"
	"testing"
)

func TestBenchmarkCorpusHybridRanges(t *testing.T) {
	fixtureDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "rooms"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	rows, err := buildCorpusReport(fixtureDir, 3)
	if err != nil {
		t.Fatalf("buildCorpusReport() error = %v", err)
	}

	if len(rows) != len(defaultCorpusCases) {
		t.Fatalf("buildCorpusReport() returned %d rows, want %d", len(rows), len(defaultCorpusCases))
	}

	casesByName := make(map[string]corpusCase, len(defaultCorpusCases))
	for _, corpusCase := range defaultCorpusCases {
		casesByName[corpusCase.name] = corpusCase
	}

	for _, row := range rows {
		corpusCase, ok := casesByName[row.name]
		if !ok {
			t.Errorf("unexpected corpus row %q", row.name)
			continue
		}

		t.Logf("%s: T60 %.3f s, EDT %.3f s, C80 %.3f dB", row.name, row.t60, row.edt, row.c80)

		if !inRange(row.t60, corpusCase.t60Range) {
			t.Errorf("%s T60 = %.6f, want %v", row.name, row.t60, corpusCase.t60Range)
		}

		if !inRange(row.edt, corpusCase.edtRange) {
			t.Errorf("%s EDT = %.6f, want %v", row.name, row.edt, corpusCase.edtRange)
		}

		if !inRange(row.c80, corpusCase.c80Range) {
			t.Errorf("%s C80 = %.6f, want %v", row.name, row.c80, corpusCase.c80Range)
		}
	}

	if !corpusRowsPass(rows) {
		t.Fatal("corpusRowsPass() = false, want all corrected hybrid rows to pass")
	}
}
