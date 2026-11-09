package apextest

import (
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

func TestSObjectShadowNativeProject(t *testing.T) {
	runCase159ProjectVersions(t, "case159_shadow", func(t *testing.T, index typesys.Index) {
		run := Run(index, Options{SelectedClasses: []string{"Case159ShadowTest"}, NoDiskCache: true})
		if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
			t.Fatalf("shadow record and list casts: summary=%#v problem=%s", summary, firstRunProblem(run))
		}
	})
}
