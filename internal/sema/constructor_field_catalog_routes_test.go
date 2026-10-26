package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestConstructorFieldCatalogAnonymousAndNamedRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, field := range []string{"Missing__other", "pkg__Missing__other", "Missing__c", "Bloom__c", "Tradestyle", "FirstName", "LastName"} {
			for _, route := range []string{"anonymous", "named"} {
				t.Run(api+"/"+route+"/"+field, func(t *testing.T) {
					root := t.TempDir()
					file := filepath.Join(root, "ConstructorFieldProbe.cls")
					body := "Account row = new Account(" + field + " = 'x');"
					writeSemaFile(t, file, "public class ConstructorFieldProbe { public static void observe() { "+body+" } }")
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{file}}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Bloom__c", Type: "Text"}}}}})
					result := Analyze(index)
					if route == "anonymous" {
						result = AnalyzeAnonymous(index, body, api)
					}
					want := ""
					if strings.Contains(field, "Missing") || field == "FirstName" || field == "LastName" {
						want = "Field does not exist: " + field + " on Account"
					}
					var got []string
					for _, d := range result.Diagnostics {
						if d.NativeMessage != "" {
							got = append(got, d.NativeMessage)
						} else {
							got = append(got, d.Message)
						}
					}
					if strings.Join(got, " | ") != want {
						t.Fatalf("native %q; Glade %q", want, strings.Join(got, " | "))
					}
				})
			}
		}
	}
}

// Direct captures: constructor_field_catalog/org.tsv:90-137; see ORG-RUN.txt.
// Every complete-object expectation uses the exact source and diagnostic from
// constructor_field_catalog/org.tsv. Partial metadata is a local loading control.
func TestConstructorFieldCatalogManagedAnonymousAndNamedRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/constructor_field_catalog"
	var cases []struct{ ID, Source, Object string }
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
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[2] != "anonymous" && row[2] != "named" {
			continue
		}
		if !strings.HasPrefix(row[1], "managed_") && !strings.HasPrefix(row[1], "meadow_") {
			continue
		}
		var source, object string
		for _, c := range cases {
			if c.ID == row[1] {
				source, object = c.Source, c.Object
			}
		}
		if source == "" {
			t.Fatal("missing native source: " + row[1])
		}
		for _, partial := range []bool{false, true} {
			state := "complete"
			if partial {
				state = "partial-control"
			}
			t.Run(strings.Join(row[:3], "/")+"/"+state, func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, "ConstructorFieldProbe.cls")
				writeSemaFile(t, file, "public class ConstructorFieldProbe { public static void observe() { "+source+" } }")
				fields := []schema.Field{{Name: "Name", Type: "Text"}, {Name: "Parent__c", Type: "Lookup", ReferenceTo: []string{"Account"}}}
				if strings.HasPrefix(row[1], "managed_") {
					fields = []schema.Field{{Name: "Name", Type: "Text"}, {Name: "gladens004__Bloom__c", Type: "Text"}, {Name: "gladens004__Parent__c", Type: "Lookup", ReferenceTo: []string{"Account"}}}
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: []string{file}}, schema.Schema{Objects: []schema.Object{{Name: object, Partial: partial, Fields: fields}}})
				result := Analyze(index)
				if row[2] == "anonymous" {
					result = AnalyzeAnonymous(index, source, row[0])
				}
				want := row[4]
				if row[2] == "anonymous" && strings.Contains(want, "\n\n") {
					_, want, _ = strings.Cut(want, "\n\n")
				}
				if partial || row[3] == "true" {
					want = ""
				}
				var messages []string
				for _, d := range result.Diagnostics {
					message := d.NativeMessage
					if message == "" {
						message = d.Message
					}
					messages = append(messages, message)
				}
				if got := strings.Join(messages, " | "); got != want {
					t.Fatalf("native %q; Glade %q", want, got)
				}
			})
		}
	}
}

func TestConstructorFieldCatalogNamedHarnessLoadsCompleteMetadata(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "ConstructorFieldProbe.cls")
	writeSemaFile(t, file, "public class ConstructorFieldProbe { public static Account observe() { return new Account(Name = 'x', Missing__c = 'y'); } }")
	p := project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{file}}
	empty := prepareAnalysisIndex(typesys.Build(p, schema.Schema{}))
	for _, object := range empty.Objects {
		if object.Name == "Account" {
			t.Logf("empty-schema preparation: partial=%t fields=%v", object.Partial, object.Fields)
		}
	}
	loaded := typesys.Build(p, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}}})
	result := Analyze(loaded)
	if !result.HasErrors() {
		t.Fatalf("complete metadata did not reject the native missing field: %v", result.Diagnostics)
	}
}

func TestConstructorFieldCatalogEnabledFeatureAndDeclaredMetadataRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, c := range []struct {
			name, object, field string
			fields              []schema.Field
			features            []string
		}{
			{"enabled-feature", "Account", "FirstName", []schema.Field{{Name: "Name", Type: "Text"}}, []string{"PersonAccounts"}},
			{"declared-feature", "Account", "FirstName", []schema.Field{{Name: "FirstName", Type: "Text"}}, nil},
			{"declared-package-field", "pkg__Meadow__c", "Name", []schema.Field{{Name: "Name", Type: "Text"}}, nil},
			{"declared-unusual-suffix", "Account", "Bloom__other", []schema.Field{{Name: "Bloom__other", Type: "Text"}}, nil},
		} {
			for _, route := range []string{"anonymous", "named"} {
				t.Run(api+"/"+c.name+"/"+route, func(t *testing.T) {
					root := t.TempDir()
					file := filepath.Join(root, "ConstructorFieldProbe.cls")
					body := c.object + " row = new " + c.object + "(" + c.field + " = 'x');"
					writeSemaFile(t, file, "public class ConstructorFieldProbe { public static void observe() { "+body+" } }")
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{file}}, schema.Schema{Objects: []schema.Object{{Name: c.object, Fields: c.fields}}})
					index.OrgShapeFeatures = c.features
					result := Analyze(index)
					if route == "anonymous" {
						result = AnalyzeAnonymous(index, body, api)
					}
					if result.HasErrors() {
						t.Fatal(result.Diagnostics)
					}
				})
			}
		}
	}
}
