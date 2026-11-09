package vm

// A Queueable carries its fields through serialization when it is enqueued. An
// AccessLevel member comes back as a distinct object: it is not identical to
// the System/User constants, does not equal them and hashes differently, while
// its rendering still names the constants (org captures
// queueable_accesslevel_identity and access_level_rendering, API 62 and 67).
// References that shared one AccessLevel before the enqueue still share one
// copy after it (async_access_boundaries transport aliasSame and repeatedSame).
func detachAccessLevels(value *Value, seen map[uint64]bool) {
	detachAccessLevelsShared(value, seen, make(map[uint64]Value))
}

func detachAccessLevelsShared(value *Value, seen map[uint64]bool, copies map[uint64]Value) {
	if value == nil {
		return
	}
	if isAccessLevelValue(*value) {
		if accessLevelHasCustomScope(*value) {
			return
		}
		if value.Ref != 0 {
			if shared, ok := copies[value.Ref]; ok {
				*value = shared
				return
			}
		}
		original := value.Ref
		*value = accessLevelDetached(*value)
		if original != 0 {
			copies[original] = *value
		}
		return
	}
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
	}
	for name, field := range value.Fields {
		detachAccessLevelsShared(&field, seen, copies)
		value.Fields[name] = field
	}
	for i := range value.List {
		detachAccessLevelsShared(&value.List[i], seen, copies)
	}
	for i := range value.Set {
		detachAccessLevelsShared(&value.Set[i], seen, copies)
	}
	for key, item := range value.Map {
		detachAccessLevelsShared(&item, seen, copies)
		value.Map[key] = item
	}
}

func accessLevelDetached(original Value) Value {
	mode := original.Fields["currentAccessPermissions"].Text
	out := Object("AccessLevel")
	out.Ref = newValueRef()
	out.Text = mode
	out.Fields["SYSTEM_MODE"] = accessLevelValue("SYSTEM_MODE")
	out.Fields["USER_MODE"] = accessLevelValue("USER_MODE")
	out.Fields["currentAccessPermissions"] = String(mode)
	out.Fields["permSetId"] = Null
	return out
}
