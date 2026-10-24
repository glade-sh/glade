package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

func TestEmailServicesMetadataNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/email_services"
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || strings.HasSuffix(path, ".cls-meta.xml") {
					return nil
				}
				rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), string(data))
				if strings.HasSuffix(path, ".cls") {
					writeFile(t, filepath.Join(root, rel)+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			index := loadTestIndex(t, root)
			for _, typ := range index.Types {
				if typ.HasSourceSnapshot() && typ.EffectiveAPIVersion != api {
					t.Fatalf("%s source API=%s, want %s", typ.Name, typ.EffectiveAPIVersion, api)
				}
			}
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org, err := orgFromIndexWithError(index)
			if err != nil {
				t.Fatal(err)
			}
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					native := "native/api" + strings.TrimSuffix(api, ".0") + "/" + route + "/"
					run := read(native + "RUN-FACTS.txt")
					for _, fact := range []string{"api=" + api, "context=" + route, "deployment_success=true", "native_complete=true"} {
						if !strings.Contains("\n"+run, "\n"+fact+"\n") {
							t.Fatalf("missing native fact %s", fact)
						}
					}
					for _, source := range []string{"classes/EmailServiceHandler.cls", "classes/EmailServiceProbe.cls", "classes/EmailServiceProbeTest.cls", "emailservices/MailReceiver.xml", "emailservices/MailDormant.xml"} {
						body := read("project/force-app/main/default/" + source)
						fact := fmt.Sprintf("force-app/main/default/%s_sha256=%x\n", source, sha256.Sum256([]byte(body)))
						if !strings.Contains(run, fact) {
							t.Fatalf("native capture detached from %s", source)
						}
					}
					want := map[string]string{}
					lines := strings.Split(strings.TrimSpace(read(native+"org.tsv")), "\n")
					if len(lines) < 10 || lines[0] != "id\troute\tvalue" {
						t.Fatal("incomplete native rows")
					}
					for _, line := range lines[1:] {
						cells := strings.SplitN(line, "\t", 3)
						if len(cells) != 3 || cells[1] != route {
							t.Fatalf("invalid row %s", line)
						}
						var value string
						if err := json.Unmarshal([]byte(cells[2]), &value); err != nil {
							t.Fatal(err)
						}
						want[cells[0]] = value
					}
					machine := base.newMachine()
					payload := ""
					if route == "anonymous" {
						source := read("anonymous.apex")
						analysis := sema.AnalyzeAnonymous(index, source, api)
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil || analysis.HasErrors() {
							t.Fatalf("anonymous compile: %v %v", err, analysis.Diagnostics)
						}
						result, err := machine.Execute(program)
						if err != nil || len(result.Debug) != 1 {
							t.Fatalf("anonymous: %v %v", err, result.Debug)
						}
						payload = strings.TrimPrefix(result.Debug[0], "PROBE|")
					} else {
						cases := Discover(index, Options{SelectedClasses: []string{"EmailServiceProbeTest"}, SelectedMethod: "observeNamed"})
						if len(cases) != 1 {
							t.Fatalf("named cases=%d", len(cases))
						}
						methods, methodErrors := compileTestMethodsForIndex(&index, cases)
						programs, programErrors := compileTestInvokePrograms(cases)
						key := testCaseKey(cases[0])
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteTestInClass(programs[key], cases[0].ClassName)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
							t.Fatalf("named transport: %v", err)
						}
						_, payload, _ = strings.Cut(runtimeErr.ExceptionMessage(), "PROBE|")
					}
					got := map[string]string{}
					if err := json.Unmarshal([]byte(payload), &got); err != nil {
						t.Fatalf("payload %q: %v", payload, err)
					}
					if len(got) != len(want) {
						t.Errorf("rows=%d, native=%d", len(got), len(want))
					}
					for id, value := range want {
						t.Run(id, func(t *testing.T) {
							if got[id] != value {
								t.Errorf("native %q; Glade %q", value, got[id])
							}
						})
					}
				})
			}
			t.Run("project_test_runner", func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"EmailServiceProbeTest"}, SelectedMethod: "projectRunnerActiveAndInactive", NoDiskCache: true, Parallelism: 1})
				count := 0
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						count++
						if c.Status != testreport.StatusPass {
							t.Errorf("%s: %s %s", c.MethodName, c.Status, fmt.Sprint(c))
						}
					}
				}
				if count != 1 {
					t.Fatalf("runner cases=%d", count)
				}
				transport := Run(index, Options{SelectedClasses: []string{"EmailServiceProbeTest"}, SelectedMethod: "observeNamed", NoDiskCache: true, Parallelism: 1})
				observed := 0
				for _, suite := range transport.Suites {
					for _, c := range suite.Cases {
						observed++
						if c.Status != testreport.StatusFail || c.Problem == nil {
							t.Fatalf("runner native transport: %#v", c)
						}
						_, payload, found := strings.Cut(c.Problem.Message, "PROBE|")
						if !found {
							t.Fatalf("missing runner payload: %s", c.Problem.Message)
						}
						got := map[string]string{}
						if err := json.Unmarshal([]byte(payload), &got); err != nil {
							t.Fatal(err)
						}
						rows := strings.Split(strings.TrimSpace(read("native/api"+strings.TrimSuffix(api, ".0")+"/istest/org.tsv")), "\n")
						if len(got) != len(rows)-1 {
							t.Errorf("runner rows=%d; native=%d", len(got), len(rows)-1)
						}
						for _, row := range rows[1:] {
							cells := strings.SplitN(row, "\t", 3)
							var want string
							if err := json.Unmarshal([]byte(cells[2]), &want); err != nil {
								t.Fatal(err)
							}
							if got[cells[0]] != want {
								t.Errorf("runner %s: native %q; Glade %q", cells[0], want, got[cells[0]])
							}
						}
					}
				}
				if observed != 1 {
					t.Fatalf("runner transport cases=%d", observed)
				}
			})
		})
	}
}

func TestEmailServicesMetadataNativeRejectedProjectRoutes(t *testing.T) {
	const fixture = "testdata/conformance/email_services"
	handler, err := os.ReadFile(filepath.Join(fixture, "project/force-app/main/default/classes/EmailServiceHandler.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, name := range []string{"UnknownUser", "UnknownClass", "TooLong"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				body, err := os.ReadFile(filepath.Join(fixture, "rejected/"+name+".xml"))
				if err != nil {
					t.Fatal(err)
				}
				index := flowRefsIndex(t, api, map[string]string{"EmailServiceHandler": string(handler)}, map[string]string{"force-app/main/default/emailservices/Rejected.xml": string(body)})
				if _, err := orgFromIndexWithError(index); err == nil {
					t.Error("runtime org accepted native-rejected email service metadata")
				}
			})
		}
	}
}
