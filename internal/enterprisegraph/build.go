package enterprisegraph

import (
	"os"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/enterprise"
)

func Build(ctx enterprise.Context) (Graph, error) {
	var g Graph
	for _, typ := range ctx.Index.Types {
		if typ.Dependency {
			continue
		}
		kind := nodeKindFromDeclaration(typ.Kind)
		g.AddNode(Node{ID: typ.Name, Kind: kind, Name: typ.Name, File: typ.File, Metadata: testMetadata(typ.IsTest, typ.Name)})
		for _, member := range typ.Members {
			memberKind := nodeKindFromMember(member.Kind, member.IsTest)
			if memberKind == "" {
				continue
			}
			id := typ.Name + "." + member.Name
			g.AddNode(Node{ID: id, Kind: memberKind, Name: member.Name, File: typ.File, Metadata: testMetadata(member.IsTest || typ.IsTest, id)})
			g.AddEdge(Edge{From: typ.Name, To: id, Kind: EdgeKindReferences})
		}
		if typ.SuperClass != "" {
			g.AddNode(Node{ID: typ.SuperClass, Kind: NodeKindClass, Name: typ.SuperClass})
			g.AddEdge(Edge{From: typ.Name, To: typ.SuperClass, Kind: EdgeKindExtends})
		}
		for _, iface := range typ.Interfaces {
			g.AddNode(Node{ID: iface, Kind: NodeKindInterface, Name: iface})
			g.AddEdge(Edge{From: typ.Name, To: iface, Kind: EdgeKindImplements})
		}
	}
	for _, trigger := range ctx.Index.Triggers {
		if trigger.Dependency {
			continue
		}
		g.AddNode(Node{ID: trigger.Name, Kind: NodeKindTrigger, Name: trigger.Name, File: trigger.File})
		if trigger.ObjectName != "" {
			g.AddNode(Node{ID: trigger.ObjectName, Kind: sobjectKind(trigger.ObjectName), Name: trigger.ObjectName})
			g.AddEdge(Edge{From: trigger.Name, To: trigger.ObjectName, Kind: EdgeKindReferences})
		}
	}
	for _, object := range ctx.Index.Objects {
		g.AddNode(Node{ID: object.Name, Kind: sobjectKind(object.Name), Name: object.Name})
	}
	addCodeintelEdges(&g, ctx)
	addSourceScanEdges(&g, ctx)
	addMetadataReferenceEdges(&g, ctx.Project)
	return g, nil
}

func nodeKindFromDeclaration(kind apexast.DeclarationKind) NodeKind {
	switch kind {
	case apexast.DeclarationClass:
		return NodeKindClass
	case apexast.DeclarationInterface:
		return NodeKindInterface
	case apexast.DeclarationEnum:
		return NodeKindEnum
	default:
		return NodeKindClass
	}
}

func nodeKindFromMember(kind apexast.DeclarationKind, test bool) NodeKind {
	if test {
		return NodeKindTestMethod
	}
	switch kind {
	case apexast.DeclarationMethod:
		return NodeKindMethod
	case apexast.DeclarationField:
		return NodeKindField
	default:
		return ""
	}
}

func testMetadata(test bool, name string) map[string]string {
	if test {
		return map[string]string{"test": "true"}
	}
	return nil
}

func sobjectKind(name string) NodeKind {
	if strings.HasSuffix(strings.ToLower(name), "__e") {
		return NodeKindPlatformEvent
	}
	return NodeKindSObject
}

func addCodeintelEdges(g *Graph, ctx enterprise.Context) {
	cg := codeintel.Build(ctx.Index, codeintel.Options{})
	for _, symbol := range cg.SortedSymbols() {
		node := nodeFromCodeintelSymbol(symbol)
		if node.ID != "" {
			g.AddNode(node)
		}
	}
	typeByFile := typeNameByFile(ctx)
	for _, use := range cg.Uses {
		if !use.Resolved || use.Kind == codeintel.UseDeclaration {
			continue
		}
		from := typeByFile[cleanEnterprisePath(use.File)]
		if from == "" {
			continue
		}
		symbol, ok := cg.Definition(use.SymbolID)
		if !ok {
			continue
		}
		to := nodeIDFromCodeintelSymbol(symbol)
		kind := edgeKindFromCodeintelUse(use.Kind)
		if to == "" || kind == "" || to == from {
			continue
		}
		g.AddEdge(Edge{From: from, To: to, Kind: kind})
		if symbol.Kind == codeintel.SymbolApexMember && symbol.Container != "" {
			if owner := apexTypeNodeID(cg, symbol.Container); owner != "" && owner != from {
				g.AddEdge(Edge{From: from, To: owner, Kind: EdgeKindReferences})
			}
		}
	}
}

