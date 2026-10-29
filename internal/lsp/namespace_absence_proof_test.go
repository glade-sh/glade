package lsp

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
					writeReceiverMemberFile(t, path, string(source))
					writeReceiverMemberFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				h := NewHandler(index)
				defer h.parser.Close()
				for _, path := range paths {
					ds := h.diagnosticsForDocument(uriFromPath(path))
					if filepath.Base(path) == "NamespacePositions.cls" {
						assertNamespaceAbsenceLocalRejections(t, "NamespacePositions.cls", ds, namespaceAbsenceLocalRejections(namespace))
					} else if len(ds) != 0 {
						t.Errorf("%s: %+v", filepath.Base(path), ds)
					}
				}
			})
		}
	}
}

func TestNamespaceAbsenceProofLSPControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, reference := range []string{"System.NotAType", "Schema.Account.NotAThing"} {
			t.Run(api+"/"+reference, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "Absent.cls")
				writeReceiverMemberFile(t, path, "public class Absent { public Object make() { return new "+reference+"(); } }")
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				h := NewHandler(index)
				defer h.parser.Close()
				for _, d := range h.diagnosticsForDocument(uriFromPath(path)) {
					if d.NativeMessage == "Invalid type: "+reference {
						return
					}
				}
				t.Fatalf("closed catalog control accepted: %s", reference)
			})
		}
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
						writeReceiverMemberFile(t, path, string(source))
						writeReceiverMemberFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
						writeReceiverMemberFile(t, filepath.Join(root, strings.Split(rel, string(filepath.Separator))[0], "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":".","default":true}]}`)
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
					h := NewHandler(index)
					defer h.parser.Close()
					for _, path := range paths {
						for _, d := range h.diagnosticsForDocument(uriFromPath(path)) {
							if d.Code == "GLADETYPE001" && d.Severity == diagnostic.Warning {
								continue // Independent units intentionally duplicate this name.
							}
							t.Errorf("%s: %+v", filepath.Base(path), d)
						}
					}
					negative := filepath.Join(root, "c", "UniqueNegative.cls")
					writeReceiverMemberFile(t, negative, "public class UniqueNegative { public UniqueRoot.Unknown field; public SiblingRoot.Unknown other; }")
					writeReceiverMemberFile(t, negative+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					index = typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: append(paths, negative)}, schema.Schema{})
					control := NewHandler(index)
					defer control.parser.Close()
					ds := control.diagnosticsForDocument(uriFromPath(negative))
					for _, reference := range []string{"UniqueRoot.Unknown", "SiblingRoot.Unknown"} {
						found := false
						for _, d := range ds {
							found = found || d.NativeMessage == "Invalid type: "+reference
						}
						if !found {
							t.Fatal("local source catalog was opened: " + reference)
						}
					}
				})
			}
		}
	}
}
