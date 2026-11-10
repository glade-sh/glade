package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticGetterCrossClassNative(t *testing.T) {
	const fixture = "testdata/conformance/static_getter_cross_class"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(rows), "\n") != 9 {
		t.Fatal("expected eight native observations")
	}
	run := Run(loadTestIndex(t, filepath.Join(fixture, "project")), Options{NoDiskCache: true, Parallelism: 1})
	count := 0
	for _, suite := range run.Suites {
		for _, result := range suite.Cases {
			count++
			if result.MethodName == "recursiveCycle" {
				if result.Problem == nil || result.Problem.Type != "System.LimitException" || !strings.Contains(result.Problem.Message, "Maximum stack depth reached: 1001") {
					t.Errorf("%s: native limit exception, local result: %+v", result.ClassName, result)
				}
			} else if result.Status != "pass" {
				t.Errorf("%s.%s: native pass, local result: %+v", result.ClassName, result.MethodName, result.Problem)
			}
		}
	}
	if count != 8 {
		t.Fatalf("selected %d tests, expected 8", count)
	}
}
