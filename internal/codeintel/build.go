package codeintel

import (
	"os"
	"path/filepath"

	"github.com/glade-sh/glade/internal/typesys"
)

func Build(index typesys.Index) Graph {
	graph := BuildDeclarations(index)
	for _, use := range collectApexUses(index, graph) {
		graph.AddUse(use)
	}
	for _, use := range BuildSchemaUses(index).Uses {
		if use.Kind != UseDeclaration {
			graph.AddUse(use)
		}
	}
	for _, path := range apexUseFiles(index) {
		source, err := os.ReadFile(sourcePath(index.Project.Root, path))
		if err != nil {
			continue
		}
		for _, use := range CollectSOSLUses(path, string(source)) {
			graph.AddUse(use)
		}
	}
	return graph
}

func sourcePath(root, path string) string {
	if filepath.IsAbs(path) || root == "" {
		return path
	}
	return filepath.Join(root, path)
}
