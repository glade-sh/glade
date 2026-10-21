package sema

// Enhanced-for accepts Iterable inherited through interfaces or superclasses.
func semaInheritedIterableElementType(typeName string, model *semaTypeMemberView) (string, bool) {
	seen := make(map[string]bool)
	var visit func(string) (string, bool)
	visit = func(name string) (string, bool) {
		if element, ok := semaIterableElementTypeInModel(name, model); ok {
			return element, true
		}
		members, key, ok := semaLookupTypeMembers(model, name)
		if !ok || seen[key] {
			return "", false
		}
		seen[key] = true
		for _, iface := range members.interfaces {
			if element, ok := visit(resolveNestedTypeReference(model, members.name, iface)); ok {
				return element, true
			}
		}
		if members.superClass != "" {
			return visit(resolveNestedTypeReference(model, members.name, members.superClass))
		}
		return "", false
	}
	return visit(typeName)
}
