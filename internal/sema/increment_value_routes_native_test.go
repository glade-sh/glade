package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestIncrementValueRoutesNativeSema(t *testing.T) {
	runIncrementValueRoutesNativeSema(t, false)
}

// Comments preserve the captured rejection token stream, including semicolons
// embedded in comments before and after the postfix operator.
func TestIncrementValueRoutesPostfixCommentsSema(t *testing.T) {
	runIncrementValueRoutesNativeSema(t, true)
}

func runIncrementValueRoutesNativeSema(t *testing.T, comments bool) {
	const dir = "../apextest/testdata/conformance/increment_value_routes"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Body, Kind string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{}
	for _, row := range cases {
		bodies[row.ID] = row.Body
	}
	helper, err := os.ReadFile(filepath.Join(dir, "OperatorCell.cls"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 149 {
		t.Fatalf("native rows: %d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		if comments && row[2] != "string_local" && row[2] != "string_field" && row[2] != "boxed_object" {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			if row[3] == "unresolved" {
				t.Skip("native interpreter failure supplies no language result")
			}
			body := bodies[row[2]]
			if comments {
				body = strings.Replace(body, "++;", "/* ; */++/* ; */;", 1)
			}
			if body == "" {
				t.Fatal("missing captured case")
			}
			root := t.TempDir()
			path := filepath.Join(root, "OperatorCell.cls")
			if err := os.WriteFile(path, helper, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path+"-meta.xml", []byte("<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>"), 0600); err != nil {
				t.Fatal(err)
			}
			paths := []string{path}
			if row[1] == "isTest" {
				path = filepath.Join(root, "OperatorObservationTest.cls")
				source := "@IsTest private class OperatorObservationTest { @IsTest static void " + row[2] + "() {\n" + body + "\n} }"
				if err := os.WriteFile(path, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				paths = append(paths, path)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: paths}, schema.Schema{})
			result := Analyze(index)
			if row[1] == "anonymous" {
				result = AnalyzeAnonymous(index, body, row[0])
			}
			var problems []string
			for _, d := range append(index.Diagnostics, result.Diagnostics...) {
				if d.Severity == diagnostic.Error || d.Code == runtimeLoweringDiagnosticCode {
					message := d.NativeMessage
					if message == "" {
						message = d.Message
					}
					problems = append(problems, message)
				}
			}
			want := ""
			if row[3] == "rejection" {
				if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
					t.Fatal(err)
				}
			}
			if got := strings.Join(problems, " | "); got != want {
				t.Fatalf("native=%q; Glade=%q", want, got)
			}
		})
	}
}
