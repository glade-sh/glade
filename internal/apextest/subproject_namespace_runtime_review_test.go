package apextest

import "testing"

func TestSubprojectNamespaceRuntimeReview(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, qualifier := range []string{"ownNsTwin", "OWNNSTWIN"} {
			t.Run(api+"/"+qualifier, func(t *testing.T) {
				index := ownNamespaceProject(t, api, "nested", qualifier, true)
				// The latter two runs share a project and populate then reuse its cache.
				for _, noDiskCache := range []bool{true, false, false} {
					run := Run(index, Options{NoDiskCache: noDiskCache, Parallelism: 1, SelectedClasses: []string{"OwnNsConsumerTest"}})
					s := run.Summary()
					if s.Total != 1 || s.Passed != 1 || s.RuntimeErrors != 0 || s.CompileErrors != 0 {
						t.Fatalf("noDiskCache=%v summary=%+v problem=%s", noDiskCache, s, firstRunProblem(run))
					}
				}
			})
		}
	}
}
