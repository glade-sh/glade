package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
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

// Native receiver_member_opaque_package/org.tsv C01/C02 (62.0 and 67.0): in an
// org without the missingns package, ReceiverProbe is Invalid type at the
// field and Variable does not exist at the static call, with or without a
// same-spelled local class, and the runner test cannot compile. That
// supersedes this test's v0.3.2 offline expectation (Total 1, Passed 1),
// which no native row backed. A glade.yml externalNamespaces declaration of
// a package without source keeps the namespace open, so the deferred
// project still compiles and the unrelated test still runs; a same-spelled
// local class must not supply the opaque members.
func TestReceiverMemberProjectOpaquePackageCompileRoutes(t *testing.T) {
	rows := receiverMemberOpaqueRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		for _, collision := range []bool{false, true} {
			name, nativeCase := "missing-only", "C01_missing_only"
			if collision {
				name, nativeCase = "local-type-collision", "C02_local_type_collision"
			}
			classes := map[string]string{
				"ReceiverProbe":      "public class ReceiverProbe { missingns.Parcel.Options options; public void observe() { missingns.Parcel.Instance.save(); } }",
				"ReceiverRunnerTest": "@IsTest private class ReceiverRunnerTest { @IsTest static void observe() { ReceiverProbe probe = new ReceiverProbe(); System.assertNotEquals(null, probe); } }",
			}
			if collision {
				classes["Parcel"] = "public class Parcel { public String local() { return 'local'; } }"
			}
			t.Run(api+"/"+name+"/undeclared", func(t *testing.T) {
				assertReceiverMemberOpaqueNative(t, rows, nativeCase, api, classes, "ReceiverRunnerTest")
			})
			t.Run(api+"/"+name+"/declared", func(t *testing.T) {
				index := externalNamespaceProject(t, api, "missingns", classes)
				run := Run(index, Options{SelectedClasses: []string{"ReceiverRunnerTest"}, NoDiskCache: true, Parallelism: 1})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("summary=%+v problem=%s", s, firstRunProblem(run))
				}
			})
		}
	}
}

const receiverMemberOpaqueDir = "../../testdata/conformance/receiver_member_opaque_package"

type receiverMemberOpaqueRow struct {
	Case, API, Route, Outcome, Component, Observed, Source string
}

func receiverMemberOpaqueRows(t *testing.T) []receiverMemberOpaqueRow {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(receiverMemberOpaqueDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 71 {
		t.Fatalf("native rows=%d", len(records))
	}
	var rows []receiverMemberOpaqueRow
	for _, r := range records[1:] {
		rows = append(rows, receiverMemberOpaqueRow{Case: r[0], API: r[1], Route: r[3], Outcome: r[4], Component: r[5], Observed: strings.ReplaceAll(r[7], `\n`, "\n"), Source: r[8]})
	}
	return rows
}

// assertReceiverMemberOpaqueNative runs classes through the project runner
// and checks the native outcome of nativeCase at api: a compiled case passes
// its test; a rejected case reports exactly the native ReceiverProbe errors,
// excludes the unit and gives the dependent runner the native problem text.
func assertReceiverMemberOpaqueNative(t *testing.T, rows []receiverMemberOpaqueRow, nativeCase, api string, classes map[string]string, testClass string) {
	t.Helper()
	var probeErrors []string
	runnerProblem, compiled := "", false
	for _, row := range rows {
		if row.Case != nativeCase || row.API != api {
			continue
		}
		switch {
		case row.Outcome == "COMPILED_TESTS_PASSED":
			compiled = true
		case row.Component == "ReceiverProbe":
			probeErrors = append(probeErrors, row.Observed)
		case row.Component == testClass:
			runnerProblem = row.Observed
		}
	}
	if !compiled && len(probeErrors) == 0 {
		t.Fatalf("no native rows for %s %s", nativeCase, api)
	}
	index := nativeBoundaryIndex(t, api, classes, nil)
	var got []string
	for _, d := range sema.Analyze(index).Diagnostics {
		if d.Severity == diagnostic.Error {
			got = append(got, filepath.Base(d.File)+": "+d.NativeMessage)
		}
	}
	var want []string
	for _, message := range probeErrors {
		want = append(want, "ReceiverProbe.cls: "+message)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("errors=%q, native %q", got, want)
	}
	run := Run(index, Options{SelectedClasses: []string{testClass}, NoDiskCache: true, Parallelism: 1})
	s := run.Summary()
	if compiled {
		if s.Total != 1 || s.Passed != 1 || len(run.ExcludedClasses) != 0 {
			t.Fatalf("summary=%+v excluded=%+v problem=%s", s, run.ExcludedClasses, firstRunProblem(run))
		}
		return
	}
	if s.Total != 1 || s.Passed != 0 || s.CompileErrors != 1 {
		t.Fatalf("summary=%+v problem=%s", s, firstRunProblem(run))
	}
	probeExcluded := false
	for _, entry := range run.ExcludedClasses {
		if entry.ClassName == "ReceiverProbe" && slices.Contains(probeErrors, entry.Message) {
			probeExcluded = true
		}
	}
	if !probeExcluded {
		t.Errorf("excluded=%+v, native ReceiverProbe errors %q", run.ExcludedClasses, probeErrors)
	}
	if runnerProblem != "" && firstRunProblem(run) != runnerProblem {
		t.Errorf("problem=%q, native %q", firstRunProblem(run), runnerProblem)
	}
}

// Every captured case in receiver_member_opaque_package/org.tsv, through the
// project runner: each declaration position of an absent namespace's nested
// type plus its static member call, the static call alone, the same-spelled
// local class twins, and the accept and reject controls.
func TestReceiverMemberOpaquePackageNativeRows(t *testing.T) {
	rows := receiverMemberOpaqueRows(t)
	type key struct{ Case, API string }
	seen := map[key]receiverMemberOpaqueRow{}
	var order []key
	for _, row := range rows {
		k := key{row.Case, row.API}
		if _, ok := seen[k]; !ok {
			seen[k] = row
			order = append(order, k)
		}
	}
	if len(order) != 32 {
		t.Fatalf("native cases=%d", len(order))
	}
	for _, k := range order {
		row := seen[k]
		t.Run(k.API+"/"+k.Case, func(t *testing.T) {
			dir := filepath.Join(receiverMemberOpaqueDir, row.Source)
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			classes := map[string]string{}
			for _, entry := range entries {
				data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				classes[strings.TrimSuffix(entry.Name(), ".cls")] = string(data)
			}
			testClass := "ReceiverRunnerTest"
			if row.Route == "istest-checkonly-runspecified" {
				testClass = "ReceiverProbe"
			}
			assertReceiverMemberOpaqueNative(t, rows, k.Case, k.API, classes, testClass)
		})
	}
}
