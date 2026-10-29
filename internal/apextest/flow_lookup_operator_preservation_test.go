package apextest

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// This synthetic matrix preserves the release dispatcher, including its
// unsupported-criteria boundary. It makes no uncaptured Salesforce claims.
func TestFlowLookupUncapturedOperatorProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			files := map[string]string{}
			methods := map[string]string{}
			failures := map[string]string{}
			for _, operator := range []string{"WasSet", "In", "NotIn", "GreaterThan", "GreaterThanOrEqualTo", "LessThan", "LessThanOrEqualTo", "HasError", "IsChanged", "EqualTo"} {
				name := "LookupOperator_" + operator
				operand := "<stringValue>Owned operator seed</stringValue>"
				if operator == "WasSet" || operator == "HasError" || operator == "IsChanged" {
					operand = "<booleanValue>true</booleanValue>"
				}
				nodes := `<recordLookups><name>Find</name><object>Account</object><filterLogic>and</filterLogic><filters><field>Name</field><operator>EqualTo</operator><value><stringValue>Owned operator seed</stringValue></value></filters><getFirstRecordOnly>true</getFirstRecordOnly><storeOutputAutomatically>true</storeOutputAutomatically><connector><targetReference>Choose</targetReference></connector></recordLookups><decisions><name>Choose</name><defaultConnector><targetReference>Miss</targetReference></defaultConnector><rules><name>Match</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>Find.Name</leftValueReference><operator>` + operator + `</operator><rightValue>` + operand + `</rightValue></conditions><connector><targetReference>Hit</targetReference></connector></rules></decisions>`
				for _, result := range []string{"Hit", "Miss"} {
					nodes += `<assignments><name>` + result + `</name><assignmentItems><assignToReference>outcome</assignToReference><operator>Assign</operator><value><stringValue>` + result + `</stringValue></value></assignmentItems></assignments>`
				}
				files["force-app/main/default/flows/"+name+".flow-meta.xml"] = flowPreservationXML(api, "Find", nodes, flowPreservationVariable("outcome", "String", false, true, false))
				body := `insert new Account(Name='Owned operator seed'); Flow.Interview flow=Flow.Interview.createInterview('` + name + `',new Map<String,Object>());`
				switch operator {
				case "WasSet", "HasError":
					body += `flow.start(); System.assertEquals('Miss',flow.getVariableValue('outcome'),'preserve release lookup-field ` + operator + ` dispatch');`
				case "EqualTo":
					body += `flow.start(); System.assertEquals('Hit',flow.getVariableValue('outcome'));`
				default:
					// The local unsupported boundary is a host runtime error, not
					// an Apex exception. Assert it at each route's Go adapter.
					body += `flow.start();`
					failures["preserve"+operator] = "dml: flow decision branch Match has unsupported criteria"
				}
				methods["preserve"+operator] = body
			}
			runFlowLookupOperatorProjectRoutes(t, api, files, methods, failures)
		})
	}
}

func runFlowLookupOperatorProjectRoutes(t *testing.T, api string, files, methods, failures map[string]string) {
	t.Helper()
	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)
	var source strings.Builder
	for _, name := range names {
		fmt.Fprintf(&source, "@IsTest static void %s(){%s}\n", name, methods[name])
	}
	index := flowCoreIndex(t, api, map[string]string{"LookupOperatorProbe": "@IsTest public class LookupOperatorProbe {" + source.String() + "}"}, files)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	t.Run("runner", func(t *testing.T) {
		run := Run(index, Options{SelectedClasses: []string{"LookupOperatorProbe"}, NoDiskCache: true, Parallelism: 1})
		if run.Summary().Total != len(methods) || len(run.Suites) != 1 {
			t.Fatalf("route selection: %+v", run.Summary())
		}
		for _, result := range run.Suites[0].Cases {
			t.Run(result.MethodName, func(t *testing.T) {
				if want := failures[result.MethodName]; want != "" {
					if result.Status != "fail" || result.Problem == nil || result.Problem.Type != "RuntimeError" || result.Problem.Message != want {
						t.Fatalf("want release boundary %q; got %s: %+v", want, result.Status, result.Problem)
					}
				} else if result.Status != "pass" {
					t.Fatalf("%s: %+v", result.Status, result.Problem)
				}
			})
		}
	})
	t.Run("anonymous", func(t *testing.T) {
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				if analysis := sema.AnalyzeAnonymous(index, methods[name], api); analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(methods[name], vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = runner.newMachine().Execute(program)
				if want := failures[name]; want != "" {
					if err == nil || err.Error() != want {
						t.Fatalf("want release boundary %q; got %v", want, err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			})
		}
	})
}
