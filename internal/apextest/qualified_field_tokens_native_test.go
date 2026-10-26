package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

func TestQualifiedFieldTokensProjectNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyProjects(t, "../sema/testdata/qualified_field_tokens")
}

func runCapturedSemanticBodyProjects(t *testing.T, dir string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID, Body       string
		Reject         bool
		CarryAnonymous string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 1+6*len(cases) {
		t.Fatal("incomplete native matrix", err)
	}
	for _, row := range rows[1:] {
		carry := ""
		for _, c := range cases {
			if c.ID == row[2] {
				carry = c.CarryAnonymous
			}
		}
		if row[1] == "anonymous" && carry != "" {
			t.Logf("carried native row %s: %s", strings.Join(row[:3], "/"), carry)
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			body := ""
			for _, c := range cases {
				if c.ID == row[2] {
					body = c.Body
				}
			}
			if body == "" {
				t.Fatal("missing native source")
			}
			classes := map[string]string{}
			files, err := filepath.Glob(filepath.Join(dir, "classes/*.cls"))
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range files {
				raw, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				classes[strings.TrimSuffix(filepath.Base(file), ".cls")] = string(raw)
			}
			objects := map[string]string{}
			object, err := os.ReadFile(filepath.Join(dir, "metadata/TokenEntry__c.object-meta.xml"))
			if err == nil {
				field, err := os.ReadFile(filepath.Join(dir, "metadata/Value__c.field-meta.xml"))
				if err != nil {
					t.Fatal(err)
				}
				fields := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(string(field)), "</CustomField>"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">`)
				objects["TokenEntry__c"] = strings.Replace(string(object), "</CustomObject>", "<fields>"+fields+"</fields></CustomObject>", 1)
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if row[1] == "named" {
				classes["GardenBodyHelper"] = "public class GardenBodyHelper { public static void observe() { " + body + " } }"
				classes["GardenBodyTest"] = "@IsTest private class GardenBodyTest { @IsTest static void observe() { GardenBodyHelper.observe(); } }"
			} else if row[1] == "istest" {
				classes["GardenBodyTest"] = "@IsTest private class GardenBodyTest { @IsTest static void observe() { " + body + " } }"
			}
			idx := nativeBoundaryIndex(t, row[0], classes, objects)
			analysis := sema.Analyze(idx)
			if row[1] == "anonymous" {
				analysis = sema.AnalyzeAnonymous(idx, body, row[0])
			}
			if (idx.HasErrors() || analysis.HasErrors()) != (row[3] == "rejected") {
				t.Fatalf("native %s (%s); Glade index=%v diagnostics=%v", row[3], row[4], idx.Diagnostics, analysis.Diagnostics)
			}
			if row[3] == "rejected" {
				return
			}
			if row[1] != "anonymous" {
				run := Run(idx, Options{SelectedClasses: []string{"GardenBodyTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Status != testreport.StatusPass {
					t.Fatalf("native passed; Glade run=%+v", run)
				}
				return
			}
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(idx)
			org.APIVersion = row[0]
			runner := newConformanceRunner(t, idx, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
