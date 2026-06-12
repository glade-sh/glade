package lwc

import (
	"fmt"
	"html"
	"strings"
)

type RenderContext struct {
	Index      Index
	Properties PropertyBag
	WireData   map[string]Value
	Namespace  string
}

func RenderTemplate(root *TemplateNode, ctx *RenderContext) (string, error) {
	if root == nil {
		return "", nil
	}
	if ctx == nil {
		ctx = &RenderContext{Properties: PropertyBag{}}
	}
	if strings.EqualFold(root.Tag, "template") {
		var b strings.Builder
		for _, child := range root.Children {
			out, err := renderNode(child, ctx)
			if err != nil {
				return "", err
			}
			b.WriteString(out)
		}
		return b.String(), nil
	}
	return renderNode(root, ctx)
}

func renderNode(node *TemplateNode, ctx *RenderContext) (string, error) {
	if node == nil {
		return "", nil
	}
	switch node.Type {
	case NodeText:
		if strings.Contains(node.Text, "{") {
			return ResolveBinding(strings.TrimSpace(node.Text), ctx.Properties)
		}
		return html.EscapeString(node.Text), nil
	case NodeElement:
		if cond, ok := node.Directives["if:true"]; ok {
			val, err := bindingTruthy(cond, ctx.Properties)
			if err != nil {
				return "", err
			}
			if !val {
				return "", nil
			}
		}
		if cond, ok := node.Directives["if:false"]; ok {
			val, err := bindingTruthy(cond, ctx.Properties)
			if err != nil {
				return "", err
			}
			if val {
				return "", nil
			}
		}
		if eachExpr, ok := node.Directives["for:each"]; ok {
			return renderForEach(node, eachExpr, ctx)
		}
		for key, val := range node.Directives {
			if strings.HasPrefix(key, "iterator:") && val != "" {
				return renderForEach(node, val, ctx)
			}
		}
		if strings.HasPrefix(node.Tag, "lightning-") {
			return renderLightningComponent(node, ctx)
		}
		if isCustomComponentTag(node.Tag, ctx.Namespace) {
			return renderCustomComponent(node, ctx)
		}
		return renderHTMLElement(node, ctx)
	default:
		return "", nil
	}
}

func bindingTruthy(expr string, bag PropertyBag) (bool, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "{") && strings.HasSuffix(expr, "}") {
		path := strings.TrimSpace(expr[1 : len(expr)-1])
		val, ok := resolvePath(path, bag)
		if !ok {
			return false, fmt.Errorf("unresolved binding %q", path)
		}
		return Truthy(val), nil
	}
	return strings.TrimSpace(expr) == "true", nil
}

func renderForEach(node *TemplateNode, eachExpr string, ctx *RenderContext) (string, error) {
	itemName := node.Directives["for:item"]
	if itemName == "" {
		for key := range node.Directives {
			if strings.HasPrefix(key, "iterator:") {
				itemName = strings.TrimPrefix(key, "iterator:")
				break
			}
		}
	}
	if itemName == "" {
		itemName = "item"
	}
	path := strings.Trim(eachExpr, "{}")
	val, ok := resolvePath(path, ctx.Properties)
	if !ok || val.Kind != ValueArray {
		return "", fmt.Errorf("for:each target %q is not an array", path)
	}
	var b strings.Builder
	for _, item := range val.Items {
		loopCtx := *ctx
		loopCtx.Properties = cloneBag(ctx.Properties)
		loopCtx.Properties[itemName] = item
		for _, child := range node.Children {
			out, err := renderNode(child, &loopCtx)
			if err != nil {
				return "", err
			}
			b.WriteString(out)
		}
	}
	return b.String(), nil
}

func renderHTMLElement(node *TemplateNode, ctx *RenderContext) (string, error) {
	var b strings.Builder
	b.WriteString("<")
	b.WriteString(node.Tag)
	for key, raw := range node.Attributes {
		val, err := resolveAttributeValue(raw, ctx.Properties)
		if err != nil {
			return "", err
		}
		b.WriteString(` `)
		b.WriteString(key)
		b.WriteString(`="`)
		b.WriteString(val)
		b.WriteString(`"`)
	}
	b.WriteString(">")
	for _, child := range node.Children {
		out, err := renderNode(child, ctx)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	b.WriteString("</")
	b.WriteString(node.Tag)
	b.WriteString(">")
	return b.String(), nil
}

func renderCustomComponent(node *TemplateNode, ctx *RenderContext) (string, error) {
	name := componentNameFromTag(node.Tag, ctx.Namespace)
	child, ok := ctx.Index.Bundle(name)
	if !ok {
		child, ok = ctx.Index.Bundle(kebabToCamel(name))
	}
	if !ok {
		return fmt.Sprintf(`<!-- unknown lwc component %q -->`, name), nil
	}
	childSource, err := child.ReadHTML()
	if err != nil {
		return "", err
	}
	childTree, err := ParseTemplate(childSource)
	if err != nil {
		return "", err
	}
	childProps := mapAPIAttributes(node.Attributes, ctx.Properties)
	childCtx := *ctx
	childCtx.Properties = childProps
	return RenderTemplate(childTree, &childCtx)
}

func isCustomComponentTag(tag, ns string) bool {
	if strings.HasPrefix(tag, "c-") {
		return true
	}
	if ns != "" && strings.HasPrefix(tag, strings.ToLower(ns)+"-") {
		return true
	}
	return false
}

func componentNameFromTag(tag, ns string) string {
	tag = strings.TrimPrefix(tag, "c-")
	if ns != "" {
		tag = strings.TrimPrefix(tag, strings.ToLower(ns)+"-")
	}
	return kebabToCamel(tag)
}

func kebabToCamel(tag string) string {
	parts := strings.Split(tag, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

func mapAPIAttributes(attrs map[string]string, parent PropertyBag) PropertyBag {
	out := PropertyBag{}
	for key, raw := range attrs {
		if strings.Contains(raw, "{") {
			val, err := ResolveBinding(raw, parent)
			if err != nil {
				continue
			}
			out[key] = StringValue(unescapeHTML(val))
			continue
		}
		out[key] = StringValue(raw)
	}
	return out
}

func resolveAttributeValue(raw string, bag PropertyBag) (string, error) {
	if strings.Contains(raw, "{") {
		return ResolveBinding(raw, bag)
	}
	return html.EscapeString(raw), nil
}

func cloneBag(in PropertyBag) PropertyBag {
	out := make(PropertyBag, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func unescapeHTML(s string) string {
	return html.UnescapeString(s)
}
