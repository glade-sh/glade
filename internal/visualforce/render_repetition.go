package visualforce

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Repetition offset and row-count cases use a zero-based
// first offset and zero rows means all remaining items. Select before omitting
// null table/list row objects so the offset addresses the source collection.
func repetitionItems(node *MarkupNode, ctx *RenderContext) ([]vm.Value, error) {
	items, err := evaluateListExpression(node.Attribute("value"), ctx)
	if err != nil {
		return nil, err
	}
	first, err := repetitionInteger(node.Attribute("first"), ctx)
	if err != nil {
		return nil, err
	}
	rows, err := repetitionInteger(node.Attribute("rows"), ctx)
	if err != nil {
		return nil, err
	}
	if first < 0 {
		first = 0
	}
	if first >= len(items) {
		return nil, nil
	}
	end := len(items)
	if rows > 0 && rows < end-first {
		end = first + rows
	}
	return items[first:end], nil
}

func repetitionInteger(raw string, ctx *RenderContext) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := RenderExpressionTemplate(raw, ctx.Expression)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("Value '%s' cannot be converted from Text to int.", value)
	}
	return number, nil
}

func repetitionExpressionValid(raw string) bool {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{!") && strings.HasSuffix(raw, "}") {
		raw = strings.TrimSpace(raw[2 : len(raw)-1])
	}
	_, err := parseExpression(raw)
	return err == nil
}

// Normal/generated tables render nested outputText and literal children,
// in addition to the column's value attribute. Child renderers already escape
// their output; only the value attribute follows the value-only escape path.
func renderRepetitionColumn(col dataTableColumn, ctx *RenderContext, rowVariable string) (string, error) {
	node := col.node
	previousFieldAccess := ctx.Expression.repetitionFieldAccess
	ctx.Expression.repetitionFieldAccess = previousFieldAccess || len(col.bindings) != 0
	defer func() { ctx.Expression.repetitionFieldAccess = previousFieldAccess }()
	var out strings.Builder
	if value, exists := node.Attributes["value"]; exists {
		value, err := RenderExpressionTemplate(value, ctx.Expression)
		if err != nil {
			return "", err
		}
		out.WriteString(html.EscapeString(value))
	}
	if len(col.bindings) != 0 {
		node = repetitionColumnNode(node)
	}
	children, err := renderRepetitionChildren(node, ctx, rowVariable)
	if err != nil {
		return "", err
	}
	out.WriteString(children)
	return out.String(), nil
}

func renderRepetitionChildren(node *MarkupNode, ctx *RenderContext, variable string) (string, error) {
	previous := ctx.repetitionVariables
	if variable != "" {
		ctx.repetitionVariables = append(ctx.repetitionVariables, variable)
	}
	defer func() { ctx.repetitionVariables = previous }()
	return renderChildren(node, ctx)
}

// r_{repeat,dataTable,pageBlockTable,dataList}_dates render a bare Date row
// binding as GMT date text. Keep Date values typed for formulas and comparisons;
// literals, other value types, controller getters and non-row Date paths retain
// their existing conversion.
func renderRepetitionOutputTemplate(raw string, ctx *RenderContext) (string, error) {
	rendered, err := RenderExpressionTemplate(raw, ctx.Expression)
	if err != nil || ctx.Scope == nil || len(ctx.repetitionVariables) == 0 {
		return rendered, err
	}
	name := strings.TrimSpace(raw)
	if !strings.HasPrefix(name, "{!") || !strings.HasSuffix(name, "}") {
		return rendered, nil
	}
	name = strings.TrimSpace(name[2 : len(name)-1])
	if strings.HasPrefix(name, "$") || strings.EqualFold(name, "this") || strings.EqualFold(name, "currentpage") {
		return rendered, nil
	}
	for _, variable := range ctx.repetitionVariables {
		if !strings.EqualFold(variable, name) {
			continue
		}
		value, ok := ctx.Scope.Get(name)
		if !ok || value.Kind != vm.ValueObject || !strings.EqualFold(value.Type, "Date") {
			return rendered, nil
		}
		if date, err := time.Parse("2006-01-02", rendered); err == nil {
			return date.Format("Mon Jan 02 15:04:05 GMT 2006"), nil
		}
	}
	return rendered, nil
}

