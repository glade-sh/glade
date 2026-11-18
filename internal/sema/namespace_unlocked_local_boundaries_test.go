package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestUnlockedSubscriberLocalAliasBoundaries(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, source string
			dependency   typesys.DependencyInfo
			reject       bool
		}{
			{"ambiguous unnamespaced alias has no inventory", "V036PkgOuter.Box localValue;", typesys.DependencyInfo{Namespace: "v036-unlocked", Status: "ambiguous"}, true},
			{"missing configured namespace", "V036PkgOuter.Box localValue;", typesys.DependencyInfo{Namespace: "otherns", Status: "missing", SourceRoot: "/synthetic/absent"}, true},
			{"three-part unknown", "V036PkgOuter.Box.Absent localValue;", typesys.DependencyInfo{Namespace: "v036-unlocked", Status: "missing"}, true},
			{"closed platform local", "System.String localValue;", typesys.DependencyInfo{Namespace: "v036-unlocked", Status: "missing"}, true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				index := typesys.Build(project.Project{Root: t.TempDir(), SourceAPIVersion: api}, schema.Schema{})
				index.Dependencies = []typesys.DependencyInfo{tc.dependency}
				var errors int
				for _, d := range AnalyzeAnonymous(index, tc.source, api).Diagnostics {
					if d.Severity == diagnostic.Error {
						errors++
					}
				}
				if (errors != 0) != tc.reject {
					t.Fatalf("diagnostic count=%d, reject=%v", errors, tc.reject)
				}
			})
		}
	}
}
