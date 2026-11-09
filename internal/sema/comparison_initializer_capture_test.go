package sema

import (
	"os"
	"strings"
	"testing"
)

func TestCapturedComparisonInitializer(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": `
public class Probe {
  public static Boolean run() {
    Integer a=1,b=2,c=3,d=2;
    Boolean x = a < b && c > d;
    return x;
  }
}`}, api)
			if result.HasErrors() {
				t.Fatalf("captured expression rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestComparisonInitializerDoesNotCreateLocal(t *testing.T) {
	body := "Boolean x = a < b && c > d; List<String> later = new List<String>();"
	locals := map[string]string{}
	for _, m := range findSemaLocalDeclMatches(body) {
		locals[body[m[4]:m[5]]] = body[m[2]:m[3]]
	}
	if _, ok := locals["d"]; ok {
		t.Errorf("comparison operand treated as a declaration: %v", locals)
	}
	if locals["later"] != "List<String>" {
		t.Errorf("genuine generic declaration not found: %v", locals)
	}
}

func TestCapturedMultilineComparisonInitializer(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": `public class Probe { public static Boolean run() { Integer a=1,b=2,c=3,d=2; Boolean x =
  a < b && c > d; return x; } }`}, api)
			if result.HasErrors() {
				t.Fatalf("captured multiline initializer rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestCapturedCommentBeforeLocal(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				reject bool
			}{{"comment", false}, {"commentreject", true}} {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": capturedCommentSource(t, tc.name)}, api)
				if result.HasErrors() != tc.reject {
					t.Errorf("%s diagnostics: %#v", tc.name, result.Diagnostics)
				}
			}
		})
	}
}

func capturedCommentSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/comment_local_boundary/" + name + ".apex")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCapturedCommentLocalRemainsVisible(t *testing.T) {
	bodyText := "Integer first = 1; // =\nInteger second = 2; return first + second;"
	idx := capturedBodyIndex(t, "testdata/comment_local_boundary", "62.0", "named", bodyText)
	model := buildSemaTypeMemberView(idx)
	a := NewAnalyzer()
	a.prepareAnalysisModel(idx)
	checked := false
	for _, typ := range idx.Types {
		if typ.Name != "GardenBodyProbe" {
			continue
		}
		raw, err := os.ReadFile(typ.File)
		if err != nil {
			t.Fatal(err)
		}
		for _, member := range typ.Members {
			if member.Name != "observe" {
				continue
			}
			checked = true
			body, offset, ok := semaBodyFromRange(string(raw), member.BodyRange)
			if !ok {
				t.Fatal("missing body")
			}
			scopes, _ := a.collectBodyScopes(typ, member, body, offset, string(raw), map[string]string{semaCurrentTypeScopeKey: typ.Name}, model)
			if got, ok := scopes.visibleAt("second", strings.Index(body, "return")); !ok || got != "Integer" {
				t.Fatalf("captured local missing: %q/%t", got, ok)
			}
		}
	}
	if !checked {
		t.Fatal("missing method")
	}
}
