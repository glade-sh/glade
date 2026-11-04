package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native rows: subproject_namespace_runtime/org.tsv:2,5. The subproject
// layout itself is a local project-loading contract.
func TestSubprojectNamespaceAnonymousExecutionReview(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, qualifier := range []string{"ownNsTwin", "OWNNSTWIN"} {
			t.Run(api+"/"+qualifier, func(t *testing.T) {
				index := ownNamespaceProject(t, api, "nested", qualifier, true)
				source := ownNamespaceAnonymous(qualifier)
				if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
					t.Fatalf("anonymous diagnostics: %v", analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
