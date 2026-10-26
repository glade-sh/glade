package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestConstructorFieldCatalogNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/constructor_field_catalog"
	var cases []struct{ ID, Source string }
	raw, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 1+len(cases)*6 {
		t.Fatalf("native matrix rows=%d: %v", len(rows), err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 5 || (row[0] != "62.0" && row[0] != "67.0") || (row[3] != "true" && row[3] != "false") {
			t.Fatalf("invalid native row: %v", row)
		}
		key := strings.Join(row[:3], "/")
		if seen[key] {
			t.Fatal("duplicate native row: " + key)
		}
		seen[key] = true
		var source string
		for _, c := range cases {
			if c.ID == row[1] {
				source = c.Source
			}
		}
		if source == "" {
			t.Fatal("missing native source: " + key)
		}
		// Direct managed/lookup evidence: org.tsv:90-137 and ORG-RUN.txt.
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			if err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "67.0", row[0]))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			// Load the standard baseline as complete metadata. A field-only
			// project delta is partial and permits source field inference.
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><fields><fullName>Name</fullName><type>Text</type></fields></CustomObject>`)
			body, want := source, row[4]
			if row[2] == "anonymous" && strings.Contains(want, "\n\n") {
				_, want, _ = strings.Cut(want, "\n\n")
			}
			class := "ConstructorFieldProbe"
			if row[2] == "isTest" {
				class += "Test"
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/"+class+".cls"), "@IsTest private class "+class+" { @IsTest static void observe() { "+body+" System.assert(row != null); } }")
			} else if row[2] == "named" {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/"+class+".cls"), "public class "+class+" { public static void observe() { "+body+" } }")
			} else if row[2] != "anonymous" {
				t.Fatal("unknown route: " + row[2])
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			accepted := row[3] == "true"
			if row[2] == "isTest" {
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("test runner did not select one method: %+v", run)
				}
				got := run.Suites[0].Cases[0]
				if accepted {
					if got.Status != testreport.StatusPass {
						t.Fatalf("native accepted; test runner: %+v", got)
					}
				} else if got.Status != testreport.StatusCompileError || got.Problem == nil || !strings.Contains(got.Problem.Message, want) {
					t.Fatalf("native rejected %q; test runner: %+v", want, got)
				}
				return
			}
			analysis := sema.Analyze(index)
			if row[2] == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, body, row[0])
			}
			var messages []string
			for _, d := range analysis.Diagnostics {
				if d.Severity == diagnostic.Error {
					message := d.NativeMessage
					if message == "" {
						message = d.Message
					}
					messages = append(messages, message)
				}
			}
			if accepted {
				want = ""
			}
			if got := strings.Join(messages, " | "); got != want {
				t.Fatalf("native %q; Glade %q", want, got)
			}
		})
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, c := range cases {
			for _, route := range []string{"anonymous", "named", "isTest"} {
				if !seen[api+"/"+c.ID+"/"+route] {
					t.Fatalf("missing native cell: %s/%s/%s", api, c.ID, route)
				}
			}
		}
	}
}
