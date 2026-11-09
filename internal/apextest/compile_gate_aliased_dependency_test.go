package apextest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// A declared SFDX package dependency with no source degrades per reference,
// whether or not the manifest lists the package in packageAliases (a 04t
// version id). Salesforce treats an absent referenced package as a failure of
// the code that references it, not of the whole project.
func TestDependencyGateAliasedAndUnaliasedMissingPackagesDegradeAlike(t *testing.T) {
	const aliases = `,"packageAliases":{"absent-data-model":"04t000000000000AAA","absent-toggle":"04t000000000001AAA"}`
	variants := map[string]string{"aliased": aliases, "unaliased": ""}
	got := map[string]map[string]testreport.Status{}
	for name, aliasJSON := range variants {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true,"package":"consumer","dependencies":[{"package":"absent-data-model","versionNumber":"1.0.0.LATEST"},{"package":"absent-toggle"}]}]%s}`, aliasJSON))
		for cls, source := range map[string]string{
			"NamedTest":       `@IsTest private class NamedTest { @IsTest static void observe(){ Object o = absent_data_model.Tool.value(); } }`,
			"UnnamespacedTst": `@IsTest private class UnnamespacedTst { @IsTest static void observe(){ Object o = new AbsentUtility(); } }`,
			"IndependentTest": `@IsTest private class IndependentTest { @IsTest static void observe(){ System.assertEquals(7, 7); } }`,
		} {
			path := filepath.Join(root, "force-app/main/default/classes", cls+".cls")
			writeFile(t, path, source)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
		}
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.DependencyDiagnostics) != 2 {
			t.Fatalf("%s: dependency records = %#v", name, p.DependencyDiagnostics)
		}
		for _, d := range p.DependencyDiagnostics {
			if d.Code != "dependency_missing" || d.Namespace == "" || !strings.Contains(d.Message, `"`+d.Namespace+`"`) {
				t.Fatalf("%s: diagnostic must name the package: %#v", name, d)
			}
		}
		run := RunCasesContext(context.Background(), typesys.Build(p, gladeschema.Schema{}), Options{NoDiskCache: true}, nil)
		statuses := map[string]testreport.Status{}
		for _, suite := range run.Suites {
			for _, c := range suite.Cases {
				statuses[c.ClassName] = c.Status
			}
		}
		if statuses["IndependentTest"] != testreport.StatusPass {
			t.Fatalf("%s: the independent class must still run: %v", name, statuses)
		}
		if statuses["UnnamespacedTst"] != testreport.StatusCompileError {
			t.Fatalf("%s: the referencing class must fail per reference: %v", name, statuses)
		}
		got[name] = statuses
	}
	for cls, status := range got["unaliased"] {
		if got["aliased"][cls] != status {
			t.Fatalf("aliased and unaliased differ for %s: %v vs %v", cls, got["aliased"][cls], status)
		}
	}
}
