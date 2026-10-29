package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func receiverMemberRows(t *testing.T, name string) [][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../testdata/conformance/receiver_member_routes", name))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestReceiverMemberProjectRunnerNamedIsTestTriggerRoutes(t *testing.T) {
	bodies := map[string]string{}
	for _, row := range receiverMemberRows(t, "cases.tsv")[1:] {
		bodies[row[0]] = row[1]
	}
	rows := receiverMemberRows(t, "org.tsv")
	if len(rows) != 97 {
		t.Fatalf("native rows=%d", len(rows))
	}
	for _, row := range rows[1:] {
		if row[1] == "anonymous" {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
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
			for _, name := range []string{"ReceiverOwner.cls", "ReceiverParent.cls", "GatewayService.cls"} {
				data, err := os.ReadFile(filepath.Join("../../testdata/conformance/receiver_member_routes", name))
				if err != nil {
					t.Fatal(err)
				}
				write(name, string(data))
			}
			body := bodies[row[2]]
			if body == "" {
				t.Fatal("missing captured body")
			}
			invocation := "ReceiverProbe.observe();"
			switch row[1] {
			case "named":
				write("ReceiverProbe.cls", "public class ReceiverProbe { public static void observe() { "+body+" } }")
			case "istest":
				write("ReceiverProbe.cls", "@IsTest private class ReceiverProbe { @IsTest static void observe() { "+body+" } }")
				invocation = ""
			case "trigger":
				write("ReceiverProbe.trigger", "trigger ReceiverProbe on Account (before insert) { "+body+" }")
				invocation = "insert new Account(Name='Receiver');"
			}
			if row[3] == "false" {
				invocation = "System.assert(true);"
			}
			if invocation != "" {
				write("ReceiverRunnerTest.cls", "@IsTest private class ReceiverRunnerTest { @IsTest static void observe() { "+invocation+" } }")
			}
			metadata, err := os.ReadFile("../../testdata/conformance/receiver_member_routes/AccountSummary.fieldSet-meta.xml")
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fieldSets/AccountSummary.fieldSet-meta.xml"), string(metadata))
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			if row[3] == "true" {
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 || len(run.ExcludedClasses) != 0 {
					t.Fatalf("summary=%+v excluded=%+v suites=%+v", s, run.ExcludedClasses, run.Suites)
				}
				return
			}
			for _, entry := range run.ExcludedClasses {
				if entry.ClassName == "ReceiverProbe" && strings.Contains(entry.Message, row[4]) {
					if run.Summary().Passed != 1 {
						t.Fatal(run.Summary())
					}
					return
				}
			}
			t.Fatalf("native diagnostic %q; excluded=%+v summary=%+v", row[4], run.ExcludedClasses, run.Summary())
		})
	}
}

// Missing dependencies are deferred by the compiler; a synthetic unrelated test
// still runs. A same-spelled local class must not supply the opaque members.
func TestReceiverMemberProjectOpaquePackageCompileRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, collision := range []bool{false, true} {
			name := "missing-only"
			if collision {
				name = "local-type-collision"
			}
			t.Run(api+"/"+name, func(t *testing.T) {
				classes := map[string]string{
					"ReceiverProbe":      "public class ReceiverProbe { missingns.Parcel.Options options; public void observe() { missingns.Parcel.Instance.save(); } }",
					"ReceiverRunnerTest": "@IsTest private class ReceiverRunnerTest { @IsTest static void observe() { ReceiverProbe probe = new ReceiverProbe(); System.assertNotEquals(null, probe); } }",
				}
				if collision {
					classes["Parcel"] = "public class Parcel { public String local() { return 'local'; } }"
				}
				index := nativeBoundaryIndex(t, api, classes, nil)
				run := Run(index, Options{SelectedClasses: []string{"ReceiverRunnerTest"}, NoDiskCache: true, Parallelism: 1})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("summary=%+v problem=%s", s, firstRunProblem(run))
				}
			})
		}
	}
}
