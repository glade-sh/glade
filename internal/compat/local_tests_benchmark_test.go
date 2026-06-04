package compat

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func BenchmarkAnalyzeLocalTestProject(b *testing.B) {
	root := os.Getenv("GLADE_LOCAL_TEST_BENCH_PROJECT")
	if root == "" {
		b.Skip("set GLADE_LOCAL_TEST_BENCH_PROJECT to benchmark a local test project")
	}
	if _, err := os.Stat(root); err != nil {
		b.Skipf("local test benchmark project unavailable: %v", err)
	}
	index, _, err := loadLocalTestIndex(root)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := sema.Analyze(index)
		if len(result.Diagnostics) == 0 {
			b.Fatal("expected diagnostics from benchmark project")
		}
	}
}
