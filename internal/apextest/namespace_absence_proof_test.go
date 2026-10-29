package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// namespaceAbsenceLocalRejections lists the fixture's local declarations of
// unresolved namespace types. Under the v0.3.1 rule (A04 C055/N010/N011) only
// these reject; other positions stay open. With fooNs declared, fooNs.X
// names the project class X.
func namespaceAbsenceLocalRejections(namespace string) []string {
	want := []string{"bar.Wrapper", "bar.wrapper", "baz.Type", "fooNs.Unknown", "q.Type"}
	if namespace != "fooNs" {
		want = append(want, "fooNs.X")
	}
	return want
}

func assertNamespaceAbsenceLocalRejections(t *testing.T, route string, ds []diagnostic.Diagnostic, want []string) {
	t.Helper()
	got := map[string]bool{}
	for _, d := range ds {
		if d.Severity != diagnostic.Error {
			continue
		}
		ref, ok := strings.CutPrefix(d.NativeMessage, "Invalid type: ")
		if d.Code != "GLADESEMA006" || !ok || !slices.Contains(want, ref) || filepath.Base(d.File) != "NamespacePositions.cls" && d.File != "" {
			t.Errorf("%s: unexpected error %s %q | %q", route, d.Code, d.NativeMessage, d.Message)
			continue
		}
		got[ref] = true
	}
	if len(got) != len(want) {
		t.Errorf("%s: rejected=%v, want %v", route, got, want)
	}
}

// Owned synthetic offline boundaries; native controls are cited in the fixture README.
func TestNamespaceAbsenceProofRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/namespace_absence"
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "fooNs", "Account"} {
			t.Run(api+"/namespace="+namespace, func(t *testing.T) {
				root := t.TempDir()
				files, err := filepath.Glob(filepath.Join(fixture, "*.cls"))
				if err != nil || len(files) != 11 {
					t.Fatalf("fixture files=%d err=%v", len(files), err)
				}
				var paths []string
				for _, file := range files {
					source, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					if namespace == "Account" && filepath.Base(file) == "NamespacePositions.cls" {
						source = []byte(strings.Replace(string(source), "public static void guarded() {", "public static void guarded() { if (false) { Account.Uncatalogued.log(); Object value = new Account.Uncatalogued(); }", 1))
					}
					path := filepath.Join(root, filepath.Base(file))
					writeFile(t, path, string(source))
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				assertNamespaceAbsenceLocalRejections(t, "project", sema.Analyze(index).Diagnostics, namespaceAbsenceLocalRejections(namespace))
				for _, warm := range []bool{false, true} {
					run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"NamespacePositionsTest"}})
					summary := run.Summary()
					if summary.Passed != 0 || summary.CompileErrors != 1 || len(run.ExcludedClasses) != 2 {
						t.Fatalf("warm=%v summary=%+v excluded=%+v problem=%s", warm, summary, run.ExcludedClasses, firstRunProblem(run))
					}
					for _, entry := range run.ExcludedClasses {
						switch {
						case entry.ClassName == "NamespacePositions" && strings.Contains(entry.Message, "Invalid type: "):
						case entry.ClassName == "NamespacePositionsTest" && entry.DependsOn == "NamespacePositions":
						default:
							t.Fatalf("warm=%v unexpected exclusion: %+v", warm, entry)
						}
					}
				}
			})
		}
	}
}

func TestNamespaceAbsenceProofRunnerControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{
				"SystemAbsentTest": "@IsTest private class SystemAbsentTest { @IsTest static void observe() { Object value = new System.NotAType(); } }",
				"SchemaAbsentTest": "@IsTest private class SchemaAbsentTest { @IsTest static void observe() { Object value = new Schema.Account.NotAThing(); } }",
			}, nil)
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			if len(run.ExcludedClasses) != 2 {
				t.Fatalf("closed catalog controls not excluded: %+v", run)
			}
		})
	}
}

