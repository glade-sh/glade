package visualforce

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// standardControllerDirectBindings finds root fields referenced by parsed
// Visualforce expressions. Relationship paths are deliberately not admitted by
// this scalar projection.
func standardControllerDirectBindings(root *MarkupNode, objectName string, org *storage.OrgState) []string {
	if root == nil || org == nil {
		return nil
	}
	controllerObject, ok := storage.ResolveObjectName(*org, objectName)
	if !ok {
		return nil
	}
	var fields []string
	addPath := func(parts []string) {
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			return
		}
		boundObject, ok := storage.ResolveObjectName(*org, parts[0])
		if ok && strings.EqualFold(boundObject, controllerObject) {
			fields = append(fields, strings.TrimSpace(parts[1]))
		}
	}
	var visitExpr func(Expression)
	visitExpr = func(expr Expression) {
		if parts, static := standardStaticFieldPath(expr); static {
			addPath(parts)
			return
		}
		switch value := expr.(type) {
		case functionExpr:
			for _, arg := range value.args {
				visitExpr(arg)
			}
		case visualforceFunctionExpr:
			for _, arg := range value.args {
				visitExpr(arg)
			}
		case binaryExpr:
			visitExpr(value.left)
			visitExpr(value.right)
		case unaryExpr:
			visitExpr(value.value)
		case indexExpr:
			visitExpr(value.target)
			visitExpr(value.key)
		case memberExpr:
			visitExpr(value.target)
		case methodCallExpr:
			visitExpr(value.target)
			for _, arg := range value.args {
				visitExpr(arg)
			}
		}
	}
	scanTemplate := func(raw string) {
		for offset := 0; offset < len(raw); {
			start := strings.Index(raw[offset:], "{!")
			if start < 0 {
				return
			}
			start += offset
			end := findExpressionTemplateEnd(raw, start+2)
			if end < 0 {
				return
			}
			expr, err := parseExpression(raw[start+2 : end])
			if err == nil {
				visitExpr(expr)
			}
			offset = end + 1
		}
	}
	var walk func(*MarkupNode)
	walk = func(node *MarkupNode) {
		if node == nil {
			return
		}
		if node.Type == MarkupNodeText {
			scanTemplate(node.Text)
		} else if node.Type == MarkupNodeElement {
			for _, raw := range node.Attributes {
				scanTemplate(raw)
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(root)
	return fields
}

func standardStaticFieldPath(expr Expression) ([]string, bool) {
	switch value := expr.(type) {
	case identifierExpr:
		return value.parts, true
	case visualforceIdentifierExpr:
		return value.parts, true
	case memberExpr:
		parts, ok := standardStaticFieldPath(value.target)
		if !ok {
			return nil, false
		}
		return append(append([]string(nil), parts...), value.field), true
	default:
		return nil, false
	}
}
