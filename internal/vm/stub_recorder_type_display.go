package vm

import (
	"sort"
	"strings"
)

// Captured recorder fields display Type tokens as names, retaining any package
// qualification. Ordinary object fields keep their existing nested formatter.
func (vm *VM) stubRecorderProviderDisplayString(provider Value, seen map[uint64]bool) string {
	if provider.Ref != 0 {
		if seen[provider.Ref] {
			return "(already output)"
		}
		seen[provider.Ref] = true
		defer delete(seen, provider.Ref)
	}
	names := make([]string, 0, len(provider.Fields))
	for name := range provider.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+vm.stubRecorderFieldDisplayString(provider.Fields[name], seen))
	}
	return provider.Type + ":[" + strings.Join(parts, ", ") + "]"
}

func (vm *VM) stubRecorderFieldDisplayString(value Value, seen map[uint64]bool) string {
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Type") {
		if text := typeValueText(value); text != "" {
			return text
		}
	}
	if value.Kind == ValueList {
		parts := make([]string, 0, len(value.List))
		for _, item := range value.List {
			parts = append(parts, vm.stubRecorderFieldDisplayString(item, seen))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	}
	return vm.nestedDisplayString(value, seen)
}
