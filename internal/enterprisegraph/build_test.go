package enterprisegraph

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/enterprise"
)

func TestBuildGraphFromEnterpriseComposed(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	ctx, err := enterprise.LoadContext(root)
	if err != nil {
		t.Fatalf("LoadContext() error = %v", err)
	}

	g, err := Build(ctx)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(g.Nodes) == 0 {
		t.Fatalf("nodes = 0, want nonempty")
	}
	if !hasNodeKind(g, NodeKindClass) {
		t.Fatalf("graph has no class nodes")
	}
}

func hasNodeKind(g Graph, kind NodeKind) bool {
	for _, node := range g.Nodes {
		if node.Kind == kind {
			return true
		}
	}
	return false
}
