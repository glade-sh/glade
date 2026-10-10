package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The owned API 41 Salesforce packet deploys both components unchanged and
// passes repeatedReadsInitializeOnce. Its getter reads and writes its own
// backing value without declaring a setter.
const legacyStaticGetterSource = `public class GladeB1Static41 {
    public static Integer initializations = 0;
    public static String value {
        get { if (value == null) { initializations++; value = 'ready'; } return value; }
    }
}`

func TestPropertyOwnStaticGetterMutationVersionBoundary(t *testing.T) {
	for _, version := range []string{"24.0", "41.0", "42.0", "67.0"} {
		t.Run(version, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"GladeB1Static41.cls": legacyStaticGetterSource,
			}, version)
			if version == "24.0" || version == "41.0" {
				if result.HasErrors() {
					t.Fatalf("legacy own static getter assignment rejected: %#v", result.Diagnostics)
				}
			} else if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA019" || result.Diagnostics[0].Message != "Variable is not visible: GladeB1Static41.value" {
				t.Fatalf("expected only the Salesforce visibility diagnostic: %#v", result.Diagnostics)
			}
		})
	}
}

func TestPropertyOwnStaticGetterUsesComponentAPIVersion(t *testing.T) {
	for _, test := range []struct {
		component, project string
		wantReject         bool
	}{{"24.0", "67.0", false}, {"41.0", "67.0", false}, {"42.0", "41.0", true}, {"67.0", "41.0", true}} {
		t.Run(test.component+"_under_"+test.project, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "GladeB1Static41.cls")
			writeSemaFile(t, path, legacyStaticGetterSource)
			writeSemaFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+test.component+`</apiVersion></ApexClass>`)
			result := Analyze(typesys.Build(project.Project{
				Root: root, SourceAPIVersion: test.project, ApexFiles: []string{path},
			}, schema.Schema{}))
			if test.wantReject {
				if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA019" || result.Diagnostics[0].Message != "Variable is not visible: GladeB1Static41.value" {
					t.Fatalf("expected only the Salesforce visibility diagnostic: %#v", result.Diagnostics)
				}
			} else if result.HasErrors() {
				t.Fatalf("legacy component getter rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestPropertyStaticGetterAssignmentContextControls(t *testing.T) {
	for _, test := range []struct {
		name       string
		source     string
		wantReject bool
	}{
		{
			name: "ordinary method cannot write read-only property",
			source: `public class Probe {
    public static String value { get { return 'ready'; } }
    public static void change() { value = 'changed'; }
}`,
			wantReject: true,
		},
		{
			name: "getter cannot write a different read-only property",
			source: `public class Probe {
    public static String other { get { return 'ready'; } }
    public static String value { get { other = 'changed'; return other; } }
}`,
			wantReject: true,
		},
		{
			name: "local shadows property",
			source: `public class Probe {
    public static String value { get { String value = 'local'; value = 'changed'; return value; } }
}`,
		},
		{
			name: "setter present",
			source: `public class Probe {
    public static String value { get { value = 'ready'; return value; } set; }
}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": test.source}, "41.0")
			if test.wantReject {
				if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA019" || !strings.Contains(result.Diagnostics[0].Message, "property has no setter") {
					t.Fatalf("expected only the missing-setter diagnostic: %#v", result.Diagnostics)
				}
			} else if result.HasErrors() {
				t.Fatalf("assignment rejection = %v, want %v: %#v", result.HasErrors(), test.wantReject, result.Diagnostics)
			}
		})
	}
}

// Exact source rejected by Salesforce, including unchanged unexecuted test classes.
func TestPropertyQualifiedThisGetterRequiresSetter(t *testing.T) {
	t.Run("41", func(t *testing.T) {
		files := map[string]string{
			"GladeB1This41.cls":     "public class GladeB1This41 {\n    public Integer initializations = 0;\n    public String value { get { if (this.value == null) { initializations++; this.value = 'ready'; } return this.value; } }\n}\n",
			"GladeB1This41Test.cls": "@IsTest private class GladeB1This41Test {\n    @IsTest static void repeatedReadsInitializeOnce() {\n        GladeB1This41 a = new GladeB1This41(); GladeB1This41 b = new GladeB1This41();\n        System.assertEquals('ready', a.value); System.assertEquals('ready', a.value);\n        System.assertEquals(1, a.initializations); System.assertEquals(0, b.initializations);\n        System.assertEquals('ready', b.value); System.assertEquals(1, b.initializations);\n    }\n}\n",
		}
		result := analyzeDeclarationProjectWithAPIVersion(t, files, "41.0")
		found := false
		for _, d := range result.Diagnostics {
			if d.Code == "GLADESEMA019" && strings.Contains(d.Message, "property has no setter") {
				found = true
			}
		}
		if !found {
			t.Fatalf("qualified getter assignment accepted: %#v", result.Diagnostics)
		}
	})
	t.Run("42", func(t *testing.T) {
		files := map[string]string{
			"GladeB1This42.cls": "public class GladeB1This42 {\n    public Integer initializations = 0;\n    public String value { get { if (this.value == null) { initializations++; this.value = 'ready'; } return this.value; } }\n}\n",
		}
		result := analyzeDeclarationProjectWithAPIVersion(t, files, "42.0")
		found := false
		for _, d := range result.Diagnostics {
			if d.Code == "GLADESEMA019" && strings.Contains(d.Message, "property has no setter") {
				found = true
			}
		}
		if !found {
			t.Fatalf("qualified getter assignment accepted: %#v", result.Diagnostics)
		}
	})
	t.Run("67", func(t *testing.T) {
		files := map[string]string{
			"GladeB1This67.cls": "public class GladeB1This67 {\n    public Integer initializations = 0;\n    public String value { get { if (this.value == null) { initializations++; this.value = 'ready'; } return this.value; } }\n}\n",
		}
		result := analyzeDeclarationProjectWithAPIVersion(t, files, "67.0")
		found := false
		for _, d := range result.Diagnostics {
			if d.Code == "GLADESEMA019" && strings.Contains(d.Message, "property has no setter") {
				found = true
			}
		}
		if !found {
			t.Fatalf("qualified getter assignment accepted: %#v", result.Diagnostics)
		}
	})
}
