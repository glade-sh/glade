package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native capture: testdata/case377_trigger_token/org.tsv, API 62.0 and 67.0.
func TestCase377TriggerTokenNative(t *testing.T) {
	for _, version := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, object string
			rejected     bool
		}{
			{"reference-only", "FieldServiceMobileExtension", false},
			{"boundary", "Account", false},
			{"unknown", "Case377Missing", true},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, "Case377Probe.trigger")
				writeSemaFile(t, file, "trigger Case377Probe on Account (before insert) { Schema.SObjectType token = "+tc.object+".SObjectType; }")
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: version, ApexFiles: []string{file}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
				result := Analyze(index)
				found := false
				for _, diagnostic := range result.Diagnostics {
					if diagnostic.NativeMessage == "Variable does not exist: "+tc.object {
						found = true
					}
				}
				if found != tc.rejected {
					t.Fatalf("object diagnostic found=%v, want %v; all diagnostics: %#v", found, tc.rejected, result.Diagnostics)
				}
			})
		}
	}
}
