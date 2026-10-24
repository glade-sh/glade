package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const scalarTextFixture = "../dml/testdata/conformance/flow_text_prefix_suffix"

// The plain trigger is the captured project, with no prelude or subflow before
// its scalar decision. Other entries are local route twins of that project.
func TestFlowTextPrefixSuffixNativeConformanceRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		want := scalarTextNativeRows(t, api)
		for _, route := range []string{"trigger_plain", "trigger_prelude", "trigger_subflow", "interview", "interview_subflow", "invocable", "invocable_subflow"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := scalarTextProject(t, api, route)
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native project accepted; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				cases := Discover(index, Options{SelectedClasses: []string{"ScalarTextProbeTest"}})
				if len(cases) != 14 {
					t.Fatalf("named cases=%d, want 14", len(cases))
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				for _, context := range []string{"anonymous", "istest"} {
					t.Run(context, func(t *testing.T) {
						for _, tc := range cases {
							if route != "trigger_plain" && strings.HasPrefix(tc.MethodName, "direct_") {
								continue
							}
							t.Run(tc.MethodName, func(t *testing.T) {
								key := testCaseKey(tc)
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
								}
								machine := runner.newMachine()
								var payload string
								if context == "istest" {
									machine.EnableTestContext()
									if err := machine.RegisterMethod(methods[key]); err != nil {
										t.Fatal(err)
									}
									_, err := machine.ExecuteTestInClass(programs[key], tc.ClassName)
									var runtimeErr *vm.RuntimeError
									if !errors.As(err, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
										t.Fatalf("native=%v; runtime before observation: %v", want[context+"/"+tc.MethodName], err)
									}
									_, payload, _ = strings.Cut(runtimeErr.ExceptionMessage(), "OBS|")
									if payload == "" {
										t.Fatalf("native=%v; named route failed before observation: %v", want[context+"/"+tc.MethodName], err)
									}
								} else {
									program, err := vm.CompileAnonymousWithOptions("System.debug(LoggingLevel.ERROR,'OBS|'+JSON.serialize(ScalarTextProbe."+tc.MethodName+"()));", vm.CompileOptions{APIVersion: api})
									if err != nil {
										t.Fatal(err)
									}
									result, err := machine.Execute(program)
									if err != nil {
										t.Fatalf("native=%v; Glade runtime: %v", want[context+"/"+tc.MethodName], err)
									}
									for _, line := range result.Debug {
										if value, ok := strings.CutPrefix(line, "OBS|"); ok {
											payload = value
										}
									}
								}
								var actual bool
								if err := json.Unmarshal([]byte(payload), &actual); err != nil {
									t.Fatalf("observation %q: %v", payload, err)
								}
								expected, ok := want[context+"/"+tc.MethodName]
								if !ok {
									t.Fatal("missing native row")
								}
								if actual != expected {
									t.Fatalf("native matched=%t; Glade matched=%t", expected, actual)
								}
							})
						}
					})
				}
			})
		}
	}
}

