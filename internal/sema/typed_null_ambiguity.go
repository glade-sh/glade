package sema

import "strings"

// The captured rows reject one untyped null against the captured three
// String/Object/Id collection overloads even when their return types agree.
// Other generic signatures and unknown argument types keep their deferral.
func capturedBareNullCollectionAmbiguity(candidates []resolvedMember, argTypes []string) bool {
	return len(argTypes) == 1 && strings.EqualFold(argTypes[0], "null") && capturedCollectionOverloadBase(candidates) != ""
}

func capturedCollectionOverloadBase(candidates []resolvedMember) string {
	if len(candidates) != 3 {
		return ""
	}
	base := ""
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if len(candidate.member.Parameters) != 1 {
			return ""
		}
		param := semaCanonicalAssignableType(candidate.member.Parameters[0].Type)
		current, args := semaGenericBaseAndArgs(param)
		current = normalizeName(current)
		if base != "" && base != current {
			return ""
		}
		base = current
		switch base {
		case "list", "set":
			if len(args) != 1 {
				return ""
			}
		case "map":
			if len(args) != 2 || !strings.EqualFold(args[0], "String") {
				return ""
			}
		default:
			return ""
		}
		seen[normalizeName(args[len(args)-1])] = true
	}
	if len(seen) == 3 && seen["string"] && seen["object"] && seen["id"] {
		return base
	}
	return ""
}
