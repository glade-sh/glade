package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccessorCollisionNative(t *testing.T) {
	const fixture = "testdata/conformance/accessor_collision"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(rows), "\n") != 9 {
		t.Fatal("expected eight native observations")
	}
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			run := Run(loadTestIndex(t, filepath.Join(fixture, "api"+api, "project")), Options{NoDiskCache: true, Parallelism: 1})
			count := 0
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					count++
					if result.MethodName == "ownSetterRead" {
						if result.Problem == nil || result.Problem.Type != "System.AssertException" || !strings.Contains(result.Problem.Message, "Expected: own-getter, Actual: null") {
							t.Errorf("native assertion failure, local result: %+v", result)
						}
					} else if result.Status != "pass" {
						t.Errorf("%s: native pass, local result: %+v", result.MethodName, result.Problem)
					}
				}
			}
			if count != 3 {
				t.Fatalf("selected %d cases, expected 3", count)
			}
		})
	}
	run := Run(loadTestIndex(t, filepath.Join(fixture, "backing/project")), Options{NoDiskCache: true, Parallelism: 1})
	if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("backing summary=%#v, problem=%s", summary, firstRunProblem(run))
	}
}
