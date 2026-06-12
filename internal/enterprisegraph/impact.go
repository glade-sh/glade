package enterprisegraph

import "strings"

type ImpactResult struct {
	Impacted         []string
	RecommendedTests []string
}

func Impact(g Graph, roots []string) ImpactResult {
	seen := make(map[string]bool)
	queue := append([]string{}, roots...)
	for _, root := range roots {
		seen[root] = true
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range g.Edges {
			if edge.To != current || seen[edge.From] {
				continue
			}
			seen[edge.From] = true
			queue = append(queue, edge.From)
		}
	}
	rootSet := make(map[string]bool, len(roots))
	for _, root := range roots {
		rootSet[root] = true
	}
	impactedSet := make(map[string]bool)
	tests := make(map[string]bool)
	for id := range seen {
		if rootSet[id] {
			continue
		}
		impactedSet[id] = true
		if isTestNode(g.Nodes[id]) {
			tests[id] = true
		}
	}
	return ImpactResult{Impacted: sortedIDs(impactedSet), RecommendedTests: sortedIDs(tests)}
}

func isTestNode(node Node) bool {
	if node.Kind == NodeKindTestMethod {
		return true
	}
	if node.Metadata != nil && node.Metadata["test"] == "true" {
		return true
	}
	return strings.HasSuffix(strings.ToLower(node.Name), "test") || strings.HasSuffix(strings.ToLower(node.ID), "test")
}
