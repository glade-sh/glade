package enterprisegraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/enterprise"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
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

func TestBuildUsesCodeintelReferencesWithoutCommentWordScanEdges(t *testing.T) {
	root := t.TempDir()
	serviceFile := filepath.Join(root, "Service.cls")
	helperFile := filepath.Join(root, "Helper.cls")
	eventFile := filepath.Join(root, "Shipment__e.cls")
	writeEnterpriseGraphFile(t, serviceFile, `
public class Service {
	public void run() {
	    new Helper().go();
	    List<Account> accounts = [SELECT Id FROM Account];
	    Account account = new Account(Name = 'Acme');
	    insert account;
	    // GhostHelper.go();
	  }
	}`)
	writeEnterpriseGraphFile(t, helperFile, "public class Helper { public void go() {} }")
	writeEnterpriseGraphFile(t, filepath.Join(root, "GhostHelper.cls"), "public class GhostHelper { public static void go() {} }")
	writeEnterpriseGraphFile(t, eventFile, "")

	ctx := enterprise.Context{Index: typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass, Name: "Service", File: serviceFile,
				Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationMethod, Name: "run", Type: "void"}},
			},
			{
				Kind: apexast.DeclarationClass, Name: "Helper", File: helperFile,
				Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationMethod, Name: "go", Type: "void"}},
			},
			{
				Kind: apexast.DeclarationClass, Name: "GhostHelper", File: filepath.Join(root, "GhostHelper.cls"),
				Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationMethod, Name: "go", Type: "void", Modifiers: []string{"static"}}},
			},
		},
		Objects: []schema.Object{{Name: "Account"}, {Name: "Shipment__e"}},
	}}

	g, err := Build(ctx)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertEnterpriseEdge(t, g, "Service", "Helper.go", EdgeKindCalls)
	assertEnterpriseEdge(t, g, "Service", "Helper", EdgeKindReferences)
	assertEnterpriseEdge(t, g, "Service", "Account", EdgeKindQueries)
	assertEnterpriseEdge(t, g, "Service", "Account", EdgeKindMutates)
	assertNoEnterpriseEdge(t, g, "Service", "GhostHelper", EdgeKindReferences)
	assertEnterpriseNode(t, g, "Shipment__e", NodeKindPlatformEvent)
}

func writeEnterpriseGraphFile(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertEnterpriseEdge(t *testing.T, g Graph, from, to string, kind EdgeKind) {
	t.Helper()
	for _, edge := range g.Edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return
		}
	}
	t.Fatalf("missing edge %s -[%s]-> %s; edges=%#v", from, kind, to, g.Edges)
}

func assertNoEnterpriseEdge(t *testing.T, g Graph, from, to string, kind EdgeKind) {
	t.Helper()
	for _, edge := range g.Edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			t.Fatalf("unexpected edge %s -[%s]-> %s; edges=%#v", from, kind, to, g.Edges)
		}
	}
}

func assertEnterpriseNode(t *testing.T, g Graph, id string, kind NodeKind) {
	t.Helper()
	node, ok := g.Nodes[id]
	if !ok {
		t.Fatalf("missing node %s", id)
	}
	if node.Kind != kind {
		t.Fatalf("node %s kind = %q, want %q", id, node.Kind, kind)
	}
}
