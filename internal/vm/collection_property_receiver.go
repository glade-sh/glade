package vm

import "strings"

// Resolve accessor metadata from stored parent values without evaluating getters.
func (vm *VM) sourceCollectionPropertyReceiver(name string) bool {
	sourceGetter := func(field Field, owner string, found bool) bool {
		if !found || !field.Property || field.Getter == nil {
			return false
		}
		class, ok := vm.lookupClass(owner)
		return ok && !class.PlatformType
	}
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		if _, local := vm.lookupGlobalName(name); local {
			return false
		}
		if receiver, ok := vm.Globals["this"]; ok && receiver.Kind == ValueObject {
			if field, owner, found := vm.lookupReceiverField(runtimeObjectType(receiver), name); found {
				return sourceGetter(field, owner, found)
			}
		}
		return sourceGetter(vm.lookupStaticField(vm.currentClass, name))
	}
	var parent Value
	next := 1
	if actual, found := vm.lookupGlobalName(parts[0]); found {
		parent = vm.Globals[actual]
	} else {
		field, owner, found := Field{}, "", false
		if receiver, ok := vm.Globals["this"]; ok && receiver.Kind == ValueObject {
			field, owner, found = vm.lookupReceiverField(runtimeObjectType(receiver), parts[0])
			if found && field.Getter == nil {
				_, parent, _ = vm.instanceFieldValue(receiver, owner, field)
			}
		}
		if !found {
			field, owner, found = vm.lookupStaticField(vm.currentClass, parts[0])
			parent = field.Value
		}
		if !found {
			for split := len(parts) - 1; split > 0; split-- {
				className := strings.Join(parts[:split], ".")
				preferDependency := vm.classMemberReferenceUsesExplicitNamespace(strings.Join(parts[:split+1], "."), className)
				field, owner, found = vm.lookupStaticFieldForReceiver(className, parts[split], preferDependency)
				if found {
					parent = field.Value
					next = split + 1
					break
				}
			}
		}
		if !found {
			return false
		}
		if next == len(parts) {
			return sourceGetter(field, owner, true)
		}
		// An intermediate accessor may compute a different object on each read.
		if field.Getter != nil {
			return false
		}
	}
	for ; next < len(parts); next++ {
		if parent.Kind != ValueObject {
			return false
		}
		field, owner, found := vm.lookupReceiverField(runtimeObjectType(parent), parts[next])
		if !found {
			return false
		}
		if next == len(parts)-1 {
			return sourceGetter(field, owner, true)
		}
		if field.Getter != nil {
			return false
		}
		_, parent, found = vm.instanceFieldValue(parent, owner, field)
		if !found {
			return false
		}
	}
	return false
}
