package sema

import (
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// A shared member view must not change what the declaration-contract pass or
// the lifecycle rewrite report. The nil-state route builds a private model per
// pass, the way the passes did before they shared the analysis' model.
func TestSharedMemberViewKeepsDiagnosticsIdentical(t *testing.T) {
	roots := []string{memberViewSharingProject(t, 60)}
	for _, base := range []string{"../../testdata", "testdata"} {
		_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && d.Name() == "sfdx-project.json" {
				roots = append(roots, filepath.Dir(path))
			}
			return nil
		})
	}
	compared := 0
	for i, root := range roots {
		index, ok := memberViewSharingIndex(t, root)
		if !ok {
			continue
		}
		a := NewAnalyzer()
		a.prepareAnalysisContext(index, AnalyzeOptions{Diagnostics: true})
		index = prepareAnalysisIndexWithSources(index, a.sources)
		a.prepareAnalysisModel(index)
		state := buildSemaTypeMemberState(index, nil, a.sources)
		private := a.checkDeclarationContracts(index, nil)
		shared := a.checkDeclarationContracts(index, state)
		if !reflect.DeepEqual(private, shared) {
			t.Fatalf("%s: declaration contract diagnostics differ\nprivate: %v\nshared:  %v", strings.TrimPrefix(root, "../../"), private, shared)
		}
		if i == 0 {
			clash := false
			for _, d := range private {
				clash = clash || strings.Contains(d.NativeMessage, "Method return types clash")
			}
			if !clash {
				t.Fatalf("generated project must reach the captured redeclaration rules: %v", private)
			}
		}
		input := append(append([]diagnostic.Diagnostic(nil), private...), a.checkMemberTypes(index, state.view())...)
		if got, want := nativeLifecycleDiagnostics(index, input, state), nativeLifecycleDiagnostics(index, input, nil); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: lifecycle diagnostics differ\nprivate: %v\nshared:  %v", strings.TrimPrefix(root, "../../"), want, got)
		}
		compared++
	}
	if compared < 5 {
		t.Fatalf("compared %d projects, want the generated project and the repo testdata projects", compared)
	}
}
