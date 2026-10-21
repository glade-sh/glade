package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAssignmentDiagnosticNativeConformance(t *testing.T) {
	runCompileDiagnosticNativeRows(t, func(id string) bool {
		return strings.HasPrefix(id, "assignment_") || id == "control_assignment" || id == "control_token"
	})
}

func TestDateOperandDiagnosticNativeConformance(t *testing.T) {
	runCompileDiagnosticNativeRows(t, func(id string) bool {
		return strings.HasPrefix(id, "date_") || strings.HasPrefix(id, "datetime_") || id == "control_date"
	})
}

func TestUnknownReceiverDiagnosticNativeConformance(t *testing.T) {
	runCompileDiagnosticNativeRows(t, func(id string) bool {
		return id == "unknown_call" || id == "unknown_field" || id == "known_call"
	})
}

func TestDuplicateMethodDiagnosticNativeConformance(t *testing.T) {
	runCompileDiagnosticNativeRows(t, func(id string) bool { return id == "duplicate_method" })
}

func TestAPIVersionDiagnosticNativeConformance(t *testing.T) {
	runCompileDiagnosticNativeRows(t, func(id string) bool { return id == "invalid_version" })
}

func TestLocalDescribeDiagnosticNativeConformance(t *testing.T) {
	runCompileDiagnosticNativeRows(t, func(id string) bool { return id == "local_describe" || id == "control_describe" })
}

func TestMissingCustomFieldDiagnosticNativeConformance(t *testing.T) {
	runCompileDiagnosticNativeRows(t, func(id string) bool {
		return id == "missing_field" || id == "missing_field_write" || id == "assignment_field"
	})
}

func TestMissingLabelDiagnosticNativeConformance(t *testing.T) {
	runCompileDiagnosticNativeRows(t, func(id string) bool { return id == "missing_label" })
}

func runCompileDiagnosticNativeRows(t *testing.T, selected func(string) bool) {
	t.Helper()
	const data = "testdata/conformance/compile_diagnostics"
	type probe struct {
		ID, Body           string
		Classes, ClassAPIs map[string]string
	}
	raw, err := os.ReadFile(filepath.Join(data, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var probes []probe
	if err := json.Unmarshal(raw, &probes); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(data, "org.tsv"))
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
	if len(rows) < 2 || strings.Join(rows[0], ",") != "id,api,route,message,line" {
		t.Fatal("missing native observations")
	}
	count := 0
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 5 {
			t.Fatalf("invalid native row: %v", row)
		}
		if row[1] != "62.0" && row[1] != "67.0" || row[2] != "anonymous" && row[2] != "isTest" {
			t.Fatalf("invalid native API or route: %v", row)
		}
		key := row[0] + "/" + row[1] + "/" + row[2]
		if seen[key] {
			t.Fatalf("duplicate native observation %s", key)
		}
		seen[key] = true
		if !selected(row[0]) {
			continue
		}
		count++
		t.Run(row[1]+"/"+row[2]+"/"+row[0], func(t *testing.T) {
			var p probe
			index := -1
			for i, candidate := range probes {
				if candidate.ID == row[0] {
					p = candidate
					index = i
					break
				}
			}
			if index < 0 {
				t.Fatal("missing synthetic source")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
			sources := p.Classes
			if p.Body != "" && row[2] == "isTest" {
				name := "DiagnosticProbe" + strconv.Itoa(index)
				sources = map[string]string{name: "@IsTest private class " + name + " {\n @IsTest static void observe() {\n " + p.Body + "\n }\n}\n"}
			}
			for name, source := range sources {
				path := filepath.Join(root, "force-app", "main", "default", "classes", name+".cls")
				writeFile(t, path, source)
				api := row[1]
				if p.ClassAPIs[name] != "" {
					api = p.ClassAPIs[name]
				}
				writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			proj, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := schema.LoadProject(proj)
			if err != nil {
				t.Fatal(err)
			}
			idx := typesys.Build(proj, metadata)
			result := sema.Analyze(idx)
			if p.Body != "" && row[2] == "anonymous" {
				result = sema.AnalyzeAnonymous(idx, p.Body+"\n", row[1])
			}
			var actual []string
			line, err := strconv.Atoi(row[4])
			if err != nil || line < 0 {
				t.Fatalf("invalid native line %q", row[4])
			}
			lineMatched := line == 0
			for _, d := range result.Diagnostics {
				if d.Severity != diagnostic.Error {
					continue
				}
				message := d.NativeMessage
				if message == "" {
					message = d.Message
				}
				duplicate := false
				for _, s := range actual {
					duplicate = duplicate || s == message
				}
				if !duplicate {
					actual = append(actual, message)
				}
				gotLine := 0
				if d.Range != nil {
					gotLine = d.Range.Start.Line
				}
				if d.NativeLine != nil {
					gotLine = *d.NativeLine
				}
				lineMatched = lineMatched || gotLine == line
			}
			message := "compiled"
			if len(actual) > 0 {
				message = strings.Join(actual, "; ")
			}
			if message != row[3] {
				t.Errorf("native <%s>, Glade <%s>", row[3], message)
			}
			if !lineMatched {
				t.Errorf("native line %d absent from diagnostics: %#v", line, result.Diagnostics)
			}
		})
	}
	for _, p := range probes {
		if !selected(p.ID) {
			continue
		}
		routes := []string{"isTest"}
		if p.Body != "" {
			routes = append(routes, "anonymous")
		}
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range routes {
				if !seen[p.ID+"/"+api+"/"+route] {
					t.Errorf("missing native observation %s/%s/%s", p.ID, api, route)
				}
			}
		}
	}
	if count == 0 {
		t.Fatal("empty native selection")
	}
}
