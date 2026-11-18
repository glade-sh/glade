package sema

import (
	"maps"
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

// Synthetic: ../../testdata/conformance/undeclared_namespace_types/cases.md.
// Glade in a previous release accepted every case in cases.tsv marked accepted. Ruling 49
// reverses that deferral: an org without the package rejects an undeclared
// namespace in every position (namespace_external_positions/org.tsv), and a
// dependency declared without source keeps the namespace open.
const undeclaredNamespaceFixture = "../../testdata/conformance/undeclared_namespace_types"

func undeclaredNamespaceIndex(t *testing.T, api, namespace string, extra map[string]string) typesys.Index {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(undeclaredNamespaceFixture, "project/force-app/main/default/classes/*.cls"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 5 {
		t.Fatalf("fixture classes=%d, want 5", len(files))
	}
	sources := make(map[string]string, len(files)+len(extra))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		sources[strings.TrimSuffix(filepath.Base(file), ".cls")] = string(data)
	}
	for name, source := range extra {
		sources[name] = source
	}
	root := t.TempDir()
	var paths []string
	for name, source := range sources {
		path := filepath.Join(root, name+".cls")
		writeSemaFile(t, path, source)
		writeSemaFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
		paths = append(paths, path)
	}
	index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
	if index.HasErrors() {
		t.Fatalf("index diagnostics: %v", index.Diagnostics)
	}
	return index
}

// Check the shared fixture as well as the class under test. Its rejections
// are native-backed by namespace_external_positions/org.tsv at APIs 62/67.
func assertUndeclaredSharedFixtureDiagnostics(t *testing.T, ds []diagnostic.Diagnostic, target, namespace string) {
	t.Helper()
	want := map[string]int{}
	add := func(file, code, ref string, n int) { want[file+"|"+code+"|Invalid type: "+ref] = n }
	add("BarExtension.cls", "GLADESEMA006", "bar.BaseHandler", 1)
	add("BarExtension.cls", "GLADESEMA006", "bar.Callback", 1)
	add("BarExtension.cls", "GLADESEMA004", "fooNs.Unknown", 1)
	add("BarSignatures.cls", "GLADESEMA002", "bar.Wrapper", 2)
	add("BarSignatures.cls", "GLADESEMA002", "bar.OtherWrapper", 2)
	add("BarSignatures.cls", "GLADESEMA002", "fooNs.Unknown", 1)
	add("BarSignatures.cls", "GLADESEMA004", "bar.Wrapper", 2)
	add("BarSignatures.cls", "GLADESEMA004", "bar.OtherWrapper", 1)
	add("BarSignatures.cls", "GLADESEMA004", "fooNs.Unknown", 1)
	add("BarConstructions.cls", "GLADESEMA006", "bar.Wrapper", 5)
	add("BarConstructions.cls", "GLADESEMA006", "bar.OtherWrapper", 2)
	add("BarConstructions.cls", "GLADESEMA006", "bar.LogBuilder", 1)
	add("BarConstructions.cls", "GLADESEMA006", "fooNs.Unknown", 1)
	add("BarConsumerTest.cls", "GLADESEMA006", "bar.Wrapper", 1)
	if namespace == "" {
		add("BarExtension.cls", "GLADESEMA004", "fooNs.LocalThing", 1)
		add("BarSignatures.cls", "GLADESEMA002", "fooNs.LocalThing", 1)
		add("BarConstructions.cls", "GLADESEMA006", "fooNs.LocalThing", 1)
	}
	got := map[string]int{}
	for _, d := range ds {
		if filepath.Base(d.File) == target {
			continue
		}
		if d.Severity != diagnostic.Error || d.Message != d.NativeMessage {
			t.Errorf("unexpected shared fixture diagnostic: %+v", d)
		}
		got[filepath.Base(d.File)+"|"+d.Code+"|"+d.NativeMessage]++
	}
	if !maps.Equal(got, want) {
		t.Errorf("shared fixture diagnostics=%v, want %v", got, want)
	}
}

func TestUndeclaredNamespaceCaseTableNamesFixtureReferences(t *testing.T) {
	rows := readQualifiedReceiverTSV(t, filepath.Join(undeclaredNamespaceFixture, "cases.tsv"), 6)
	if len(rows) != 8 {
		t.Fatalf("case rows=%d, want header plus 7", len(rows))
	}
	want := map[string]bool{"own_type_qualified": true, "absent_type_qualified": true}
	for _, row := range rows[1:] {
		delete(want, row[0])
		for _, file := range strings.Split(row[4], ",") {
			path := filepath.Join(undeclaredNamespaceFixture, file)
			if !strings.Contains(file, "/") && strings.HasSuffix(file, ".cls") {
				path = filepath.Join(undeclaredNamespaceFixture, "project/force-app/main/default/classes", file)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%s: %v", row[0], err)
			}
			found := false
			for _, reference := range strings.Split(row[2], ",") {
				found = found || strings.Contains(string(data), reference)
			}
			if !found {
				t.Fatalf("%s: %s names none of %s", row[0], file, row[2])
			}
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing named cases: %v", want)
	}
}

// undeclaredNamespaceErrorsInFile requires every error in file (all files when
// file is empty) to be Invalid type for an undeclared bar or fooNs type, and at
// least one.
func undeclaredNamespaceErrorsInFile(t *testing.T, route, namespace, file string, ds []diagnostic.Diagnostic) {
	t.Helper()
	count := 0
	for _, d := range ds {
		if d.Severity != diagnostic.Error || file != "" && filepath.Base(d.File) != file {
			continue
		}
		count++
		ref, ok := strings.CutPrefix(d.NativeMessage, "Invalid type: ")
		if !ok || !strings.HasPrefix(ref, "bar.") && ref != "fooNs.Unknown" && !(namespace == "" && ref == "fooNs.LocalThing") {
			t.Errorf("%s: unexpected error %s %q | %q", route, d.Code, d.NativeMessage, d.Message)
		}
	}
	if count == 0 {
		t.Errorf("%s: undeclared namespaces accepted", route)
	}
}

// own_type_qualified, absent_type_qualified and the managed_* rows: a project
// with "namespace": "" (and the same source under its own declared namespace).
// Undeclared, they reject; declared without source, they compile.
func TestUndeclaredNamespaceTypesDeferredInProjectAndAnonymous(t *testing.T) {
	anonymous, err := os.ReadFile(filepath.Join(undeclaredNamespaceFixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "fooNs"} {
			t.Run(api+"/namespace="+namespace, func(t *testing.T) {
				index := undeclaredNamespaceIndex(t, api, namespace, nil)
				undeclaredNamespaceErrorsInFile(t, "project", namespace, "", Analyze(index).Diagnostics)
				undeclaredNamespaceErrorsInFile(t, "anonymous", namespace, "", AnalyzeAnonymous(index, string(anonymous), api).Diagnostics)
				if namespace != "" {
					return
				}
				index.Dependencies = []typesys.DependencyInfo{{Namespace: "bar", Status: "missing"}, {Namespace: "fooNs", Status: "missing"}}
				if result := Analyze(index); result.HasErrors() {
					t.Fatalf("declared project check: %+v", result.Diagnostics)
				}
				if result := AnalyzeAnonymous(index, string(anonymous), api); result.HasErrors() {
					t.Fatalf("declared anonymous: %+v", result.Diagnostics)
				}
			})
		}
	}
}

// With the project's own namespace declared, fooNs.LocalThing names the
// project class: its size() result types as Integer through that spelling.
func TestDeclaredProjectNamespaceQualifiedTypeResolvesToProjectClass(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := undeclaredNamespaceIndex(t, api, "fooNs", map[string]string{
				"OwnedProbe": "public with sharing class OwnedProbe {\n" +
					"    public static Integer good() { fooNs.LocalThing owned = new fooNs.LocalThing(); return owned.size(); }\n" +
					"    public static String mismatch() { fooNs.LocalThing owned = new fooNs.LocalThing(); String s = owned.size(); return s; }\n" +
					"}\n",
			})
			diagnostics := Analyze(index).Diagnostics
			assertUndeclaredSharedFixtureDiagnostics(t, diagnostics, "OwnedProbe.cls", "fooNs")
			var got []string
			for _, d := range diagnostics {
				if filepath.Base(d.File) == "OwnedProbe.cls" {
					got = append(got, d.Message)
				}
			}
			want := `method "mismatch" initializes String local "s" with Integer`
			if len(got) != 1 || got[0] != want {
				t.Fatalf("diagnostics=%q, want only %q", got, want)
			}
		})
	}
}