func nodeFromCodeintelSymbol(symbol codeintel.Symbol) Node {
	switch symbol.Kind {
	case codeintel.SymbolApexType:
		return Node{
			ID:       symbol.Name,
			Kind:     nodeKindFromCodeintelType(symbol),
			Name:     symbol.Name,
			File:     symbol.File,
			Metadata: testMetadata(symbol.Metadata["test"] == "true", symbol.Name),
		}
	case codeintel.SymbolApexMember:
		owner := symbol.Metadata["owner"]
		if owner == "" {
			owner = apexOwnerFromMemberID(symbol.ID)
		}
		if owner == "" {
			return Node{}
		}
		id := owner + "." + symbol.Name
		return Node{
			ID:       id,
			Kind:     nodeKindFromCodeintelMember(symbol),
			Name:     symbol.Name,
			File:     symbol.File,
			Metadata: testMetadata(symbol.Metadata["test"] == "true", id),
		}
	case codeintel.SymbolSObject:
		return Node{ID: symbol.Name, Kind: sobjectKind(symbol.Name), Name: symbol.Name}
	default:
		return Node{}
	}
}

func nodeIDFromCodeintelSymbol(symbol codeintel.Symbol) string {
	node := nodeFromCodeintelSymbol(symbol)
	return node.ID
}

func nodeKindFromCodeintelType(symbol codeintel.Symbol) NodeKind {
	switch apexast.DeclarationKind(symbol.Metadata["declarationKind"]) {
	case apexast.DeclarationInterface:
		return NodeKindInterface
	case apexast.DeclarationEnum:
		return NodeKindEnum
	default:
		return NodeKindClass
	}
}

func nodeKindFromCodeintelMember(symbol codeintel.Symbol) NodeKind {
	if symbol.Metadata["test"] == "true" {
		return NodeKindTestMethod
	}
	switch apexast.DeclarationKind(symbol.Metadata["declarationKind"]) {
	case apexast.DeclarationMethod:
		return NodeKindMethod
	case apexast.DeclarationField, apexast.DeclarationProperty:
		return NodeKindField
	default:
		return NodeKindMethod
	}
}

func edgeKindFromCodeintelUse(kind codeintel.UseKind) EdgeKind {
	switch kind {
	case codeintel.UseCall:
		return EdgeKindCalls
	case codeintel.UseRead, codeintel.UseWrite, codeintel.UseConstruct:
		return EdgeKindReferences
	case codeintel.UseQuery:
		return EdgeKindQueries
	case codeintel.UseMutate:
		return EdgeKindMutates
	case codeintel.UseMetadata:
		return EdgeKindMetadataReferences
	default:
		return ""
	}
}

func apexTypeNodeID(g codeintel.Graph, id codeintel.SymbolID) string {
	symbol, ok := g.Definition(id)
	if !ok || symbol.Kind != codeintel.SymbolApexType {
		return ""
	}
	return symbol.Name
}

func apexOwnerFromMemberID(id codeintel.SymbolID) string {
	parts := codeintel.ParseID(id)
	if len(parts) >= 5 && parts[0] == "apex" && parts[1] == "member" {
		return parts[3]
	}
	return ""
}

func typeNameByFile(ctx enterprise.Context) map[string]string {
	out := make(map[string]string, len(ctx.Index.Types))
	for _, typ := range ctx.Index.Types {
		if typ.Dependency || typ.File == "" {
			continue
		}
		out[cleanEnterprisePath(typ.File)] = typ.Name
	}
	return out
}

func cleanEnterprisePath(path string) string {
	if path == "" {
		return ""
	}
	return strings.TrimSpace(path)
}

func addSourceScanEdges(g *Graph, ctx enterprise.Context) {
	for _, typ := range ctx.Index.Types {
		if typ.Dependency || typ.File == "" {
			continue
		}
		data, err := os.ReadFile(typ.File)
		if err != nil {
			continue
		}
		source := string(data)
		if strings.Contains(source, "System.enqueueJob") {
			g.AddEdge(Edge{From: typ.Name, To: "ApexQueue", Kind: EdgeKindEnqueues})
		}
		for _, endpoint := range endpoints(source) {
			id := "endpoint:" + endpoint
			g.AddNode(Node{ID: id, Kind: NodeKindExternalEndpoint, Name: endpoint})
			g.AddEdge(Edge{From: typ.Name, To: id, Kind: EdgeKindCalloutTo})
		}
	}
}

func endpoints(source string) []string {
	re := regexp.MustCompile(`(?i)(?:https?://|callout:)[A-Za-z0-9_./:%?=&-]+`)
	matches := re.FindAllString(source, -1)
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		seen[strings.Trim(match, `"'`)] = true
	}
	return sortedIDs(seen)
}
