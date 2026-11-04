package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The foreign-qualifier rejection has native coverage in
// subproject_namespace_runtime/org.tsv:4,7. Project layout is a local contract.
func TestUnusedAncestorNamespaceRoutesReview(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"packs/child/force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeFile(t, filepath.Join(root, "packs", "sfdx-project.json"), `{"namespace":"First"}`)
			writeFile(t, filepath.Join(root, "packs", "child", "sfdx-project.json"), `{"namespace":"Second"}`)
			classes := filepath.Join(root, "packs", "child", "force-app", "classes")
			for name, body := range map[string]string{"OwnNsOuter": ownNamespaceOuter, "OwnNsConsumer": ownNamespaceConsumer("First"), "OwnNsConsumerTest": ownNamespaceConsumerTest("First")} {
				writeFile(t, filepath.Join(classes, name+".cls"), body)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, schema.Schema{})
			want := "Invalid type: First.OwnNsOuter.OwnNsInner"
			hasError := func(ds []diagnostic.Diagnostic) bool {
				for _, d := range ds {
					if d.Severity == diagnostic.Error && d.NativeMessage == want {
						return true
					}
				}
				return false
			}
			if !hasError(sema.Analyze(index).Diagnostics) {
				t.Fatal("named source accepted unused ancestor qualifier")
			}
			if !hasError(sema.AnalyzeAnonymous(index, ownNamespaceAnonymous("First"), api).Diagnostics) {
				t.Fatal("anonymous source accepted unused ancestor qualifier")
			}
			for _, warm := range []bool{false, true} {
				run := Run(index, Options{NoDiskCache: !warm, Parallelism: 1, SelectedClasses: []string{"OwnNsConsumerTest"}})
				if run.Summary().CompileErrors != 1 || !strings.Contains(firstRunProblem(run), want) {
					t.Fatalf("warm=%v summary=%+v problem=%s", warm, run.Summary(), firstRunProblem(run))
				}
			}
		})
	}
}
