package sema

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAPI67StandardExceptionConstructorVisibility(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		arguments  string
		wantErrors bool
	}{
		{name: "Exception", arguments: "", wantErrors: true},
		{name: "Exception", arguments: "'message'", wantErrors: true},
		{name: "Exception", arguments: "cause", wantErrors: true},
		{name: "Exception", arguments: "'message', cause", wantErrors: true},
		{name: "InvalidParameterValueException", arguments: "", wantErrors: true},
		{name: "InvalidParameterValueException", arguments: "'message'", wantErrors: true},
		{name: "InvalidParameterValueException", arguments: "cause", wantErrors: true},
		{name: "InvalidParameterValueException", arguments: "'message', cause", wantErrors: true},
		{name: "InvalidParameterValueException", arguments: "'message', 'type'", wantErrors: false},
		{name: "NoAccessException", arguments: "", wantErrors: false},
		{name: "NoAccessException", arguments: "'message'", wantErrors: true},
		{name: "NoAccessException", arguments: "cause", wantErrors: true},
		{name: "NoAccessException", arguments: "'message', cause", wantErrors: true},
		{name: "NoDataFoundException", arguments: "", wantErrors: false},
		{name: "NoDataFoundException", arguments: "'message'", wantErrors: true},
		{name: "NoDataFoundException", arguments: "cause", wantErrors: true},
		{name: "NoDataFoundException", arguments: "'message', cause", wantErrors: true},
		{name: "NullPointerException", arguments: "", wantErrors: false},
		{name: "NullPointerException", arguments: "'message'", wantErrors: true},
		{name: "NullPointerException", arguments: "cause", wantErrors: true},
		{name: "NullPointerException", arguments: "'message', cause", wantErrors: true},
		{name: "TouchHandledException", arguments: "", wantErrors: true},
		{name: "TouchHandledException", arguments: "'message'", wantErrors: false},
		{name: "TouchHandledException", arguments: "cause", wantErrors: true},
		{name: "TouchHandledException", arguments: "'message', cause", wantErrors: true},
	}
	for _, test := range tests {
		for _, qualified := range []bool{false, true} {
			typeName := test.name
			if qualified {
				typeName = "System." + typeName
			}
			t.Run(typeName+"/"+test.arguments, func(t *testing.T) {
				source := "public class Probe { public void run(Exception cause) { throw new " + typeName + "(" + test.arguments + "); } }"
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, "67.0")
				if result.HasErrors() != test.wantErrors {
					t.Fatalf("new %s(%s) errors = %v, diagnostics = %#v", typeName, test.arguments, result.HasErrors(), result.Diagnostics)
				}
				if test.wantErrors && !declarationDiagnosticMatching(result, "constructor") && !declarationDiagnosticMatching(result, "constructs non-instantiable") {
					t.Fatalf("new %s(%s) diagnostics = %#v, want constructor diagnostic", typeName, test.arguments, result.Diagnostics)
				}
			})
		}
	}
}

func TestNullPointerZeroConstructorRemainsAcceptedAtAPIVersions40And41(t *testing.T) {
	for _, apiVersion := range []string{"40.0", "41.0"} {
		t.Run(apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": `public class Probe { public void run() { throw new NullPointerException(); } }`,
			}, apiVersion)
			if hasErrorOtherThan(result.Diagnostics, "GLADESEMA_VERSION") {
				t.Fatalf("API %s rejected throw new NullPointerException(): %#v", apiVersion, result.Diagnostics)
			}
		})
	}
}

func TestCustomExceptionCannotRedeclareInheritedStringConstructor(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"SuppliedException.cls": `public class SuppliedException extends Exception {
  public SuppliedException(String message) {}
}`,
		"Probe.cls": `public class Probe { public void run() { throw new SuppliedException('supplied'); } }`,
	})
	// A06 C008: the String constructor is supplied by the runtime.
	if !result.HasErrors() || !declarationDiagnosticMatching(result, "System exception constructor already defined: void <init>(String)") {
		t.Fatalf("custom exception redeclared the native String constructor: %#v", result.Diagnostics)
	}
}

