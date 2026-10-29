package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticObjectChainProjectNamedTestAndTriggerRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/static_object_chain_routes"
	deferred := namespaceAbsenceDeferrals(t, fixture)
	cases, err := readTypeNamePinRowsForApextest(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	statements := map[string]string{}
	for _, row := range cases[1:] {
		statements[row[0]] = row[2]
	}
	rows, err := readTypeNamePinRowsForApextest(filepath.Join(fixture, "org.tsv"))
	if err != nil || len(rows) != 49 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		if row[1] == "anonymous" {
			continue
		}
		for _, namespace := range []string{"", "orchard"} {
			t.Run(strings.Join(row[:3], "/")+"/namespace="+namespace, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, namespace, row[0]))
				write := func(name, source string) {
					dir, kind := "classes", "ApexClass"
					if strings.HasSuffix(name, ".trigger") {
						dir, kind = "triggers", "ApexTrigger"
					}
					path := filepath.Join(root, "force-app/main/default", dir, name)
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", "<"+kind+"><apiVersion>"+row[0]+"</apiVersion><status>Active</status></"+kind+">")
				}
				for _, name := range []string{"Holder.cls", "ReceiverValues.cls"} {
					data, err := os.ReadFile(filepath.Join(fixture, name))
					if err != nil {
						t.Fatal(err)
					}
					write(name, string(data))
				}
				statement, message := statements[row[2]], row[5]
				if namespace != "" && row[2] != "missing_namespace" {
					statement = strings.ReplaceAll(statement, "ReceiverValues.", namespace+".ReceiverValues.")
					message = strings.ReplaceAll(message, "ReceiverValues.", namespace+".ReceiverValues.")
				}
				// Compile deferred package calls without executing an unavailable API.
				if deferred[row[2]] {
					statement = "if (false) { " + statement + " }"
				}
				name, source, invocation := "ChainProbe.cls", "", ""
				switch row[1] {
				case "named":
					source = "public class ChainProbe { public static void observe() { " + statement + " } }"
					invocation = "ChainProbe.observe();"
				case "istest":
					source = "@IsTest private class ChainProbe { @IsTest static void observe() { " + statement + " } }"
				case "trigger":
					name = "ChainProbe.trigger"
					source = "trigger ChainProbe on Account (before insert) { " + statement + " }"
					invocation = "insert new Account(Name='Chain');"
				default:
					t.Fatalf("unknown route: %v", row)
				}
				write(name, source)
				if row[3] == "false" && !deferred[row[2]] {
					invocation = "System.assert(true);"
				}
				if row[1] != "istest" || row[3] == "false" && !deferred[row[2]] {
					write("ChainRunnerTest.cls", "@IsTest private class ChainRunnerTest { @IsTest static void observe() { "+invocation+" } }")
				}
				index := loadTestIndex(t, root)
				if index.HasErrors() {
					t.Fatalf("index errors: %v", index.Diagnostics)
				}
				run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
				if row[3] == "true" || deferred[row[2]] {
					if got := run.Summary(); got.Total != 1 || got.Passed != 1 || len(run.ExcludedClasses) != 0 {
						t.Fatalf("native accepted chain: summary=%+v excluded=%+v suites=%+v", got, run.ExcludedClasses, run.Suites)
					}
				} else {
					for _, entry := range run.ExcludedClasses {
						matches := strings.Contains(entry.Message, message)
						if row[2] == "missing_instance_field" && strings.Contains(entry.Message, ".Missing") {
							matches = true
							t.Log("missing-field rejection status matches; diagnostic wording carried in LOCAL-CARRIES.txt")
						}
						if entry.ClassName == "ChainProbe" && matches {
							if run.Summary().Passed != 1 {
								t.Fatalf("unrelated control did not run: %+v", run.Summary())
							}
							return
						}
					}
					t.Fatalf("native rejected with %q: summary=%+v excluded=%+v", message, run.Summary(), run.ExcludedClasses)
				}
			})
		}
	}
}
