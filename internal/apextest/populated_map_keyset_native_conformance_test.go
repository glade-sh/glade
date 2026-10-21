package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPopulatedMapKeySetNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/populated_map/"
	var data struct {
		Classes map[string]string
		Rows    []struct {
			API, Route, ID, ClassName, Method, Source, SourceSHA256, Outcome string
			Expected                                                         json.RawMessage
		}
	}
	raw, err := os.ReadFile(dir + "keyset-boundary.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	classes, err := os.ReadFile(dir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var captured struct{ Classes map[string]string }
	if err := json.Unmarshal(classes, &captured); err != nil {
		t.Fatal(err)
	}
	data.Classes = captured.Classes
	native, err := os.ReadFile(dir + "keyset-boundary.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 4 || len(data.Classes) != 2 {
		t.Fatal("incomplete populated-map key-set observations")
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		t.Run(row.API+"/"+row.Route+"/"+row.ID, func(t *testing.T) {
			key := row.API + "/" + row.Route + "/" + row.ID
			if seen[key] || (row.API != "62.0" && row.API != "67.0") {
				t.Fatal("duplicate or uncaptured map cell")
			}
			seen[key] = true
			source := row.Source
			classes := map[string]string{}
			if row.Route == "isTest" {
				source = data.Classes[row.ClassName]
				classes[row.ClassName] = source
			} else if row.Route != "anonymous" {
				t.Fatal("uncaptured map route")
			}
			sum := sha256.Sum256([]byte(source))
			if source == "" || hex.EncodeToString(sum[:]) != row.SourceSHA256 {
				t.Fatal("captured map source changed")
			}
			var want any
			if err := json.Unmarshal(row.Expected, &want); err != nil {
				t.Fatal(err)
			}
			bound := false
			for _, line := range strings.Split(string(native), "\n") {
				fields := strings.SplitN(line, "\t", 6)
				if len(fields) != 6 || fields[0] != row.API || fields[1] != row.Route || fields[2] != row.ID || fields[3] != row.Outcome || fields[5] != row.SourceSHA256 {
					continue
				}
				var answer any
				bound = json.Unmarshal([]byte(fields[4]), &answer) == nil && reflect.DeepEqual(answer, want)
			}
			if !bound {
				t.Fatal("map observation missing from native rows")
			}
			index := nativeBoundaryIndex(t, row.API, classes, nil)
			analysis := sema.Analyze(index)
			if row.Route == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, source, row.API)
			}
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			marker := "MAP|" + row.ID + "|"
			var payload, failure string
			if row.Route == "isTest" {
				run := Run(index, Options{SelectedClasses: []string{row.ClassName}, SelectedMethod: row.Method, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing native assertion/fatal transport: %+v", run.Summary())
				}
				problem := run.Suites[0].Cases[0].Problem
				if _, value, found := strings.Cut(problem.Message, marker); found {
					payload = value
				} else {
					failure = problem.Type + ": " + problem.Message
				}
			} else {
				local := source
				if row.Outcome == "observation" {
					local = strings.Replace(local, "System.assert(false,", "System.debug(", 1)
				}
				program, err := vm.CompileAnonymousWithOptions(local, vm.CompileOptions{APIVersion: row.API})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					failure = err.Error()
				}
				for _, line := range result.Debug {
					if strings.HasPrefix(line, marker) {
						payload = strings.TrimPrefix(line, marker)
					}
				}
			}
			if row.Outcome == "fatal" {
				if failure == "" || !strings.Contains(failure, fmt.Sprint(want)) || payload != "" {
					t.Fatalf("native fatal=%s; Glade failure=%q observation=%s", row.Expected, failure, payload)
				}
			} else {
				var got any
				if row.Outcome != "observation" || failure != "" || json.Unmarshal([]byte(payload), &got) != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("native=%s; Glade observation=%s failure=%s", row.Expected, payload, failure)
				}
			}
		})
	}
}