func TestAPI67ExceptionInheritedMembersAndSubtypeConstructors(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe {
  public void run() {
    Exception cause = new QueryException('cause');
    Exception value = new BigObjectException();
    Exception header = new InvalidHeaderException('header');
    Exception readOnly = new InvalidReadOnlyUserDmlException('read only');
    Map<String, Set<String>> fields = new QueryException('query').getInaccessibleFields();
    value.setMessage('updated');
    value.initCause(cause);
    Exception recovered = value.getCause();
    System.assertNotEquals(null, header);
    System.assertNotEquals(null, readOnly);
  }
}`,
	}, "67.0")
	if result.HasErrors() {
		t.Fatalf("Salesforce API 67 exception members/subtypes were rejected: %#v", result.Diagnostics)
	}
}

// D001-D003 capture a separately loaded base and intermediate exception at
// API 62/67. Namespace and artifact flags describe the same captured classes
// when their declarations arrive through a dependency rather than local source.
func TestExceptionDependencyAncestryOrgControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, namespace := range []string{"", "pkgx"} {
				for _, artifact := range []bool{false, true} {
					for _, qualified := range []bool{false, true} {
						for _, row := range []struct{ id, name, parent, expected string }{
							{"D001", "A06DependencyChildException", "A06DependencyBaseException", ""},
							{"D002", "A06DependencyBadName", "A06DependencyBaseException", "Classes extending Exception must have a name ending in Exception: A06DependencyBadName"},
							{"D003", "A06DependencyChildException", "A06DependencyMidException", ""},
						} {
							parent := row.parent
							if qualified && namespace != "" {
								parent = namespace + "." + parent
							}
							index := typesys.Index{Project: typesys.ProjectInfo{Namespace: namespace}, Types: []typesys.TypeSymbol{
								{Kind: apexast.DeclarationClass, Name: row.name, Namespace: namespace, SuperClass: parent, EffectiveAPIVersion: api},
								{Kind: apexast.DeclarationClass, Name: "A06DependencyBaseException", Namespace: namespace, SuperClass: "Exception", Dependency: true, Artifact: artifact, EffectiveAPIVersion: api},
								{Kind: apexast.DeclarationClass, Name: "A06DependencyMidException", Namespace: namespace, SuperClass: "A06DependencyBaseException", Dependency: true, Artifact: artifact, EffectiveAPIVersion: api},
								// An unrelated artifact with incomplete ancestry must not overwrite
								// the known same-namespace base just because its short name matches.
								{Kind: apexast.DeclarationClass, Name: "A06DependencyBaseException", Namespace: "other", Dependency: true, Artifact: true, EffectiveAPIVersion: api},
							}}
							diagnostics := checkExceptionDeclarations(index)
							observed := ""
							if len(diagnostics) > 0 {
								observed = diagnostics[0].NativeMessage
							}
							if observed != row.expected || len(diagnostics) > 1 {
								t.Fatalf("%s namespace=%q artifact=%v qualified=%v expected <%s> actual <%s>: %#v", row.id, namespace, artifact, qualified, row.expected, observed, diagnostics)
							}
						}
					}
				}
			}
		})
	}
}

// Inherited-inner named captures at APIs 62/64/67 are exported in
// exceptions_inherited_inner.json. Namespace variants remain local regressions;
// the capture route cannot provision a package namespace.
func TestExceptionDeclarationInheritedInnerType(t *testing.T) {
	for _, api := range []string{"62.0", "64.0", "67.0"} {
		for _, tc := range []struct {
			name, base, parent, child, expected, access, code string
			middle, shadow, local, implicit                   bool
		}{
			{name: "direct", base: "Exception", parent: "BaseException", child: "InnerException"},
			{name: "qualified", base: "Exception", parent: "BaseWidget.BaseException", child: "InnerException"},
			{name: "case-insensitive", base: "Exception", parent: "bAsEeXcEpTiOn", child: "InnerException"},
			{name: "transitive", base: "Exception", parent: "BaseException", child: "InnerException", middle: true},
			{name: "invalid-name", base: "Exception", parent: "BaseException", child: "Detail", expected: "Classes extending Exception must have a name ending in Exception: Widget.Detail"},
			{name: "invalid-base", base: "Object", parent: "BaseException", child: "InnerException", expected: "Exception class must extend another Exception class:"},
			{name: "local-shadow", base: "Exception", parent: "BaseException", child: "InnerException", shadow: true, expected: "Exception class must extend another Exception class: Widget.BaseException"},
			{name: "private-base-rejection-twin", base: "Exception", parent: "BaseException", child: "InnerException", access: "private", code: "GLADESEMA017", expected: "Type is not visible: BaseWidget.BaseException"},
			{name: "private-transitive-rejection-twin", base: "Exception", parent: "BaseException", child: "InnerException", access: "private", middle: true, code: "GLADESEMA017", expected: "Type is not visible: BaseWidget.BaseException"},
			{name: "implicit-private-base-rejection-twin", base: "Exception", parent: "BaseException", child: "InnerException", implicit: true, code: "GLADESEMA017", expected: "Type is not visible: BaseWidget.BaseException"},
			{name: "implicit-private-transitive-rejection-twin", base: "Exception", parent: "BaseException", child: "InnerException", implicit: true, middle: true, code: "GLADESEMA017", expected: "Type is not visible: BaseWidget.BaseException"},
			{name: "private-lexical-positive", base: "Exception", parent: "BaseException", child: "InnerException", access: "private", local: true},
			{name: "implicit-private-lexical-positive", base: "Exception", parent: "BaseException", child: "InnerException", implicit: true, local: true},
		} {
			for _, namespace := range []string{"", "pkgx"} {
				t.Run(api+"/"+namespace+"/"+tc.name, func(t *testing.T) {
					access := tc.access
					if access == "" && !tc.implicit {
						access = "public"
					}
					files := map[string]string{
						"BaseWidget.cls": fmt.Sprintf("public virtual class BaseWidget { %s virtual class BaseException extends %s {} }", access, tc.base),
					}
					parent := "BaseWidget"
					if tc.middle {
						files["MiddleWidget.cls"] = "public virtual class MiddleWidget extends BaseWidget {}"
						parent = "MiddleWidget"
					}
					shadow := ""
					if tc.shadow {
						shadow = "public virtual class BaseException {}"
					}
					files["Widget.cls"] = fmt.Sprintf("public class Widget extends %s { %s public class %s extends %s {} }", parent, shadow, tc.child, tc.parent)
					if tc.local {
						files["BaseWidget.cls"] = fmt.Sprintf("public virtual class BaseWidget { %s virtual class BaseException extends Exception {} public class InnerException extends BaseException {} }", access)
						delete(files, "Widget.cls")
					}
					if tc.expected == "" && !tc.local {
						files["Probe.cls"] = `public class Probe {
  public void run() {
    try { throw new Widget.InnerException('owned'); }
    catch (Exception e) { System.assertEquals('owned', e.getMessage()); }
  }
}`
					}
					root := t.TempDir()
					paths := make([]string, 0, len(files))
					for name, contents := range files {
						path := filepath.Join(root, name)
						writeSemaFile(t, path, contents)
						paths = append(paths, path)
					}
					result := Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: api, Namespace: namespace, ApexFiles: paths}, schema.Schema{}))
					if tc.expected == "" {
						if result.HasErrors() {
							t.Fatalf("inherited inner exception rejected: %#v", result.Diagnostics)
						}
						return
					}
					code := tc.code
					if code == "" {
						code = "GLADESEMA030"
					}
					expected := tc.expected
					if namespace != "" && tc.code == "GLADESEMA017" {
						// Native namespace spelling is not observed by this capture.
						expected = "Type is not visible:"
					}
					for _, d := range result.Diagnostics {
						if filepath.Base(d.File) == "Widget.cls" && d.Code == code && strings.Contains(d.NativeMessage, expected) {
							return
						}
					}
					t.Fatalf("missing native exception rejection %q: %#v", tc.expected, result.Diagnostics)
				})
			}
		}
	}
}

// The inherited-inner A06 guard must not introduce ordinary-class visibility
// semantics: that behavior has no W5 native control. The private exception
// rejection twin is captured as C007; the ordinary row tests only A06's scope.
func TestExceptionInheritedInnerVisibilityScope(t *testing.T) {
	for _, tc := range []struct{ name, inner, base, access, expected string }{
		{"ordinary-private-base-scope-control", "Base", "", "private", ""},
		{"ordinary-implicit-private-base-scope-control", "Base", "", "", ""},
		{"public-exception-positive", "BaseException", " extends Exception", "public", ""},
		{"private-exception-rejection-twin", "BaseException", " extends Exception", "private", "Type is not visible: BaseWidget.BaseException"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			paths := []string{filepath.Join(root, "BaseWidget.cls"), filepath.Join(root, "Widget.cls")}
			writeSemaFile(t, paths[0], "public virtual class BaseWidget { "+tc.access+" virtual class "+tc.inner+tc.base+" {} }")
			child := "Child"
			if tc.base != "" {
				child = "InnerException"
			}
			writeSemaFile(t, paths[1], "public class Widget extends BaseWidget { public class "+child+" extends "+tc.inner+" {} }")
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: paths}, schema.Schema{})
			diagnostics := checkExceptionDeclarations(index)
			observed := ""
			for _, d := range diagnostics {
				observed += d.NativeMessage
			}
			if observed != tc.expected || (tc.expected == "" && len(diagnostics) != 0) || (tc.expected != "" && len(diagnostics) != 1) {
				t.Fatalf("A06 scope expected <%s> actual <%s>: %#v", tc.expected, observed, diagnostics)
			}
		})
	}
}

// W5 matrix C001/C011/C012 reject virtual @IsTest outer classes at every
// captured API. Unannotated virtual declarations and nonvirtual test classes
// remain admitted by the earlier W5 named and A05 annotation captures.
func TestExceptionMatrixTestClassVirtualRejectionTwin(t *testing.T) {
	for _, api := range []string{"62.0", "64.0", "67.0"} {
		for _, tc := range []struct{ name, source, expected string }{
			{"ordinary-virtual-positive", "public virtual class Widget {}", ""},
			{"nonvirtual-test-positive", "@IsTest private class Widget {}", ""},
			{"virtual-test-rejection-twin", "@IsTest public virtual class Widget {}", "IsTest classes cannot be virtual"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Widget.cls": tc.source}, api)
				observed := ""
				for _, d := range result.Diagnostics {
					if d.Severity == diagnostic.Error {
						observed += d.NativeMessage
					}
				}
				if result.HasErrors() != (tc.expected != "") || observed != tc.expected {
					t.Fatalf("expected <%s> actual <%s>: %#v", tc.expected, observed, result.Diagnostics)
				}
			})
		}
	}
}

// W5 anonymous declaration rows capture this precedence. The virtual-only
// rejection twin retains A04's modifier error; no diagnostics are suppressed.
func TestExceptionMatrixAnonymousNestingPrecedesVirtual(t *testing.T) {
	for _, api := range []string{"62.0", "64.0", "67.0"} {
		for _, tc := range []struct {
			name, source, expected string
			count                  int
		}{
			{"nested-virtual-rejection", "public virtual class BaseWidget { public virtual class BaseException extends Exception {} }", "Inner types are not allowed to have inner types", 3},
			{"virtual-only-rejection-twin", "public virtual class BaseWidget {}", "classes are by default virtual", 1},
			{"implicit-virtual-positive", "class BaseWidget {}", "", 0},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "BaseWidget.cls")
				writeSemaFile(t, path, tc.source)
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				result := AnalyzeAnonymousDeclarations(WithAnonymousDeclarationContext(index))
				observed := ""
				if len(result.Diagnostics) > 0 {
					observed = result.Diagnostics[0].NativeMessage
					if observed == "" {
						observed = result.Diagnostics[0].Message
					}
				}
				if observed != tc.expected || len(result.Diagnostics) != tc.count {
					t.Fatalf("expected <%s>, %d diagnostics; actual <%s>: %#v", tc.expected, tc.count, observed, result.Diagnostics)
				}
			})
		}
	}
}
