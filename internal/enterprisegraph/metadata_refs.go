package enterprisegraph

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

func addMetadataReferenceEdges(g *Graph, p project.Project) {
	for _, path := range metadataFiles(p) {
		g.AddNode(Node{ID: metadataNodeID(path), Kind: NodeKindMetadataFile, Name: filepath.Base(path), File: path})
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, ref := range metadataReferences(string(data)) {
			if _, ok := g.Nodes[ref]; ok {
				g.AddEdge(Edge{From: metadataNodeID(path), To: ref, Kind: EdgeKindMetadataReferences})
			}
		}
	}
}

func metadataNodeID(path string) string {
	return "metadata:" + filepath.Clean(path)
}

func metadataFiles(p project.Project) []string {
	var out []string
	out = append(out, p.VisualforcePageFiles...)
	out = append(out, p.VisualforceComponentFiles...)
	out = append(out, p.AuraFiles...)
	out = append(out, p.LWCFiles...)
	out = append(out, p.LWCHTMLFiles...)
	out = append(out, p.LWCMetaFiles...)
	out = append(out, p.FlowFiles...)
	out = append(out, p.WorkflowFiles...)
	out = append(out, p.CustomMetadataFiles...)
	out = append(out, p.FlexiPageFiles...)
	out = append(out, p.ApplicationFiles...)
	return out
}

func metadataReferences(source string) []string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:controller|extensions)="([A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*)"`),
		regexp.MustCompile(`(?i)@salesforce/apex/([A-Za-z_][A-Za-z0-9_]*)\.[A-Za-z_][A-Za-z0-9_]*`),
		regexp.MustCompile(`(?i)<actionName>\s*([A-Za-z_][A-Za-z0-9_]*)\.[A-Za-z_][A-Za-z0-9_]*\s*</actionName>`),
	}
	seen := make(map[string]bool)
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(source, -1) {
			for _, part := range strings.Split(match[1], ",") {
				ref := strings.TrimSpace(part)
				if ref != "" {
					seen[ref] = true
				}
			}
		}
	}
	return sortedIDs(seen)
}
