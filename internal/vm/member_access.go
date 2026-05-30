package vm

import (
	"fmt"
	"strings"
)

func (vm *VM) checkMemberAccess(ownerClass, access, member string, modifierSets ...[]string) error {
	if err := vm.checkClassAccess(ownerClass, member, modifierSets...); err != nil {
		return err
	}
	if err := vm.checkNamespaceAccess(ownerClass, access, member, modifierSets...); err != nil {
		return err
	}
	switch strings.ToLower(access) {
	case "", "public", "global", "webservice":
		return nil
	case "private":
		if vm.currentClassIsTest() && hasAnyMethodModifier(modifierSets, "testvisible") {
			return nil
		}
		if vm.currentClassIsTest() && vm.hasTestVisibleAncestorMember(ownerClass, member) {
			return nil
		}
		if vm.sameAccessScope(vm.currentClass, ownerClass) {
			return nil
		}
		methodOwner := vm.currentMethod.ClassName
		if methodOwner == "" {
			methodOwner = classNameFromMethod(vm.currentMethod.Name)
		} else if vm.classNamespace(methodOwner) == "" {
			if owner := classNameFromMethod(vm.currentMethod.Name); owner != "" {
				methodOwner = owner
			}
		}
		if vm.sameAccessScope(methodOwner, ownerClass) {
			return nil
		}
	case "protected":
		if vm.currentClassIsTest() && hasAnyMethodModifier(modifierSets, "testvisible") {
			return nil
		}
		if vm.currentClassIsTest() && vm.hasTestVisibleAncestorMember(ownerClass, member) {
			return nil
		}
		if vm.sameAccessScope(vm.currentClass, ownerClass) || vm.isSubclass(vm.currentClass, ownerClass) || vm.isSubclass(ownerClass, vm.currentClass) {
			return nil
		}
		methodOwner := vm.currentMethod.ClassName
		if methodOwner == "" {
			methodOwner = classNameFromMethod(vm.currentMethod.Name)
		} else if vm.classNamespace(methodOwner) == "" {
			if owner := classNameFromMethod(vm.currentMethod.Name); owner != "" {
				methodOwner = owner
			}
		}
		if vm.sameAccessScope(methodOwner, ownerClass) || vm.isSubclass(methodOwner, ownerClass) || vm.isSubclass(ownerClass, methodOwner) {
			return nil
		}
	default:
		return nil
	}
	if vm.currentClass == "" {
		return fmt.Errorf("%s is %s and not visible", member, access)
	}
	return fmt.Errorf("%s is %s and not visible from %s", member, access, vm.currentClass)
}
func (vm *VM) hasTestVisibleAncestorMember(ownerClass, member string) bool {
	memberName := apexMethodMemberName(member)
	for superClass := vm.superClassName(ownerClass); superClass != ""; superClass = vm.superClassName(superClass) {
		methodKey := superClass + "." + memberName
		if method, ok := vm.Methods[methodKey]; ok && hasAnyMethodModifier([][]string{method.Modifiers}, "testvisible") {
			return true
		}
		for _, method := range vm.MethodOverloads[methodKey] {
			if hasAnyMethodModifier([][]string{method.Modifiers}, "testvisible") {
				return true
			}
		}
		if class, ok := vm.Classes[superClass]; ok {
			for _, field := range class.Fields {
				if strings.EqualFold(field.Name, memberName) && hasAnyMethodModifier([][]string{field.Modifiers}, "testvisible") {
					return true
				}
			}
		}
	}
	return false
}
func sameLexicalTopLevel(a, b string) bool {
	aTop, aNested := lexicalTopLevel(a)
	bTop, bNested := lexicalTopLevel(b)
	return aNested && bNested && strings.EqualFold(aTop, bTop)
}
func (vm *VM) sameAccessScope(left, right string) bool {
	var lbuf, rbuf [3]string
	ln := vm.fillAccessScopeNames(left, &lbuf)
	rn := vm.fillAccessScopeNames(right, &rbuf)
	for i := 0; i < ln; i++ {
		for j := 0; j < rn; j++ {
			if sameOrNestedTypeFold(lbuf[i], rbuf[j]) || sameLexicalTopLevel(lbuf[i], rbuf[j]) {
				return true
			}
		}
	}
	return false
}
func (vm *VM) fillAccessScopeNames(name string, buf *[3]string) int {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0
	}
	n := 0
	buf[n] = name
	n++
	if class, ok := vm.classForAccess(name); ok {
		buf[n] = class.Name
		n++
		if class.Namespace != "" {
			buf[n] = runtimeClassName(class)
			n++
		}
	}
	return n
}
func sameOrNestedTypeFold(left, right string) bool {
	return strings.EqualFold(left, right) || hasTypePrefixFold(left, right) || hasTypePrefixFold(right, left)
}
func hasTypePrefixFold(value, prefix string) bool {
	if prefix == "" || len(value) <= len(prefix) || value[len(prefix)] != '.' {
		return false
	}
	return strings.EqualFold(value[:len(prefix)], prefix)
}
func hasSuffixFold(value, suffix string) bool {
	if len(value) < len(suffix) {
		return false
	}
	return strings.EqualFold(value[len(value)-len(suffix):], suffix)
}
func hasPrefixFold(value, prefix string) bool {
	if len(value) < len(prefix) {
		return false
	}
	return strings.EqualFold(value[:len(prefix)], prefix)
}
func lexicalTopLevel(className string) (string, bool) {
	dot := strings.IndexByte(className, '.')
	if dot <= 0 {
		return className, false
	}
	return className[:dot], true
}
func (vm *VM) checkClassAccess(ownerClass, member string, modifierSets ...[]string) error {
	class, ok := vm.classForAccess(ownerClass)
	if !ok || class.Namespace == "" {
		return nil
	}
	if vm.memberAccessScopeMatches(ownerClass) {
		return nil
	}
	callerNS := vm.currentCallerNamespace()
	if callerNS == class.Namespace {
		return nil
	}
	switch strings.ToLower(class.Access) {
	case "global", "webservice":
		return nil
	}
	if hasAnyMethodModifier([][]string{class.Modifiers}, "namespaceaccessible") || hasAnyMethodModifier(modifierSets, "namespaceaccessible") {
		return nil
	}
	if vm.hasAccessibleInheritedMember(ownerClass, member) {
		return nil
	}
	if vm.currentClass == "" {
		return fmt.Errorf("%s is not global and not visible outside namespace %s", member, class.Namespace)
	}
	return fmt.Errorf("%s is not global and not visible from namespace %s", member, callerNS)
}
func (vm *VM) currentClassIsTest() bool {
	class, ok := vm.Classes[vm.currentClass]
	return ok && class.IsTest
}
func hasAnyMethodModifier(modifierSets [][]string, expected string) bool {
	for _, modifiers := range modifierSets {
		for _, modifier := range modifiers {
			if strings.EqualFold(strings.TrimPrefix(modifier, "@"), expected) {
				return true
			}
		}
	}
	return false
}
func (vm *VM) checkNamespaceAccess(ownerClass, access, member string, modifierSets ...[]string) error {
	class, ok := vm.classForAccess(ownerClass)
	ownerNS := ""
	if ok {
		ownerNS = class.Namespace
	}
	if ownerNS == "" {
		return nil
	}
	if vm.memberAccessScopeMatches(ownerClass) {
		return nil
	}
	callerNS := vm.currentCallerNamespace()
	if callerNS == ownerNS {
		return nil
	}
	switch strings.ToLower(access) {
	case "global", "webservice":
		return nil
	}
	if hasAnyMethodModifier(modifierSets, "namespaceaccessible") {
		return nil
	}
	if vm.hasAccessibleInheritedMember(ownerClass, member) {
		return nil
	}
	if vm.currentClass == "" {
		return fmt.Errorf("%s is not global and not visible outside namespace %s", member, ownerNS)
	}
	return fmt.Errorf("%s is not global and not visible from namespace %s", member, callerNS)
}
func (vm *VM) hasAccessibleInheritedMember(ownerClass, member string) bool {
	memberName := apexMethodMemberName(member)
	if memberName == "" {
		return false
	}
	class, ok := vm.classForAccess(ownerClass)
	if !ok {
		return false
	}
	return vm.hasAccessibleInheritedMemberFromClass(class, memberName, make(map[string]bool))
}
func (vm *VM) hasAccessibleInheritedMemberFromClass(class Class, memberName string, seen map[string]bool) bool {
	className := runtimeClassName(class)
	if className == "" || seen[strings.ToLower(className)] {
		return false
	}
	seen[strings.ToLower(className)] = true
	for _, parentName := range append([]string{vm.resolvedSuperClassName(class)}, vm.resolvedInterfaceNames(class)...) {
		parentName = strings.TrimSpace(parentName)
		if parentName == "" {
			continue
		}
		if vm.classHasAccessibleMember(parentName, memberName) {
			return true
		}
		parent, ok := vm.classForAccess(parentName)
		if !ok {
			continue
		}
		if vm.hasAccessibleInheritedMemberFromClass(parent, memberName, seen) {
			return true
		}
	}
	return false
}
func (vm *VM) classHasAccessibleMember(className, memberName string) bool {
	className = strings.TrimSpace(className)
	if className == "" || memberName == "" {
		return false
	}
	names := []string{className}
	if class, ok := vm.classForAccess(className); ok {
		names = append(names, class.Name, runtimeClassName(class), shortTypeName(class.Name))
	}
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		if vm.methodSurfaceAccessible(name+"."+memberName, name) {
			return true
		}
	}
	return false
}
func (vm *VM) methodSurfaceAccessible(methodKey, ownerClass string) bool {
	for _, method := range vm.MethodOverloads[methodKey] {
		if vm.memberSurfaceMethodAccessible(method, ownerClass) {
			return true
		}
	}
	if method, ok := vm.Methods[methodKey]; ok && vm.memberSurfaceMethodAccessible(method, ownerClass) {
		return true
	}
	return false
}
func (vm *VM) memberSurfaceMethodAccessible(method Method, ownerClass string) bool {
	if !vm.methodClassMatchesAccessOwner(method.ClassName, ownerClass) {
		return false
	}
	switch strings.ToLower(method.Access) {
	case "global", "webservice":
		return true
	}
	return methodHasModifier(method.Modifiers, "namespaceaccessible")
}
func (vm *VM) methodClassMatchesAccessOwner(methodClass, ownerClass string) bool {
	methodClass = strings.TrimSpace(methodClass)
	ownerClass = strings.TrimSpace(ownerClass)
	if methodClass == "" || ownerClass == "" {
		return true
	}
	if strings.EqualFold(methodClass, ownerClass) || vm.sameAccessScope(methodClass, ownerClass) {
		return true
	}
	methodOwner, methodOK := vm.classForAccess(methodClass)
	accessOwner, ownerOK := vm.classForAccess(ownerClass)
	if methodOK && ownerOK {
		return strings.EqualFold(runtimeClassName(methodOwner), runtimeClassName(accessOwner))
	}
	return false
}
func (vm *VM) memberAccessScopeMatches(ownerClass string) bool {
	if vm.sameAccessScope(vm.currentClass, ownerClass) {
		return true
	}
	methodOwner := vm.currentMethod.ClassName
	if methodOwner == "" {
		methodOwner = classNameFromMethod(vm.currentMethod.Name)
	}
	return vm.sameAccessScope(methodOwner, ownerClass)
}
