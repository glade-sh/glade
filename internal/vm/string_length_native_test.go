package vm_test

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStringLengthNativeConformance(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	const dir = "testdata/string_length/"
	var cases []struct{ ID, Expression, Body string }
	raw, err := os.ReadFile(dir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	var hashes map[string]map[string]string
	raw, err = os.ReadFile(dir + "source_sha256.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &hashes); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		named := stringLengthNamedSource(i, c.Body)
		for route, source := range map[string]string{"anonymous": c.Body, "isTest": named} {
			if fmt.Sprintf("%x", sha256.Sum256([]byte(source))) != hashes[c.ID][route] {
				t.Fatalf("captured source changed: %s/%s", c.ID, route)
			}
		}
	}
	file, err := os.Open(dir + "org.tsv")
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
	if len(cases) != 10 || len(rows) != 41 || strings.Join(rows[0], ",") != "id,api,route,type,message,line" {
		t.Fatal("incomplete native String allocation matrix")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 6 {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[1] + "/" + row[2] + "/" + row[0]
		line, err := strconv.Atoi(row[5])
		if seen[key] || (row[1] != "62.0" && row[1] != "67.0") || (row[2] != "anonymous" && row[2] != "isTest") || err != nil || line < 0 {
			t.Fatalf("invalid native cell: %v", row)
		}
		seen[key] = true
		index := -1
		for i, c := range cases {
			if c.ID == row[0] {
				index = i
			}
		}
		if index < 0 {
			t.Fatalf("missing native source: %s", key)
		}
		t.Run(key, func(t *testing.T) {
			c := cases[index]
			var named typesys.Index
			name := "StringLengthProbe" + strconv.Itoa(index)
			if row[2] == "isTest" {
				root := t.TempDir()
				path := filepath.Join(root, name+".cls")
				source := stringLengthNamedSource(index, c.Body)
				if err := os.WriteFile(path, []byte(source), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path+"-meta.xml", []byte("<ApexClass><apiVersion>"+row[1]+"</apiVersion></ApexClass>"), 0644); err != nil {
					t.Fatal(err)
				}
				named = typesys.Build(project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: row[1]}, schema.Schema{})
				if named.HasErrors() {
					t.Fatalf("native source parser: %v", named.Diagnostics)
				}
				if analysis := sema.Analyze(named); analysis.HasErrors() {
					t.Fatalf("native source semantics: %v", analysis.Diagnostics)
				}
			}
			for _, mode := range []vm.LimitMode{vm.LimitModePermissive, vm.LimitModeStrict} {
				t.Run(string(mode), func(t *testing.T) {
					gotType, gotMessage, gotLine := "", "", 0
					if row[2] == "anonymous" {
						program, err := vm.CompileAnonymousWithOptions(c.Body, vm.CompileOptions{APIVersion: row[1]})
						if err != nil {
							t.Fatal(err)
						}
						machine := vm.New(nil)
						machine.SetLimitMode(mode)
						_, err = machine.Execute(program)
						if err != nil {
							var failure *vm.RuntimeError
							if !errors.As(err, &failure) {
								t.Fatalf("native runtime outcome: %v", err)
							}
							gotType, gotMessage = failure.Type, failure.ExceptionMessage()
							if len(failure.Stack) > 0 {
								gotLine = failure.Stack[0].Line
							}
						}
					} else {
						run := apextest.Run(named, apextest.Options{SelectedClasses: []string{name}, LimitMode: mode, NoDiskCache: true, Parallelism: 1})
						if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("native named selection: %+v", run.Summary())
						}
						result := run.Suites[0].Cases[0]
						if result.Problem != nil {
							gotType, gotMessage = result.Problem.Type, result.Problem.Message
							if !strings.HasPrefix(gotType, "System.") {
								gotType = "System." + gotType
							}
							if len(result.Problem.Stack) > 0 {
								gotLine = result.Problem.Stack[0].Line
							}
						} else if result.Status != "pass" {
							t.Fatalf("native named outcome: %+v", result)
						}
					}
					got := fmt.Sprintf("%s|%s|%d", gotType, gotMessage, gotLine)
					want := fmt.Sprintf("%s|%s|%d", row[3], row[4], line)
					if got != want {
						t.Fatalf("native <%s>; Glade <%s>", want, got)
					}
				})
			}
		})
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, c := range cases {
				if !seen[api+"/"+route+"/"+c.ID] {
					t.Fatalf("missing native cell %s/%s/%s", api, route, c.ID)
				}
			}
		}
	}
}

func stringLengthNamedSource(index int, body string) string {
	return "@IsTest private class StringLengthProbe" + strconv.Itoa(index) + " {\n @IsTest static void observe() {\n" + body + "\n }\n}\n"
}
