package codeintel

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/glade-sh/glade/internal/typesys"
)

type Options struct {
	IncludeDependencies bool
	IncludeUnresolved   bool
	UseCache            bool
}

func Build(index typesys.Index, opts ...Options) Graph {
	var options Options
	if len(opts) > 0 {
		options = opts[0]
	}
	if options.UseCache {
		if cached, _, err := ReadCache(index.Project.Root); err == nil {
			if CacheFresh(index.Project.Root, index) {
				return cached
			}
			if cachedGraphIsSchemaOnly(cached) && len(index.Objects) == 0 && len(index.CustomMetadataRecords) == 0 {
				graph := BuildDeclarations(index)
				mergeCachedGraph(&graph, cached)
				return graph
			}
		}
	}
	graph := BuildDeclarations(index)
	addArtifactContracts(&graph, index.CodeIntelSymbols, index.CodeIntelUses)
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

func cachedGraphIsSchemaOnly(cached Graph) bool {
	for _, symbol := range cached.Symbols {
		switch symbol.Kind {
		case SymbolSObject, SymbolSObjectField, SymbolCustomMetadata:
		default:
			return false
		}
	}
	for _, use := range cached.Uses {
		if use.Kind != UseDeclaration {
			return false
		}
	}
	return len(cached.Symbols) > 0
}

func mergeCachedGraph(graph *Graph, cached Graph) {
	for _, symbol := range cached.Symbols {
		graph.AddSymbol(symbol)
	}
	seen := make(map[string]struct{}, len(graph.Uses))
	for _, use := range graph.Uses {
		seen[useKey(use)] = struct{}{}
	}
	for _, use := range cached.Uses {
		key := useKey(use)
		if _, ok := seen[key]; ok {
			continue
		}
		graph.AddUse(use)
		seen[key] = struct{}{}
	}
}

func useKey(use Use) string {
	return string(use.SymbolID) + "\x00" + string(use.Kind) + "\x00" + use.Name + "\x00" + use.File + "\x00" +
		string(use.Context) + "\x00" + strconv.Itoa(use.Range.Start.Offset) + "\x00" + strconv.Itoa(use.Range.End.Offset)
}

func sourcePath(root, path string) string {
	if filepath.IsAbs(path) || root == "" {
		return path
	}
	return filepath.Join(root, path)
}
