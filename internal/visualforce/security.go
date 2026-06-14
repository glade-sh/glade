package visualforce

import (
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func EscapeVisualforceOutput(raw string, escape bool) string {
	if !escape {
		return raw
	}
	return html.EscapeString(raw)
}

func RenderVisualforceText(raw string, ctx *ExpressionContext) (string, error) {
	return renderVisualforceText(raw, ctx, true)
}

func RenderVisualforceRawText(raw string, ctx *ExpressionContext) (string, error) {
	return renderVisualforceText(raw, ctx, false)
}

func renderVisualforceText(raw string, ctx *ExpressionContext, escapeExpressions bool) (string, error) {
	if !strings.Contains(raw, "{!") {
		return raw, nil
	}
	var out strings.Builder
	pos := 0
	for pos < len(raw) {
		next := strings.Index(raw[pos:], "{!")
		if next < 0 {
			out.WriteString(raw[pos:])
			break
		}
		start := pos + next
		out.WriteString(raw[pos:start])
		end := findExpressionTemplateEnd(raw, start+2)
		if end < 0 {
			out.WriteString(raw[start:])
			break
		}
		exprText := strings.TrimSpace(raw[start+2 : end])
		value, err := EvaluateExpression(exprText, ctx)
		if err != nil {
			return "", err
		}
		if escapeExpressions {
			value = EscapeVisualforceOutput(value, true)
		}
		out.WriteString(value)
		pos = end + 1
	}
	return out.String(), nil
}

func EscapeVisualforceJavaScriptString(raw string) string {
	var builder strings.Builder
	for _, r := range raw {
		switch r {
		case '\\':
			builder.WriteString(`\\`)
		case '"':
			builder.WriteString(`\"`)
		case '\'':
			builder.WriteString(`\'`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		case '\b':
			builder.WriteString(`\b`)
		case '\f':
			builder.WriteString(`\f`)
		case '\u2028':
			builder.WriteString(`\u2028`)
		case '\u2029':
			builder.WriteString(`\u2029`)
		default:
			if r < 0x20 {
				builder.WriteString(fmt.Sprintf(`\u%04x`, r))
				continue
			}
			builder.WriteRune(r)
		}
	}
	return strings.ReplaceAll(builder.String(), "</", `<\/`)
}

func VisualforceCSPHeaderValue() string {
	return "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'self'; base-uri 'self'"
}

type VisualforcePageHeaderOptions struct {
	ContentType    string
	FileName       string
	Cache          bool
	CacheSet       bool
	ExpiresSeconds int
	ExpiresSet     bool
	CSPHeader      bool
}

func VisualforcePageHeaderOptionsFromMarkup(source string) (VisualforcePageHeaderOptions, error) {
	root, err := ParseMarkupTree(source)
	if err != nil {
		return VisualforcePageHeaderOptions{}, err
	}
	return VisualforcePageHeaderOptionsFromNode(root), nil
}

func VisualforcePageHeaderOptionsFromNode(node *MarkupNode) VisualforcePageHeaderOptions {
	page := firstVisualforcePageNode(node)
	if page == nil {
		return VisualforcePageHeaderOptions{}
	}
	options := VisualforcePageHeaderOptions{}
	if raw := strings.TrimSpace(page.Attribute("cspHeader")); raw != "" {
		options.CSPHeader = truthyVisualforceAttribute(raw)
	}
	if raw := strings.TrimSpace(page.Attribute("contentType")); raw != "" {
		options.ContentType, options.FileName = splitVisualforceContentType(raw)
	}
	if raw := strings.TrimSpace(page.Attribute("cache")); raw != "" {
		options.CacheSet = true
		options.Cache = truthyVisualforceAttribute(raw)
	}
	if raw := strings.TrimSpace(page.Attribute("expires")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 0 {
			options.ExpiresSet = true
			options.ExpiresSeconds = seconds
		}
	}
	return options
}

func (o VisualforcePageHeaderOptions) Apply(header http.Header, now time.Time) {
	if header == nil {
		return
	}
	if o.CSPHeader {
		header.Set("Content-Security-Policy", VisualforceCSPHeaderValue())
	}
	if strings.TrimSpace(o.ContentType) != "" {
		header.Set("Content-Type", strings.TrimSpace(o.ContentType))
	}
	if strings.TrimSpace(o.FileName) != "" {
		header.Set("Content-Disposition", `attachment; filename="`+escapeHeaderQuotedString(o.FileName)+`"`)
	}
	if o.CacheSet && !o.Cache {
		header.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		header.Set("Pragma", "no-cache")
		header.Set("Expires", "0")
		return
	}
	if o.ExpiresSet {
		header.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", o.ExpiresSeconds))
		header.Set("Expires", now.UTC().Add(time.Duration(o.ExpiresSeconds)*time.Second).Format(http.TimeFormat))
	}
}

func firstVisualforcePageNode(node *MarkupNode) *MarkupNode {
	if node == nil {
		return nil
	}
	if node.Type == MarkupNodeElement && strings.EqualFold(node.Namespace, "apex") && strings.EqualFold(node.Name, "page") {
		return node
	}
	for _, child := range node.Children {
		if page := firstVisualforcePageNode(child); page != nil {
			return page
		}
	}
	return nil
}

func splitVisualforceContentType(raw string) (string, string) {
	contentType, fileName, _ := strings.Cut(strings.TrimSpace(raw), "#")
	return strings.TrimSpace(contentType), strings.TrimSpace(fileName)
}

func truthyVisualforceAttribute(raw string) bool {
	raw = strings.TrimSpace(raw)
	return raw == "" || strings.EqualFold(raw, "true") || raw == "1" || strings.EqualFold(raw, "yes")
}

func escapeHeaderQuotedString(raw string) string {
	escaped := strings.ReplaceAll(raw, `\`, `\\`)
	return strings.ReplaceAll(escaped, `"`, `\"`)
}
