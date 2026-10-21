package visualforce

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type controllerMultiSelectFormBinding struct {
	receiver vm.Value
	property string
	field    string
}

// applyControllerMultiSelectFormValues binds the named backing select to the
// SObject in the component's value expression. The visible lists have no name.
// Native field and input rendering review_choices_* controls distinguish mutation of a persistent
// getter result from a getter that returns a fresh SObject on every call.
func applyControllerMultiSelectFormValues(root *MarkupNode, ctx *RenderContext, values map[string]string, action string) (map[string]string, error) {
	if ctx == nil || ctx.VM == nil || ctx.VM.Org == nil || ctx.Expression == nil {
		return values, nil
	}
	// Metadata-only preflight avoids invoking rendered predicates or allocating
	// component references on POSTs that do not contain this widget. The path
	// set also keeps unrelated siblings out of the later predicate walk.
	bindings := map[*MarkupNode]controllerMultiSelectFormBinding{}
	paths := map[*MarkupNode]bool{}
	var findBindings func(*MarkupNode, bool) bool
	findBindings = func(node *MarkupNode, inForm bool) bool {
		if node == nil || isApexStructureTag(node, "repeat") || isApexStructureTag(node, "dataTable") || isApexStructureTag(node, "pageBlockTable") {
			return false
		}
		inForm = inForm || isApexStructureTag(node, "form")
		found := false
		if inForm && isApexStructureTag(node, "inputField") {
			if binding, ok := controllerMultiSelectFormMetadata(node, ctx); ok {
				bindings[node] = binding
				found = true
			}
		}
		for _, child := range node.Children {
			found = findBindings(child, inForm) || found
		}
		paths[node] = found
		return found
	}
	if !findBindings(root, false) {
		return values, nil
	}
	ctx.ComponentReferences = newComponentReferenceResolver(root, nil)
	var forms []*MarkupNode
	var actionForms []*MarkupNode
	var collect func(*MarkupNode, *MarkupNode)
	collect = func(node, form *MarkupNode) {
		if node == nil {
			return
		}
		if isApexStructureTag(node, "form") {
			form = node
			forms = append(forms, node)
		}
		if form != nil && strings.TrimSpace(action) != "" && strings.TrimSpace(node.Attribute("action")) != "" &&
			strings.EqualFold(actionMethodName(node.Attribute("action")), actionMethodName(action)) {
			found := false
			for _, existing := range actionForms {
				found = found || existing == form
			}
			if !found {
				actionForms = append(actionForms, form)
			}
		}
		for _, child := range node.Children {
			collect(child, form)
		}
	}
	collect(root, nil)
	var submittedForm *MarkupNode
	if len(forms) == 1 {
		submittedForm = forms[0]
	} else if len(actionForms) == 1 {
		submittedForm = actionForms[0]
	}

	remaining := make(map[string]string, len(values))
	for name, value := range values {
		remaining[name] = value
	}
	var walk func(*MarkupNode, *MarkupNode) error
	walk = func(node, form *MarkupNode) error {
		if node == nil || !paths[node] {
			return nil
		}
		if strings.EqualFold(node.Namespace, "apex") {
			visible, err := visualforceComponentShouldRender(node, ctx)
			if err != nil || !visible {
				return err
			}
		}
		if isApexStructureTag(node, "form") {
			form = node
		}
		if binding, candidate := bindings[node]; candidate && form != nil {
			name := firstNonEmpty(visualforceExplicitComponentClientID(node, ctx), inputFieldName(node))
			raw, submitted := values[name]
			if submitted || form == submittedForm {
				bound, err := applyControllerMultiSelectValue(binding, ctx, raw, submitted)
				if err != nil {
					return err
				}
				if bound {
					delete(remaining, name)
				}
			}
		}
		for _, child := range node.Children {
			if err := walk(child, form); err != nil {
				return err
			}
		}
		return nil
	}
	return remaining, walk(root, nil)
}

