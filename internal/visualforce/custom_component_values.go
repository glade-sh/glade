package visualforce

import (
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

// Custom attribute bindings retain VM values rather than round-tripping
// through rendered text. This preserves nulls, records and collections when
// attributes feed a controller's assignTo property or another component.
func evaluatedTypedComponentAttributes(node *MarkupNode, ctx *RenderContext, definitions []Attribute) (map[string]vm.Value, error) {
	values := make(map[string]vm.Value, len(definitions))
	for _, attr := range definitions {
		raw, supplied := componentAttributeValue(node.Attributes, attr.Name)
		value := vm.Null
		if supplied {
			exprText := strings.TrimSpace(raw)
			if strings.HasPrefix(exprText, "{!") && findExpressionTemplateEnd(exprText, 2) == len(exprText)-1 {
				expr, err := parseExpression(strings.TrimSpace(exprText[2 : len(exprText)-1]))
				if err != nil {
					return nil, err
				}
				value, err = evaluateExpressionNode(expr, ctx.Expression)
				if err != nil {
					return nil, err
				}
			} else {
				text, err := RenderExpressionTemplate(raw, ctx.Expression)
				if err != nil {
					return nil, err
				}
				// Captured attribute_string_escaped_runtime preserves numeric
				// XML references in a literal String binding. Bound HTML strings
				// (composition_escaped_output_runtime) retain their Apex value.
				if !strings.Contains(raw, "{!") && componentBindingTypesEqual(attr.Type, "String") {
					text = strings.NewReplacer("&", "&#38;", "<", "&#60;", ">", "&#62;").Replace(text)
				}
				value = vm.String(text)
			}
		}
		if value.Kind == vm.ValueNull && attr.Default != "" {
			value = vm.String(attr.Default)
		}
		converted, err := componentBindingValue(value, attr.Type, attr.Name)
		if err != nil {
			return nil, err
		}
		values[attr.Name] = converted
	}
	return values, nil
}

func typedComponentAttributeValue(values map[string]vm.Value, name string) (vm.Value, bool) {
	for key, value := range values {
		if strings.EqualFold(key, name) {
			return value, true
		}
	}
	return vm.Null, false
}

func componentBindingTypesEqual(left, right string) bool {
	normalize := func(typ string) string {
		typ = strings.ToLower(strings.Join(strings.Fields(typ), ""))
		typ = strings.TrimPrefix(typ, "system.")
		if strings.HasSuffix(typ, "[]") {
			typ = "list<" + strings.TrimSuffix(typ, "[]") + ">"
		}
		return typ
	}
	return normalize(left) == normalize(right)
}

func componentBindingValue(value vm.Value, typeName, attribute string) (vm.Value, error) {
	if value.Kind == vm.ValueNull {
		return value, nil
	}
	typ := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(typeName)), "system.")
	if typ == "string" {
		return vm.String(visualforceFormulaText(value)), nil
	}
	// Literal numeric/Boolean attributes have the declared type. Bound values
	// already carry their Apex type and should reach assignTo unchanged.
	if value.Kind == vm.ValueString {
		switch typ {
		case "integer", "long", "decimal", "double", "boolean":
			return visualforceAssignmentValue(value.Text, typeName, attribute)
		}
	}
	return value, nil
}

// Body literals have been entity-decoded by the markup parser. Escape those
// segments while leaving formula source on the shared expression path.
func escapeComponentBodyLiterals(raw string) string {
	var out strings.Builder
	for pos := 0; pos < len(raw); {
		next := strings.Index(raw[pos:], "{!")
		if next < 0 {
			out.WriteString(html.EscapeString(raw[pos:]))
			break
		}
		start := pos + next
		out.WriteString(html.EscapeString(raw[pos:start]))
		end := findExpressionTemplateEnd(raw, start+2)
		if end < 0 {
			out.WriteString(raw[start:])
			break
		}
		out.WriteString(raw[start : end+1])
		pos = end + 1
	}
	return out.String()
}

