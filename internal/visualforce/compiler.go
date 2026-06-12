package visualforce

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var selfClosingVFTagRE = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9:.-]*)([^>]*)/>`)

func normalizeSelfClosingVFTags(source string) string {
	return selfClosingVFTagRE.ReplaceAllString(source, "<$1$2></$1>")
}

// MarkupNodeType enumerates parsed Visualforce node kinds used by the renderer.
type MarkupNodeType int

const (
	MarkupNodeElement MarkupNodeType = iota
	MarkupNodeText
)

type MarkupNode struct {
	Type       MarkupNodeType
	Namespace  string
	Name       string
	Attributes map[string]string
	Text       string
	Children   []*MarkupNode
}

func ParseMarkupTree(source string) (*MarkupNode, error) {
	source = normalizeSelfClosingVFTags(source)
	nodes, err := html.ParseFragment(bytes.NewReader([]byte(source)), nil)
	if err != nil {
		return nil, err
	}
	var filtered []*html.Node
	for _, node := range nodes {
		if isSkippableNode(node) {
			continue
		}
		filtered = append(filtered, node)
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no Visualforce markup to parse")
	}
	root := &MarkupNode{
		Type:       MarkupNodeElement,
		Name:       "_vfroot",
		Attributes: map[string]string{},
	}
	for _, node := range filtered {
		parsed := convertHTMLNode(node)
		if parsed == nil {
			continue
		}
		root.Children = append(root.Children, parsed)
	}
	if len(root.Children) == 0 {
		return nil, fmt.Errorf("no renderable Visualforce markup")
	}
	return root, nil
}

func isSkippableNode(node *html.Node) bool {
	if node == nil {
		return true
	}
	switch node.Type {
	case html.CommentNode, html.DoctypeNode:
		return true
	}
	if node.Type == html.TextNode {
		return strings.TrimSpace(node.Data) == ""
	}
	return false
}

func convertHTMLNode(node *html.Node) *MarkupNode {
	if node == nil {
		return nil
	}
	switch node.Type {
	case html.TextNode:
		return &MarkupNode{Type: MarkupNodeText, Text: node.Data}
	case html.ElementNode:
		rawName := strings.TrimSpace(node.Data)
		if rawName == "" {
			return nil
		}
		ns := ""
		name := strings.ToLower(rawName)
		if idx := strings.Index(name, ":"); idx >= 0 {
			ns = name[:idx]
			name = name[idx+1:]
		}
		out := &MarkupNode{
			Type:       MarkupNodeElement,
			Namespace:  ns,
			Name:       name,
			Attributes: make(map[string]string),
		}
		for _, attr := range node.Attr {
			key := strings.ToLower(strings.TrimSpace(attr.Key))
			if key == "" {
				continue
			}
			out.Attributes[key] = attr.Val
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			parsed := convertHTMLNode(child)
			if parsed == nil {
				continue
			}
			out.Children = append(out.Children, parsed)
		}
		return out
	}
	return nil
}

func (node *MarkupNode) HasAttribute(name string) bool {
	if node == nil || node.Attributes == nil {
		return false
	}
	_, ok := node.Attributes[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func (node *MarkupNode) Attribute(name string) string {
	if node == nil || node.Attributes == nil {
		return ""
	}
	return strings.TrimSpace(node.Attributes[strings.ToLower(strings.TrimSpace(name))])
}
