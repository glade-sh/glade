package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// Native static-reference controls: testdata/conformance/declared_dependency_alias_capture/
// api{62,67}/org.tsv, missing_static and loaded_static. Alias handling and
// independent-test scoping are local project-resolver policy.
func TestDeclaredDependencyAliasAvailabilityControls(t *testing.T) {
	t.Setenv("GLADE_SIBLING_DEPENDENCIES", "1")
	for _, api := range []string{"62.0", "67.0"} {
		f, err := os.Open(filepath.Join("testdata/conformance/declared_dependency_alias_capture", "api"+api[:2], "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(f)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		native := map[string]string{}
		for _, row := range rows[1:] {
			if len(row) != 5 || row[1] != api {
				t.Fatalf("invalid native row: %v", row)
			}
			native[row[0]+"/"+row[2]] = row[3]
		}
		for _, route := range []string{"anonymous", "istest"} {
			if native["missing_static/"+route] != "rejected" || native["loaded_static/"+route] != "accepted" {
				t.Fatalf("native controls: %v", native)
			}
		}
		for _, alias := range []bool{false, true} {
			for _, present := range []bool{false, true} {
				t.Run(fmt.Sprintf("api%s/alias%t/present%t", api, alias, present), func(t *testing.T) {
					workspace := t.TempDir()
					root := filepath.Join(workspace, "consumer")
					aliasField := ""
					if alias {
						aliasField = `,"packageAliases":{"CedarLibrary":"0Ho000000000000AAA"}`
					}
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"CedarLibrary"}]}]`+aliasField+`}`)
					writeClass := func(dir, name, source string) {
						path := filepath.Join(dir, "force-app/main/default/classes", name+".cls")
						writeFile(t, path, source)
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
					}
					writeClass(root, "CedarDependentTest", `@IsTest private class CedarDependentTest { @IsTest static void observe() { System.assertEquals(7,CedarLibrary.value()); } }`)
					writeClass(root, "CedarIndependentTest", `@IsTest private class CedarIndependentTest { @IsTest static void observe() { System.assertEquals(7,7); } }`)
					if present {
						supplier := filepath.Join(workspace, "supplier")
						writeFile(t, filepath.Join(supplier, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"CedarLibrary"}]}`)
						writeClass(supplier, "CedarLibrary", `public class CedarLibrary { public static Integer value() { return 7; } }`)
					}
					run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
					got := map[string]testreport.Status{}
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							got[c.ClassName] = c.Status
						}
					}
					wantDependent := testreport.StatusCompileError
					if present {
						wantDependent = testreport.StatusPass
					}
					if len(got) != 2 || got["CedarDependentTest"] != wantDependent || got["CedarIndependentTest"] != testreport.StatusPass {
						t.Fatalf("statuses=%v exclusions=%+v", got, run.ExcludedClasses)
					}
				})
			}
		}
	}
}

// A mixed manifest reproduces the work-queue shape: one alias and one bare
// package are both unavailable. Neither should exclude an unrelated test.
func TestDeclaredDependencyMixedAliasesScope(t *testing.T) {
	t.Setenv("GLADE_SIBLING_DEPENDENCIES", "1")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"BirchLibrary"},{"package":"CedarLibrary"}]}],"packageAliases":{"BirchLibrary":"04t000000000000AAA"}}`)
			for name, body := range map[string]string{
				"BirchTest":       `BirchLibrary.value();`,
				"CedarTest":       `CedarLibrary.value();`,
				"IndependentTest": `System.assertEquals(7,7);`,
			} {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, `@IsTest private class `+name+` { @IsTest static void observe(){`+body+`} }`)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			got := map[string]testreport.Case{}
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					got[c.ClassName] = c
				}
			}
			if len(got) != 3 || got["IndependentTest"].Status != testreport.StatusPass {
				t.Fatalf("independent test gated: %+v exclusions=%+v", got, run.ExcludedClasses)
			}
			for _, name := range []string{"BirchTest", "CedarTest"} {
				c := got[name]
				if c.Status != testreport.StatusCompileError || c.Problem == nil || c.Problem.Type != "dependency_missing" {
					t.Errorf("%s: %+v", name, c)
				}
			}
		})
	}
}
