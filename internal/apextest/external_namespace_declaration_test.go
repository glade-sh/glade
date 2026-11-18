package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// externalNamespaceProject loads a project like namespaceFamilyProject, with
// an optional glade.yml project.externalNamespaces declaration.
func externalNamespaceProject(t *testing.T, api, declared string, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	if declared != "" {
		writeFile(t, filepath.Join(root, "glade.yml"), "project:\n  externalNamespaces: ["+declared+"]\n")
	}
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
	for name, code := range classes {
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), code)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), meta)
	}
	return loadTestIndex(t, root)
}

// Ruling 49(1): glade.yml project.externalNamespaces declares an installed
// package whose source Glade does not have. The undeclared twin of every row
// is namespace_external_positions/org.tsv (TestNamespaceExternalPositionsNativeConformance):
// MissingNsV035 rejects in local, new, cast, instanceof, generic argument,
// field, parameter, return, extends, implements, nested, lowercase and static
// method/field receiver positions. Declared, the same sources compile with no
// error and no dependency_missing diagnostic; the System control still rejects.
// A static METHOD call into the declared package keeps the canonical opaque
// call rule that every source-less dependency has (v0.3.1, cases.md in
// testdata/conformance/undeclared_namespace_types): offline execution has no
// method body to run, so the calling unit is a compile exclusion and
// independent tests still run. Static field reads, such as an enum value of
// the package, compile.
func TestExternalNamespaceDeclarationOpensNamespace(t *testing.T) {
	rows := readNamespaceFamilyRows(t, namespaceExternalPositionsDir)
	for _, api := range []string{"62.0", "67.0"} {
		for _, declared := range []string{"MissingNsV035", "missingnsv035"} {
			t.Run(api+"/"+declared, func(t *testing.T) {
				index := externalNamespaceProject(t, api, declared, nil)
				found := false
				for _, dep := range index.Dependencies {
					if strings.EqualFold(dep.Namespace, "MissingNsV035") {
						found = true
						if dep.Status != project.DependencyStatusDeclared || len(dep.Diagnostics) != 0 {
							t.Fatalf("declared dependency %+v", dep)
						}
					}
				}
				if !found {
					t.Fatalf("dependencies %+v lack the declared namespace", index.Dependencies)
				}
				for _, d := range index.Diagnostics {
					if strings.HasPrefix(d.Code, "dependency_") {
						t.Fatalf("declared namespace reports %s %q", d.Code, d.Message)
					}
				}
				checked := 0
				for _, row := range rows {
					if row.API != api || row.Compile != "COMPILE_ERROR" {
						continue
					}
					control := strings.Contains(row.Observed, "System.NotATypeV035")
					opaqueCall := row.Case == "A05_static_receiver" || row.Case == "ExtStaticReceiverV035"
					checked++
					switch row.Route {
					case "anonymous":
						source := namespaceFamilyFixture(t, namespaceExternalPositionsDir, row.Source)
						errs := subscriberErrors(sema.AnalyzeAnonymous(index, source, api).Diagnostics)
						if control {
							if len(errs) == 0 || errs[0] != "Invalid type: System.NotATypeV035" {
								t.Errorf("%s: System control %q", row.Case, errs)
							}
							continue
						}
						if opaqueCall {
							if len(errs) != 1 || errs[0] != "Variable does not exist: MissingNsV035.Thing" {
								t.Errorf("%s anonymous: opaque static call %q", row.Case, errs)
							}
							continue
						}
						if len(errs) > 0 {
							t.Errorf("%s anonymous: declared namespace rejected %q", row.Case, errs)
						}
					case "istest-checkonly":
						name := strings.TrimSuffix(filepath.Base(row.Source), ".cls")
						classes := map[string]string{name: namespaceFamilyFixture(t, namespaceExternalPositionsDir, row.Source)}
						named := externalNamespaceProject(t, api, declared, classes)
						errs := namespaceFamilyFileErrors(sema.Analyze(named).Diagnostics, name)
						if opaqueCall {
							if len(errs) != 1 || errs[0] != "Variable does not exist: MissingNsV035.Thing" {
								t.Errorf("%s named: opaque static call %q", row.Case, errs)
							}
							continue
						}
						if len(errs) > 0 {
							t.Errorf("%s named: declared namespace rejected %q", row.Case, errs)
						}
					default:
						t.Fatalf("%s: route %q", row.Case, row.Route)
					}
				}
				if checked != 20 {
					t.Fatalf("checked %d rows, want 20", checked)
				}
			})
		}
	}
}

// A declared external namespace does not open another undeclared one.
func TestExternalNamespaceDeclarationKeepsOtherNamespacesClosed(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := externalNamespaceProject(t, api, "OtherNsV035", nil)
			errs := subscriberErrors(sema.AnalyzeAnonymous(index, "MissingNsV035.Thing t = new MissingNsV035.Thing();", api).Diagnostics)
			if len(errs) == 0 || errs[0] != "Invalid type: MissingNsV035.Thing" {
				t.Fatalf("undeclared namespace beside a declared one: %q", errs)
			}
			if errs := subscriberErrors(sema.AnalyzeAnonymous(index, "OtherNsV035.Thing t = new OtherNsV035.Thing();", api).Diagnostics); len(errs) > 0 {
				t.Fatalf("declared namespace rejected %q", errs)
			}
		})
	}
}

// The declared state reaches the test runner: a class that names the
// declared namespace compiles, so an unrelated test in it runs instead of
// failing behind the compile gate. Undeclared, the same class is invalid.
func TestExternalNamespaceDeclarationRunnerRoute(t *testing.T) {
	classes := map[string]string{
		"ExtUserV035":     "public class ExtUserV035 {\n    public MissingNsV035.Thing held;\n    public static Integer unrelated() { return 7; }\n}",
		"ExtUserV035Test": "@IsTest\nprivate class ExtUserV035Test {\n    @IsTest static void runs() { System.assertEquals(7, ExtUserV035.unrelated()); }\n}",
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, declared := range []string{"", "MissingNsV035"} {
				index := externalNamespaceProject(t, api, declared, classes)
				run := Run(index, Options{SelectedClasses: []string{"ExtUserV035Test"}, NoDiskCache: true, Parallelism: 1})
				summary := run.Summary()
				if declared == "" {
					if summary.Passed != 0 {
						t.Fatalf("undeclared: %+v; native rejects the class (ExtFieldOnlyV035)", summary)
					}
					continue
				}
				if summary.Passed != 1 || summary.Failed != 0 {
					t.Fatalf("declared: %+v problem %s", summary, firstRunProblem(run))
				}
			}
		})
	}
}
