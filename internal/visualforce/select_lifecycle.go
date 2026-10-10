package visualforce

import (
	"strings"

	"github.com/glade-sh/glade/internal/vm"
)

func selectSelectedValues(node *MarkupNode, ctx *RenderContext) (map[string]bool, error) {
	if ctx.formLifecycle != nil {
		if raw, ok := ctx.formLifecycle.submitted[node]; ok {
			selected := map[string]bool{}
			for _, value := range strings.Split(raw, ";") {
				selected[value] = true
			}
			return selected, nil
		}
	}
	raw := node.Attribute("value")
	if isApexStructureTag(node, "selectList") && !strings.Contains(raw, "{!") {
		// Preserve selectList's template rendering for literal attribute values.
		// A literal that names a controller property is still literal text.
		value, err := RenderExpressionTemplate(raw, ctx.Expression)
		if err != nil {
			return nil, err
		}
		selected := map[string]bool{}
		if value != "" {
			selected[value] = true
		}
		return selected, nil
	}
	return selectedValueSet(raw, ctx)
}

// Selection controls participate in prepareFormLifecycle's shared discovery,
// immediate, actionRegion and setter phases. Only selection decoding and
// option validation differ from ordinary text/checkbox conversion.
func formSelectionControl(node *MarkupNode) bool {
	return isApexStructureTag(node, "selectList") || isApexStructureTag(node, "selectRadio") || isApexStructureTag(node, "selectCheckboxes")
}

func formSelectionType(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "string", "integer", "boolean", "list<string>":
		return true
	default:
		return false
	}
}

func selectValidationMessage(node *MarkupNode, ctx *RenderContext) string {
	if ctx.formLifecycle == nil {
		return ""
	}
	return ctx.formLifecycle.selectionErrors[node]
}

func formSelectionValue(raw, typeName string, supplied bool, node *MarkupNode, ctx *RenderContext, clientID string) (vm.Value, string, error) {
	options, err := selectOptionNodes(node, ctx)
	if err != nil {
		return vm.Null, "", err
	}
	values := []string{}
	if supplied {
		values = strings.Split(raw, ";")
	}
	for _, value := range values {
		if value == "" {
			continue
		}
		found := false
		for _, option := range options {
			found = found || option.value == value
		}
		if !found {
			return vm.Null, clientID + ": Validation Error: Value is not valid", nil
		}
	}
	if strings.EqualFold(typeName, "List<String>") {
		items := make([]vm.Value, 0, len(values))
		for _, value := range values {
			items = append(items, vm.String(value))
		}
		return vm.List(items...), "", nil
	}
	if raw == "" {
		return vm.Null, "", nil
	}
	converted, message := formConvertedValue(raw, typeName, node.Name)
	return converted, message, nil
}
