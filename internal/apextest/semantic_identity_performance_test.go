package apextest

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/semanticcache"
	"github.com/glade-sh/glade/internal/typesys"
)

func semanticIdentityPerformanceFixture(b *testing.B) (typesys.Index, typesys.BuildArtifacts) {
	b.Helper()
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: b.TempDir()}, schema.Schema{})
	for i := 0; i < 3000; i++ {
		index.Types = append(index.Types, typesys.TypeSymbol{Name: fmt.Sprintf("Class%05d", i), Members: []typesys.MemberSymbol{{Name: "field", Type: "String"}}})
	}
	return index, artifacts
}

func BenchmarkSemanticGateIdentity(b *testing.B) {
	index, artifacts := semanticIdentityPerformanceFixture(b)
	options := sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true, SuppressPerformanceDiagnostics: true, BuildArtifacts: &artifacts}
	want, err := semanticcache.IdentityForBuild(index, &artifacts, options)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := semanticcache.IdentityForBuild(index, &artifacts, options)
		if err != nil || got != want {
			b.Fatal("identity changed", err)
		}
	}
}
