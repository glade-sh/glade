package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// A flow action whose Apex class is absent from a declared but unloaded
// dependency is a dependency_missing failure. Without a declared dependency,
// or with the class present but lacking an invocable method, the original
// message stays.
func TestFlowActionAbsentClassDependencyMissing(t *testing.T) {
	flow := `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><actionCalls><name>Act</name><actionName>DependencyAction</actionName><actionType>apex</actionType></actionCalls><processType>AutoLaunchedFlow</processType><start><connector><targetReference>Act</targetReference></connector></start><status>Active</status></Flow>`
	testClass := `@IsTest private class DependencyProbeTest { @IsTest static void run() { Flow.Interview.createInterview('DependencyProbe', new Map<String,Object>()).start(); } }`
	cases := []struct {
		name      string
		declared  bool
		class     string
		wantType  string
		wantInMsg string
	}{
		{"declared_unloaded", true, "", "dependency_missing", "DependencyAction"},
		{"undeclared", false, "", "", "no static @InvocableMethod found"},
		{"present_without_invocable", true, "public class DependencyAction { public static void apply() {} }", "", "no static @InvocableMethod found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			deps := ""
			if tc.declared {
				deps = `,"dependencies":[{"package":"cedar-library"}]`
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true`+deps+`}]}`)
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "flows/DependencyProbe.flow-meta.xml"), flow)
			if tc.class != "" {
				writeFile(t, filepath.Join(base, "classes/DependencyAction.cls"), tc.class)
				writeFile(t, filepath.Join(base, "classes/DependencyAction.cls-meta.xml"), `<ApexClass><apiVersion>67.0</apiVersion></ApexClass>`)
			}
			writeFile(t, filepath.Join(base, "classes/DependencyProbeTest.cls"), testClass)
			writeFile(t, filepath.Join(base, "classes/DependencyProbeTest.cls-meta.xml"), `<ApexClass><apiVersion>67.0</apiVersion></ApexClass>`)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"DependencyProbeTest"}, NoDiskCache: true, Parallelism: 1})
			var problem *testreport.Problem
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					problem = c.Problem
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
