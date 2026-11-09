package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// A flow action whose Apex class is present but excluded by the
// dependency-scoped compile gate, because it references a type from a declared
// but unloaded dependency, fails as dependency_missing with the class's own
// unresolved reference. The same class without the unresolved reference runs:
// a static void @InvocableMethod taking List<Inner> in a non-default package
// folder, with and without a project namespace. A class invalid for a reason
// that is not an unloaded dependency keeps the original message.
func TestFlowActionInvalidClassDependencyMissing(t *testing.T) {
	flow := `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><actionCalls><name>Act</name><actionName>CedarTravelAction</actionName><actionType>apex</actionType><inputParameters><name>label</name><value><stringValue>x</stringValue></value></inputParameters></actionCalls><processType>AutoLaunchedFlow</processType><start><connector><targetReference>Act</targetReference></connector></start><status>Active</status></Flow>`
	testClass := `@IsTest private class CedarFlowProbeTest { @IsTest static void run() { Flow.Interview.createInterview('CedarFlowProbe', new Map<String,Object>()).start(); System.assertEquals(1, CedarFlowSink.calls); } }`
	sink := `public class CedarFlowSink { public static Integer calls = 0; }`
	action := func(body string) string {
		return `public class CedarTravelAction {
    public class Request { @InvocableVariable public String label; }
    @InvocableMethod(label='Cedar travel' description='synthetic')
    public static void apply(List<Request> requests) {
        ` + body + `
        CedarFlowSink.calls += requests.size();
    }
}`
	}
	cases := []struct {
		name      string
		namespace string
		declared  bool
		body      string
		wantType  string
		wantInMsg string
		wantPass  bool
	}{
		{name: "valid", declared: true, body: "String marker = requests[0].label;", wantPass: true},
		{name: "valid_namespaced", namespace: "cedarns", declared: true, body: "String marker = requests[0].label;", wantPass: true},
		{name: "unloaded_reference", declared: true, body: "CedarMissingOps ops = new CedarMissingOps();", wantType: "dependency_missing", wantInMsg: `"CedarMissingOps"`},
		{name: "unloaded_reference_namespaced", namespace: "cedarns", declared: true, body: "CedarMissingOps ops = new CedarMissingOps();", wantType: "dependency_missing", wantInMsg: `"CedarMissingOps"`},
		{name: "undeclared_reference", declared: false, body: "CedarMissingOps ops = new CedarMissingOps();", wantInMsg: "no static @InvocableMethod found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			deps := ""
			if tc.declared {
				deps = `,"dependencies":[{"package":"cedar-library"}]`
			}
			ns := ""
			if tc.namespace != "" {
				ns = `"namespace":"` + tc.namespace + `",`
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{`+ns+`"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true`+deps+`}]}`)
			base := filepath.Join(root, "force-app/main/default")
			feature := filepath.Join(root, "force-app/cedarTravel/classes")
			meta := `<ApexClass><apiVersion>67.0</apiVersion></ApexClass>`
			writeFile(t, filepath.Join(base, "flows/CedarFlowProbe.flow-meta.xml"), flow)
			writeFile(t, filepath.Join(feature, "CedarTravelAction.cls"), action(tc.body))
			writeFile(t, filepath.Join(feature, "CedarTravelAction.cls-meta.xml"), meta)
			writeFile(t, filepath.Join(base, "classes/CedarFlowSink.cls"), sink)
			writeFile(t, filepath.Join(base, "classes/CedarFlowSink.cls-meta.xml"), meta)
			writeFile(t, filepath.Join(base, "classes/CedarFlowProbeTest.cls"), testClass)
			writeFile(t, filepath.Join(base, "classes/CedarFlowProbeTest.cls-meta.xml"), meta)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"CedarFlowProbeTest"}, NoDiskCache: true, Parallelism: 1})
			if tc.wantPass {
				if run.Summary().Passed != 1 {
					t.Fatalf("valid action failed: %s", firstRunProblem(run))
				}
				return
			}
			var problem *testreport.Problem
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					if c.Problem != nil {
						problem = c.Problem
					}
				}
			}
			if problem == nil {
				t.Fatalf("expected a failure: %+v", run.Summary())
			}
			if tc.wantType != "" && problem.Type != tc.wantType {
				t.Fatalf("type %q, message %q", problem.Type, problem.Message)
			}
			if tc.wantType == "" && problem.Type == "dependency_missing" {
				t.Fatalf("unexpected dependency_missing: %q", problem.Message)
			}
			if !strings.Contains(problem.Message, tc.wantInMsg) {
				t.Fatalf("message %q lacks %q", problem.Message, tc.wantInMsg)
			}
		})
	}
}
