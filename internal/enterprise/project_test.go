package enterprise

import (
	"path/filepath"
	"testing"
)

func TestLoadContextBuildsProjectSummary(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	ctx, err := LoadContext(root)
	if err != nil {
		t.Fatalf("LoadContext: %v", err)
	}
	if ctx.Project.Root == "" {
		t.Fatalf("project root missing")
	}
	summary := ctx.Summary()
	if summary.ApexClasses == 0 {
		t.Fatalf("expected Apex classes in summary: %#v", summary)
	}
	if summary.MetadataFiles == 0 {
		t.Fatalf("expected metadata files in summary: %#v", summary)
	}
}
