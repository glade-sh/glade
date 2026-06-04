package vm

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func platformVersionTypeName(typeName string) bool {
	return strings.EqualFold(typeName, "Version") || strings.EqualFold(typeName, "Package.Version")
}
func (vm *VM) resolveTypeNameInCurrentExecutionContext(typeName string) string {
	for _, owner := range vm.currentTypeResolutionOwners() {
		if resolved := vm.resolveTypeNameInClass(owner, typeName); resolved != "" && !strings.EqualFold(resolved, typeName) {
			return resolved
		}
	}
	return vm.resolveTypeNameInClass(vm.currentClass, typeName)
}
func (vm *VM) resolveNestedTypeNameInCurrentExecutionContext(typeName string) string {
	if strings.TrimSpace(typeName) == "" || strings.Contains(typeName, ".") {
		return ""
	}
	for _, owner := range vm.currentTypeResolutionOwners() {
		if resolved, ok := vm.resolveNestedTypeInClassHierarchy(owner, typeName); ok {
			return resolved
		}
	}
	return ""
}
func (vm *VM) resolveRuntimeTypeName(typeName string) string {
	if resolved := vm.resolveTypeNameInClass(vm.currentClass, typeName); resolved != "" {
		return resolved
	}
	if resolved, ok := vm.resolveClassName(typeName); ok {
		if !strings.Contains(typeName, ".") && !strings.Contains(resolved, ".") {
			if nested, nestedOK := vm.resolveUniqueNestedTypeName(typeName); nestedOK {
				return nested
			}
		}
		return resolved
	}
	if resolved, ok := vm.resolveUniqueNestedTypeName(typeName); ok {
		return resolved
	}
	return typeName
}
func (vm *VM) uniqueConcreteOverride(receiver Value, baseType, method string, args []Value) (Method, bool) {
	var found Method
	count := 0
	var fieldMatched Method
	fieldMatchCount := 0
	var qualifierMatched Method
	qualifierMatchCount := 0
	for className, class := range vm.Classes {
		if strings.EqualFold(className, baseType) || (!vm.typeMatches(className, baseType, make(map[string]bool)) && !classExtendsType(class, baseType)) {
			continue
		}
		if vm.classIsTestScoped(className) && !sameLexicalTopLevel(vm.currentClass, className) {
			continue
		}
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(className, method, args)
		if (!ok || methodHasModifier(target.Modifiers, "abstract")) && !ambiguous {
			if staticTarget, found := vm.staticConcreteOverrideForAbstract(baseType, className, method, args); found {
				target = staticTarget
				ok = true
			}
		}
		if !ok || ambiguous || methodHasModifier(target.Modifiers, "abstract") {
			continue
		}
		count++
		found = target
		if sameTypeQualifier(baseType, className) {
			qualifierMatched = target
			qualifierMatchCount++
		}
		if class, ok := vm.Classes[className]; ok && receiverHasClassField(receiver, class) {
			fieldMatched = target
			fieldMatchCount++
		}
	}
	if count > 1 {
		if qualifierMatchCount == 1 {
			return qualifierMatched, true
		}
		return fieldMatched, fieldMatchCount == 1
	}
	return found, count == 1
}
func (vm *VM) staticTypeNameCandidates(typeName string) []string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return nil
	}
	out := make([]string, 0, 4)
	appendUnique := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		for _, existing := range out {
			if strings.EqualFold(existing, name) {
				return
			}
		}
		out = append(out, name)
	}
	if !strings.Contains(typeName, ".") {
		if namespace := strings.TrimSpace(vm.currentExecutionNamespace()); namespace != "" {
			appendUnique(namespace + "." + typeName)
			if class, ok := vm.lookupClassInNamespace(namespace, typeName); ok {
				appendUnique(class.Name)
				if class.Namespace != "" {
					appendUnique(class.Namespace + "." + class.Name)
				}
			}
		}
	}
	appendUnique(typeName)
	return out
}
func (vm *VM) platformQualifiedStaticTypeMatches(staticType, runtimeType string) bool {
	if strings.TrimSpace(staticType) == "" || strings.TrimSpace(runtimeType) == "" {
		return false
	}
	if short, ok := stripLeadingSystemNamespace(staticType); ok && strings.EqualFold(short, runtimeType) {
		return true
	}
	if !strings.Contains(staticType, ".") || !strings.EqualFold(shortTypeName(staticType), runtimeType) {
		return false
	}
	return generatedPlatformTypeName(staticType) ||
		isCanonicalRuntimeTypeName(staticType) ||
		platformVersionTypeName(staticType)
}
func (vm *VM) bestMethodByConversionScore(applicable []Method, args []Value) (Method, bool) {
	bestIndex := -1
	bestScore := math.MinInt
	for i, candidate := range applicable {
		if len(candidate.Params) != len(args) {
			return Method{}, false
		}
		score := 0
		for j, param := range candidate.Params {
			resolutionClass := vm.methodTypeResolutionClass(candidate)
			paramType := vm.resolveTypeNameInClass(resolutionClass, param.Type)
			part := vm.conversionScore(paramType, vm.valueWithTypesResolvedInClass(resolutionClass, args[j]))
			if part < 0 {
				return Method{}, false
			}
			score += part
		}
		if score > bestScore {
			bestScore = score
			bestIndex = i
			continue
		}
		if score == bestScore {
			bestIndex = -1
		}
	}
	if bestIndex < 0 {
		return Method{}, false
	}
	return applicable[bestIndex], true
}
func (vm *VM) resolveTypeNameInClass(className, typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	if strings.EqualFold(typeName, "Type") {
		return "Type"
	}
	if base := collectionBase(typeName); base != "" {
		element, ok := collectionElementType(typeName)
		if !ok {
			return typeName
		}
		return base + "<" + vm.resolveTypeNameInClass(className, element) + ">"
	}
	if keyType, valueType, ok := mapTypeArgs(typeName); ok {
		return "Map<" + vm.resolveTypeNameInClass(className, keyType) + "," + vm.resolveTypeNameInClass(className, valueType) + ">"
	}
	if strings.Contains(typeName, ".") && className != "" {
		if owner, member, ok := strings.Cut(typeName, "."); ok && strings.EqualFold(owner, shortTypeName(className)) {
			if nested, nestedOK := vm.lookupClass(typeName); nestedOK && strings.Contains(nested.Name, ".") {
				return typeName
			}
			if class, classOK := vm.lookupClass(className); classOK {
				if namespace := strings.TrimSpace(class.Namespace); namespace != "" {
					if topLevel, topLevelOK := vm.lookupClassInNamespace(namespace, member); topLevelOK && !strings.Contains(topLevel.Name, ".") {
						return runtimeClassName(topLevel)
					}
				}
			}
		}
	}
	if strings.Contains(typeName, ".") || className == "" {
		return typeName
	}
	if platformType, ok := platformShortTypeAlias(typeName); ok {
		if nested, nestedOK := vm.resolveExactNestedTypeInClassHierarchy(className, typeName); nestedOK {
			return nested
		}
		return platformType
	}
	if resolved, ok := vm.resolveNestedTypeInClassHierarchy(className, typeName); ok {
		return resolved
	}
	if class, ok := vm.lookupClass(className); ok {
		if namespace := strings.TrimSpace(class.Namespace); namespace != "" {
			if sameNamespaceClass, ok := vm.lookupClassInNamespace(namespace, typeName); ok {
				if !strings.Contains(sameNamespaceClass.Name, ".") {
					return runtimeClassName(sameNamespaceClass)
				}
			}
		}
	} else if namespace := strings.TrimSpace(vm.currentCallerNamespace()); namespace != "" {
		if sameNamespaceClass, ok := vm.lookupClassInNamespace(namespace, typeName); ok {
			if !strings.Contains(sameNamespaceClass.Name, ".") {
				return runtimeClassName(sameNamespaceClass)
			}
		}
	}
	if isCommonSObjectTypeName(typeName) {
		return typeName
	}
	return typeName
}
func (vm *VM) resolveNestedTypeInClassHierarchy(className, typeName string) (string, bool) {
	key := nestedTypeKey{ClassName: className, TypeName: typeName}
	if cached, ok := vm.nestedTypeHierarchyCache[key]; ok {
		return cached.Name, cached.OK
	}
	name, ok := vm.resolveNestedTypeInClassHierarchyUncached(className, typeName)
	if vm.nestedTypeHierarchyCache == nil {
		vm.nestedTypeHierarchyCache = make(map[nestedTypeKey]nestedTypeResult)
	}
	vm.nestedTypeHierarchyCache[key] = nestedTypeResult{Name: name, OK: ok}
	return name, ok
}
func (vm *VM) resolveNestedTypeInClassHierarchyUncached(className, typeName string) (string, bool) {
	for owner := className; owner != ""; {
		var ownerBuf [2]string
		ownerN := fillLexicalOwnerCandidates(owner, &ownerBuf)
		for oi := 0; oi < ownerN; oi++ {
			ownerCandidate := ownerBuf[oi]
			candidate := ownerCandidate + "." + typeName
			if class, ok := vm.lookupClass(candidate); ok {
				if namespacedRuntimeClassMatch(candidate, class) {
					return runtimeClassName(class), true
				}
				if isNamespaceClassAlias(candidate, class) {
					return class.Name, true
				}
				if class.Namespace != "" && !strings.Contains(class.Name, ".") && strings.Contains(candidate, ".") {
					continue
				}
				if strings.Contains(candidate, ".") && !strings.Contains(class.Name, ".") {
					return candidate, true
				}
				return class.Name, true
			}
		}
		seenSupers := map[string]bool{}
		for super := vm.superClassName(owner); super != ""; super = vm.superClassName(super) {
			key := strings.ToLower(super)
			if seenSupers[key] {
				break
			}
			seenSupers[key] = true
			var ownerBuf [2]string
			ownerN := fillLexicalOwnerCandidates(super, &ownerBuf)
			for oi := 0; oi < ownerN; oi++ {
				ownerCandidate := ownerBuf[oi]
				candidate := ownerCandidate + "." + typeName
				if class, ok := vm.lookupClass(candidate); ok {
					if namespacedRuntimeClassMatch(candidate, class) {
						return runtimeClassName(class), true
					}
					if isNamespaceClassAlias(candidate, class) {
						return class.Name, true
					}
					if class.Namespace != "" && !strings.Contains(class.Name, ".") && strings.Contains(candidate, ".") {
						continue
					}
					if strings.Contains(candidate, ".") && !strings.Contains(class.Name, ".") {
						return candidate, true
					}
					return class.Name, true
				}
			}
		}
		dot := strings.LastIndex(owner, ".")
		if dot < 0 {
			break
		}
		owner = owner[:dot]
	}
	return "", false
}
func (vm *VM) resolveLexicalNestedTypeName(owner, typeName string) (string, bool) {
	for owner = strings.TrimSpace(owner); owner != ""; {
		var ownerBuf [2]string
		ownerN := fillLexicalOwnerCandidates(owner, &ownerBuf)
		for oi := 0; oi < ownerN; oi++ {
			ownerCandidate := ownerBuf[oi]
			candidate := ownerCandidate + "." + typeName
			if class, ok := vm.lookupClass(candidate); ok {
				if namespacedRuntimeClassMatch(candidate, class) {
					return runtimeClassName(class), true
				}
				if isNamespaceClassAlias(candidate, class) {
					return class.Name, true
				}
				if class.Namespace != "" && !strings.Contains(class.Name, ".") && strings.Contains(candidate, ".") {
					continue
				}
				if strings.Contains(candidate, ".") && !strings.Contains(class.Name, ".") {
					return candidate, true
				}
				return class.Name, true
			}
		}
		dot := strings.LastIndex(owner, ".")
		if dot < 0 {
			break
		}
		owner = owner[:dot]
	}
	return "", false
}
func (vm *VM) typeAssignableTo(from, to string) bool {
	from = canonicalRuntimePlatformType(from)
	to = canonicalRuntimePlatformType(to)
	if strings.EqualFold(from, to) || strings.EqualFold(to, "Object") {
		return true
	}
	if vm.namespaceAliasEquivalent(from, to) {
		return true
	}
	if vm.namespaceTopLevelMemberAliasEquivalent(from, to) {
		return true
	}
	if namespaceQualifiedTypeEquivalent(from, to) {
		return true
	}
	if strings.EqualFold(to, "ApexPages.Component") && hasPrefixFold(strings.TrimSpace(from), "component.") {
		return true
	}
	if platformTokenTypeAlias(from, to) {
		return true
	}
	if frameworkMockSupportTypesMatch(from, to) {
		return true
	}
	if (strings.EqualFold(from, "String") && strings.EqualFold(to, "Id")) ||
		(strings.EqualFold(from, "Id") && strings.EqualFold(to, "String")) {
		return true
	}
	if messagingEmailAssignable(from, to) {
		return true
	}
	if strings.EqualFold(to, "Cache.Partition") &&
		(strings.EqualFold(from, "Cache.OrgPartition") || strings.EqualFold(from, "Cache.SessionPartition")) {
		return true
	}
	if strings.EqualFold(from, "Date") && strings.EqualFold(to, "Datetime") {
		return true
	}
	if strings.EqualFold(to, "sObject") && vm.isSObjectLikeType(from) {
		return true
	}
	if vm.isSObjectLikeType(from) && vm.isSObjectLikeType(to) && sObjectTypeNamespaceEquivalent(from, to) {
		return true
	}
	if collectionBase(from) != "" && strings.EqualFold(collectionBase(from), collectionBase(to)) {
		if vm.sObjectCollectionDowncastAssignable(from, to) {
			return true
		}
		fromElement, fromOK := collectionElementType(from)
		toElement, toOK := collectionElementType(to)
		if fromOK && toOK {
			return vm.typeAssignableTo(fromElement, toElement)
		}
	}
	if collectionBase(to) == "Iterable" && (collectionBase(from) == "List" || collectionBase(from) == "Set") {
		fromElement, fromOK := collectionElementType(from)
		toElement, toOK := collectionElementType(to)
		if fromOK && toOK {
			return vm.typeAssignableTo(fromElement, toElement)
		}
	}
	if fromKey, fromValue, fromOK := mapTypeArgs(from); fromOK {
		toKey, toValue, toOK := mapTypeArgs(to)
		if toOK {
			return vm.typeAssignableTo(fromKey, toKey) && vm.typeAssignableTo(fromValue, toValue)
		}
	}
	if !strings.Contains(to, ".") && vm.typeHasShortAncestor(from, to, make(map[string]bool)) {
		return true
	}
	if strings.Contains(to, ".") && !hasPrefixFold(to, "System.") && vm.typeHasShortAncestor(from, shortTypeName(to), make(map[string]bool)) {
		return true
	}
	if numericConversionScore(to, from) >= 0 {
		return true
	}
	if _, ok := vm.typeDistance(from, to, make(map[string]bool)); ok {
		return true
	}
	return false
}
func stripLeadingTypeNamespace(typeName string) string {
	first := strings.Index(typeName, ".")
	if first < 0 {
		return typeName
	}
	rest := typeName[first+1:]
	if strings.Contains(rest, ".") {
		return rest
	}
	return typeName
}
func sObjectTypeNamespaceEquivalent(left, right string) bool {
	leftBase := stripSObjectNamespacePrefix(canonicalRuntimePlatformType(left))
	rightBase := stripSObjectNamespacePrefix(canonicalRuntimePlatformType(right))
	return strings.EqualFold(leftBase, rightBase)
}
func messagingEmailAssignable(from, to string) bool {
	if !strings.EqualFold(to, "Messaging.Email") {
		return false
	}
	return strings.EqualFold(from, "Messaging.SingleEmailMessage") ||
		strings.EqualFold(from, "Messaging.MassEmailMessage")
}
func (vm *VM) conversionScore(paramType string, value Value) int {
	if value.Kind == ValueNull {
		if value.Type != "" {
			if strings.EqualFold(paramType, value.Type) {
				return 1000
			}
			if collectionBase(value.Type) != "" && collectionBase(paramType) != "" {
				if vm.typeAssignableTo(value.Type, paramType) {
					return 900
				}
				if vm.sObjectCollectionDowncastAssignable(value.Type, paramType) {
					return 850
				}
				return -1
			}
			if isMapType(value.Type) && isMapType(paramType) {
				if vm.typeAssignableTo(value.Type, paramType) {
					return 900
				}
				return -1
			}
			if vm.typeAssignableTo(value.Type, paramType) {
				return 900
			}
			return 1
		}
		if collectionBase(paramType) != "" || isMapType(paramType) {
			return 0
		}
		return 1
	}
	if value.Static != "" {
		if strings.EqualFold(paramType, value.Static) {
			return 1000
		}
		if platformTokenTypeAlias(value.Static, paramType) {
			return 1000
		}
		if vm.typeAssignableTo(value.Static, paramType) {
			return 900
		}
	}
	valueType := valueTypeName(value)
	if strings.EqualFold(paramType, valueType) {
		return 1000
	}
	if platformTokenTypeAlias(valueType, paramType) {
		return 1000
	}
	if isDescribeSObjectResultType(paramType) && isSObjectTypeToken(value) {
		return 900
	}
	if collectionBase(valueType) != "" && collectionBase(paramType) != "" {
		if vm.typeAssignableTo(valueType, paramType) {
			return 900
		}
		if vm.sObjectCollectionDowncastAssignable(valueType, paramType) {
			return 850
		}
		if vm.collectionElementsAssignable(paramType, value) {
			return 850
		}
		return -1
	}
	if isMapType(valueType) && isMapType(paramType) {
		if vm.typeAssignableTo(valueType, paramType) {
			return 900
		}
		if vm.mapEntriesAssignable(paramType, value) {
			return 850
		}
		return -1
	}
	if score := numericConversionScore(paramType, valueType); score >= 0 {
		return score
	}
	if value.Kind == ValueObject && strings.EqualFold(paramType, value.Type) {
		return 950
	}
	if value.Kind == ValueObject && value.Runtime != "" && strings.EqualFold(paramType, value.Runtime) {
		return 925
	}
	if vm.typeAssignableTo(valueType, paramType) {
		return 900
	}
	if strings.EqualFold(paramType, "Object") {
		return 10
	}
	if value.Kind == ValueObject {
		if distance, ok := vm.typeDistance(value.Type, paramType, make(map[string]bool)); ok {
			return 800 - distance
		}
	}
	if value.Kind == ValueList && collectionBase(paramType) == "" && vm.isSObjectLikeType(paramType) {
		if vm.inlineSOQLListAssignableTo(paramType, value) {
			return 850
		}
		return -1
	}
	if err := vm.ensureAssignable(paramType, value); err != nil {
		return -1
	}
	return 1
}

