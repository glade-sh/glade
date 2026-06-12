package enterprisegraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestMetadataReferenceScanDetectsControllersComponentsAndInvocables(t *testing.T) {
	dir := t.TempDir()
	page := writeTestFile(t, dir, "AccountWorkbench.page", `<apex:page controller="AccountController" extensions="AccountExtension"></apex:page>`)
	aura := writeTestFile(t, dir, "AccountPanel.cmp", `<aura:component controller="AccountAuraController"><c:accountTile /></aura:component>`)
	lwc := writeTestFile(t, dir, "accountPanel.js", `import save from '@salesforce/apex/AccountAction.save';`)
	flow := writeTestFile(t, dir, "Account_Update.flow-meta.xml", `<actionName>AccountInvocable.updateAccounts</actionName>`)

	var g Graph
	g.AddNode(Node{ID: "AccountController", Kind: NodeKindClass, Name: "AccountController"})
	g.AddNode(Node{ID: "AccountExtension", Kind: NodeKindClass, Name: "AccountExtension"})
	g.AddNode(Node{ID: "AccountAuraController", Kind: NodeKindClass, Name: "AccountAuraController"})
	g.AddNode(Node{ID: "AccountAction", Kind: NodeKindClass, Name: "AccountAction"})
	g.AddNode(Node{ID: "AccountInvocable", Kind: NodeKindClass, Name: "AccountInvocable"})

	addMetadataReferenceEdges(&g, project.Project{
		VisualforcePageFiles: []string{page},
		AuraFiles:            []string{aura},
		LWCFiles:             []string{lwc},
		FlowFiles:            []string{flow},
	})

	assertEdge(t, g, metadataNodeID(page), "AccountController", EdgeKindMetadataReferences)
	assertEdge(t, g, metadataNodeID(page), "AccountExtension", EdgeKindMetadataReferences)
	assertEdge(t, g, metadataNodeID(aura), "AccountAuraController", EdgeKindMetadataReferences)
	assertEdge(t, g, metadataNodeID(lwc), "AccountAction", EdgeKindMetadataReferences)
	assertEdge(t, g, metadataNodeID(flow), "AccountInvocable", EdgeKindMetadataReferences)
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func assertEdge(t *testing.T, g Graph, from, to string, kind EdgeKind) {
	t.Helper()
	for _, edge := range g.Edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return
		}
	}
	t.Fatalf("missing edge %s -[%s]-> %s in %#v", from, kind, to, g.Edges)
}
