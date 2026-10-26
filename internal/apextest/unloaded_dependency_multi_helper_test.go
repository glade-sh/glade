package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

func multiHelperProject(t *testing.T, api string, count int, loaded bool) (string, string) {
	t.Helper()
	dir := filepath.Join("testdata/conformance/unloaded_dependency/multi_helper")
	suffix := fmt.Sprintf("%s%d", api[:2], count)
	root := filepath.Join(t.TempDir(), "consumer")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"cedar-library"}]}]}`)
	files, err := os.ReadDir(filepath.Join(dir, "sources", fmt.Sprintf("%d_helpers", count)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		name := strings.ReplaceAll(file.Name(), "SUFFIX", suffix)
		body, err := os.ReadFile(filepath.Join(dir, "sources", fmt.Sprintf("%d_helpers", count), file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		destination := root
		if strings.HasPrefix(name, "CedarLibrary") {
			if !loaded {
				continue
			}
			destination = filepath.Join(filepath.Dir(root), "supplier")
			writeFile(t, filepath.Join(destination, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"cedar-library"}]}`)
		}
		kind, sub := "ApexClass", "classes"
		if strings.HasSuffix(name, ".trigger") {
			kind, sub = "ApexTrigger", "triggers"
		}
		path := filepath.Join(destination, "force-app/main/default", sub, name)
		writeFile(t, path, strings.ReplaceAll(string(body), "SUFFIX", suffix))
		writeFile(t, path+"-meta.xml", "<"+kind+"><apiVersion>"+api+"</apiVersion><status>Active</status></"+kind+">")
	}
	return root, suffix
}

func TestUnloadedDependencyMultiHelperNamedTestRunnerLSPNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		f, err := os.Open(filepath.Join("testdata/conformance/unloaded_dependency/multi_helper", "api"+api[:2], "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(f)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		f.Close()
		if err != nil || len(rows) != 9 {
			t.Fatalf("native matrix: rows=%d error=%v", len(rows), err)
		}
		native := map[string][]string{}
		for _, row := range rows[1:] {
			if len(row) != 5 || row[1] != api || row[4] == "" {
				t.Fatalf("invalid native row: %v", row)
			}
			native[row[0]+"/"+row[2]] = row
		}
		for _, count := range []int{2, 3} {
			for _, loaded := range []bool{false, true} {
				state, outcome := "absent", "compile-rejected"
				if loaded {
					state, outcome = "loaded", "accepted"
				}
				t.Run(fmt.Sprintf("%s/%d_helpers_%s/named_test_runner_lsp", api, count, state), func(t *testing.T) {
					for _, route := range []string{"anonymous", "istest"} {
						if row := native[fmt.Sprintf("%d_helpers_%s/%s", count, state, route)]; len(row) != 5 || row[3] != outcome {
							t.Fatalf("native %s: %v", route, row)
						}
					}
					root, _ := multiHelperProject(t, api, count, loaded)
					body := "@IsTest private class CedarMultiObserve { "
					for _, letter := range "ABC"[:count] {
						source := fmt.Sprintf("Account row = new Account(Name='%c'); insert row; System.assertEquals('%c', [SELECT Description FROM Account WHERE Id = :row.Id].Description);", letter, letter)
						body += fmt.Sprintf("@IsTest static void observe%c() { %s } ", letter, source)
					}
					body += "@IsTest static void independent() { insert new Account(Name='none'); } }"
					path := filepath.Join(root, "force-app/main/default/classes/CedarMultiObserve.cls")
					writeFile(t, path, body)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
					seen := 0
					for _, suite := range run.Suites {
						for _, result := range suite.Cases {
							seen++
							if loaded || result.MethodName == "independent" {
								if result.Status != testreport.StatusPass {
									t.Errorf("loaded/boundary route: %+v problem=%+v", result, result.Problem)
								}
								continue
							}
							if result.Status != testreport.StatusRuntimeError || string(result.Reason) != "dependency-missing" || result.Problem == nil || result.Problem.Type != "dependency_missing" {
								t.Errorf("%s: want dependency_missing at reached library, got status=%s problem=%+v", result.MethodName, result.Status, result.Problem)
								continue
							}
							letter := strings.TrimPrefix(result.MethodName, "observe")
							if !strings.Contains(result.Problem.Message, "CedarLibrary"+letter) || !strings.Contains(result.Problem.Message, "cedar-library") || len(result.Problem.Stack) == 0 {
								t.Errorf("lost reached helper context: %+v", result.Problem)
							}
						}
					}
					if seen != count+1 {
						t.Fatalf("cases=%d want=%d", seen, count+1)
					}
					diagnostics := lsp.TestDiagnostics(run)
					if !loaded {
						for _, letter := range "ABC"[:count] {
							found := false
							for _, diag := range diagnostics {
								if strings.Contains(diag.Message, "dependency_missing") && strings.Contains(diag.Message, "CedarLibrary"+string(letter)) {
									found = true
								}
							}
							if !found {
								t.Errorf("LSP missing reached helper %c: %+v", letter, diagnostics)
							}
						}
					}
				})
			}
		}
	}
}

