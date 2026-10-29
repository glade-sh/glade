package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fieldProxyRows(t *testing.T) [][]string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/conformance/field_proxy_diagnostics/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows=%d: %v", len(rows), err)
	}
	return rows[1:]
}

func TestFieldProxyCorpusShapeRunnerNativeDiagnosticRoutes(t *testing.T) {
	for _, row := range fieldProxyRows(t) {
		if row[1] != "isTest" {
			continue
		}
		for _, expression := range []string{"Schema.Account.fields.getMap()", "Account.fields.getMap()", "Schema.Account.fields.get('Name')", "Account.fields.get('Name')"} {
			for _, route := range []string{"named", "isTest", "trigger"} {
				t.Run(row[0]+"/"+expression+"/"+route, func(t *testing.T) {
					want := row[4]
					if strings.Contains(expression, ".get(") {
						want = strings.ReplaceAll(want, "getMap()", "get(String)")
					}
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
					write := func(name, source string) {
						dir, kind := "classes", "ApexClass"
						if strings.HasSuffix(name, ".trigger") {
							dir, kind = "triggers", "ApexTrigger"
						}
						path := filepath.Join(root, "force-app/main/default", dir, name)
						writeFile(t, path, source)
						writeFile(t, path+"-meta.xml", "<"+kind+"><apiVersion>"+row[0]+"</apiVersion><status>Active</status></"+kind+">")
					}
					name, source := "FieldProxyProbe.cls", "public class FieldProxyProbe { public static void observe() { Object value = Schema.Account.fields.getMap(); } }"
					if route == "isTest" {
						source = "@IsTest private class FieldProxyProbe { @IsTest static void observe() { Object value = Schema.Account.fields.getMap(); } }"
					}
					if route == "trigger" {
						name = "FieldProxyProbe.trigger"
						source = "trigger FieldProxyProbe on Account (before insert) { Object value = Schema.Account.fields.getMap(); }"
					}
					source = strings.ReplaceAll(source, "Schema.Account.fields.getMap()", expression)
					write(name, source)
					write("FieldProxyControlTest.cls", "@IsTest private class FieldProxyControlTest { @IsTest static void observe() { System.assert(true); } }")
					index := loadTestIndex(t, root)
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
					for _, entry := range run.ExcludedClasses {
						if entry.ClassName == "FieldProxyProbe" && strings.Contains(entry.Message, want) {
							if run.Summary().Passed != 1 {
								t.Fatal(run.Summary())
							}
							return
						}
					}
					t.Fatalf("native diagnostic %q; excluded=%+v summary=%+v", want, run.ExcludedClasses, run.Summary())
				})
			}
		}
	}
}
