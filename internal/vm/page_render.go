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

func NewUnsupportedFeatureError(message string) error {
	return &RuntimeError{Type: "UnsupportedFeature", Message: message}
}

func NewVisualforceException(message string) error {
	return newExceptionError("VisualforceException", message)
}

func NewExecutionException(message string) error {
	return newExceptionError("ExecutionException", message)
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

// InstancePropertyType reports a controller property's declared type without
// evaluating its getter.
func (vm *VM) InstancePropertyType(receiver Value, name string) (string, bool, error) {
	if vm == nil || receiver.Kind != ValueObject || strings.TrimSpace(name) == "" {
		return "", false, nil
	}
	field, owner, fieldOK := vm.lookupReceiverField(receiver.Type, name)
	if fieldOK && field.Property {
		if !field.HasSetter && field.Setter == nil {
			return "", false, nil
		}
		if !field.HasGetter && field.Getter == nil {
			return "", false, nil
		}
		if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name, field.Modifiers); err != nil {
			return "", true, err
		}
		if field.Getter != nil {
			if err := vm.checkMemberAccess(field.Getter.ClassName, field.Getter.Access, field.Getter.Name, field.Getter.Modifiers); err != nil {
				return "", true, err
			}
		}
		if field.Setter != nil {
			if err := vm.checkMemberAccess(field.Setter.ClassName, field.Setter.Access, field.Setter.Name, field.Setter.Modifiers); err != nil {
				return "", true, err
			}
		}
		typeName := vm.resolveTypeNameInClass(owner, field.Type)
		if field.Getter != nil && strings.TrimSpace(field.Getter.ReturnType) != "" && !strings.EqualFold(typeName, vm.resolveTypeNameInClass(field.Getter.ClassName, field.Getter.ReturnType)) {
			return "", true, fmt.Errorf("Visualforce getter type does not match %s.%s", receiver.Type, name)
		}
		if field.Setter != nil {
			if len(field.Setter.Params) != 1 || !strings.EqualFold(typeName, vm.resolveTypeNameInClass(field.Setter.ClassName, field.Setter.Params[0].Type)) {
				return "", true, fmt.Errorf("Visualforce setter type does not match %s.%s", receiver.Type, name)
			}
		}
		return typeName, true, nil
	}
	getter, getterOK, getterAmbiguous := vm.resolveInstanceMethodByArity(receiver.Type, "get"+propertyGetterSuffix(name), 0)
	setter, setterOK, setterAmbiguous := vm.resolveInstanceMethodByArity(receiver.Type, "set"+propertyGetterSuffix(name), 1)
	if getterAmbiguous || setterAmbiguous {
		return "", true, fmt.Errorf("ambiguous Visualforce property accessors for %s.%s", receiver.Type, name)
	}
	if !getterOK || !setterOK || len(setter.Params) != 1 {
		return "", false, nil
	}
	if err := vm.checkMemberAccess(getter.ClassName, getter.Access, getter.Name, getter.Modifiers); err != nil {
		return "", true, err
	}
	if err := vm.checkMemberAccess(setter.ClassName, setter.Access, setter.Name, setter.Modifiers); err != nil {
		return "", true, err
	}
	getterType := vm.resolveTypeNameInClass(getter.ClassName, getter.ReturnType)
	setterType := vm.resolveTypeNameInClass(setter.ClassName, setter.Params[0].Type)
	if strings.TrimSpace(getterType) == "" || !strings.EqualFold(getterType, setterType) {
		return "", true, fmt.Errorf("Visualforce property accessors for %s.%s have different types", receiver.Type, name)
	}
	return getterType, true, nil
}

// AssignInstanceProperty applies a value through the VM's ordinary typed
// property assignment and setter path, returning the possibly updated receiver.
func (vm *VM) AssignInstanceProperty(receiver Value, name string, value Value) (Value, error) {
	if vm == nil {
		return receiver, fmt.Errorf("cannot assign Visualforce property without a VM")
	}
	if _, ok, err := vm.InstancePropertyType(receiver, name); err != nil {
		return receiver, err
	} else if !ok {
		return receiver, fmt.Errorf("%s.%s is not a readable and writable property", receiver.Type, name)
	}
	if field, _, ok := vm.lookupReceiverField(receiver.Type, name); ok && field.Property && (field.HasSetter || field.Setter != nil) {
		if err := vm.assignPath(receiver, []string{name}, value); err != nil {
			return receiver, err
		}
		if receiver.Ref != 0 {
			if updated, ok := vm.findValueByRef(receiver.Ref); ok {
				receiver = updated
			}
		}
		return receiver, nil
	}
	typeName, _, err := vm.InstancePropertyType(receiver, name)
	if err != nil {
		return receiver, err
	}
	setter, ok, ambiguous := vm.resolveInstanceMethodByArity(receiver.Type, "set"+propertyGetterSuffix(name), 1)
	if ambiguous || !ok || len(setter.Params) != 1 {
		return receiver, fmt.Errorf("no unambiguous setter for %s.%s", receiver.Type, name)
	}
	coerced, err := vm.coerceAssignable(typeName, value)
	if err != nil {
		return receiver, err
	}
	if _, err := vm.callMethodWithReceiver(setter, receiver, []Value{coerced}, resultForLookup()); err != nil {
		return receiver, err
	}
	if receiver.Ref != 0 {
		if updated, ok := vm.findValueByRef(receiver.Ref); ok {
			receiver = updated
		}
	}
	return receiver, nil
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
