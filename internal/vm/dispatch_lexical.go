package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// An implicit receiver belongs to the enclosing declaration. Do not resolve
// that declaration as a bare type in the ambient namespace: a qualified type
// with the same simple name is a different class.
func (vm *VM) callLexicalMember(owner, member string, args []Value, result *Result) (Value, bool, error) {
	if owner == "" {
		return Null, false, nil
	}
	class, ok := vm.lookupClass(owner)
	if !ok {
		return Null, false, nil
	}
	// A local class and a dependency class can share one runtime name. The
	// executing declaration, not the name lookup, says which one encloses us.
	dependency := class.Dependency
	if current := vm.currentMethod.ClassName; strings.EqualFold(current, owner) || strings.EqualFold(current, runtimeClassName(class)) {
		dependency = vm.currentMethod.Dependency
	}
	owner = runtimeClassName(class)
	receiver := vm.Globals["this"]
	allowInstance := receiver.Kind == ValueObject
	method, found, ambiguous := vm.resolveLexicalMember(owner, member, args, allowInstance, dependency)
	if !found && !ambiguous {
		for _, outer := range lexicalOuterClasses(owner) {
			method, found, ambiguous = vm.resolveLexicalMember(outer, member, args, false, dependency)
			if found || ambiguous {
				break
			}
		}
	}
	if ambiguous {
		return Null, true, vm.ambiguousOverloadError(member, args)
	}
	if !found {
		return Null, false, nil
	}
	if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
		return Null, true, err
	}
	// Select the lexical surface first, then retain instance override dispatch.
	// Private methods bind to their declaration from API 61 onward; older
	// declarations allow overrides even without an explicit virtual modifier.
	if !method.IsStatic && (!strings.EqualFold(method.Access, "private") || apexversion.Before(method.APIVersion, 61)) {
		method = vm.resolveLexicalOverride(method, receiver.Type, member)
	}
	if err := vm.ensureClassInitialized(method.ClassName); err != nil {
		return Null, true, err
	}
	if vm.shouldEnqueueFuture(method) {
		value, err := vm.enqueueFuture(method, args, result)
		return value, true, err
	}
	if method.IsStatic {
		value, err := vm.callMethod(method, args, result)
		return value, true, err
	}
	value, err := vm.callMethodWithReceiver(method, receiver, args, result)
	return value, true, err
}

// Walk exact declaration owners, then their inherited members. Filtering the
// owner also excludes short aliases registered for a different namespace.
// A dependency class never extends a local class, so dependency provenance
// carries up the hierarchy; a local class may extend a dependency base.
func (vm *VM) resolveLexicalMember(owner, member string, args []Value, allowInstance, dependency bool) (Method, bool, bool) {
	seen := make(map[string]bool)
	for owner != "" && !seen[strings.ToLower(owner)] {
		seen[strings.ToLower(owner)] = true
		class, ok := vm.lookupClass(owner)
		if !ok {
			break
		}
		owner = runtimeClassName(class)
		dependency = dependency || class.Dependency
		var static, instance []Method
		for _, candidate := range vm.registeredMethodCandidates(owner + "." + member) {
			if !lexicalDeclarationMember(candidate, owner, dependency) {
				continue
			}
			if candidate.IsStatic {
				static = append(static, candidate)
			} else if allowInstance {
				instance = append(instance, candidate)
			}
		}
		if target, ok, ambiguous := vm.matchMethodByArgs(static, args); ok || ambiguous {
			return target, ok, ambiguous
		}
		if target, ok, ambiguous := vm.matchMethodByArgs(instance, args); ok || ambiguous {
			return target, ok, ambiguous
		}
		owner = vm.resolvedSuperClassName(class)
	}
	return Method{}, false, false
}

func (vm *VM) resolveLexicalOverride(surface Method, runtimeType, member string) Method {
	target := surface
	found := false
	dependency := false
	seen := make(map[string]bool)
	for runtimeType != "" && !seen[strings.ToLower(runtimeType)] {
		seen[strings.ToLower(runtimeType)] = true
		class, ok := vm.lookupClass(runtimeType)
		if !ok {
			break
		}
		owner := runtimeClassName(class)
		dependency = dependency || class.Dependency
		if strings.EqualFold(owner, surface.ClassName) {
			return target
		}
		if !found {
			for _, candidate := range vm.registeredMethodCandidates(owner + "." + member) {
				if !lexicalDeclarationMember(candidate, owner, dependency) || candidate.IsStatic || methodHasModifier(candidate.Modifiers, "abstract") || !vm.sameResolvedMethodParams(candidate, surface) {
					continue
				}
				if !found || vm.preferMethodCandidate(candidate, target) {
					target, found = candidate, true
				}
			}
		}
		runtimeType = vm.resolvedSuperClassName(class)
	}
	// A passive type or unrelated same-name class is not an implementation of
	// the selected lexical member.
	return surface
}

// A same-name dependency class registers its methods under the bare
// declaration name too, and a dependency in the project namespace shares the
// qualified name. The owner name alone cannot separate a local class from
// that package class; the declaration's provenance must also match.
func lexicalDeclarationMember(candidate Method, owner string, dependency bool) bool {
	return strings.EqualFold(candidate.ClassName, owner) && candidate.Dependency == dependency
}
