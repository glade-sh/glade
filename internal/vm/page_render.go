package vm

import (
	"fmt"
	"strings"
)

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

func NewVisualforceException(message string) error {
	return newExceptionError("VisualforceException", message)
}

func (vm *VM) ConstructController(className string) (Value, error) {
	return vm.constructValue(className, nil, nil, nil)
}

func (vm *VM) ConstructControllerWithArgs(className string, args []Value) (Value, error) {
	return vm.constructValue(className, args, nil, nil)
}

// ReadInstanceProperty resolves a controller property against a live instance,
// invoking Apex getters when field metadata defines them.
func (vm *VM) ReadInstanceProperty(receiver Value, name string) (Value, bool, error) {
	if vm == nil || receiver.Kind != ValueObject || strings.TrimSpace(name) == "" {
		return Null, false, nil
	}
	typeName := strings.TrimSpace(receiver.Type)
	if typeName != "" {
		field, owner, ok := vm.lookupReceiverField(typeName, name)
		if ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name, field.Modifiers); err != nil {
				return Null, true, err
			}
			if field.Getter != nil {
				value, err := vm.callGetter(owner, field, receiver)
				return value, true, err
			}
			if _, value, ok := objectFieldValue(receiver, name); ok {
				return value, true, nil
			}
			return defaultValue(field.Type, field.InitialValue), true, nil
		}
		if method, ok, ambiguous := vm.resolveInstanceMethodByArity(typeName, "get"+propertyGetterSuffix(name), 0); ambiguous {
			return Null, true, fmt.Errorf("ambiguous Visualforce getter for %s.%s", typeName, name)
		} else if ok {
			if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
				return Null, true, err
			}
			value, err := vm.callMethodWithReceiver(method, receiver, nil, resultForLookup())
			return value, true, err
		}
	}
	if _, value, ok := objectFieldValue(receiver, name); ok {
		return value, true, nil
	}
	return Null, false, nil
}

func propertyGetterSuffix(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func renderPageContent(vm *VM, pageURL string, asPDF bool) (Value, error) {
	if pageContentRenderer == nil {
		return Null, unsupportedCallError("PageReference.getContent local Visualforce page rendering surface")
	}
	return pageContentRenderer(vm, pageURL, asPDF)
}
