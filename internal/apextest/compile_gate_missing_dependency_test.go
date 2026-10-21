package apextest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// Local resolver-policy tests. Native the captured rows bound static
// absence against dynamic/string lookups; they do not define dependency_missing.
func TestDependencyGateMissingDependencyPolicy(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"AbsentPkg"}]}]}`, api))
			for name, source := range map[string]string{
				"StaticUse":       `public class StaticUse { public static Object value() { return AbsentPkg.Tool.value(); } }`,
				"UseTest":         `@IsTest private class UseTest { @IsTest static void observe(){ StaticUse.value(); } }`,
				"IndependentTest": `@IsTest private class IndependentTest { @IsTest static void observe(){System.assertEquals(7,7);} }`,
				"ByNameTest":      `@IsTest private class ByNameTest { @IsTest static void observe(){Type t=Type.forName('AbsentPkg.Tool');System.assertEquals('AbsentPkg.Tool','AbsentPkg.Tool');} }`,
				"ShadowTest":      `@IsTest private class ShadowTest { @IsTest static void observe(){String AbsentPkg='boundary';System.assertEquals(8,AbsentPkg.length());} }`,
			} {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Namespace != "AbsentPkg" || p.DependencyDiagnostics[0].Code != "dependency_missing" {
				t.Fatalf("structured dependency records: %#v", p.DependencyDiagnostics)
			}
			index := typesys.Build(p, gladeschema.Schema{})
			run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true}, nil)
			statuses := map[string]testreport.Status{}
			for _, suite := range run.Suites {
				for _, testCase := range suite.Cases {
					statuses[testCase.ClassName] = testCase.Status
				}
			}
			if len(statuses) != 4 || statuses["UseTest"] != testreport.StatusCompileError || statuses["IndependentTest"] != testreport.StatusPass || statuses["ByNameTest"] != testreport.StatusPass || statuses["ShadowTest"] != testreport.StatusPass || run.Summary().OK() {
				t.Fatalf("scope: %#v statuses=%v", run, statuses)
			}
			var excluded []string
			for _, entry := range run.ExcludedClasses {
				excluded = append(excluded, entry.ClassName)
			}
			if strings.Join(excluded, ",") != "StaticUse,UseTest" {
				t.Fatalf("exclusions=%v", excluded)
			}
		})
	}
}

func TestDependencyGateMissingDependencyAttributionBoundaries(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		source string
		want   bool
	}{
		{`public class Boundary { AbsentPkg.Tool field; }`, true},
		{`public class Boundary { AbsentPkg__Record__c field; }`, true},
		{`public class Boundary { Object x=AbsentPkg.Tool.value(); }`, true},
		{`public class Boundary { String x='AbsentPkg.Tool'; /* AbsentPkg.Tool */ }`, false},
		{`public class Boundary { Object x=Type.forName('AbsentPkg.Tool'); }`, false},
		{`public class Boundary { void x(String AbsentPkg){AbsentPkg.length();} }`, false},
		{`public class Boundary { void x(){Object AbsentPkg__Record__c=null;System.debug(AbsentPkg__Record__c);} }`, false},
		{`public class Boundary { OtherPkg.Tool field; }`, false},
	}
	for i, cell := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			path := filepath.Join(root, "Boundary.cls")
			writeFile(t, path, cell.source)
			index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
			got := compileGateReferencesMissingNamespace(path, cell.source, newCompileGateUnits(index), []string{"AbsentPkg"})
			if got != cell.want {
				t.Fatalf("bound reference=%v want=%v", got, cell.want)
			}
		})
	}
	// Unknown file-less errors retain the whole-project fallback. In particular,
	// schema relationship diagnostics need their own admitted capture shape.
	gate := &compileGate{}
	for _, code := range []string{"dependency_missing", "GLADESEMA003", "other"} {
		_, ok := gate.attributeDependencyDiagnostics(newCompileGateUnits(typesys.Index{}), []diagnostic.Diagnostic{{Severity: diagnostic.Error, Code: code, Message: "unattributed"}}, newSourceCache())
		if ok {
			t.Fatalf("unattributed %s was swallowed", code)
		}
	}
}
