package lwc

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

type NodeType int

const (
	NodeElement NodeType = iota
	NodeText
)

type TemplateNode struct {
	Type       NodeType
	Tag        string
	Attributes map[string]string
	Directives map[string]string
	Text       string
	Children   []*TemplateNode
}

var lwcDirectivePrefixes = []string{
	"if:true", "if:false", "for:each", "for:item", "for:index", "iterator:",
	"key", "lwc:dom", "lwc:ref",
}

func ParseTemplate(source string) (*TemplateNode, error) {
	source = strings.TrimSpace(source)
	if !strings.HasPrefix(source, "<template") {
		return nil, fmt.Errorf("lwc template must start with <template>")
	}
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return nil, err
	}
	root := findElement(doc, "template")
	if root == nil {
		return nil, fmt.Errorf("missing <template> root")
	}
	node := convertNode(root)
	if node == nil {
		return nil, fmt.Errorf("empty template")
	}
	return node, nil
}

func convertNode(node *html.Node) *TemplateNode {
	if node == nil {
		return nil
	}
	switch node.Type {
	case html.TextNode:
		text := node.Data
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return &TemplateNode{Type: NodeText, Text: text}
	case html.ElementNode:
		out := &TemplateNode{
			Type:       NodeElement,
			Tag:        strings.ToLower(node.Data),
			Attributes: map[string]string{},
			Directives: map[string]string{},
		}
		for _, attr := range node.Attr {
			key := attr.Key
			if isLWCDirective(key) {
				out.Directives[key] = attr.Val
				continue
			}
			out.Attributes[key] = attr.Val
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			parsed := convertNode(child)
			if parsed != nil {
				out.Children = append(out.Children, parsed)
			}
		}
		return out
	default:
		return nil
	}
}

func isLWCDirective(key string) bool {
	lower := strings.ToLower(key)
	for _, prefix := range lwcDirectivePrefixes {
		if lower == prefix || strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func findElement(node *html.Node, name string) *html.Node {
	if node == nil {
		return nil
	}
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, name) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findElement(child, name); found != nil {
			return found
		}
	}
	return nil
}
