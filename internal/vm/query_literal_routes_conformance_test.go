package vm_test

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const queryLiteralFixture = "testdata/query_literal_routes"

// Each cell has its own native observation: static queries, dynamic queries,
// anonymous execution, and project test methods must retain their route twins.
func TestQueryLiteralRoutesNativeConformance(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	shapes := readQueryLiteralTable(t, "cases.tsv", "shape|literal|field")
	routes := readQueryLiteralTable(t, "routes.tsv", "route|template")
	rows := readQueryLiteralTable(t, "org.tsv", "source_api|context|shape|route|outcome|detail")
	if len(shapes) != 9 || len(routes) != 7 || len(rows) != 2*2*len(shapes)*len(routes) {
		t.Fatalf("incomplete literal matrix: shapes=%d routes=%d native=%d", len(shapes), len(routes), len(rows))
	}
	sources := map[string]string{}
	for _, shape := range shapes {
		for _, route := range routes {
			raw, err := os.ReadFile(filepath.Join(queryLiteralFixture, route[1]))
			if err != nil {
				t.Fatal(err)
			}
			key := shape[0] + "/" + route[0]
			if _, exists := sources[key]; exists {
				t.Fatalf("duplicate shape/route %s", key)
			}
			sources[key] = strings.NewReplacer("{{literal}}", shape[1], "{{field}}", shape[2]).Replace(string(raw))
		}
	}
	wants := map[string][]string{}
	for _, row := range rows {
		key := strings.Join(row[:4], "/")
		if _, exists := wants[key]; exists || sources[row[2]+"/"+row[3]] == "" ||
			(row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "istest") ||
			(row[4] != "accept" && row[4] != "reject" && row[4] != "compile_reject") {
			t.Fatalf("invalid native cell %v", row)
		}
		wants[key] = row
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "istest"} {
			for _, shape := range shapes {
				for _, route := range routes {
					key := api + "/" + context + "/" + shape[0] + "/" + route[0]
					row, ok := wants[key]
					if !ok {
						t.Fatalf("missing native cell %s", key)
					}
					t.Run(key, func(t *testing.T) {
						source := sources[shape[0]+"/"+route[0]]
						if context == "istest" {
							checkQueryLiteralProject(t, api, source, row[4], row[5])
							return
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							if row[4] == "compile_reject" {
								return
							}
							t.Fatalf("compile: %v; native=%s %s", err, row[4], row[5])
						}
						org := storage.NewOrgState()
						org.APIVersion = api
						storage.EnsureStandardObject(&org, "Contact")
						machine := vm.New(nil)
						machine.SetOrg(&org)
						_, err = machine.Execute(program)
						if row[4] == "accept" {
							if err != nil {
								t.Fatalf("execute: %v; native accepted", err)
							}
							return
						}
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || strings.TrimPrefix(runtimeErr.Type, "System.") != "QueryException" {
							t.Fatalf("execute=%v; want query rejection, native=%s %s", err, row[4], row[5])
						}
					})
				}
			}
		}
	}
}

func checkQueryLiteralProject(t *testing.T, api, source, outcome, detail string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api),
		"force-app/main/default/classes/ClockLiteralRouteTest.cls":          "@IsTest private class ClockLiteralRouteTest { @IsTest static void literalRoute() {\n" + source + "\n} }\n",
		"force-app/main/default/classes/ClockLiteralRouteTest.cls-meta.xml": "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>",
	}
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, s)
	if index.HasErrors() {
		if outcome == "compile_reject" {
			return
		}
		t.Fatalf("index: %v", index.Diagnostics)
	}
	report := apextest.Run(index, apextest.Options{NoDiskCache: true, Parallelism: 1})
	summary := report.Summary()
	if outcome == "accept" {
		if summary.Total != 1 || summary.Passed != 1 || !summary.OK() {
			t.Fatalf("project: %+v; report=%+v; native accepted", summary, report)
		}
		return
	}
	if outcome == "compile_reject" && (summary.CompileErrors > 0 || summary.ExcludedClasses > 0) {
		return
	}
	for _, suite := range report.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil && strings.TrimPrefix(c.Problem.Type, "System.") == "QueryException" {
				return
			}
		}
	}
	t.Fatalf("project: %+v; report=%+v; want query rejection, native=%s %s", summary, report, outcome, detail)
}

func readQueryLiteralTable(t *testing.T, name, header string) [][]string {
	t.Helper()
	f, err := os.Open(filepath.Join(queryLiteralFixture, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 || strings.Join(records[0], "|") != header {
		t.Fatalf("invalid %s header", name)
	}
	return records[1:]
}
