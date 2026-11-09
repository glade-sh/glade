package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// b5b9_feature_objects/org.tsv: anonymous and named unqualified Builder
// reject on API62/67; the System-qualified builder is accepted on both.
func TestB5B9BuilderNativeTypeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, typ string
			wantError bool
		}{
			{"unqualified", "QueueableDuplicateSignature.Builder", true},
			{"system", "System.QueueableDuplicateSignature.Builder", false},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "OwnedNativeProbe.cls")
				writeSemaFile(t, path, "public class OwnedNativeProbe { public static void run(){ "+tc.typ+" b; } }")
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				named := index.HasErrors() || Analyze(index).HasErrors()
				if named != tc.wantError {
					t.Errorf("named rejected=%t, native=%t: %v", named, tc.wantError, index.Diagnostics)
				}
				empty := typesys.Build(project.Project{Root: root, SourceAPIVersion: api}, schema.Schema{})
				anon := AnalyzeAnonymous(empty, tc.typ+" b;", api).HasErrors()
				if anon != tc.wantError {
					t.Errorf("anonymous rejected=%t, native=%t", anon, tc.wantError)
				}
			})
		}
	}
}