func scalarTextNativeRows(t *testing.T, api string) map[string]bool {
	t.Helper()
	read := func(path string) string {
		body, err := os.ReadFile(filepath.Join(scalarTextFixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	directory := "api" + strings.TrimSuffix(api, ".0")
	capture := read(directory + "/RUN-FACTS.txt")
	for _, fact := range []string{"api=" + api, "native_complete=true", "deployment_success=true", "anonymous_rows=14", "istest_rows=14", "istest_runtime_failures_before_observation=0"} {
		if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
			t.Fatalf("missing native fact %s", fact)
		}
	}
	if !strings.Contains(capture, fmt.Sprintf("anonymous_sha256=%x\n", sha256.Sum256([]byte(read("anonymous.apex"))))) {
		t.Fatal("anonymous source changed")
	}
	rows := strings.Split(strings.TrimSpace(read(directory+"/org.tsv")), "\n")
	if len(rows) != 29 || rows[0] != "context\tid\tobservation" {
		t.Fatal("incomplete native matrix")
	}
	result := map[string]bool{}
	for _, line := range rows[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatal("invalid native row")
		}
		key := fields[0] + "/" + fields[1]
		if _, ok := result[key]; ok {
			t.Fatal("duplicate native row")
		}
		var value bool
		if err := json.Unmarshal([]byte(fields[2]), &value); err != nil {
			t.Fatal(err)
		}
		result[key] = value
	}
	// The earlier direct interview captures already pin case-insensitive matches.
	prior, err := os.ReadFile(filepath.Join("../dml/testdata/conformance/flow_text_email", directory, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, context := range []string{"anonymous", "istest"} {
		for _, pair := range [][2]string{{"FS", "startswith_case"}, {"FF", "endswith_case"}} {
			if !strings.Contains(string(prior), context+"\tdirect_"+pair[0]+"\t{\"matched\":true}") || !result[context+"/"+pair[1]] {
				t.Fatal("prefix/suffix observation disagrees with prior oracle")
			}
		}
	}
	return result
}

func scalarTextProject(t *testing.T, api, route string) string {
	t.Helper()
	root := t.TempDir()
	capture, err := os.ReadFile(filepath.Join(scalarTextFixture, "api"+strings.TrimSuffix(api, ".0"), "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(scalarTextFixture, "project")
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := strings.ReplaceAll(string(body), "62.0", api)
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), sha256.Sum256([]byte(text)))) {
			t.Fatalf("native source changed: %s", rel)
		}
		if strings.HasSuffix(path, ".flow-meta.xml") && !strings.Contains(filepath.Base(path), "_direct_") && route != "trigger_plain" {
			start, end := strings.Index(text, "<start>"), strings.Index(text, "</start>")+len("</start>")
			originalStart := text[start:end]
			text = text[:start] + `<start><locationX>0</locationX><locationY>0</locationY><connector><targetReference>Choose</targetReference></connector></start>` + text[end:]
			name := strings.TrimSuffix(filepath.Base(path), ".flow-meta.xml")
			if strings.HasSuffix(route, "_subflow") || route == "trigger_prelude" {
				childName := name + "Child"
				wrapperStart := `<start><connector><targetReference>Call</targetReference></connector></start>`
				if strings.HasPrefix(route, "trigger") {
					wrapperStart = strings.ReplaceAll(originalStart, ">Choose<", ">Call<")
				}
				wrapperNodes := `<subflows><name>Call</name><flowName>` + childName + `</flowName></subflows>`
				if route == "trigger_prelude" {
					// An inert child precedes the original decision, preserving the old route.
					child := `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status><start><connector><targetReference>Prepare</targetReference></connector></start><assignments><name>Prepare</name><assignmentItems><assignToReference>Marker</assignToReference><operator>Assign</operator><value><stringValue>ready</stringValue></value></assignmentItems></assignments><variables><name>Marker</name><dataType>String</dataType><isCollection>false</isCollection></variables></Flow>`
					writeFile(t, filepath.Join(root, filepath.Dir(rel), childName+".flow-meta.xml"), child)
					text = strings.Replace(text, `<start><locationX>0</locationX><locationY>0</locationY><connector><targetReference>Choose</targetReference></connector></start>`, wrapperStart, 1)
					text = strings.Replace(text, "</Flow>", strings.Replace(wrapperNodes, "</flowName>", "</flowName><connector><targetReference>Choose</targetReference></connector>", 1)+"</Flow>", 1)
				} else {
					writeFile(t, filepath.Join(root, filepath.Dir(rel), childName+".flow-meta.xml"), text)
					// Preserve a scalar input and forward it just like the captured
					// interview-to-child fixture, so Invocable.Action enters the child.
					variableStart := strings.Index(text, "<variables>")
					variableEnd := strings.Index(text, "</variables>") + len("</variables>")
					variables := text[variableStart:variableEnd]
					wrapperNodes = strings.Replace(wrapperNodes, "</flowName>", `</flowName><inputAssignments><name>InputText</name><value><elementReference>InputText</elementReference></value></inputAssignments>`, 1)
					text = `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status>` + wrapperStart + wrapperNodes + variables + `</Flow>`
				}
			}
		}
		if filepath.Base(path) == "ScalarTextProbe.cls" && !strings.HasPrefix(route, "trigger") {
			for _, line := range strings.Split(text, "\n") {
				_, after, found := strings.Cut(line, "insert new Account(Name='")
				if !found {
					continue
				}
				name, _, _ := strings.Cut(after, "');")
				call := `Flow.Interview interview=Flow.Interview.createInterview('` + name + `',new Map<String,Object>()); interview.start();`
				if strings.HasPrefix(route, "invocable") {
					call = `Invocable.Action action=Invocable.Action.createCustomAction('flow','` + name + `'); action.setInvocations(new List<Map<String,Object>>{new Map<String,Object>()}); Invocable.Action.Result result=action.invoke()[0]; System.assertEquals(true,result.isSuccess(),JSON.serialize(result.getErrors()));`
				}
				text = strings.Replace(text, "insert new Account(Name='"+name+"');", call, 1)
			}
		}
		writeFile(t, filepath.Join(root, rel), text)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}
