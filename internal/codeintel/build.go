package codeintel

import (
	"os"
	"path/filepath"

	"github.com/glade-sh/glade/internal/typesys"
)

func Build(index typesys.Index) Graph {
	graph := BuildDeclarations(index)
	for _, path := range indexedApexFiles(index) {
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

func indexedApexFiles(index typesys.Index) []string {
	seen := make(map[string]bool)
	var files []string
	add := func(path string, dependency bool) {
		if path == "" || dependency || seen[path] {
			return
		}
		seen[path] = true
		files = append(files, path)
	}
	for _, typ := range index.Types {
		add(typ.File, typ.Dependency || typ.Artifact)
	}
	for _, trigger := range index.Triggers {
		add(trigger.File, trigger.Dependency)
	}
	return files
}

func sourcePath(root, path string) string {
	if filepath.IsAbs(path) || root == "" {
		return path
	}
	return filepath.Join(root, path)
}
