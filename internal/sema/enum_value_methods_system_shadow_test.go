package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// A project class named LoggingLevel does not hide the explicit
// System.LoggingLevel enum. Native: testdata/enum_value_methods/org.tsv:64-67,
// 72-77 (name, ordinal, equals, hashCode, toString accepted) and 144-147 at
// API 67.0; rejections stay as org.tsv:62,68,70,142,148,150.
func TestEnumValueMethodsExplicitSystemEnumWithProjectShadow(t *testing.T) {
	const valid = `String a = System.LoggingLevel.ERROR.name();
Integer b = System.LoggingLevel.WARN.ordinal();
String c = System.LoggingLevel.DEBUG.toString();
Boolean d = System.LoggingLevel.INFO.equals(System.LoggingLevel.INFO);
Integer e = System.LoggingLevel.FINE.hashCode();
Map<String, Object> input = new Map<String, Object>{ 'level' => System.LoggingLevel.ERROR.name() };`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			var paths []string
			for name, source := range map[string]string{
				"LoggingLevel.cls":     "public class LoggingLevel {}",
				"GardenLevels.cls":     "public class GardenLevels {\n    public static Object observe() {\n" + valid + "\n        return input;\n    }\n    private class Helper {\n        Object observe() { return System.LoggingLevel.ERROR.name() + System.LoggingLevel.WARN.ordinal(); }\n    }\n}",
				"GardenLevelsTest.cls": "@IsTest private class GardenLevelsTest {\n    @IsTest static void observe() {\n" + valid + "\n        System.assert(a != null);\n    }\n}",
			} {
				path := filepath.Join(root, name)
				writeSemaFile(t, path, source)
				paths = append(paths, path)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if result := Analyze(index); result.HasErrors() {
				t.Fatalf("named diagnostics=%+v", result.Diagnostics)
			}
			if result := AnalyzeAnonymous(index, valid, api); result.HasErrors() {
				t.Fatalf("anonymous diagnostics=%+v", result.Diagnostics)
			}
			for body, want := range map[string]string{
				// Rejected either way; the shadowed spelling of this message is uncaptured.
				"Object x = System.LoggingLevel.ERROR.bogusMethod();": "bogusMethod",
				"Object x = System.LoggingLevel.ERROR.values();":      "Static method cannot be referenced from a non static context: List<System.LoggingLevel> System.LoggingLevel.values()",
			} {
				result := AnalyzeAnonymous(index, body, api)
				found := false
				for _, d := range result.Diagnostics {
					found = found || strings.Contains(d.NativeMessage, want) || strings.Contains(d.Message, want)
				}
				if !found {
					t.Fatalf("%s: want %q, diagnostics=%+v", body, want, result.Diagnostics)
				}
			}
		})
	}
}

// Methods that exist only under the bare enum name stay visible when the call
// spells the enum System.-qualified: AccessLevel.USER_MODE.withPermissionSetId
// is accepted either way (corpus: QueryObject uses System.AccessLevel).
func TestEnumValueMethodsQualifiedSystemEnumKeepsBareOnlyMethods(t *testing.T) {
	const valid = `Id permissionSetId = null;
System.AccessLevel a = System.AccessLevel.USER_MODE.withPermissionSetId(permissionSetId);
System.AccessLevel b = AccessLevel.USER_MODE.withPermissionSetId(permissionSetId);
System.AccessLevel c = System.AccessLevel.SYSTEM_MODE;`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "QueryLike.cls")
			writeSemaFile(t, path, "public class QueryLike {\n    public static void observe() {\n"+valid+"\n    }\n}")
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if result := Analyze(index); result.HasErrors() {
				t.Fatalf("named diagnostics=%+v", result.Diagnostics)
			}
			if result := AnalyzeAnonymous(index, valid, api); result.HasErrors() {
				t.Fatalf("anonymous diagnostics=%+v", result.Diagnostics)
			}
		})
	}
}