// Historical base_boundary records the old local/static rejection.
// defers the unindexed namespace type positions. The static call keeps the
// release unknown-method rejection: offline execution has no method to run,
// so its unit stays a compile exclusion (native missing_static rows in
// testdata/conformance/unloaded_dependency reject the same shape).
func TestUndeclaredNamespaceBaseBoundaryDeferred(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(undeclaredNamespaceFixture, "base_boundary/BarLocals.cls.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := undeclaredNamespaceIndex(t, api, "", map[string]string{"BarLocals": string(source)})
			diagnostics := Analyze(index).Diagnostics
			assertUndeclaredSharedFixtureDiagnostics(t, diagnostics, "BarLocals.cls", "")
			var errors []string
			for _, d := range diagnostics {
				if d.Severity == diagnostic.Error && filepath.Base(d.File) == "BarLocals.cls" {
					errors = append(errors, d.Code+" "+d.Message+" | "+d.NativeMessage)
				}
			}
			// The static call keeps the release rejection; the qualified locals
			// reject under the v0.3.1 local-declaration rule (A04 C055).
			want := []string{`GLADESEMA008 method "run" calls unknown method "bar.Logger.log" | Variable does not exist: bar.Logger`}
			for _, ref := range []string{"bar.LogBuilder", "bar.Wrapper", "bar.OtherWrapper", "fooNs.LocalThing", "fooNs.Unknown"} {
				want = append(want, "GLADESEMA006 Invalid type: "+ref+" | Invalid type: "+ref)
			}
			if !slices.Equal(errors, want) {
				t.Fatalf("errors=%q, want %q", errors, want)
			}
			for _, source := range []string{"bar.Logger.log('x');", "if (false) { fooNs.Unknown.log(); }", "if (false) { q.logger.log(); }"} {
				if result := AnalyzeAnonymous(index, source, api); !result.HasErrors() {
					t.Fatalf("%s: unindexed static call accepted", source)
				}
			}
		})
	}
}

// Proven absence stays rejected. System's inventory is closed; the native
// invalid_system rows (testdata/conformance/platform_type_references/org.tsv)
// reject System.NotAType. The project source fully enumerates LocalThing's
// inner types; native known_bogus_local_type
// (testdata/qualified_receiver_shapes/org.tsv) rejects that shape.
func TestUndeclaredNamespaceDeferralKeepsProvenAbsence(t *testing.T) {
	cases := []struct{ name, source, native string }{
		{"system_local", "System.NotAType value = null;", "Invalid type: System.NotAType"},
		{"system_new", "Object value = new System.NotAType();", "Invalid type: System.NotAType"},
		{"project_inner_local", "LocalThing.Bogus value = null;", "Invalid type: LocalThing.Bogus"},
	}
	for _, api := range []string{"62.0", "67.0"} {
		index := undeclaredNamespaceIndex(t, api, "", nil)
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				result := AnalyzeAnonymous(index, tc.source, api)
				for _, d := range result.Diagnostics {
					if d.NativeMessage == tc.native {
						return
					}
				}
				t.Fatalf("want %q, got %+v", tc.native, result.Diagnostics)
			})
		}
	}
}
