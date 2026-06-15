package codeintel

import "github.com/glade-sh/glade/internal/typesys"

func Build(index typesys.Index) Graph {
	graph := BuildDeclarations(index)
	for _, use := range collectApexUses(index, graph) {
		graph.AddUse(use)
	}
	return graph
}