func TestUnloadedDependencyMultiHelperRestorationUnitRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, count := range []int{2, 3} {
			t.Run(fmt.Sprintf("%s/%d_helpers/restoration_unit", api, count), func(t *testing.T) {
				root, suffix := multiHelperProject(t, api, count, false)
				index := loadTestIndex(t, root)
				sources := newSourceCache()
				sources.expectedDigestLookup = index.SourceDigest
				gate := &compileGate{dependencyIndex: &index, dependencySources: sources, fileEntry: map[string]int{}, classEntry: map[string]int{}}
				for _, letter := range "ABC"[:count] {
					name := "CedarHelper" + string(letter) + suffix
					file := filepath.Join(root, "force-app/main/default/classes", name+".cls")
					gate.add(testreport.CompileExclusion{ClassName: name, File: file, Root: name, Message: fmt.Sprintf(`method "apply" calls unknown method "CedarLibrary%c%s.value"`, letter, suffix)})
				}
				trigger := index.Triggers[0]
				gate.add(testreport.CompileExclusion{ClassName: trigger.Name, File: trigger.File, Root: "CedarHelperA" + suffix, DependsOn: "CedarHelperA" + suffix})
				machine := vm.New(nil)
				restored, err := gate.restoreDependencyTriggers(machine)
				if err != nil || len(restored) != 1 {
					t.Fatalf("restore: triggers=%v error=%v", restored, err)
				}
				for _, letter := range "ABC"[:count] {
					program, err := vm.CompileAnonymousWithOptions(fmt.Sprintf("CedarHelper%c%s.apply(null);", letter, suffix), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(program)
					explained := ExplainDependencyError(index, "", err)
					if runtimeErr, ok := explained.(*vm.RuntimeError); !ok || runtimeErr.Type != "dependency_missing" || !strings.Contains(runtimeErr.Message, fmt.Sprintf("CedarLibrary%c%s", letter, suffix)) {
						t.Errorf("helper %c was not restored to its missing library: %v", letter, explained)
					}
				}
			})
		}
	}
}

func TestUnloadedDependencyUnreachedHelperRestorationUnitRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/unreached_helper/restoration_unit", func(t *testing.T) {
			root, suffix := multiHelperProject(t, api, 2, false)
			index := loadTestIndex(t, root)
			sources := newSourceCache()
			sources.expectedDigestLookup = index.SourceDigest
			gate := &compileGate{dependencyIndex: &index, dependencySources: sources, fileEntry: map[string]int{}, classEntry: map[string]int{}}
			name := "CedarHelperA" + suffix
			gate.add(testreport.CompileExclusion{ClassName: name, File: filepath.Join(root, "force-app/main/default/classes", name+".cls"), Root: name, Message: `method "apply" calls unknown method "CedarLibraryA` + suffix + `.value"`})
			// No trigger is selected for restoration: excluded helpers alone
			// must not install runtime definitions.
			machine := vm.New(nil)
			restored, err := gate.restoreDependencyTriggers(machine)
			if err != nil || len(restored) != 0 {
				t.Fatalf("unreached restoration: triggers=%v error=%v", restored, err)
			}
			program, err := vm.CompileAnonymousWithOptions(name+".apply(null);", vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), `unsupported call "`+name+`.apply"`) {
				t.Fatalf("unreached helper was registered: %v", err)
			}
		})
	}
}
