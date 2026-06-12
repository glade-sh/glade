package lwc

import "strings"

func renderLightningComponent(node *TemplateNode, ctx *RenderContext) (string, error) {
	switch node.Tag {
	case "lightning-button":
		label, _ := attributeValue(node, "label", ctx.Properties)
		variant, _ := attributeValue(node, "variant", ctx.Properties)
		className := "slds-button"
		if variant == "brand" {
			className += " slds-button_brand"
		}
		inner, err := renderChildren(node, ctx)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(inner) == "" {
			inner = label
		}
		return `<button type="button" class="` + className + `">` + inner + `</button>`, nil
	case "lightning-formatted-text":
		value, _ := attributeValue(node, "value", ctx.Properties)
		return `<span class="slds-form-element__static">` + value + `</span>`, nil
	case "lightning-input":
		label, _ := attributeValue(node, "label", ctx.Properties)
		value, _ := attributeValue(node, "value", ctx.Properties)
		return `<label class="slds-form-element__label">` + label + `</label><input class="slds-input" value="` + value + `" />`, nil
	default:
		return renderHTMLElement(node, ctx)
	}
}

func attributeValue(node *TemplateNode, name string, bag PropertyBag) (string, error) {
	raw := node.Attributes[name]
	if raw == "" {
		return "", nil
	}
	if strings.Contains(raw, "{") {
		return ResolveBinding(raw, bag)
	}
	return resolveAttributeValue(raw, bag)
}

func renderChildren(node *TemplateNode, ctx *RenderContext) (string, error) {
	var b strings.Builder
	for _, child := range node.Children {
		out, err := renderNode(child, ctx)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	return b.String(), nil
}
