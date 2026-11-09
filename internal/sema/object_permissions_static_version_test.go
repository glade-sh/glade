package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native anonymous ST1-ST4: case289_followups/org.tsv and static-boundary.tsv.
func TestObjectPermissionsStaticFieldSourceVersion(t *testing.T) {
	cases := map[string]string{
		"token":       `Schema.SObjectField f=ObjectPermissions.PermissionsViewAllFields;`,
		"assignment":  `ObjectPermissions g=new ObjectPermissions();g.PermissionsViewAllFields=true;`,
		"query":       `List<ObjectPermissions> r=[SELECT Id, PermissionsViewAllFields FROM ObjectPermissions LIMIT 1];`,
		"constructor": `ObjectPermissions g=new ObjectPermissions(PermissionsViewAllFields=true);`,
	}
	for _, api := range []string{"62.0", "63.0", "67.0", "68.0"} {
		index := typesys.Build(project.Project{SourceAPIVersion: api}, schema.Schema{})
		for name, source := range cases {
			t.Run(api+"/"+name, func(t *testing.T) {
				result := AnalyzeAnonymous(index, source, api)
				if got, want := result.HasErrors(), api == "62.0"; got != want {
					t.Fatalf("native rejection=%t, got %t: %v", want, got, result.Diagnostics)
				}
				if api == "62.0" {
					want := "Variable does not exist: PermissionsViewAllFields"
					if name == "query" {
						want = "No such column 'PermissionsViewAllFields' on entity 'ObjectPermissions'"
					} else if name == "constructor" {
						want = "Field does not exist: PermissionsViewAllFields on ObjectPermissions"
					}
					var messages strings.Builder
					for _, d := range result.Diagnostics {
						messages.WriteString(d.Message)
						messages.WriteString(d.NativeMessage)
					}
					if !strings.Contains(messages.String(), want) {
						t.Fatalf("native diagnostic %q not present: %v", want, result.Diagnostics)
					}
				}
			})
		}
	}
}

// The named source counterparts are in case289_followups/static-named.tsv.
func TestObjectPermissionsStaticFieldNamedSourceVersion(t *testing.T) {
	cases := map[string]string{
		"token":       `Schema.SObjectField f=ObjectPermissions.PermissionsViewAllFields;`,
		"assignment":  `ObjectPermissions g=new ObjectPermissions();g.PermissionsViewAllFields=true;`,
		"query":       `List<ObjectPermissions> r=[SELECT Id, PermissionsViewAllFields FROM ObjectPermissions LIMIT 1];`,
		"constructor": `ObjectPermissions g=new ObjectPermissions(PermissionsViewAllFields=true);`,
	}
	for _, api := range []string{"62.0", "63.0", "67.0", "68.0"} {
		for name, source := range cases {
			t.Run(api+"/"+name, func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, "StaticVisibilityProbe.cls")
				writeSemaFile(t, file, "public class StaticVisibilityProbe { public static void observe() { "+source+" } }")
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{file}}, schema.Schema{})
				for _, pass := range []string{"cold", "warm"} {
					result := Analyze(index)
					if got, want := result.HasErrors(), api == "62.0"; got != want {
						t.Fatalf("%s: native rejection=%t, got %t: %v", pass, want, got, result.Diagnostics)
					}
				}
			})
		}
	}
}

// The test-class compile controls are in case289_followups/static-istest.tsv.
func TestObjectPermissionsStaticFieldIsTestSourceVersion(t *testing.T) {
	cases := map[string]string{
		"token":       `Schema.SObjectField f=ObjectPermissions.PermissionsViewAllFields;`,
		"assignment":  `ObjectPermissions g=new ObjectPermissions();g.PermissionsViewAllFields=true;`,
		"query":       `List<ObjectPermissions> r=[SELECT Id, PermissionsViewAllFields FROM ObjectPermissions LIMIT 1];`,
		"constructor": `ObjectPermissions g=new ObjectPermissions(PermissionsViewAllFields=true);`,
	}
	for _, api := range []string{"62.0", "63.0", "67.0", "68.0"} {
		for name, source := range cases {
			t.Run(api+"/"+name, func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, "StaticVisibilityProbe.cls")
				writeSemaFile(t, file, "@IsTest private class StaticVisibilityProbe { @IsTest static void observe() { "+source+" } }")
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{file}}, schema.Schema{})
				result := Analyze(index)
				if got, want := result.HasErrors(), api == "62.0"; got != want {
					t.Fatalf("native rejection=%t, got %t: %v", want, got, result.Diagnostics)
				}
			})
		}
	}
}
