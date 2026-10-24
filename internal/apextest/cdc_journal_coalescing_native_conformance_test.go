package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The source is the CDC-only slice of the captured multi-family probe. The
// original source and its native hash remain beside the unchanged native rows.
func TestChangeDataCaptureCoalescingNativeProjectAndVMAPIRoutes(t *testing.T) {
	const fixture = "testdata/conformance/cdc_journal_delivery/extended"
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			receipt, err := os.ReadFile(filepath.Join(fixture, "cdc/native/api"+api+"/istest/RUN-FACTS.txt"))
			if err != nil {
				t.Fatal(err)
			}
			for _, token := range []string{"native_complete=true", "deployment_success=true"} {
				if !strings.Contains(string(receipt), token) {
					t.Fatalf("missing capture fact %s", token)
				}
			}
			sourceRoot := filepath.Join(fixture, "project")
			err = filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				// The retained project is the floor deployment. Reconstruct the
				// ceiling metadata and verify it against that capture's hashes.
				body = []byte(strings.ReplaceAll(string(body), "<apiVersion>62.0</apiVersion>", "<apiVersion>"+api+".0</apiVersion>"))
				rel, err := filepath.Rel(sourceRoot, path)
				if err != nil {
					return err
				}
				if !strings.Contains(string(receipt), fmt.Sprintf("source_sha256:project/%s=%x", filepath.ToSlash(rel), sha256.Sum256(body))) {
					t.Fatalf("capture source changed: %s", rel)
				}
				if strings.HasSuffix(rel, "PathObservation.cls") {
					s := string(body)
					helpers := s[strings.Index(s, " public static void cdcLog("):strings.Index(s, " public static Map<String,Object> observe(")]
					start := strings.Index(s, "    if(key=='CDC03')")
					end := strings.Index(s[start:], "   } else if(key.startsWith('DR'))") + start
					body = []byte("public class PathObservation {\n" + helpers + " public static Map<String,Object> observe(String key) { Map<String,Object> out=new Map<String,Object>{'exception'=>null}; try {\n" + s[start:end] + " } catch(Exception failure) {out.put('exception',failure.getTypeName());} return out; } public static void emit(String key) { System.assert(false,'PATH_OBS|'+key+'|'+JSON.serialize(observe(key))); } }")
				}
				writeFile(t, filepath.Join(root, rel), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`.0"}`)
			meta := `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `.0</apiVersion><status>Active</status></ApexClass>`
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/PathObservation.cls-meta.xml"), meta)
			methods := []string{"CDC01", "CDC03", "CDC04", "CDC05", "CDC06", "CDC07", "CDC08", "CDC09", "CDC10", "CDC11", "CDC12", "CDC13", "CDC14", "CDC15"}
			body := "@IsTest private class JournalObservationTest {\n"
			for _, method := range methods {
				body += "@IsTest static void " + method + "() { PathObservation.emit('" + method + "'); }\n"
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/JournalObservationTest.cls"), body+"}")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/JournalObservationTest.cls-meta.xml"), meta)
			methods = append(methods, "CDC17")
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			raw, err := os.ReadFile(filepath.Join(fixture, "cdc/native/api"+api+"/istest/org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := map[string]string{}
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				cells := strings.SplitN(line, "\t", 5)
				if len(cells) != 5 {
					t.Fatal(line)
				}
				rows[cells[0]] = cells[4]
			}
			anonymousRaw, err := os.ReadFile(filepath.Join(fixture, "cdc/native/api"+api+"/anonymous/org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			anonymousRows := map[string]string{}
			for _, line := range strings.Split(strings.TrimSpace(string(anonymousRaw)), "\n")[1:] {
				cells := strings.SplitN(line, "\t", 5)
				if len(cells) != 5 {
					t.Fatal(line)
				}
				anonymousRows[cells[0]] = cells[4]
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			triggers, triggerErrors := compileProjectTriggers(index)
			if len(triggerErrors) != 0 {
				t.Fatalf("compile native subscribers: %v", triggerErrors)
			}
			for _, trigger := range triggers {
				if err := runner.base.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
			}
			for _, method := range methods {
				t.Run(method, func(t *testing.T) {
					check := func(t *testing.T, message, expected string) {
						t.Helper()
						_, payload, ok := strings.Cut(message, "PATH_OBS|"+method+"|")
						var want, got any
						if !ok || json.Unmarshal([]byte(expected), &want) != nil || json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatalf("missing observation: %s", message)
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("native=%s; Glade=%s", expected, payload)
						}
					}
					t.Run("direct_vm_istest", func(t *testing.T) {
						machine := runner.newMachine()
						machine.EnableTestContext()
						if method == "CDC17" {
							setup, err := vm.CompileAnonymousWithOptions("insert new Account(Name='oracle-setup');", vm.CompileOptions{APIVersion: api + ".0"})
							if err != nil {
								t.Fatal(err)
							}
							if _, err := machine.Execute(setup); err != nil {
								t.Fatal(err)
							}
						}
						program, err := vm.CompileAnonymousWithOptions("PathObservation.emit('"+method+"');", vm.CompileOptions{APIVersion: api + ".0"})
						if err != nil {
							t.Fatal(err)
						}
						_, err = machine.Execute(program)
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("observation missing: %v", err)
						}
						check(t, fault.ExceptionMessage(), rows[method])
					})
					if method == "CDC03" || method == "CDC17" {
						t.Run("anonymous", func(t *testing.T) {
							program, err := vm.CompileAnonymousWithOptions("PathObservation.emit('"+method+"');", vm.CompileOptions{APIVersion: api + ".0"})
							if err != nil {
								t.Fatal(err)
							}
							_, err = runner.newMachine().Execute(program)
							var fault *vm.RuntimeError
							if !errors.As(err, &fault) {
								t.Fatal(err)
							}
							check(t, fault.ExceptionMessage(), anonymousRows[method])
						})
					}
					for _, route := range []struct {
						name        string
						parallelism int
						cached      bool
					}{
						{"project_istest", 1, false}, {"cached_parallel_project_istest", 2, true},
					} {
						t.Run(route.name, func(t *testing.T) {
							className := "JournalObservationTest"
							if method == "CDC17" {
								className = "PathSetupObservationTest"
							}
							opts := Options{SelectedClasses: []string{className}, SelectedMethod: method, NoDiskCache: !route.cached, Parallelism: route.parallelism}
							wantCases := 1
							// CDC17: the capture has one setup method and one test method
							// (extended/cdc/native/api*/istest/org.tsv:16). Multi-method and
							// multi-class setup replay has no native row (CAPTURE-NEEDED G1).
							runs := 1
							if route.cached {
								t.Cleanup(EnableDiskCacheForTesting())
								t.Cleanup(InvalidateRuntimeCaches)
								opts.PerfCounters = true
								runs = 2
							}
							for run := 0; run < runs; run++ {
								if route.cached {
									InvalidateRuntimeCaches()
								}
								report := Run(index, opts)
								if route.cached && run > 0 && SnapshotPerfCounters().Phases.DiskCacheHits == 0 {
									t.Fatal("warm project route did not load the runtime from disk")
								}
								seen := 0
								for _, suite := range report.Suites {
									for _, result := range suite.Cases {
										if result.Problem == nil {
											t.Fatalf("missing observation: %+v", result)
										}
										check(t, result.Problem.Message, rows[method])
										seen++
									}
								}
								if seen != wantCases {
									t.Fatalf("observed %d cases, want %d", seen, wantCases)
								}
							}
						})
					}
				})
			}
		})
	}
}
