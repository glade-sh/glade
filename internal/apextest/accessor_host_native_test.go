package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native JSON and FormulaEval observations: accessor_host_routes/org.tsv:2-29.
func TestAccessorHostRoutesNative(t *testing.T) {
	const fixture = "testdata/conformance/accessor_host_routes"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 29 {
		t.Fatalf("got %d native rows including header, want 29", len(rows))
	}

	run := Run(loadTestIndex(t, filepath.Join(fixture, "project")), Options{NoDiskCache: true, Parallelism: 1})
	counts := make(map[string]int)
	for _, suite := range run.Suites {
		for _, result := range suite.Cases {
			counts[result.MethodName]++
			switch result.MethodName {
			case "jsonRecursive", "formulaRecursive":
				if result.Problem == nil || result.Problem.Type != "System.LimitException" || !strings.Contains(result.Problem.Message, "Maximum stack depth reached: 1001") {
					t.Errorf("%s.%s: native limit exception, local result: %+v", result.ClassName, result.MethodName, result)
				}
			case "jsonDirect", "jsonFinite", "formulaDirect", "formulaFinite", "formulaReject":
				if result.Status != "pass" {
					t.Errorf("%s.%s: native pass, local result: %+v", result.ClassName, result.MethodName, result.Problem)
				}
			default:
				t.Errorf("unexpected test case %s.%s", result.ClassName, result.MethodName)
			}
		}
	}
	for _, name := range []string{"jsonDirect", "jsonFinite", "jsonRecursive", "formulaDirect", "formulaFinite", "formulaRecursive", "formulaReject"} {
		if counts[name] != 2 {
			t.Errorf("selected %d %s cases, want 2", counts[name], name)
		}
	}
}
