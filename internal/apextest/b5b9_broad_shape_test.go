package apextest

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// The synthetic broad-shape project exercises unrelated type-resolution and
// runtime routes at the semantic rejection boundary; it is not a native row.
func TestB5B9BroadShapeSyntheticProject(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/b5b9_broad_shape")); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("synthetic project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	result := Run(index, Options{SelectedClasses: []string{"OwnedBroadTest"}, NoDiskCache: true, Parallelism: 1})
	if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem != nil {
		t.Fatalf("synthetic test: %+v", result.Summary())
	}
}
