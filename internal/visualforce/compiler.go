package visualforce

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
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
	Type          MarkupNodeType
	Namespace     string
	Name          string
	RawName       string
	Attributes    map[string]string
	RawAttributes map[string]string
	Text          string
	Children      []*MarkupNode
	Line          int
	Column        int
}

func ParseMarkupTree(source string) (*MarkupNode, error) {
	sourceTags := scanSourceTags(source)
	normalized := normalizeSelfClosingVFTags(source)
	nodes, err := html.ParseFragment(bytes.NewReader([]byte(normalized)), &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Body,
		Data:     "body",
	})
	if err != nil {
		return nil, err
	}
	tagIndex := 0
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
		parsed := convertHTMLNode(node, sourceTags, &tagIndex)
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

func convertHTMLNode(node *html.Node, sourceTags []sourceTag, tagIndex *int) *MarkupNode {
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
			Type:          MarkupNodeElement,
			Namespace:     ns,
			Name:          name,
			RawName:       rawName,
			Attributes:    make(map[string]string),
			RawAttributes: make(map[string]string),
		}
		if tag, ok := nextSourceTag(sourceTags, tagIndex, rawName); ok {
			out.RawName = tag.name
			out.RawAttributes = tag.attrs
			out.Line = tag.line
			out.Column = tag.column
		}
		for _, attr := range node.Attr {
			key := strings.ToLower(strings.TrimSpace(attr.Key))
			if key == "" {
				continue
			}
			out.Attributes[key] = attr.Val
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			parsed := convertHTMLNode(child, sourceTags, tagIndex)
			if parsed == nil {
				continue
			}
			out.Children = append(out.Children, parsed)
		}
		return out
	}
	return nil
}

func nextSourceTag(sourceTags []sourceTag, tagIndex *int, rawName string) (sourceTag, bool) {
	if tagIndex == nil {
		return sourceTag{}, false
	}
	want := strings.ToLower(strings.TrimSpace(rawName))
	for i := *tagIndex; i < len(sourceTags); i++ {
		tag := sourceTags[i]
		if strings.EqualFold(tag.name, want) {
			*tagIndex = i + 1
			return tag, true
		}
	}
	return sourceTag{}, false
}

type sourceTag struct {
	name   string
	attrs  map[string]string
	line   int
	column int
}

func scanSourceTags(source string) []sourceTag {
	var tags []sourceTag
	for i := 0; i < len(source); {
		if source[i] != '<' {
			i++
			continue
		}
		if i+1 >= len(source) {
			break
		}
		next := source[i+1]
		if next == '/' || next == '!' || next == '?' {
			i++
			continue
		}
		if !isTagNameByte(next) {
			i++
			continue
		}
		end := findStartTagEnd(source, i+1)
		if end < 0 {
			break
		}
		raw := source[i : end+1]
		line, column := lineColumnAt(source, i)
		name, attrs := parseRawStartTag(raw)
		tags = append(tags, sourceTag{name: name, attrs: attrs, line: line, column: column})
		i = end + 1
		switch strings.ToLower(name) {
		case "script", "style":
			closeTag := "</" + strings.ToLower(name)
			if closeIdx := strings.Index(strings.ToLower(source[i:]), closeTag); closeIdx >= 0 {
				i += closeIdx
			}
		}
	}
	return tags
}

func findStartTagEnd(source string, offset int) int {
	var quote byte
	for i := offset; i < len(source); i++ {
		ch := source[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			continue
		}
		if ch == '>' {
			return i
		}
	}
	return -1
}

func isTagNameByte(ch byte) bool {
	return ch == ':' || ch == '_' || ch == '-' || ch == '.' ||
		ch >= 'A' && ch <= 'Z' ||
		ch >= 'a' && ch <= 'z' ||
		ch >= '0' && ch <= '9'
}

func lineColumnAt(source string, offset int) (int, int) {
	line, column := 1, 1
	for i, r := range source {
		if i >= offset {
			break
		}
		if r == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}

func parseRawStartTag(raw string) (string, map[string]string) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "<")
	raw = strings.TrimSuffix(raw, ">")
	raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "/"))
	nameEnd := 0
	for nameEnd < len(raw) && !unicode.IsSpace(rune(raw[nameEnd])) {
		nameEnd++
	}
	name := raw[:nameEnd]
	attrs := map[string]string{}
	parseRawAttributes(raw[nameEnd:], attrs)
	return name, attrs
}

func parseRawAttributes(raw string, attrs map[string]string) {
	for i := 0; i < len(raw); {
		for i < len(raw) && unicode.IsSpace(rune(raw[i])) {
			i++
		}
		if i >= len(raw) {
			return
		}
		keyStart := i
		for i < len(raw) && !unicode.IsSpace(rune(raw[i])) && raw[i] != '=' {
			i++
		}
		key := strings.TrimSpace(raw[keyStart:i])
		for i < len(raw) && unicode.IsSpace(rune(raw[i])) {
			i++
		}
		if i >= len(raw) || raw[i] != '=' {
			if key != "" {
				attrs[key] = ""
			}
			continue
		}
		i++
		for i < len(raw) && unicode.IsSpace(rune(raw[i])) {
			i++
		}
		if i >= len(raw) {
			attrs[key] = ""
			return
		}
		value := ""
		if raw[i] == '"' || raw[i] == '\'' {
			quote := raw[i]
			i++
			valueStart := i
			for i < len(raw) && raw[i] != quote {
				i++
			}
			value = raw[valueStart:i]
			if i < len(raw) {
				i++
			}
		} else {
			valueStart := i
			for i < len(raw) && !unicode.IsSpace(rune(raw[i])) {
				i++
			}
			value = raw[valueStart:i]
		}
		if key != "" {
			attrs[key] = value
		}
	}
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
