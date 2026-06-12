package enterprisegraph

import "sort"

type NodeKind string

const (
	NodeKindClass            NodeKind = "class"
	NodeKindInterface        NodeKind = "interface"
	NodeKindEnum             NodeKind = "enum"
	NodeKindMethod           NodeKind = "method"
	NodeKindField            NodeKind = "field"
	NodeKindTrigger          NodeKind = "trigger"
	NodeKindTestMethod       NodeKind = "test_method"
	NodeKindMetadataFile     NodeKind = "metadata_file"
	NodeKindSObject          NodeKind = "sobject"
	NodeKindExternalEndpoint NodeKind = "external_endpoint"
	NodeKindPlatformEvent    NodeKind = "platform_event"
)

type EdgeKind string

const (
	EdgeKindCalls              EdgeKind = "calls"
	EdgeKindReferences         EdgeKind = "references"
	EdgeKindExtends            EdgeKind = "extends"
	EdgeKindImplements         EdgeKind = "implements"
	EdgeKindQueries            EdgeKind = "queries"
	EdgeKindMutates            EdgeKind = "mutates"
	EdgeKindEnqueues           EdgeKind = "enqueues"
	EdgeKindPublishes          EdgeKind = "publishes"
	EdgeKindCalloutTo          EdgeKind = "callout_to"
	EdgeKindTestCovers         EdgeKind = "test_covers"
	EdgeKindMetadataReferences EdgeKind = "metadata_references"
	EdgeKindExposesAPI         EdgeKind = "exposes_api"
)

type Node struct {
	ID       string            `json:"id"`
	Kind     NodeKind          `json:"kind"`
	Name     string            `json:"name"`
	File     string            `json:"file,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Edge struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Kind EdgeKind `json:"kind"`
}

type Graph struct {
	Nodes map[string]Node `json:"nodes"`
	Edges []Edge          `json:"edges"`
}

func (g *Graph) AddNode(node Node) {
	if node.ID == "" {
		return
	}
	if g.Nodes == nil {
		g.Nodes = make(map[string]Node)
	}
	if existing, ok := g.Nodes[node.ID]; ok {
		if existing.Kind == "" && node.Kind != "" {
			existing.Kind = node.Kind
		}
		if existing.Name == "" && node.Name != "" {
			existing.Name = node.Name
		}
		if existing.File == "" && node.File != "" {
			existing.File = node.File
		}
		if len(node.Metadata) > 0 {
			if existing.Metadata == nil {
				existing.Metadata = make(map[string]string, len(node.Metadata))
			}
			for key, value := range node.Metadata {
				if _, exists := existing.Metadata[key]; !exists {
					existing.Metadata[key] = value
				}
			}
		}
		g.Nodes[node.ID] = existing
		return
	}
	g.Nodes[node.ID] = node
}

func (g *Graph) AddEdge(edge Edge) {
	if edge.From == "" || edge.To == "" || edge.Kind == "" {
		return
	}
	for _, existing := range g.Edges {
		if existing == edge {
			return
		}
	}
	g.Edges = append(g.Edges, edge)
}

func sortedIDs(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
