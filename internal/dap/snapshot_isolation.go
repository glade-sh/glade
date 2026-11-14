package dap

import "github.com/glade-sh/glade/internal/vm"

// isolateSnapshotValues detaches the debug view while the VM is stopped. VM
// debug clones can retain repeated stub-provider references into live state.
func isolateSnapshotValues(values map[string]vm.Value) map[string]vm.Value {
	if values == nil {
		return nil
	}
	out := make(map[string]vm.Value, len(values))
	for name, value := range values {
		out[name] = isolateSnapshotValue(value, make(map[uint64]bool))
	}
	return out
}

func isolateSnapshotValue(value vm.Value, seen map[uint64]bool) vm.Value {
	if value.Ref != 0 {
		if seen[value.Ref] {
			value.Fields = nil
			value.Map = nil
			value.MapKeys = nil
			value.List = nil
			value.Set = nil
			value.MapOrder = nil
			return value
		}
		seen[value.Ref] = true
		defer delete(seen, value.Ref)
	}
	cloneMap := func(m map[string]vm.Value) map[string]vm.Value {
		if m == nil {
			return nil
		}
		out := make(map[string]vm.Value, len(m))
		for key, child := range m {
			out[key] = isolateSnapshotValue(child, seen)
		}
		return out
	}
	cloneSlice := func(items []vm.Value) []vm.Value {
		if items == nil {
			return nil
		}
		out := make([]vm.Value, len(items))
		for i, child := range items {
			out[i] = isolateSnapshotValue(child, seen)
		}
		return out
	}
	value.Fields = cloneMap(value.Fields)
	value.Map = cloneMap(value.Map)
	value.MapKeys = cloneMap(value.MapKeys)
	value.List = cloneSlice(value.List)
	value.Set = cloneSlice(value.Set)
	value.MapOrder = append([]string(nil), value.MapOrder...)
	return value
}
