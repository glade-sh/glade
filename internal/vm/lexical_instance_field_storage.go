package vm

import "strings"

// Preserve independently declared private field state for the observed plain
// source-class String-field boundary. The most-derived field keeps its ordinary storage key.
func (vm *VM) lexicalInstanceFieldStorageName(receiver Value, owner string, field Field, name string) string {
	if field.Getter != nil || field.Setter != nil || !strings.EqualFold(field.Access, "private") || !strings.EqualFold(field.Type, "String") {
		return name
	}
	declaring, ok := vm.lookupClass(owner)
	if !ok || declaring.Dependency || declaring.Namespace != "" {
		return name
	}
	runtime, ok := vm.lookupClass(receiver.Type)
	if !ok || runtime.Dependency || runtime.Namespace != "" || runtime.APIVersion != declaring.APIVersion {
		return name
	}
	switch declaring.APIVersion {
	case "62.0", "67.0", "68.0":
	default:
		return name
	}
	derived, derivedOwner, ok := vm.lookupFieldForReceiver(receiver.Type, field.Name, false)
	if !ok || !strings.EqualFold(derivedOwner, receiver.Type) || strings.EqualFold(derivedOwner, owner) || !vm.isSubclass(derivedOwner, owner) || derived.Getter != nil || derived.Setter != nil || !strings.EqualFold(derived.Access, "private") || !strings.EqualFold(derived.Type, "String") {
		return name
	}
	return "__gladeLexicalInstance$" + strings.ToLower(owner) + "$" + strings.ToLower(name)
}

func isLexicalInstanceFieldStorage(name string) bool {
	return strings.HasPrefix(name, "__gladeLexicalInstance$")
}
