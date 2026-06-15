package codeintel

import (
	"github.com/glade-sh/glade/internal/packageartifact"
)

func addArtifactContracts(graph *Graph, symbols []packageartifact.CodeIntelSymbol, uses []packageartifact.CodeIntelUse) {
	for _, symbol := range symbols {
		graph.AddSymbol(symbolFromArtifact(symbol))
	}
	for _, use := range uses {
		graph.AddUse(useFromArtifact(use))
	}
}

func symbolFromArtifact(symbol packageartifact.CodeIntelSymbol) Symbol {
	return Symbol{
		ID:         SymbolID(symbol.ID),
		Kind:       SymbolKind(symbol.Kind),
		Name:       symbol.Name,
		Container:  SymbolID(symbol.Container),
		Namespace:  symbol.Namespace,
		Type:       symbol.Type,
		Signature:  symbol.Signature,
		File:       symbol.File,
		Range:      symbol.Range,
		Dependency: true,
		Artifact:   true,
		Metadata:   copyArtifactMetadata(symbol.Metadata),
	}
}

func useFromArtifact(use packageartifact.CodeIntelUse) Use {
	return Use{
		SymbolID: SymbolID(use.SymbolID),
		Kind:     UseKind(use.Kind),
		Name:     use.Name,
		File:     use.File,
		Range:    use.Range,
		Context:  SymbolID(use.Context),
		Resolved: use.Resolved,
		Metadata: copyArtifactMetadata(use.Metadata),
	}
}

func copyArtifactMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}
