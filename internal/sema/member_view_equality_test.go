package sema

import (
	"io/fs"
	"os"
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
	roots = append(roots, memberViewFinalWriteProject(t))
	compared, finalFed := 0, 0
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
		// The lifecycle pass rewrites body diagnostics GLADESEMA010 and 027
		// through the member view, so feed it the real ones too.
		for _, d := range a.checkMethodBodiesWithView(index, state.view(), nil) {
			if d.Code == "GLADESEMA010" || d.Code == "GLADESEMA027" {
				input = append(input, d)
				finalFed++
			}
		}
		got, want := nativeLifecycleDiagnostics(index, input, state, nil), nativeLifecycleDiagnostics(index, input, nil, nil)
		if i == len(roots)-1 {
			rewritten := false
			for _, d := range want {
				rewritten = rewritten || d.NativeMessage == "Variable is not visible: FinalBase.q"
			}
			if !rewritten {
				t.Fatalf("final-write project must reach the member-view rewrite: %v", want)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: lifecycle diagnostics differ\nprivate: %v\nshared:  %v", strings.TrimPrefix(root, "../../"), want, got)
		}
		compared++
	}
	if finalFed == 0 {
		t.Fatal("no GLADESEMA010 or GLADESEMA027 diagnostic reached the lifecycle pass")
	}
	if compared < 5 {
		t.Fatalf("compared %d projects, want the generated project and the repo testdata projects", compared)
	}
}

// memberViewFinalWriteProject writes a public API 67 class whose zero-argument
// constructor assigns an inherited private final field of its virtual base.
// Body analysis reports that write as GLADESEMA010, and the lifecycle pass
// reads the member view to give it the native text.
func memberViewFinalWriteProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	classes := filepath.Join(root, "force-app/main/default/classes")
	if err := os.MkdirAll(classes, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	write := func(name, body string) {
		writeSemaFile(t, filepath.Join(classes, name+".cls"), body)
		writeSemaFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	write("FinalBase", "public virtual class FinalBase {\n private final Integer q = 1;\n}\n")
	write("FinalChild", "public class FinalChild extends FinalBase {\n public FinalChild() {\n  q = 7;\n }\n}\n")
	return root
}
