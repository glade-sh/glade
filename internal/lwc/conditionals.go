package lwc

import (
	"fmt"
	"strings"
)

func conditionalDirective(node *TemplateNode) (string, string) {
	for _, key := range []string{"lwc:if", "lwc:elseif", "lwc:else"} {
		if value, ok := node.Directives[key]; ok {
			return key, value
		}
	}
	return "", ""
}

func validateConditionalDirectives(node *TemplateNode) error {
	count := 0
	for _, key := range []string{"lwc:if", "lwc:elseif", "lwc:else"} {
		if _, ok := node.Directives[key]; ok {
			count++
		}
	}
	if count > 1 {
		return fmt.Errorf("LWC1162: conditional directives cannot be combined on the same element")
	}
	kind, value := conditionalDirective(node)
	if kind != "" {
		for _, legacy := range []string{"if:true", "if:false"} {
			if _, ok := node.Directives[legacy]; ok {
				return fmt.Errorf("LWC1166: %s cannot be combined with %s", legacy, kind)
			}
		}
		if kind == "lwc:else" {
			if value != "" {
				return fmt.Errorf("LWC1161: lwc:else directive cannot have a value")
			}
		} else {
			value = strings.TrimSpace(value)
			if len(value) < 3 || value[0] != '{' || value[len(value)-1] != '}' {
				code := "LWC1159"
				if kind == "lwc:elseif" {
					code = "LWC1160"
				}
				return fmt.Errorf("%s: %s directive value should be an expression", code, kind)
			}
		}
	}
	previous := ""
	for _, child := range node.Children {
		kind, _ := conditionalDirective(child)
		if (kind == "lwc:elseif" || kind == "lwc:else") && previous != "lwc:if" && previous != "lwc:elseif" {
			return fmt.Errorf("LWC1165: %s must immediately follow lwc:if or lwc:elseif", kind)
		}
		if err := validateConditionalDirectives(child); err != nil {
			return err
		}
		previous = kind
	}
	return nil
}

// A conditional chain selects one sibling. Later conditions are not evaluated
// once a branch has matched; each parent's sibling list has its own chain state.
func renderSiblings(children []*TemplateNode, ctx *RenderContext) (string, error) {
	var out strings.Builder
	matched := false
	for _, child := range children {
		kind, expr := conditionalDirective(child)
		switch kind {
		case "lwc:if":
			matched = false
			fallthrough
		case "lwc:elseif":
			if matched {
				continue
			}
			truthy, err := bindingTruthy(expr, ctx.Properties)
			if err != nil {
				return "", err
			}
			if !truthy {
				continue
			}
			matched = true
		case "lwc:else":
			if matched {
				continue
			}
			matched = true
		default:
			matched = false
		}
		rendered, err := renderNode(child, ctx)
		if err != nil {
			return "", err
		}
		out.WriteString(rendered)
	}
	return out.String(), nil
}
