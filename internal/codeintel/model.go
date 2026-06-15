package codeintel

import (
	"sort"

	"github.com/glade-sh/glade/internal/diagnostic"
)

type SymbolKind string

const (
	SymbolApexType       SymbolKind = "apex_type"
	SymbolApexMember     SymbolKind = "apex_member"
	SymbolApexLocal      SymbolKind = "apex_local"
	SymbolTrigger        SymbolKind = "trigger"
	SymbolSObject        SymbolKind = "sobject"
	SymbolSObjectField   SymbolKind = "sobject_field"
	SymbolCustomMetadata SymbolKind = "custom_metadata"
	SymbolLabel          SymbolKind = "label"
	SymbolStaticResource SymbolKind = "static_resource"
	SymbolUnknown        SymbolKind = "unknown"
)

type UseKind string

const (
	UseDeclaration UseKind = "declaration"
	UseRead        UseKind = "read"
	UseWrite       UseKind = "write"
	UseCall        UseKind = "call"
	UseConstruct   UseKind = "construct"
	UseExtends     UseKind = "extends"
	UseImplements  UseKind = "implements"
	UseQuery       UseKind = "query"
	UseMutate      UseKind = "mutate"
	UseMetadata    UseKind = "metadata"
)

type SymbolID string

type Location struct {
	File  string           `json:"file,omitempty"`
	Range diagnostic.Range `json:"range"`
}

type Symbol struct {
	ID         SymbolID          `json:"id"`
	Kind       SymbolKind        `json:"kind"`
	Name       string            `json:"name"`
	Container  SymbolID          `json:"container,omitempty"`
	Namespace  string            `json:"namespace,omitempty"`
	Type       string            `json:"type,omitempty"`
	Signature  string            `json:"signature,omitempty"`
	File       string            `json:"file,omitempty"`
	Range      diagnostic.Range  `json:"range"`
	Dependency bool              `json:"dependency,omitempty"`
	Artifact   bool              `json:"artifact,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type Use struct {
	SymbolID SymbolID          `json:"symbolId,omitempty"`
	Kind     UseKind           `json:"kind"`
	Name     string            `json:"name"`
	File     string            `json:"file"`
	Range    diagnostic.Range  `json:"range"`
	Context  SymbolID          `json:"context,omitempty"`
	Resolved bool              `json:"resolved"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Graph struct {
	ProjectRoot string                  `json:"projectRoot"`
	Symbols     map[SymbolID]Symbol     `json:"symbols"`
	Uses        []Use                   `json:"uses"`
	Diagnostics []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}

func NewGraph(projectRoot string) Graph {
	return Graph{
		ProjectRoot: projectRoot,
		Symbols:     make(map[SymbolID]Symbol),
	}
}

func (g *Graph) AddSymbol(symbol Symbol) {
	if symbol.ID == "" {
		return
	}
	if g.Symbols == nil {
		g.Symbols = make(map[SymbolID]Symbol)
	}
	if existing, ok := g.Symbols[symbol.ID]; ok {
		g.Symbols[symbol.ID] = mergeSymbol(existing, symbol)
		return
	}
	g.Symbols[symbol.ID] = symbol
}

func mergeSymbol(existing, next Symbol) Symbol {
	if existing.Kind == "" || existing.Kind == SymbolUnknown {
		existing.Kind = next.Kind
	}
	if existing.Name == "" {
		existing.Name = next.Name
	}
	if existing.Container == "" {
		existing.Container = next.Container
	}
	if existing.Namespace == "" {
		existing.Namespace = next.Namespace
	}
	if existing.Type == "" {
		existing.Type = next.Type
	}
	if existing.Signature == "" {
		existing.Signature = next.Signature
	}
	if existing.File == "" {
		existing.File = next.File
		existing.Range = next.Range
	}
	existing.Dependency = existing.Dependency || next.Dependency
	existing.Artifact = existing.Artifact || next.Artifact
	if len(next.Metadata) > 0 {
		if existing.Metadata == nil {
			existing.Metadata = make(map[string]string, len(next.Metadata))
		}
		for key, value := range next.Metadata {
			if _, ok := existing.Metadata[key]; !ok {
				existing.Metadata[key] = value
			}
		}
	}
	return existing
}

func (g *Graph) AddUse(use Use) {
	if use.Name == "" && use.SymbolID == "" {
		return
	}
	g.Uses = append(g.Uses, use)
}

func (g Graph) SortedSymbols() []Symbol {
	out := make([]Symbol, 0, len(g.Symbols))
	for _, symbol := range g.Symbols {
		out = append(out, symbol)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			if out[i].Name == out[j].Name {
				return out[i].ID < out[j].ID
			}
			return out[i].Name < out[j].Name
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func (g Graph) Definition(id SymbolID) (Symbol, bool) {
	symbol, ok := g.Symbols[id]
	return symbol, ok
}

func (g Graph) References(id SymbolID, includeDeclaration bool) []Use {
	out := make([]Use, 0)
	for _, use := range g.Uses {
		if use.SymbolID != id {
			continue
		}
		if !includeDeclaration && use.Kind == UseDeclaration {
			continue
		}
		out = append(out, use)
	}
	sortUses(out)
	return out
}

func (g Graph) UsesByFile(file string) []Use {
	out := make([]Use, 0)
	for _, use := range g.Uses {
		if use.File == file {
			out = append(out, use)
		}
	}
	sortUses(out)
	return out
}

func sortUses(uses []Use) {
	sort.Slice(uses, func(i, j int) bool {
		if uses[i].File == uses[j].File {
			if uses[i].Range.Start.Line == uses[j].Range.Start.Line {
				if uses[i].Range.Start.Column == uses[j].Range.Start.Column {
					return uses[i].Name < uses[j].Name
				}
				return uses[i].Range.Start.Column < uses[j].Range.Start.Column
			}
			return uses[i].Range.Start.Line < uses[j].Range.Start.Line
		}
		return uses[i].File < uses[j].File
	})
}
