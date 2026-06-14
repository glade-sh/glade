package visualforce

import (
	"strings"
	"testing"
)

func TestParseMarkupTreePreservesScriptStyleAndRawAttributes(t *testing.T) {
	source := `<apex:page>
<script>if (a < b) { console.log("{!Name}"); }</script>
<style>.x::before { content: "{!Name} < ok"; }</style>
<apex:outputText styleClass="x" value="{!Name}"/>
</apex:page>`
	root, err := ParseMarkupTree(source)
	if err != nil {
		t.Fatal(err)
	}
	page := root.Children[0]
	if page.RawName != "apex:page" {
		t.Fatalf("raw page name = %q", page.RawName)
	}
	if page.Line != 1 || page.Column != 1 {
		t.Fatalf("page position = %d:%d, want 1:1", page.Line, page.Column)
	}
	out := findFirstSourceNode(page, "apex", "outputtext")
	if out == nil {
		t.Fatal("missing apex:outputText node")
	}
	if out.RawName != "apex:outputText" {
		t.Fatalf("raw output name = %q", out.RawName)
	}
	if out.RawAttributes["styleClass"] != "x" || out.RawAttributes["value"] != "{!Name}" {
		t.Fatalf("raw attributes = %#v", out.RawAttributes)
	}
	script := findFirstSourceNode(page, "", "script")
	if script == nil {
		t.Fatal("missing script node")
	}
	scriptText := sourceTextChildren(script)
	if !strings.Contains(scriptText, "a < b") || !strings.Contains(scriptText, `{!Name}`) {
		t.Fatalf("script text = %q", scriptText)
	}
	style := findFirstSourceNode(page, "", "style")
	if style == nil {
		t.Fatal("missing style node")
	}
	styleText := sourceTextChildren(style)
	if !strings.Contains(styleText, `{!Name} < ok`) {
		t.Fatalf("style text = %q", styleText)
	}
}

func TestParseMarkupTreePositionsUseOriginalSourceBeforeSelfClosingExpansion(t *testing.T) {
	source := `<apex:page><apex:outputText value="{!First}"/><apex:outputText value="{!Second}"/></apex:page>`
	root, err := ParseMarkupTree(source)
	if err != nil {
		t.Fatal(err)
	}
	page := root.Children[0]
	outputs := collectSourceNodes(page, "apex", "outputtext")
	if len(outputs) != 2 {
		t.Fatalf("outputs = %d", len(outputs))
	}
	wantSecondColumn := strings.Index(source, `<apex:outputText value="{!Second}"`) + 1
	if outputs[1].Column != wantSecondColumn {
		t.Fatalf("second output column = %d, want original column %d", outputs[1].Column, wantSecondColumn)
	}
}

func TestParseMarkupTreeSourceTagsSkipParserInsertedElements(t *testing.T) {
	source := `<apex:page><table><tr><td><apex:outputText value="{!Name}"/></td></tr></table></apex:page>`
	root, err := ParseMarkupTree(source)
	if err != nil {
		t.Fatal(err)
	}
	output := findFirstSourceNode(root, "apex", "outputtext")
	if output == nil {
		t.Fatal("missing apex:outputText")
	}
	wantColumn := strings.Index(source, `<apex:outputText`) + 1
	if output.RawName != "apex:outputText" || output.Column != wantColumn {
		t.Fatalf("output source = %q %d:%d, want apex:outputText column %d", output.RawName, output.Line, output.Column, wantColumn)
	}
}

func findFirstSourceNode(node *MarkupNode, namespace, name string) *MarkupNode {
	if node == nil {
		return nil
	}
	if node.Type == MarkupNodeElement && node.Namespace == namespace && node.Name == name {
		return node
	}
	for _, child := range node.Children {
		if found := findFirstSourceNode(child, namespace, name); found != nil {
			return found
		}
	}
	return nil
}

func sourceTextChildren(node *MarkupNode) string {
	var b strings.Builder
	for _, child := range node.Children {
		if child.Type == MarkupNodeText {
			b.WriteString(child.Text)
		}
	}
	return b.String()
}

func collectSourceNodes(node *MarkupNode, namespace, name string) []*MarkupNode {
	if node == nil {
		return nil
	}
	var out []*MarkupNode
	if node.Type == MarkupNodeElement && node.Namespace == namespace && node.Name == name {
		out = append(out, node)
	}
	for _, child := range node.Children {
		out = append(out, collectSourceNodes(child, namespace, name)...)
	}
	return out
}
