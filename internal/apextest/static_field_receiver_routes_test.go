package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticFieldReceiverProjectNamedTestAndTriggerRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/static_field_receiver_routes"
	rows, err := readTypeNamePinRowsForApextest(filepath.Join(fixture, "org.tsv"))
	if err != nil || len(rows) != 9 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"named", "istest", "trigger"} {
			found := false
			for _, row := range rows[1:] {
				if row[0] == api && row[1] == route && row[2] == "true" && row[3] == "true" {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing native %s/%s", api, route)
			}
		}
		for _, namespace := range []string{"", "orchard"} {
			t.Run(api+"/named_istest_trigger/namespace="+namespace, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, namespace, api))
				for _, name := range []string{"ReceiverValues.cls", "StaticFieldReceiverProbe.cls", "StaticFieldReceiverProbeTest.cls", "StaticFieldReceiverProbeTrigger.trigger"} {
					data, err := os.ReadFile(filepath.Join(fixture, name))
					if err != nil {
						t.Fatal(err)
					}
					source := string(data)
					if namespace != "" {
						source = strings.ReplaceAll(source, "ReceiverValues.Text", namespace+".ReceiverValues.Text")
					}
					dir, kind := "classes", "ApexClass"
					if strings.HasSuffix(name, ".trigger") {
						dir, kind = "triggers", "ApexTrigger"
					}
					path := filepath.Join(root, "force-app/main/default", dir, name)
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", "<"+kind+"><apiVersion>"+api+"</apiVersion><status>Active</status></"+kind+">")
				}
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
				if got := run.Summary(); got.Total != 1 || got.Passed != 1 || len(run.ExcludedClasses) != 0 {
					t.Fatalf("static field receiver project: summary=%+v excluded=%+v suites=%+v", got, run.ExcludedClasses, run.Suites)
				}
			})
		}
	}
}

func TestUnresolvedDeclaredNamespaceProjectGatePreservesLocalCollisionRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for name, source := range map[string]string{
				"Parcel":                  "public class Parcel { public Id observe() { return localValue(); } private Id localValue() { return null; } }",
				"DeclaredDependencyProbe": "public class DeclaredDependencyProbe { missingns.Parcel.Options options; public void observe() { missingns.Parcel.Instance.save(); } }",
				"LocalCollisionProbeTest": "@IsTest private class LocalCollisionProbeTest { @IsTest static void observe() { System.assertEquals(null, new Parcel().observe()); } }",
			} {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			for _, excluded := range run.ExcludedClasses {
				if strings.Contains(excluded.Message, "Variable does not exist: missingns.Parcel.Instance") {
					t.Fatalf("false local error for external dependency: %+v", excluded)
				}
			}
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
				t.Fatalf("local collision route: %+v excluded=%+v", got, run.ExcludedClasses)
			}
		})
	}
}
