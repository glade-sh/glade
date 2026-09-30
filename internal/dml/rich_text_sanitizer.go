package dml

import (
	"html"
	"net/url"
	"strings"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// sanitizeRichText persists the measured rich-text normalization rules. It is
// deliberately a storage transform: callers retain their supplied SObject text.
func sanitizeRichText(input string) string {
	input = strings.Trim(input, " ")
	nodes, err := nethtml.ParseFragment(strings.NewReader(input), &nethtml.Node{
		Type:     nethtml.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	})
	if err != nil {
		return escapeRichText(input)
	}
	var out strings.Builder
	for _, node := range nodes {
		writeSanitizedRichText(&out, node)
	}
	return out.String()
}

func writeSanitizedRichText(out *strings.Builder, node *nethtml.Node) {
	switch node.Type {
	case nethtml.TextNode:
		out.WriteString(escapeRichText(node.Data))
		return
	case nethtml.CommentNode:
		return
	case nethtml.ElementNode:
		tag := strings.ToLower(node.Data)
		if tag == "script" {
			return
		}
		if !measuredRichTextTag(tag) {
			writeSanitizedRichTextChildren(out, node)
			return
		}
		out.WriteByte('<')
		out.WriteString(tag)
		for _, attr := range sanitizedRichTextAttributes(tag, node.Attr) {
			out.WriteByte(' ')
			out.WriteString(attr.name)
			out.WriteString(`="`)
			out.WriteString(html.EscapeString(attr.value))
			out.WriteByte('"')
		}
		out.WriteByte('>')
		if tag == "br" {
			return
		}
		writeSanitizedRichTextChildren(out, node)
		out.WriteString("</")
		out.WriteString(tag)
		out.WriteByte('>')
	}
}

func writeSanitizedRichTextChildren(out *strings.Builder, node *nethtml.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		writeSanitizedRichText(out, child)
	}
}

func measuredRichTextTag(tag string) bool {
	switch tag {
	case "a", "b", "br", "div", "i", "img", "li", "p", "span", "ul", "strong", "em", "ol", "table", "tbody", "tr", "td":
		return true
	default:
		return false
	}
}

type richTextAttribute struct {
	name  string
	value string
}

func sanitizedRichTextAttributes(tag string, attrs []nethtml.Attribute) []richTextAttribute {
	result := make([]richTextAttribute, 0, len(attrs)+1)
	var style *richTextAttribute
	for _, attr := range attrs {
		name := strings.ToLower(strings.TrimSpace(attr.Key))
		if strings.HasPrefix(name, "on") || !measuredRichTextAttribute(tag, name) {
			continue
		}
		value := attr.Val
		switch name {
		case "href", "src":
			value = measuredRichTextURL(tag, value)
		case "style":
			normalized := richTextAttribute{name: name, value: normalizeMeasuredStyle(value)}
			style = &normalized
			continue
		}
		result = append(result, richTextAttribute{name: name, value: value})
	}
	if style != nil {
		result = append(result, *style)
	}
	if tag == "a" {
		result = append(result, richTextAttribute{name: "target", value: "_blank"})
	}
	if tag == "td" {
		result = append(result, richTextAttribute{name: "colspan", value: "1"}, richTextAttribute{name: "rowspan", value: "1"})
	}
	return result
}

func measuredRichTextAttribute(tag, name string) bool {
	switch tag {
	case "a":
		return name == "href" || name == "title"
	case "img":
		return name == "src" || name == "alt" || name == "width"
	case "span":
		return name == "class" || name == "id" || name == "style"
	default:
		return false
	}
}

func measuredRichTextURL(tag, value string) string {
	trimmed := strings.TrimSpace(value)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return value
	}
	// SF209 extends link targets; image sources retain their measured policy.
	if tag == "a" && (strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "mailto") ||
		(parsed.Scheme == "" && parsed.Host == "" && strings.HasPrefix(trimmed, "/"))) {
		return value
	}
	return ""
}

func normalizeMeasuredStyle(value string) string {
	var declarations []string
	for _, declaration := range strings.Split(value, ";") {
		name, raw, ok := strings.Cut(declaration, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		raw = strings.TrimSpace(raw)
		if name == "" || raw == "" {
			continue
		}
		declarations = append(declarations, name+": "+raw+";")
	}
	return strings.Join(declarations, " ")
}

func escapeRichText(text string) string {
	return strings.ReplaceAll(html.EscapeString(text), "©", "&copy;")
}
