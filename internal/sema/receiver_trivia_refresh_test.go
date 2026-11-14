package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The source resolver and receiver scan are analysis-local. Reusing an
// Analyzer must observe both source edits and the effective API version.
func TestReceiverAnalysisRefresh(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "RefreshProbe.cls")
	analyzer := NewAnalyzer()
	for _, tc := range []struct {
		api, separator string
		wantError      bool
	}{
		{"67.0", "", false},
		{"67.0", "\n", true},
		{"68.0", "\n", false},
		{"67.0", "", false},
	} {
		writeSemaFile(t, path, "public class RefreshProbe { public static void f() {} public static void run() { RefreshProbe."+tc.separator+"f(); } }")
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: tc.api, ApexFiles: []string{path}}, schema.Schema{})
		result := analyzer.Analyze(index)
		found := false
		for _, d := range result.Diagnostics {
			if d.NativeMessage == "Missing ';' at 'f'" {
				found = true
			}
		}
		if found != tc.wantError {
			t.Errorf("api %s separator %q: diagnostics %v", tc.api, tc.separator, result.Diagnostics)
		}
	}
}
