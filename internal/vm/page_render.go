package vm

import "strings"

// PageContentRenderer renders a Visualforce page URL to HTML (or PDF bytes when asPDF is true).
// The visualforce package registers this hook at init time.
type PageContentRenderer func(vm *VM, pageURL string, asPDF bool) (Value, error)

var pageContentRenderer PageContentRenderer

func SetPageContentRenderer(renderer PageContentRenderer) {
	pageContentRenderer = renderer
}

func UnsupportedFeature(message string) error {
	return unsupportedCallError(message)
}

func (vm *VM) ConstructController(className string) (Value, error) {
	return vm.constructValue(className, nil, nil, nil)
}

// ReadInstanceProperty resolves a controller property against a live instance,
// invoking Apex getters when field metadata defines them.
func (vm *VM) ReadInstanceProperty(receiver Value, name string) (Value, bool, error) {
	if vm == nil || receiver.Kind != ValueObject || strings.TrimSpace(name) == "" {
		return Null, false, nil
	}
	if _, value, ok := objectFieldValue(receiver, name); ok {
		return value, true, nil
	}
	typeName := strings.TrimSpace(receiver.Type)
	if typeName == "" {
		return Null, false, nil
	}
	field, owner, ok := vm.lookupReceiverField(typeName, name)
	if !ok {
		return Null, false, nil
	}
	if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name, field.Modifiers); err != nil {
		return Null, true, err
	}
	if field.Getter != nil {
		value, err := vm.callGetter(owner, field, receiver)
		return value, true, err
	}
	return defaultValue(field.Type, field.InitialValue), true, nil
}

func renderPageContent(vm *VM, pageURL string, asPDF bool) (Value, error) {
	if pageContentRenderer == nil {
		return Null, unsupportedCallError("PageReference.getContent local Visualforce page rendering surface")
	}
	return pageContentRenderer(vm, pageURL, asPDF)
}
