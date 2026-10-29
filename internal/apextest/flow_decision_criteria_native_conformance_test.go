package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestFlowDecisionCriteriaNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_decision_criteria_native"
	capture, err := os.ReadFile(filepath.Join(fixture, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capture), "native_complete=true\n") {
		t.Fatal("incomplete native capture")
	}

	casesRaw, err := os.ReadFile(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	compareGlade := map[string]bool{}
	for _, row := range strings.Split(strings.TrimSpace(string(casesRaw)), "\n")[1:] {
		columns := strings.Split(row, "\t")
		if len(columns) != 6 || columns[0] == "" {
			t.Fatalf("invalid case row %q", row)
		}
		want[columns[0]] = columns[4]
		switch columns[5] {
		case "compare":
			compareGlade[columns[0]] = true
		case "native-only":
			compareGlade[columns[0]] = false
		default:
			t.Fatalf("invalid Glade comparison mode %q for %s", columns[5], columns[0])
		}
	}

	nativeRaw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	native := map[string]map[string]string{}
	for _, row := range strings.Split(strings.TrimSpace(string(nativeRaw)), "\n")[1:] {
		columns := strings.SplitN(row, "\t", 4)
		if len(columns) != 4 || (columns[0] != "62.0" && columns[0] != "67.0") || columns[1] != "istest" {
			t.Fatalf("invalid native row %q", row)
		}
		if want[columns[2]] != columns[3] {
			t.Fatalf("native row disagrees with case table: %q", row)
		}
		if native[columns[0]] == nil {
			native[columns[0]] = map[string]string{}
		}
		native[columns[0]][columns[2]] = columns[3]
	}
	compared := 0
	for _, compare := range compareGlade {
		if compare {
			compared++
		}
	}
	if len(want) != 41 || compared != 39 || len(native["62.0"]) != len(want) || len(native["67.0"]) != len(want) {
		t.Fatalf("expected 41 native cases and 39 Glade comparisons at API 62.0 and 67.0; cases=%d compared=%d api62=%d api67=%d", len(want), compared, len(native["62.0"]), len(native["67.0"]))
	}

	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				text := string(body)
				if filepath.Ext(path) == ".xml" {
					text = strings.ReplaceAll(text, "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>")
				} else if filepath.Base(path) == "sfdx-project.json" {
					text = strings.ReplaceAll(text, `"sourceApiVersion": "67.0"`, `"sourceApiVersion": "`+api+`"`)
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s:%s=%x\n", api, filepath.ToSlash(name), sha256.Sum256([]byte(text)))) {
					t.Fatalf("native source mismatch: %s", name)
				}
				writeFile(t, filepath.Join(root, name), text)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"CedarDecisionCriteriaProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected selection: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing observation assertion: %+v; problem: %+v", result, result.Problem)
			}
			_, payload, found := strings.Cut(result.Problem.Message, "OBSERVED|decisionCriteria|")
			var got map[string]string
			if !found || json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatalf("invalid Glade observation: %s", result.Problem.Message)
			}
			for name, expected := range native[api] {
				if !compareGlade[name] {
					continue
				}
				if got[name] != expected {
					t.Errorf("%s: native=%q; Glade=%q", name, expected, got[name])
				}
			}
		})
	}
}