// Only direct custom attribute outputs use the component binding's display
// conversion. Controller values, other page expressions and object field
// access continue through their existing renderer paths.
func renderComponentAttributeOutput(node *MarkupNode, ctx *RenderContext, render func(*MarkupNode, *RenderContext) (string, error)) (string, error) {
	if ctx.componentValues == nil {
		return render(node, ctx)
	}
	raw := strings.TrimSpace(node.Attribute("value"))
	if !strings.HasPrefix(raw, "{!") || findExpressionTemplateEnd(raw, 2) != len(raw)-1 {
		return render(node, ctx)
	}
	name := strings.TrimSpace(raw[2 : len(raw)-1])
	// Scope bindings precede attribute variables in the shared lookup. Keep
	// their resolution and output conversion on the ordinary renderer path.
	if _, scoped := ctx.Expression.Scope.Get(name); scoped {
		return render(node, ctx)
	}
	typ, declared := ctx.componentTypes[strings.ToLower(name)]
	if !declared {
		return render(node, ctx)
	}
	value, _ := typedComponentAttributeValue(ctx.componentValues, name)
	text := componentAttributeDisplay(value, typ, ctx.VM)
	copy := *node
	copy.Attributes = make(map[string]string, len(node.Attributes))
	for key, value := range node.Attributes {
		copy.Attributes[key] = value
	}
	// Keep the value an expression so escape=false follows the bound-value
	// path, even when its display conversion yields a string containing markup.
	copy.Attributes["value"] = "{!__componentDisplay}"
	expression := *ctx.Expression
	expression.Variables = make(map[string]vm.Value, len(ctx.Expression.Variables)+1)
	for key, value := range ctx.Expression.Variables {
		expression.Variables[key] = value
	}
	expression.Variables["__componentDisplay"] = vm.String(text)
	context := *ctx
	context.Expression = &expression
	return render(&copy, &context)
}

func componentAttributeDisplay(value vm.Value, typeName string, machine *vm.VM) string {
	typ := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(typeName)), "system.")
	if value.Kind == vm.ValueNull {
		if typ == "boolean" {
			return "false"
		}
		return ""
	}
	if value.Kind == vm.ValueList {
		elementType := strings.TrimSuffix(typeName, "[]")
		items := make([]string, len(value.List))
		for i, item := range value.List {
			items[i] = componentAttributeDisplay(item, elementType, machine)
		}
		return "[" + strings.Join(items, ", ") + "]"
	}
	if typ == "date" || typ == "datetime" {
		layouts := []string{"2006-01-02", time.RFC3339Nano, "2006-01-02 15:04:05"}
		for _, layout := range layouts {
			if instant, err := time.Parse(layout, value.String()); err == nil {
				return instant.UTC().Format("Mon Jan 02 15:04:05") + " GMT " + instant.UTC().Format("2006")
			}
		}
	}
	if value.Kind == vm.ValueObject && machine != nil && machine.Org != nil {
		// Render records according to schema identity rather than blanking
		// every object value or special-casing captured object names.
		for objectName := range machine.Org.Objects {
			if strings.EqualFold(objectName, value.Type) {
				return ""
			}
		}
	}
	return visualforceFormulaText(value)
}

type componentSetterError struct {
	expression string
	cause      error
}

func (e *componentSetterError) Error() string { return e.cause.Error() }
func (e *componentSetterError) Unwrap() error { return e.cause }

func contextualComponentSetterError(err error, component string) error {
	var setter *componentSetterError
	if !errors.As(err, &setter) {
		return err
	}
	// Native assign_setter_exception_runtime reports the binding's component
	// context first, followed by the original exception message on its own line.
	message := fmt.Sprintf("Error is in expression '%s' in component <c:%s> in component c:%s\n%s", setter.expression, component, component, setter.cause.Error())
	return &contextualFormulaError{message: message, cause: setter.cause}
}
