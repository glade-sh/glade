package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnloadedDependencySchemaAliasAnonymousExecNativeRoutes(t *testing.T) {
	dir := filepath.Join("..", "apextest", "testdata/conformance/unloaded_dependency/schema_alias")
	var cases []struct{ ID, Source, Outcome string }
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	object, err := os.ReadFile(filepath.Join(dir, "sources/CedarAlias__c.object-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, cell := range cases {
			for _, route := range []string{"anonymous_exec", "anonymous_trigger_helper_exec"} {
				t.Run(api+"/"+cell.ID+"/"+route, func(t *testing.T) {
					root := t.TempDir()
					writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"cedar-optional"}]}]}`)
					writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/CedarAlias__c/CedarAlias__c.object-meta.xml"), string(object))
					source := cell.Source
					if route == "anonymous_trigger_helper_exec" {
						path := filepath.Join(root, "force-app/main/default/classes/CedarSchemaHelper.cls")
						writeTestFile(t, path, "public class CedarSchemaHelper { public static void apply() { "+source+" } }")
						writeTestFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						path = filepath.Join(root, "force-app/main/default/triggers/CedarSchemaGate.trigger")
						writeTestFile(t, path, "trigger CedarSchemaGate on Account (before insert) { CedarSchemaHelper.apply(); }")
						writeTestFile(t, path+"-meta.xml", "<ApexTrigger><apiVersion>"+api+"</apiVersion></ApexTrigger>")
						source = "insert new Account(Name='cedar');"
					}
					var stdout, stderr bytes.Buffer
					code := Run(context.Background(), []string{"exec", "--project", root, "--json", source}, &stdout, &stderr)
					output := stdout.String() + stderr.String()
					if cell.Outcome == "accepted" {
						if code != 0 {
							t.Fatalf("native accepted: code=%d %s", code, output)
						}
					} else if code == 0 || strings.Contains(output, "dependency_missing") || strings.Contains(output, "cedar-optional") || !strings.Contains(output, "absentMember") {
						t.Fatalf("known Schema object relabeled: code=%d %s", code, output)
					}
				})
			}
		}
	}
}
