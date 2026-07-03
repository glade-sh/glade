package enterprisegraph

import "testing"

func TestImpactWalksReverseEdgesAndRecommendsTest(t *testing.T) {
	var g Graph
	g.AddNode(Node{ID: "AccountService", Kind: NodeKindClass, Name: "AccountService"})
	g.AddNode(Node{ID: "AccountServiceTest", Kind: NodeKindClass, Name: "AccountServiceTest", Metadata: map[string]string{"test": "true"}})
	g.AddEdge(Edge{From: "AccountServiceTest", To: "AccountService", Kind: EdgeKindCalls})

	result := Impact(g, []string{"AccountService"})

	if !contains(result.Impacted, "AccountServiceTest") {
		t.Fatalf("impacted = %v, want AccountServiceTest", result.Impacted)
	}
	if !contains(result.RecommendedTests, "AccountServiceTest") {
		t.Fatalf("recommended tests = %v, want AccountServiceTest", result.RecommendedTests)
	}
}

func TestImpactDoesNotRecommendNameSuffixTestWithoutMetadata(t *testing.T) {
	var g Graph
	g.AddNode(Node{ID: "AccountService", Kind: NodeKindClass, Name: "AccountService"})
	g.AddNode(Node{ID: "Contest", Kind: NodeKindClass, Name: "Contest"})
	g.AddEdge(Edge{From: "Contest", To: "AccountService", Kind: EdgeKindCalls})

	result := Impact(g, []string{"AccountService"})

	if !contains(result.Impacted, "Contest") {
		t.Fatalf("impacted = %v, want Contest", result.Impacted)
	}
	if contains(result.RecommendedTests, "Contest") {
		t.Fatalf("recommended tests = %v, did not want Contest", result.RecommendedTests)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
