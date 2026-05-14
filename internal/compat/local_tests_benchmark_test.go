package compat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/sema"
)

func BenchmarkAnalyzeSFCredPkg(b *testing.B) {
	root := filepath.Join("..", "..", "example-projects", "sf-cred-pkg-develop")
	if _, err := os.Stat(root); err != nil {
		b.Skipf("sf-cred-pkg fixture unavailable: %v", err)
	}
	index, _, err := loadLocalTestIndex(root)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := sema.Analyze(index)
		if len(result.Diagnostics) == 0 {
			b.Fatal("expected current sf-cred-pkg diagnostics")
		}
	}
}
