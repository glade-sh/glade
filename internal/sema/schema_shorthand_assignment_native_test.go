package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSchemaFieldSetAssignmentNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			for src, dst := range map[string]string{
				"Probe__c.object-meta.xml":   "force-app/objects/Probe__c/Probe__c.object-meta.xml",
				"Main.fieldSet-meta.xml":     "force-app/objects/Probe__c/fieldSets/Main.fieldSet-meta.xml",
				"MainList.fieldSet-meta.xml": "force-app/objects/Probe__c/fieldSets/MainList.fieldSet-meta.xml",
			} {
				data, err := os.ReadFile(filepath.Join("testdata/schema_shorthand", src))
				if err != nil {
					t.Fatal(err)
				}
				out := filepath.Join(root, dst)
				if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(out, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			source, err := os.ReadFile("testdata/schema_shorthand/typing.apex")
			if err != nil {
				t.Fatal(err)
			}
			writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			if err := os.MkdirAll(filepath.Join(root, "force-app/classes"), 0755); err != nil {
				t.Fatal(err)
			}
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
			if len(result.Diagnostics) != 0 {
				t.Fatalf("native assignment accepted; got diagnostics: %+v", result.Diagnostics)
			}
		})
	}
}
