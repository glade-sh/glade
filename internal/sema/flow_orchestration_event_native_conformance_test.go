package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFlowOrchestrationEventNativeConformance(t *testing.T) {
	const dir = "testdata/flow_orchestration_event"
	caseFile, err := os.Open(filepath.Join(dir, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	caseReader := csv.NewReader(caseFile)
	caseReader.Comma = '\t'
	cases, readErr := caseReader.ReadAll()
	closeErr := caseFile.Close()
	if readErr != nil || closeErr != nil || len(cases) != 5 || len(cases[0]) != 3 ||
		cases[0][0] != "source_api" || cases[0][1] != "case" || cases[0][2] != "class_file" {
		t.Fatalf("invalid native case table: rows=%d read=%v close=%v", len(cases), readErr, closeErr)
	}

	classFiles := make(map[string]string, len(cases)-1)
	for _, row := range cases[1:] {
		if len(row) != 3 || (row[0] != "62.0" && row[0] != "67.0") ||
			(row[1] != "bare_type" && row[1] != "schema_type") || filepath.Base(row[2]) != row[2] || !strings.HasSuffix(row[2], ".cls") {
			t.Fatalf("invalid native case: %v", row)
		}
		key := row[0] + "/" + row[1]
		if _, exists := classFiles[key]; exists {
			t.Fatalf("duplicate native case: %s", key)
		}
		classFiles[key] = row[2]
	}

	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	orgReader := csv.NewReader(strings.NewReader(string(raw)))
	orgReader.Comma = '\t'
	rows, err := orgReader.ReadAll()
	if err != nil || len(rows) != 5 || len(rows[0]) != 4 || rows[0][0] != "source_api" ||
		rows[0][1] != "case" || rows[0][2] != "compiled" || rows[0][3] != "message" {
		t.Fatalf("invalid native observations: rows=%d err=%v", len(rows), err)
	}

	seen := make(map[string]bool, len(rows)-1)
	seenAPIs := make(map[string]bool)
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid native observation row: %v", row)
		}
		key := row[0] + "/" + row[1]
		classFile, ok := classFiles[key]
		if !ok || seen[key] {
			t.Fatalf("unexpected or duplicate native observation: %v", row)
		}
		compiled, err := strconv.ParseBool(row[2])
		if err != nil || (compiled && row[3] != "-") || (!compiled && row[3] == "-") {
			t.Fatalf("invalid native compile outcome: %v", row)
		}
		seen[key] = true
		seenAPIs[row[0]] = true
		t.Run(key, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(dir, "project", "force-app", "main", "default", "classes", classFile))
			if err != nil {
				t.Fatal(err)
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{classFile: string(source)}, row[0])
			if got := !result.HasErrors(); got != compiled {
				t.Fatalf("native compiled=%t (%s); Glade compiled=%t diagnostics=%+v", compiled, row[3], got, result.Diagnostics)
			}
		})
	}
	if len(seen) != len(classFiles) || !seenAPIs["62.0"] || !seenAPIs["67.0"] {
		t.Fatalf("incomplete floor/ceiling observations: cases=%v APIs=%v", seen, seenAPIs)
	}
}
