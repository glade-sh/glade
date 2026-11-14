package sema

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The earlier parse error retains the complete origin/main diagnostic.
func TestReceiverEarlierParseErrorUnchanged(t *testing.T) {
	source := "Integer x = ;\nReceiverProbe.\nvalue();"
	want := `[{"severity":"error","code":"GLADESEMA_ANONYMOUS_PARSE","message":"unexpected token \";\" at byte 12","range":{"start":{"line":1,"column":1},"end":{"line":1,"column":38,"offset":37}}}]`
	got, err := json.Marshal(AnalyzeAnonymous(typesys.Index{}, source, "67.0").Diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("diagnostics %s, want origin/main %s", got, want)
	}
}

// Free-dot type messages are captured at both API endpoints (org.tsv:286-291).
func TestReceiverFreeTypeContexts(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
		line, column       int
	}{
		{"generic", "{ List<System\n.String> s; }", "Unexpected token 'List'.", 1, 3},
		{"cast", "{ Object s = (System\n.String) 'x'; }", "Extra ';', at 'x'.", 2, 10},
		{"instanceof", "{ Boolean b = 'x' instanceof System\n.String; }", "Expecting ';' but was: '.'", 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := AnalyzeAnonymous(typesys.Index{}, tc.source, "67.0").Diagnostics
			if len(got) != 1 || got[0].Range == nil || got[0].NativeMessage != tc.want || got[0].Range.Start.Line != tc.line || got[0].Range.Start.Column != tc.column {
				t.Errorf("diagnostics %v, want %d:%d %s", got, tc.line, tc.column, tc.want)
			}
		})
	}
}

// A static field that shares a name with a nested type is still a value on
// the left of a free dot. S1 covers static-field values (org.tsv:174-175).
func TestReceiverStaticFieldShadowsType(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ShadowProbe.cls")
	writeSemaFile(t, path, "public class ShadowProbe { public enum Level { LOW } public static Level level = null; }")
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
		got, err := json.Marshal(AnalyzeAnonymous(index, "String s = ShadowProbe.level\n.name();", api).Diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "null" {
			t.Errorf("api %s: diagnostics %s, want origin/main null", api, got)
		}
	}
}

// A free dot can follow a field of a local value even when the field's name
// also names a type. This uncaptured neighbor is clean on origin/main.
func TestReceiverDetachedFieldContinuation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "CollisionProbe.cls")
	writeSemaFile(t, path, "public class CollisionProbe { public String String = 'x'; }")
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
		source := "CollisionProbe x = new CollisionProbe(); x .String\n.length();"
		got, err := json.Marshal(AnalyzeAnonymous(index, source, api).Diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "null" {
			t.Errorf("api %s: diagnostics %s, want origin/main null", api, got)
		}
	}
}

// A free dot in an argument resolves its left operand as a value, including
// member reads (org.tsv:82-83, 162-163).
func TestReceiverParenthesizedMemberValueResolution(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Build(project.Project{SourceAPIVersion: api}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
		got := AnalyzeAnonymous(index, "System.debug(Account\n.Name);", api).Diagnostics
		if len(got) != 1 || got[0].NativeMessage != "Variable does not exist: Account" {
			t.Errorf("api %s: diagnostics %v, want value resolution error", api, got)
		}
	}
}

// A control character inside a string is outside the captured dot-adjacent
// code shapes and keeps origin/main's diagnostic result.
func TestReceiverVerticalTabInStringUnchanged(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		got, err := json.Marshal(AnalyzeAnonymous(typesys.Index{}, "String s = 'x.\vtext';", api).Diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "null" {
			t.Errorf("api %s: diagnostics %s, want origin/main null", api, got)
		}
	}
}