// Native captures remain immutable. These rows are offline namespace deferrals,
// not assertions that the capture org compiled the source successfully.
func namespaceAbsenceDeferrals(t *testing.T, dir string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "deferred.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		name, reason, ok := strings.Cut(line, "\t")
		if !ok || name == "" || reason == "" || result[name] {
			t.Fatalf("invalid deferral row: %q", line)
		}
		result[name] = true
	}
	return result
}

func TestNamespaceAbsenceProofDuplicateCatalogs(t *testing.T) {
	const fixture = "../../testdata/conformance/namespace_absence_duplicates"
	for _, api := range []string{"62.0", "67.0"} {
		for _, mixed := range []bool{false, true} {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/reverse=%v/mixed=%v", api, reverse, mixed), func(t *testing.T) {
					root := t.TempDir()
					var files []string
					err := filepath.WalkDir(fixture, func(path string, entry os.DirEntry, err error) error {
						if err != nil {
							return err
						}
						if !entry.IsDir() && filepath.Ext(path) == ".cls" {
							files = append(files, path)
						}
						return nil
					})
					if err != nil || len(files) != 10 {
						t.Fatalf("fixture files=%d err=%v", len(files), err)
					}
					var paths []string
					for _, file := range files {
						rel, err := filepath.Rel(fixture, file)
						if err != nil {
							t.Fatal(err)
						}
						source, err := os.ReadFile(file)
						if err != nil {
							t.Fatal(err)
						}
						if mixed && filepath.Base(file) == "CatalogConsumer.cls" {
							source = []byte(strings.Replace(string(source), "public class CatalogConsumer {", "public class CatalogConsumer { public alphaNs.Unknown namespaceField; public static void namespaceGuard() { if (false) { alphaNs.Logger.log(); Object value = new alphaNs.Unknown(); } }", 1))
						}
						path := filepath.Join(root, rel)
						if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
							t.Fatal(err)
						}
						writeFile(t, path, string(source))
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
						writeFile(t, filepath.Join(root, strings.Split(rel, string(filepath.Separator))[0], "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":".","default":true}]}`)
						paths = append(paths, path)
					}
					if reverse {
						for i, j := 0, len(paths)-1; i < j; i, j = i+1, j-1 {
							paths[i], paths[j] = paths[j], paths[i]
						}
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
					if mixed {
						for i := range index.Types {
							rel, err := filepath.Rel(root, index.Types[i].File)
							if err != nil {
								t.Fatal(err)
							}
							switch strings.Split(rel, string(filepath.Separator))[0] {
							case "a":
								index.Types[i].Namespace = "alphaNs"
							case "b":
								index.Types[i].Namespace = "betaNs"
							}
						}
					}

					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					for _, warm := range []bool{false, true} {
						run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"CatalogConsumerTest"}})
						if summary := run.Summary(); summary.Passed != 1 || summary.CompileErrors != 0 || len(run.ExcludedClasses) != 0 {
							t.Fatalf("warm=%v summary=%+v excluded=%+v problem=%s", warm, summary, run.ExcludedClasses, firstRunProblem(run))
						}
					}
					negative := filepath.Join(root, "c", "UniqueNegativeTest.cls")
					writeFile(t, negative, "@IsTest private class UniqueNegativeTest { @IsTest static void observe() { Object value = new UniqueRoot.Unknown(); } }")
					writeFile(t, negative+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					index = typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: append(paths, negative)}, schema.Schema{})
					if mixed {
						for i := range index.Types {
							rel, err := filepath.Rel(root, index.Types[i].File)
							if err != nil {
								t.Fatal(err)
							}
							switch strings.Split(rel, string(filepath.Separator))[0] {
							case "a":
								index.Types[i].Namespace = "alphaNs"
							case "b":
								index.Types[i].Namespace = "betaNs"
							}
						}
					}
					run := Run(index, Options{NoDiskCache: true, Parallelism: 1, SelectedClasses: []string{"UniqueNegativeTest"}})
					if len(run.ExcludedClasses) != 1 {
						t.Fatalf("unique source negative control: summary=%+v excluded=%+v", run.Summary(), run.ExcludedClasses)
					}
				})
			}
		}
	}
}