func controllerMultiSelectFormMetadata(node *MarkupNode, ctx *RenderContext) (controllerMultiSelectFormBinding, bool) {
	var binding controllerMultiSelectFormBinding
	expression := strings.TrimSpace(node.Attribute("value"))
	if !strings.HasPrefix(expression, "{!") || !strings.HasSuffix(expression, "}") {
		return binding, false
	}
	expr, err := parseExpression(strings.TrimSpace(expression[2 : len(expression)-1]))
	if err != nil {
		return binding, false
	}
	parts, static := standardStaticFieldPath(expr)
	if !static || len(parts) != 2 {
		return binding, false
	}
	// Resolve a controller property, never reinterpret a StandardController
	// object root as a same-named getter.
	if record, ok := objectFieldIgnoreCase(ctx.Expression.StandardController, "record"); ok && strings.EqualFold(record.Type, parts[0]) {
		return binding, false
	}
	// Inspect the declared result first: unrelated scalar/lookup components
	// must not acquire extra getter evaluations during form binding.
	var receiver vm.Value
	var recordType string
	// Match expression resolution: extensions shadow the custom controller.
	for _, candidate := range append(append([]vm.Value(nil), ctx.Expression.Extensions...), ctx.Expression.Controller) {
		if candidate.Kind != vm.ValueObject {
			continue
		}
		if recordType = visualforceControllerPropertyReadType(ctx.VM, candidate.Type, parts[0]); recordType != "" {
			receiver = candidate
			break
		}
	}
	declaredField := visualforceFormFieldSchema(ctx.VM, vm.Value{Kind: vm.ValueObject, Type: recordType}, parts[1])
	if declaredField == nil || !fieldIsMultiSelect(*declaredField) || fieldIsReadonly(*declaredField) || declaredField.Formula != "" {
		return binding, false
	}
	return controllerMultiSelectFormBinding{receiver: receiver, property: parts[0], field: parts[1]}, true
}

func applyControllerMultiSelectValue(binding controllerMultiSelectFormBinding, ctx *RenderContext, raw string, submitted bool) (bool, error) {
	record, found, err := ctx.VM.ReadInstanceProperty(binding.receiver, binding.property)
	if err != nil || !found || record.Kind != vm.ValueObject {
		return false, err
	}
	field := visualforceFormFieldSchema(ctx.VM, record, binding.field)
	if field == nil || !fieldIsMultiSelect(*field) || fieldIsReadonly(*field) || field.Formula != "" {
		return false, nil
	}
	objectName, ok := storage.ResolveObjectName(*ctx.VM.Org, record.Type)
	if !ok {
		return false, nil
	}
	fieldName, ok := storage.ResolveFieldName(ctx.VM.Org.Objects[objectName].Definition, ctx.Project.Namespace, binding.field)
	if !ok {
		return false, nil
	}
	if _, err := ctx.VM.AuthorizeVisualforceRecordFields(objectName, []string{fieldName}); err != nil {
		return true, nil
	}
	// An empty successful-controls set submits no key; native controls clear
	// the persistent SObject to raw null, rather than preserving its old value.
	value := vm.Null
	if submitted && raw != "" {
		value = vm.String(raw)
	}
	if record.Fields == nil {
		record.Fields = make(map[string]vm.Value)
	}
	record.Fields[fieldName] = value
	return true, nil
}

func visualforceControllerPropertyReadType(machine *vm.VM, className, property string) string {
	seen := map[string]bool{}
	for className != "" && !seen[strings.ToLower(className)] {
		seen[strings.ToLower(className)] = true
		class, found := visualforceVMClass(machine, className)
		if !found {
			return ""
		}
		for name, field := range class.Fields {
			if strings.EqualFold(name, property) || strings.EqualFold(field.Name, property) {
				return field.Type
			}
		}
		for name, method := range class.Methods {
			if method.Name != "" {
				name = method.Name
			}
			if i := strings.LastIndex(name, "."); i >= 0 {
				name = name[i+1:]
			}
			if len(method.Params) == 0 && strings.EqualFold(name, "get"+property) {
				return method.ReturnType
			}
		}
		className = class.SuperClass
	}
	return ""
}