type repetitionFieldError struct {
	field, object string
}

func (err *repetitionFieldError) Error() string {
	return fmt.Sprintf("Invalid field %s for SObject %s", err.field, err.object)
}

// r_generated_*_{unknown,hidden_missing}: dynamic SObject field names are
// checked against schema even when the generating repeat is hidden. Restrict
// this check to generated-column evaluation; plain DTOs, maps, null receivers
// and existing declared-but-null SObject fields keep their member-read path.
func validateRepetitionFieldAccess(ctx *ExpressionContext, target, key vm.Value) error {
	if ctx == nil || !ctx.repetitionFieldAccess || target.Kind != vm.ValueObject || key.Kind != vm.ValueString {
		return nil
	}
	object, known := visualforceObjectState(ctx, target.Type)
	definition := object.Definition
	if !known {
		definition, known = storage.StandardObjectDefinition(target.Type)
	}
	if !known {
		return nil
	}
	if _, exists := storage.ResolveFieldName(definition, ctx.ProjectNamespace, key.Text); exists {
		return nil
	}
	return &repetitionFieldError{field: key.Text, object: definition.APIName}
}

func contextualRepetitionFieldError(err error, raw, component, page string) error {
	message := fmt.Sprintf("Error is in expression '%s' in component <%s> in page %s\n%s", raw, component, strings.ToLower(page), err)
	return &contextualFormulaError{message: message, cause: err}
}

func repetitionColumnNode(node *MarkupNode) *MarkupNode {
	copy := *node
	if node.Namespace == "apex" && node.Name == "outputtext" {
		copy.Attributes = make(map[string]string, len(node.Attributes))
		for key, value := range node.Attributes {
			copy.Attributes[key] = value
		}
		if _, exists := copy.Attributes["value"]; exists {
			copy.Attributes["value"] = repetitionAttributeTemplate(node, "value")
		}
	}
	copy.Children = make([]*MarkupNode, len(node.Children))
	for i, child := range node.Children {
		copy.Children[i] = repetitionColumnNode(child)
	}
	return &copy
}

// r_generated_*_escaped retains decimal entity text in the literal portions
// of a repeat-generated column header/outputText template. Only source literals
// change: dynamic values and expression arguments retain their evaluation and
// escaping. Direct columns and output outside tables keep their existing path.
func repetitionAttributeTemplate(node *MarkupNode, name string) string {
	value := node.Attribute(name)
	if !strings.Contains(value, "{!") {
		return value
	}
	raw := ""
	for key, source := range node.RawAttributes {
		if strings.EqualFold(key, name) {
			raw = source
			break
		}
	}
	if !strings.Contains(raw, "&lt;") && !strings.Contains(raw, "&gt;") && !strings.Contains(raw, "&amp;") {
		return value
	}
	// Decode other source entities as before. Protect only these three native
	// decimal strings from that one decoding pass.
	entities := strings.NewReplacer("&lt;", "&amp;#60;", "&gt;", "&amp;#62;", "&amp;", "&amp;#38;")
	var out strings.Builder
	for offset := 0; offset < len(raw); {
		next := strings.Index(raw[offset:], "{!")
		if next < 0 {
			out.WriteString(html.UnescapeString(entities.Replace(raw[offset:])))
			break
		}
		start := offset + next
		end := findExpressionTemplateEnd(raw, start+2)
		if end < 0 {
			return value
		}
		out.WriteString(html.UnescapeString(entities.Replace(raw[offset:start])))
		out.WriteString(html.UnescapeString(raw[start : end+1]))
		offset = end + 1
	}
	return out.String()
}
