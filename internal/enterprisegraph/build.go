package enterprisegraph

import (
	"os"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
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
	if test || strings.HasSuffix(strings.ToLower(name), "test") {
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

func addSourceScanEdges(g *Graph, ctx enterprise.Context) {
	classIDs := nodeIDsByKind(*g, NodeKindClass, NodeKindInterface, NodeKindEnum)
	objectIDs := nodeIDsByKind(*g, NodeKindSObject, NodeKindPlatformEvent)
	for _, typ := range ctx.Index.Types {
		if typ.Dependency || typ.File == "" {
			continue
		}
		data, err := os.ReadFile(typ.File)
		if err != nil {
			continue
		}
		source := string(data)
		for _, id := range classIDs {
			if id == typ.Name {
				continue
			}
			if wordAppears(source, id) {
				g.AddEdge(Edge{From: typ.Name, To: id, Kind: EdgeKindReferences})
			}
			if strings.Contains(source, id+".") || strings.Contains(source, "new "+id+"(") {
				g.AddEdge(Edge{From: typ.Name, To: id, Kind: EdgeKindCalls})
			}
		}
		for _, id := range objectIDs {
			if strings.Contains(source, "FROM "+id) || strings.Contains(source, "from "+id) {
				g.AddEdge(Edge{From: typ.Name, To: id, Kind: EdgeKindQueries})
			}
			if mutatesObject(source, id) {
				g.AddEdge(Edge{From: typ.Name, To: id, Kind: EdgeKindMutates})
			}
			if strings.Contains(source, "EventBus.publish") && wordAppears(source, id) {
				g.AddEdge(Edge{From: typ.Name, To: id, Kind: EdgeKindPublishes})
			}
		}
		if strings.Contains(source, "System.enqueueJob") {
			g.AddEdge(Edge{From: typ.Name, To: "ApexQueue", Kind: EdgeKindEnqueues})
		}
		for _, endpoint := range endpoints(source) {
			id := "endpoint:" + endpoint
			g.AddNode(Node{ID: id, Kind: NodeKindExternalEndpoint, Name: endpoint})
			g.AddEdge(Edge{From: typ.Name, To: id, Kind: EdgeKindCalloutTo})
		}
		if typ.IsTest || strings.HasSuffix(strings.ToLower(typ.Name), "test") {
			for _, id := range classIDs {
				if id != typ.Name && wordAppears(source, id) {
					g.AddEdge(Edge{From: typ.Name, To: id, Kind: EdgeKindTestCovers})
				}
			}
		}
	}
}

func nodeIDsByKind(g Graph, kinds ...NodeKind) []string {
	want := make(map[NodeKind]bool, len(kinds))
	for _, kind := range kinds {
		want[kind] = true
	}
	seen := make(map[string]bool)
	for id, node := range g.Nodes {
		if want[node.Kind] {
			seen[id] = true
		}
	}
	return sortedIDs(seen)
}

func wordAppears(source, word string) bool {
	return regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(word)+`\b`).FindStringIndex(source) != nil
}

func mutatesObject(source, object string) bool {
	lower := strings.ToLower(source)
	return strings.Contains(lower, "insert ") && wordAppears(source, object) ||
		strings.Contains(lower, "update ") && wordAppears(source, object) ||
		strings.Contains(lower, "delete ") && wordAppears(source, object) ||
		strings.Contains(lower, "upsert ") && wordAppears(source, object) ||
		strings.Contains(lower, "database.insert") && wordAppears(source, object) ||
		strings.Contains(lower, "database.update") && wordAppears(source, object) ||
		strings.Contains(lower, "database.delete") && wordAppears(source, object) ||
		strings.Contains(lower, "database.upsert") && wordAppears(source, object)
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
