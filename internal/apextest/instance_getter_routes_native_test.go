package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstanceGetterRoutesNative(t *testing.T) {
	const fixture = "testdata/conformance/instance_getter_routes"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(rows), "\n") != 15 {
		t.Fatal("expected fourteen native observations")
	}
	run := Run(loadTestIndex(t, filepath.Join(fixture, "project")), Options{NoDiskCache: true, Parallelism: 1})
	count := 0
	for _, suite := range run.Suites {
		for _, result := range suite.Cases {
			count++
			switch result.MethodName {
			case "otherInstance":
				if result.Problem == nil || result.Problem.Type != "System.AssertException" || !strings.Contains(result.Problem.Message, "Expected: 1, Actual: 0") {
					t.Errorf("native assertion failure, local result: %+v", result)
				}
			case "recursiveHelper":
				if result.Problem == nil || result.Problem.Type != "System.LimitException" || !strings.Contains(result.Problem.Message, "Maximum stack depth reached: 1001") {
					t.Errorf("native limit exception, local result: %+v", result)
				}
			default:
				if result.Status != "pass" {
					t.Errorf("%s.%s: native pass, local result: %+v", result.ClassName, result.MethodName, result.Problem)
				}
			}
		}
	}
	if count != 14 {
		t.Fatalf("selected %d cases, expected 14", count)
	}
}
