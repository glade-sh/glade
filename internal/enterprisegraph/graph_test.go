package enterprisegraph

import "testing"

func TestGraphAddNodeAddEdgeDedupe(t *testing.T) {
	var g Graph

	g.AddNode(Node{ID: "AccountService", Kind: NodeKindClass, Name: "AccountService"})
	g.AddNode(Node{ID: "AccountService", Kind: NodeKindClass, Name: "AccountService"})
	g.AddEdge(Edge{From: "AccountService", To: "AccountSelector", Kind: EdgeKindCalls})
	g.AddEdge(Edge{From: "AccountService", To: "AccountSelector", Kind: EdgeKindCalls})

	if got := len(g.Nodes); got != 1 {
		t.Fatalf("nodes = %d, want 1", got)
	}
	if got := len(g.Edges); got != 1 {
		t.Fatalf("edges = %d, want 1", got)
	}
	if _, ok := g.Nodes["AccountService"]; !ok {
		t.Fatalf("missing AccountService node")
	}
}

func TestGraphAddNodeMergesPlaceholderData(t *testing.T) {
	var g Graph

	g.AddNode(Node{ID: "BaseService", Kind: NodeKindClass, Name: "BaseService"})
	g.AddNode(Node{ID: "BaseService", Kind: NodeKindClass, Name: "BaseService", File: "/tmp/BaseService.cls", Metadata: map[string]string{"test": "true"}})

	node := g.Nodes["BaseService"]
	if node.File != "/tmp/BaseService.cls" {
		t.Fatalf("file = %q, want merged file", node.File)
	}
	if node.Metadata["test"] != "true" {
		t.Fatalf("metadata = %#v, want test=true", node.Metadata)
	}
}
