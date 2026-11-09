package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSchemaShorthandNative(t *testing.T) {
	cases := []struct {
		name     string
		accepted bool
	}{
		{"account_field", true}, {"account_id", true}, {"account_missing", false},
		{"custom_field", true}, {"fieldset_full", true}, {"fieldset_short", true},
		{"fieldset_missing", false}, {"object_missing", false}, {"account_fieldset", false},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				copyFixture := func(src, dst string) {
					t.Helper()
					data, err := os.ReadFile(filepath.Join("testdata/schema_shorthand", src))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, dst)
					if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err = os.WriteFile(path, data, 0644); err != nil {
						t.Fatal(err)
					}
				}
				copyFixture("Probe__c.object-meta.xml", "force-app/objects/Probe__c/Probe__c.object-meta.xml")
				copyFixture("Main.fieldSet-meta.xml", "force-app/objects/Probe__c/fieldSets/Main.fieldSet-meta.xml")
				source, err := os.ReadFile(filepath.Join("testdata/schema_shorthand", tc.name+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				writeSemaFile(t, filepath.Join(root, "force-app/classes/ProbeCase.cls"), "public class ProbeCase { public static void run() {\n"+string(source)+"\n} }")
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				sch, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				result := Analyze(typesys.Build(p, sch))
				if (len(result.Diagnostics) == 0) != tc.accepted {
					t.Fatalf("accepted=%v, want %v; diagnostics: %+v", len(result.Diagnostics) == 0, tc.accepted, result.Diagnostics)
				}
			})
		}
	}
}