func (vm *VM) inlineSOQLListAssignableTo(paramType string, value Value) bool {
	if value.Kind != ValueList || inlineSOQLQueryText(value) == "" || !vm.isSObjectLikeType(paramType) {
		return false
	}
	if elementType := vm.inlineSOQLListElementType(value); elementType != "" {
		return vm.typeAssignableTo(elementType, paramType) || vm.typeMatches(elementType, paramType, make(map[string]bool))
	}
	return false
}

func (vm *VM) inlineSOQLListElementType(value Value) string {
	for _, typeName := range []string{value.Type, value.Runtime, value.Static} {
		if elementType, ok := collectionElementType(typeName); ok && strings.TrimSpace(elementType) != "" {
			return elementType
		}
	}
	if len(value.List) == 1 {
		for _, typeName := range []string{value.List[0].Static, value.List[0].Runtime, value.List[0].Type} {
			if strings.TrimSpace(typeName) != "" {
				return typeName
			}
		}
	}
	if queryText := inlineSOQLQueryText(value); queryText != "" {
		return vm.soqlResultObjectNameWithExpander(queryText, vm.expandSOQLBinds)
	}
	return ""
}

func (vm *VM) sObjectCollectionDowncastAssignable(fromType, toType string) bool {
	fromElement, fromOK := collectionElementType(fromType)
	toElement, toOK := collectionElementType(toType)
	if !fromOK || !toOK {
		return false
	}
	if !strings.EqualFold(fromElement, "sObject") {
		return false
	}
	return vm.isSObjectLikeType(toElement)
}
func (vm *VM) collectionElementsAssignable(paramType string, value Value) bool {
	elementType, ok := collectionElementType(paramType)
	if !ok {
		return false
	}
	paramBase := collectionBase(paramType)
	switch value.Kind {
	case ValueList:
		if paramBase != "List" && paramBase != "Iterable" {
			return false
		}
		if len(value.List) == 0 {
			return false
		}
		for _, item := range value.List {
			if err := vm.ensureAssignable(elementType, item); err != nil {
				return false
			}
		}
		return true
	case ValueSet:
		if paramBase != "Set" && paramBase != "Iterable" {
			return false
		}
		if len(value.Set) == 0 {
			return false
		}
		for _, item := range value.Set {
			if err := vm.ensureAssignable(elementType, item); err != nil {
				return false
			}
		}
		return true
	default:
		return false
	}
}
func (vm *VM) mapEntriesAssignable(paramType string, value Value) bool {
	keyType, valueType, ok := mapTypeArgs(paramType)
	if !ok || value.Kind != ValueMap {
		return false
	}
	if len(value.Map) == 0 {
		// Local DTO JSON fields can carry untyped empty maps; overload specificity still picks the surface method.
		return true
	}
	for rawKey, item := range value.Map {
		if err := vm.ensureAssignable(keyType, valueFromMapKey(rawKey)); err != nil {
			return false
		}
		if err := vm.ensureAssignable(valueType, item); err != nil {
			return false
		}
	}
	return true
}
func numericConversionScore(paramType, valueType string) int {
	switch valueType {
	case "Integer":
		switch paramType {
		case "Long":
			return 900
		case "Decimal":
			return 800
		case "Double":
			return 700
		}
	case "Long":
		switch paramType {
		case "Decimal":
			return 800
		case "Double":
			return 700
		}
	case "Decimal":
		if paramType == "Double" {
			return 800
		}
	}
	return -1
}
func (vm *VM) typeDistance(typeName, target string, seen map[string]bool) (int, bool) {
	typeName = systemInterfaceAlias(typeName)
	target = systemInterfaceAlias(target)
	if resolved, ok := vm.resolveClassName(typeName); ok {
		typeName = resolved
	}
	if resolved, ok := vm.resolveClassName(target); ok {
		target = resolved
	}
	if typeName == "" || seen[typeName] {
		return 0, false
	}
	if strings.EqualFold(typeName, target) || vm.classNamesReferToSameRuntimeType(typeName, target) {
		return 0, true
	}
	if !strings.Contains(target, ".") && strings.EqualFold(shortTypeName(typeName), target) {
		return 0, true
	}
	seen[typeName] = true
	class, ok := vm.lookupClass(typeName)
	if !ok {
		return 0, false
	}
	best := 0
	found := false
	if superName := vm.resolvedSuperClassName(class); superName != "" {
		if distance, ok := vm.typeDistance(superName, target, seen); ok {
			best = distance + 1
			found = true
		}
	}
	for _, iface := range class.Interfaces {
		resolvedInterface := vm.resolvedInterfaceName(class, iface)
		if distance, ok := vm.typeDistance(resolvedInterface, target, seen); ok {
			distance++
			if !found || distance < best {
				best = distance
				found = true
			}
		}
		if !strings.Contains(resolvedInterface, ".") {
			if distance, ok := vm.typeDistance(class.Name+"."+iface, target, seen); ok {
				distance++
				if !found || distance < best {
					best = distance
					found = true
				}
			}
		}
	}
	return best, found
}
func valueTypeName(value Value) string {
	switch value.Kind {
	case ValueInt:
		if value.Type != "" {
			return value.Type
		}
		return "Integer"
	case ValueDecimal:
		return "Decimal"
	case ValueBool:
		return "Boolean"
	case ValueString:
		if value.Type != "" {
			return value.Type
		}
		return "String"
	case ValueList:
		if value.Type != "" {
			return value.Type
		}
		return "List"
	case ValueSet:
		if value.Type != "" {
			return value.Type
		}
		return "Set"
	case ValueMap:
		if value.Type != "" {
			return value.Type
		}
		return "Map"
	case ValueObject:
		if value.Static != "" {
			return value.Static
		}
		return value.Type
	case ValueNull:
		return "null"
	default:
		return string(value.Kind)
	}
}
func runtimeValueTypeName(value Value) string {
	if value.Kind == ValueObject {
		if runtime := runtimeObjectType(value); runtime != "" {
			return runtime
		}
		if value.Static != "" {
			return value.Static
		}
		return "Object"
	}
	if value.Kind == ValueList || value.Kind == ValueSet || value.Kind == ValueMap {
		if typed := typeExceptionCollectionName(value.Type); typed != "" {
			return typed
		}
		if value.Kind == ValueMap {
			keyType := "ANY"
			for _, key := range value.MapKeys {
				if key.Kind == ValueString {
					keyType = "String"
					break
				}
			}
			return "Map<" + keyType + ",ANY>"
		}
		if value.Kind == ValueList {
			return "List<ANY>"
		}
		return "Set<ANY>"
	}
	return valueTypeName(value)
}
func (vm *VM) typeMatches(typeName, target string, seen map[string]bool) bool {
	typeName = systemInterfaceAlias(typeName)
	target = systemInterfaceAlias(target)
	if resolved, ok := vm.resolveClassName(typeName); ok {
		typeName = resolved
	}
	if resolved, ok := vm.resolveClassName(target); ok {
		target = resolved
	}
	if vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(typeName); ok {
			typeName = resolved
		}
		if resolved, ok := vm.resolveObjectName(target); ok {
			target = resolved
		}
	}
	if typeName == "" || seen[typeName] {
		return false
	}
	if strings.EqualFold(typeName, target) {
		return true
	}
	if vm.classNamesReferToSameRuntimeType(typeName, target) {
		return true
	}
	if collectionBase(typeName) != "" && strings.EqualFold(collectionBase(typeName), collectionBase(target)) {
		fromElement, fromOK := collectionElementType(typeName)
		toElement, toOK := collectionElementType(target)
		if fromOK && toOK && (vm.typeAssignableTo(fromElement, toElement) || vm.typeMatches(fromElement, toElement, make(map[string]bool))) {
			return true
		}
	}
	if !strings.Contains(target, ".") && strings.EqualFold(shortTypeName(typeName), target) {
		return true
	}
	if platformTokenTypeAlias(typeName, target) {
		return true
	}
	if frameworkMockSupportTypesMatch(typeName, target) {
		return true
	}
	if strings.EqualFold(target, "sObject") && vm.isSObjectLikeType(typeName) {
		return true
	}
	if builtinExceptionTypeMatches(typeName, target) {
		return true
	}
	seen[typeName] = true
	class, ok := vm.lookupClass(typeName)
	if !ok {
		return false
	}
	if superName := vm.resolvedSuperClassName(class); superName != "" {
		if !strings.Contains(superName, ".") {
			if nestedSuperClass, ok := nestedSiblingTypeName(class.Name, superName); ok && vm.typeMatches(nestedSuperClass, target, seen) {
				return true
			}
		}
		if vm.typeMatches(superName, target, seen) {
			return true
		}
	}
	for _, iface := range class.Interfaces {
		resolvedInterface := vm.resolvedInterfaceName(class, iface)
		if strings.EqualFold(shortTypeName(resolvedInterface), shortTypeName(target)) {
			return true
		}
		if vm.typeMatches(resolvedInterface, target, seen) {
			return true
		}
		if !strings.Contains(resolvedInterface, ".") && vm.typeMatches(class.Name+"."+iface, target, seen) {
			return true
		}
	}
	return false
}
func nestedSiblingTypeName(className, typeName string) (string, bool) {
	if strings.TrimSpace(typeName) == "" || strings.Contains(typeName, ".") {
		return "", false
	}
	dot := strings.LastIndex(className, ".")
	if dot <= 0 || dot == len(className)-1 {
		return "", false
	}
	return className[:dot+1] + typeName, true
}
func shortTypeName(typeName string) string {
	if i := strings.LastIndex(typeName, "."); i >= 0 {
		return typeName[i+1:]
	}
	return typeName
}
func builtinExceptionTypeMatches(typeName, target string) bool {
	typeName = exceptionTypeName(typeName)
	target = exceptionTypeName(target)
	if typeName == "" || target == "" {
		return false
	}
	for current := typeName; current != ""; current = builtinExceptionParent(current) {
		if current == target {
			return true
		}
	}
	return false
}
func exceptionTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	typeName = strings.TrimPrefix(typeName, "System.")
	return typeName
}
func (vm *VM) coerceAssignable(typeName string, value Value) (Value, error) {
	typeName = vm.resolveAssignableTargetType(typeName)
	canonicalTypeName := typeName
	if rest, ok := stripLeadingSystemNamespace(canonicalTypeName); ok {
		canonicalTypeName = rest
	}
	canonicalValueType := value.Type
	if rest, ok := stripLeadingSystemNamespace(canonicalValueType); ok {
		canonicalValueType = rest
	}
	if canonicalValueType != "" && strings.EqualFold(canonicalValueType, canonicalTypeName) {
		text := ""
		if value.Kind == ValueString {
			text = value.Text
		} else if objectText, ok := platformScalarObjectText(value); ok {
			text = objectText
		} else if value.Kind == ValueObject && value.Text != "" {
			text = value.Text
		}
		if text != "" {
			switch {
			case strings.EqualFold(canonicalTypeName, "Date") && strings.EqualFold(strings.TrimSpace(text), "Today()"):
				return platformScalar("Date", vm.fakeNow.Format("2006-01-02")), nil
			case strings.EqualFold(canonicalTypeName, "Date"):
				parsed, err := parseDateText(text)
				if err == nil {
					return platformScalar("Date", parsed.Format("2006-01-02")), nil
				}
			case strings.EqualFold(canonicalTypeName, "Datetime") && strings.EqualFold(strings.TrimSpace(text), "Now()"):
				return platformScalar("Datetime", formatPlatformDatetime(vm.fakeNow)), nil
			}
		}
		return value, nil
	}
	if strings.EqualFold(canonicalTypeName, "Datetime") && strings.EqualFold(canonicalValueType, "Date") {
		text := ""
		if value.Kind == ValueString {
			text = value.Text
		} else if objectText, ok := platformScalarObjectText(value); ok {
			text = objectText
		} else if value.Kind == ValueObject && value.Text != "" {
			text = value.Text
		}
		if strings.EqualFold(strings.TrimSpace(text), "Today()") {
			today := time.Date(vm.fakeNow.Year(), vm.fakeNow.Month(), vm.fakeNow.Day(), 0, 0, 0, 0, time.UTC)
			return platformScalar("Datetime", formatPlatformDatetime(today)), nil
		}
	}
	if value.Type != "" && strings.EqualFold(value.Type, typeName) {
		return value, nil
	}
	if value.Kind == ValueNull {
		value.Type = typeName
		return value, nil
	}
	if value.Kind == ValueMap && (vm.isSObjectLikeType(typeName) || vm.isJSONTypedObjectTarget(typeName)) {
		return vm.typedValueFromJSON(typeName, jsonFromValue(value, false), false)
	}
	if strings.EqualFold(typeName, "String") {
		if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
			idText, ok := platformScalarObjectText(value)
			if !ok {
				return String(""), nil
			}
			if len(idText) == 15 {
				return String(apexIDTo18(idText)), nil
			}
			return String(idText), nil
		}
	}
	if strings.EqualFold(typeName, "Id") && value.Kind == ValueObject && strings.EqualFold(value.Type, "Object") {
		idText, ok := platformScalarObjectText(value)
		if ok {
			if err := validateApexIDShape(idText); err != nil {
				return Null, newExceptionError("StringException", strings.TrimPrefix(err.Error(), "System.StringException: "))
			}
			return platformScalar("Id", idText), nil
		}
	}
	if value.Kind == ValueString {
		if strings.EqualFold(typeName, "String") && strings.EqualFold(value.Type, "Id") {
			if len(value.Text) == 15 {
				return String(apexIDTo18(value.Text)), nil
			}
			return String(value.Text), nil
		}
		if class, ok := vm.resolveEnumClass(typeName); ok {
			valueText := value.Text
			if dot := strings.LastIndexByte(valueText, '.'); dot >= 0 {
				valueText = valueText[dot+1:]
			}
			for _, enumValue := range class.EnumValues {
				if enumValue == valueText {
					return Value{Kind: ValueObject, Type: class.Name, Text: enumValue}, nil
				}
			}
		}
		if apexIdentifierStartsUpper(typeName) && !isBuiltinTypeName(typeName) && !platformScalarObject(typeName) && stringEnumCoercionTarget(typeName) {
			valueText := value.Text
			if dot := strings.LastIndexByte(valueText, '.'); dot >= 0 {
				valueText = valueText[dot+1:]
			}
			return Value{Kind: ValueObject, Type: typeName, Text: valueText}, nil
		}
		switch typeName {
		case "Id":
			if err := validateApexIDShape(value.Text); err != nil {
				return Null, newExceptionError("StringException", strings.TrimPrefix(err.Error(), "System.StringException: "))
			}
			value.Type = "Id"
			return value, nil
		case "Date":
			if strings.EqualFold(strings.TrimSpace(value.Text), "Today()") {
				return platformScalar("Date", vm.fakeNow.Format("2006-01-02")), nil
			}
			parsed, err := parseDateText(value.Text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Date", parsed.Format("2006-01-02")), nil
		case "Datetime":
			if strings.EqualFold(strings.TrimSpace(value.Text), "Now()") {
				return platformScalar("Datetime", formatPlatformDatetime(vm.fakeNow)), nil
			}
			if strings.EqualFold(value.Type, "Date") {
				parsed, err := parseDateText(value.Text)
				if err != nil {
					return Null, err
				}
				return platformScalar("Datetime", formatPlatformDatetime(parsed)), nil
			}
			parsed, err := parseDatetimeTextAllowDateOnly(value.Text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Datetime", formatPlatformDatetime(parsed)), nil
		case "Time":
			parsed, err := parseTimeText(value.Text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Time", parsed), nil
		}
	}
	if value.Kind == ValueObject {
		if isStubProxy(value) && vm.typeMatches(value.Type, typeName, make(map[string]bool)) {
			if !strings.EqualFold(typeName, "Object") && !strings.EqualFold(value.Type, typeName) {
				if value.Runtime == "" {
					value.Runtime = value.Type
				}
				value.Static = typeName
			}
			return value, nil
		}
		if strings.EqualFold(typeName, "Database.QueryLocator") && value.Type == "Database.QueryLocator" {
			return value, nil
		}
		if isDescribeSObjectResultType(typeName) && isSObjectTypeToken(value) {
			return vm.describeFromSObjectTypeToken(value)
		}
		if isDescribeFieldResultType(typeName) && isSObjectFieldTokenType(value.Type) {
			return vm.describeFromSObjectFieldToken(value)
		}
		if class, ok := vm.resolveEnumClass(typeName); ok && len(class.EnumValues) > 0 {
			if strings.EqualFold(value.Type, class.Name) || vm.typeAssignableTo(value.Type, class.Name) || namespaceQualifiedTypeEquivalent(value.Type, class.Name) {
				return Value{Kind: ValueObject, Type: class.Name, Text: value.Text}, nil
			}
		}
		if strings.EqualFold(typeName, "Date") && (strings.EqualFold(value.Type, "Datetime") || strings.EqualFold(value.Type, "DateTime")) {
			parsed, err := parsePlatformDatetime(value)
			if err != nil {
				return Null, err
			}
			return platformScalar("Date", parsed.Format("2006-01-02")), nil
		}
		if strings.EqualFold(typeName, "Datetime") && strings.EqualFold(value.Type, "Date") {
			text, err := platformScalarText(value, "Date")
			if err != nil {
				return Null, err
			}
			parsed, err := parseDateText(text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Datetime", formatPlatformDatetime(parsed)), nil
		}
		if strings.EqualFold(typeName, "String") && (strings.EqualFold(value.Type, "Id") || strings.EqualFold(value.Type, "String") || strings.EqualFold(value.Type, "UUID")) {
			text, err := platformScalarText(value, value.Type)
			if err != nil {
				return Null, err
			}
			return String(text), nil
		}
		if strings.EqualFold(typeName, "Id") && strings.EqualFold(value.Type, "String") {
			text, err := platformScalarText(value, value.Type)
			if err != nil {
				return Null, err
			}
			return platformScalar("Id", text), nil
		}
		if collectionBase(typeName) == "Iterator" && isIteratorValue(value) {
			value.Type = typeName
			return value, nil
		}
		if collectionBase(typeName) == "Iterable" && value.Type == "Database.QueryLocator" {
			return vm.queryLocatorIterable(typeName, value)
		}
		if strings.EqualFold(typeName, "Object") || vm.typeAssignableTo(value.Type, typeName) || vm.typeMatches(value.Type, typeName, make(map[string]bool)) {
			if !strings.EqualFold(typeName, "Object") && !strings.EqualFold(value.Type, typeName) {
				if value.Runtime == "" {
					value.Runtime = value.Type
				}
				value.Static = typeName
			}
			return value, nil
		}
		for _, valueType := range []string{value.Runtime, value.Static} {
			if strings.TrimSpace(valueType) == "" || strings.EqualFold(valueType, value.Type) {
				continue
			}
			if vm.typeAssignableTo(valueType, typeName) || vm.typeMatches(valueType, typeName, make(map[string]bool)) {
				if !strings.EqualFold(typeName, "Object") && !strings.EqualFold(valueType, typeName) {
					if value.Runtime == "" {
						value.Runtime = valueType
					}
					value.Static = typeName
				}
				return value, nil
			}
		}
		if managedAPIVersionedTypeAssignable(value.Type, typeName) {
			if value.Runtime == "" {
				value.Runtime = value.Type
			}
			value.Static = typeName
			return value, nil
		}
		return Null, fmt.Errorf("cannot assign %s to %s", value.Type, typeName)
	}
	if value.Kind == ValueList && vm.isSObjectLikeType(typeName) {
		if len(value.List) == 0 {
			return Null, newExceptionError("QueryException", "List has no rows for assignment to SObject")
		}
		if len(value.List) > 1 {
			return Null, newExceptionError("QueryException", "List has more than 1 row for assignment to SObject")
		}
		return vm.coerceAssignable(typeName, value.List[0])
	}
	if (strings.EqualFold(typeName, "Decimal") || strings.EqualFold(typeName, "Double")) && value.Kind == ValueInt {
		decimal := Decimal(float64(value.Int))
		decimal.Text = strconv.FormatInt(value.Int, 10)
		return decimal, nil
	}
	if (strings.EqualFold(typeName, "Integer") || strings.EqualFold(typeName, "Long")) && untypedIntegralDecimalLiteral(value) {
		if value.Decimal < float64(math.MinInt64) || value.Decimal > float64(math.MaxInt64) {
			return Null, fmt.Errorf("cannot assign decimal to %s", typeName)
		}
		return Int(int64(value.Decimal)), nil
	}
	if collectionBase(typeName) == "List" && value.Kind == ValueList {
		sourceTypes := []string{value.Type, value.Runtime, value.Static}
		if value.Runtime == "" && value.Type != "" && !strings.EqualFold(value.Type, typeName) {
			value.Runtime = value.Type
		}
		value.Type = typeName
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		if strings.EqualFold(elementType, "SObject") {
			if len(value.List) == 0 {
				for _, sourceType := range sourceTypes {
					sourceElementType, ok := collectionElementType(sourceType)
					if !ok || strings.EqualFold(sourceElementType, "SObject") || strings.EqualFold(sourceElementType, "Object") {
						continue
					}
					if strings.EqualFold(sourceElementType, "AggregateResult") {
						value.Runtime = ""
						continue
					}
					if !collectionElementCarriesSObjectType(sourceElementType) {
						return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", typeExceptionAnyName(sourceType), typeExceptionAnyName(typeName)))
					}
				}
				value.Static = typeName
			}
			return value, nil
		}
		if len(value.List) == 0 && vm.isSObjectLikeType(elementType) {
			for _, sourceType := range sourceTypes {
				sourceElementType, ok := collectionElementType(sourceType)
				if !ok || strings.EqualFold(sourceElementType, "SObject") || strings.EqualFold(sourceElementType, "Object") {
					continue
				}
				if vm.typeAssignableTo(sourceElementType, elementType) {
					continue
				}
				return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", typeExceptionAnyName(sourceType), typeExceptionAnyName(typeName)))
			}
		}
		for _, sourceType := range []string{value.Runtime, value.Static} {
			if sourceElementType, ok := collectionElementType(sourceType); ok &&
				strings.EqualFold(sourceElementType, "SObject") && vm.isSObjectLikeType(elementType) {
				return value, nil
			}
		}
		for i, item := range value.List {
			if strings.EqualFold(elementType, "Id") && item.Kind == ValueString && item.Text == "" {
				continue
			}
			coerced, err := vm.coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			value.List[i] = coerced
		}
		return value, nil
	}
	if collectionBase(typeName) == "List" && value.Kind == ValueMap {
		if records, ok := queryResultRecordsList(value); ok {
			return vm.coerceAssignable(typeName, records)
		}
	}
	if strings.EqualFold(typeName, "Database.QueryLocator") && value.Kind == ValueList {
		if locator, ok := value.Fields["__queryLocator"]; ok && locator.Kind == ValueObject && locator.Type == "Database.QueryLocator" {
			return locator, nil
		}
	}
	if collectionBase(typeName) == "Set" && value.Kind == ValueSet {
		value.Type = typeName
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		out := make([]Value, 0, len(value.Set))
		for _, item := range value.Set {
			coerced, err := vm.coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			if !containsValue(out, coerced) {
				out = append(out, coerced)
			}
		}
		value.Set = out
		return value, nil
	}
	if collectionBase(typeName) == "Iterable" && (value.Kind == ValueList || value.Kind == ValueSet) {
		value.Type = typeName
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		items := collectionMembers(value)
		out := make([]Value, 0, len(items))
		for _, item := range items {
			coerced, err := vm.coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			out = append(out, coerced)
		}
		if value.Kind == ValueSet {
			value.Set = out
		} else {
			value.List = out
		}
		return value, nil
	}
	if collectionBase(typeName) == "Iterable" && value.Kind == ValueObject && value.Type == "Database.QueryLocator" {
		return vm.queryLocatorIterable(typeName, value)
	}
	if isMapType(typeName) && value.Kind == ValueMap {
		sourceType := value.Type
		keyType, valueType, ok := mapTypeArgs(typeName)
		if !ok {
			value.Type = typeName
			return value, nil
		}
		type coercedEntry struct {
			key      string
			keyValue Value
			value    Value
		}
		entries := make([]coercedEntry, 0, len(value.Map))
		for _, rawKey := range orderedValueMapKeys(value) {
			item := value.Map[rawKey]
			keyValue := mapStoredKey(value, rawKey)
			coercedKey, err := vm.coerceAssignable(keyType, keyValue)
			if err != nil {
				return Null, fmt.Errorf("key: %w", err)
			}
			coercedValue, err := vm.coerceAssignable(valueType, item)
			if err != nil {
				return Null, fmt.Errorf("value: %w", err)
			}
			entries = append(entries, coercedEntry{key: vm.mapKey(coercedKey), keyValue: coercedKey, value: coercedValue})
		}
		for rawKey := range value.Map {
			delete(value.Map, rawKey)
		}
		value.MapKeys = make(map[string]Value, len(entries))
		value.MapOrder = make([]string, 0, len(entries))
		for _, entry := range entries {
			if _, exists := value.Map[entry.key]; !exists {
				value.MapOrder = append(value.MapOrder, entry.key)
			}
			value.Map[entry.key] = entry.value
			value.MapKeys[entry.key] = entry.keyValue
		}
		if strings.EqualFold(valueType, "sObject") && mapConcreteSObjectValueType(sourceType) != "" {
			value.Type = sourceType
		} else {
			value.Type = typeName
		}
		return value, nil
	}
	return coerceAssignable(typeName, value)
}
func (vm *VM) resolveAssignableTargetType(typeName string) string {
	base := collectionBase(typeName)
	element, ok := collectionElementType(typeName)
	if base == "" || !ok || strings.Contains(element, ".") {
		return typeName
	}
	if resolved := vm.resolveTypeNameInCurrentExecutionContext(element); resolved != "" && !strings.EqualFold(resolved, element) {
		return base + "<" + resolved + ">"
	}
	if resolved, ok := vm.resolveCurrentNamespaceTopLevelClassName(element); ok {
		return base + "<" + resolved + ">"
	}
	return typeName
}
