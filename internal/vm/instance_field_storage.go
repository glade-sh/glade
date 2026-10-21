package vm

import "strings"

const inheritedInstanceFieldStoragePrefix = "__glade_instance_field:"

func isInheritedInstanceFieldStorage(name string) bool {
	return strings.HasPrefix(name, inheritedInstanceFieldStoragePrefix)
}

func plainInstanceField(field Field) bool {
	// Manually registered properties can carry accessor metadata without the
	// Property flag set. Preserve their existing backing-value behavior too.
	return !field.Static && !field.Property && field.Getter == nil && field.Setter == nil && !field.HasGetter && !field.HasSetter
}

// A retained source property has its own backing value just like a retained
// plain field. Unbound properties preserve their existing aliases.
func sourcePropertyStorageName(field Field, fallback string) string {
	if !field.Static && field.Property && field.SourceUnit != "" && field.StorageName != "" {
		return field.StorageName
	}
	return fallback
}

// Hidden plain fields retain slots belonging to their declaring class.
func (vm *VM) instanceFieldStorageName(receiverType, owner string, field Field) string {
	name := field.StorageName
	if name == "" {
		name = field.Name
	}
	if !plainInstanceField(field) || owner == "" || name == "" || strings.EqualFold(receiverType, owner) {
		return name
	}
	hidden := false
	for receiverType != "" {
		class, ok := vm.lookupClass(receiverType)
		if !ok {
			break
		}
		if strings.EqualFold(runtimeClassName(class), owner) {
			if hidden {
				return inheritedInstanceFieldStoragePrefix + owner + ":" + name
			}
			return name
		}
		if declared, ok := class.Fields[name]; ok && plainInstanceField(declared) {
			hidden = true
		}
		receiverType = vm.resolvedSuperClassName(class)
	}
	return name
}

func (vm *VM) instanceFieldValue(object Value, owner string, field Field) (string, Value, bool) {
	name := vm.instanceFieldStorageName(runtimeObjectType(object), owner, field)
	if isInheritedInstanceFieldStorage(name) {
		if value, ok := object.Fields[name]; ok {
			return name, value, true
		}
		// A declaration's missing slot must not fall back to the more-derived
		// field through case-insensitive or namespace alias lookup.
		return name, defaultValue(field.Type, field.InitialValue), true
	}
	// Plain fields retain the selected declaration slot. Unbound properties and
	// lexical fields preserve their existing backing-name and alias lookup.
	if field.Name != "" && (!plainInstanceField(field) || isLexicalInstanceFieldStorage(field.Name)) {
		name = field.Name
	}
	return objectFieldValue(object, sourcePropertyStorageName(field, name))
}
