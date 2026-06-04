package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) classNamespace(className string) string {
	if vm.classNamespaceCache == nil {
		vm.classNamespaceCache = make(map[string]string)
	}
	cacheKey := strings.TrimSpace(className)
	if cacheKey != "" {
		if namespace, ok := vm.classNamespaceCache[cacheKey]; ok {
			return namespace
		}
	}
	class, ok := vm.lookupClass(className)
	if !ok {
		if resolved, found := vm.resolveClassName(className); found {
			class, ok = vm.lookupClass(resolved)
		}
	}
	if !ok {
		for _, triggers := range vm.Triggers {
			for _, trigger := range triggers {
				if strings.EqualFold(trigger.Name, className) {
					namespace := trigger.Namespace
					if cacheKey != "" {
						vm.classNamespaceCache[cacheKey] = namespace
					}
					return namespace
				}
			}
		}
		if cacheKey != "" {
			vm.classNamespaceCache[cacheKey] = ""
		}
		return ""
	}
	if cacheKey != "" {
		vm.classNamespaceCache[cacheKey] = class.Namespace
	}
	return class.Namespace
}
func (vm *VM) currentCallerNamespace() string {
	if vm.currentMethodMatchesExecutionClass() {
		if ns := vm.classNamespace(vm.currentMethod.ClassName); ns != "" {
			return ns
		}
		if owner := classNameFromMethod(vm.currentMethod.Name); owner != "" && !strings.EqualFold(owner, vm.currentMethod.ClassName) {
			if ns := vm.classNamespace(owner); ns != "" {
				return ns
			}
		}
	}
	if count := len(vm.activeTriggerNamespaces); count > 0 {
		if ns := strings.TrimSpace(vm.activeTriggerNamespaces[count-1]); ns != "" {
			return ns
		}
	}
	if ns := vm.activeTriggerNamespace(); ns != "" {
		return ns
	}
	if ns := vm.currentTriggerNamespace(); ns != "" {
		return ns
	}
	if strings.TrimSpace(vm.currentClass) != "" && vm.classNamespace(vm.currentClass) == "" {
		if ns := strings.TrimSpace(vm.currentNamespace); ns != "" {
			return ns
		}
	}
	if ns := vm.classNamespace(vm.currentClass); ns != "" {
		return ns
	}
	if ns := strings.TrimSpace(vm.currentNamespace); ns != "" {
		return ns
	}
	return ""
}
func (vm *VM) currentMethodMatchesExecutionClass() bool {
	methodClass := strings.TrimSpace(vm.currentMethod.ClassName)
	if methodClass == "" {
		return false
	}
	currentClass := strings.TrimSpace(vm.currentClass)
	return currentClass == "" || vm.sameAccessScope(currentClass, methodClass)
}
func (vm *VM) currentTriggerNamespace() string {
	if strings.TrimSpace(vm.currentClass) == "" {
		return ""
	}
	return vm.triggerNamespaceByName(vm.currentClass)
}
func (vm *VM) activeTriggerNamespace() string {
	for i := len(vm.callStack) - 1; i >= 0; i-- {
		symbol := strings.TrimSpace(vm.callStack[i].Symbol)
		if symbol == "" {
			continue
		}
		if ns := vm.triggerNamespaceByName(symbol); ns != "" {
			return ns
		}
	}
	return ""
}
func (vm *VM) triggerNamespaceByName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	cacheKey := triggerNamespaceLookupKey{CurrentNamespace: strings.TrimSpace(vm.currentNamespace), Name: name}
	if vm.triggerNamespaceCache == nil {
		vm.triggerNamespaceCache = make(map[triggerNamespaceLookupKey]string)
	}
	if cached, ok := vm.triggerNamespaceCache[cacheKey]; ok {
		return cached
	}
	for _, triggers := range vm.Triggers {
		for _, trigger := range triggers {
			if !strings.EqualFold(trigger.Name, name) {
				continue
			}
			if ns := strings.TrimSpace(trigger.Namespace); ns != "" {
				vm.triggerNamespaceCache[cacheKey] = ns
				return ns
			}
			ns := strings.TrimSpace(vm.currentNamespace)
			vm.triggerNamespaceCache[cacheKey] = ns
			return ns
		}
	}
	vm.triggerNamespaceCache[cacheKey] = ""
	return ""
}
func (vm *VM) classForAccess(className string) (Class, bool) {
	if vm.classForAccessCache == nil {
		vm.classForAccessCache = make(map[classForAccessKey]classForAccessLookup)
	}
	cacheKey := classForAccessKey{
		ClassName:        strings.TrimSpace(className),
		CurrentClass:     strings.TrimSpace(vm.currentClass),
		CurrentNamespace: strings.TrimSpace(vm.currentNamespace),
	}
	if cached, ok := vm.classForAccessCache[cacheKey]; ok {
		return cached.Class, cached.OK
	}
	store := func(class Class, ok bool) (Class, bool) {
		vm.classForAccessCache[cacheKey] = classForAccessLookup{Class: class, OK: ok}
		return class, ok
	}
	if className != "" && !strings.Contains(className, ".") && vm.currentClass != "" {
		if currentNS := strings.TrimSpace(vm.currentNamespace); currentNS != "" {
			if class, ok := vm.lookupClassInNamespace(currentNS, className); ok {
				return store(class, true)
			}
		}
		if callerNS := vm.classNamespace(vm.currentClass); callerNS != "" {
			if class, ok := vm.lookupClassInNamespace(callerNS, className); ok {
				return store(class, true)
			}
		}
		if class, ok := vm.lookupClassInNamespace("", className); ok {
			return store(class, true)
		}
	}
	class, ok := vm.Classes[className]
	if !ok {
		if resolved, found := vm.resolveClassName(className); found {
			class, ok = vm.Classes[resolved]
		}
	}
	return store(class, ok)
}
func (vm *VM) lookupClassInNamespace(namespace, className string) (Class, bool) {
	if vm.namespaceClassLookup == nil {
		vm.namespaceClassLookup = make(map[string]map[string]namespaceClassLookup)
	}
	className = strings.TrimSpace(className)
	if className == "" {
		return Class{}, false
	}
	nsKey := strings.ToLower(strings.TrimSpace(namespace))
	shortKey := strings.ToLower(shortTypeName(className))
	if classesByShort, ok := vm.namespaceClassLookup[nsKey]; ok {
		result, found := classesByShort[shortKey]
		if !found || !result.OK {
			return Class{}, false
		}
		return result.Class, true
	}
	classesByShort := make(map[string]namespaceClassLookup)
	for _, entry := range vm.classNameSearchEntries() {
		class, ok := vm.lookupClass(entry.Name)
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(class.Namespace), nsKey) {
			continue
		}
		if !strings.Contains(class.Name, ".") {
			key := strings.ToLower(strings.TrimSpace(class.Name))
			classesByShort[key] = namespaceClassLookup{Class: class, OK: true}
			continue
		}
		key := strings.ToLower(shortTypeName(class.Name))
		if existing, exists := classesByShort[key]; exists {
			if existing.OK && strings.Contains(existing.Class.Name, ".") && !strings.EqualFold(existing.Class.Name, class.Name) {
				classesByShort[key] = namespaceClassLookup{}
			}
			continue
		}
		classesByShort[key] = namespaceClassLookup{Class: class, OK: true}
	}
	vm.namespaceClassLookup[nsKey] = classesByShort
	result, found := classesByShort[shortKey]
	if !found || !result.OK {
		return Class{}, false
	}
	return result.Class, true
}
func (vm *VM) isSubclass(child, parent string) bool {
	if resolved, ok := vm.resolveClassName(child); ok {
		child = resolved
	}
	if resolved, ok := vm.resolveClassName(parent); ok {
		parent = resolved
	}
	seen := make(map[string]bool)
	for child != "" {
		key := canonicalClassLookupKey(child)
		if seen[key] {
			return false
		}
		seen[key] = true
		class, ok := vm.lookupClass(child)
		if !ok {
			return false
		}
		superClass := vm.resolvedSuperClassName(class)
		if strings.EqualFold(superClass, parent) || vm.classNamesReferToSameRuntimeType(superClass, parent) {
			return true
		}
		child = superClass
	}
	return false
}
func (vm *VM) splitClassMember(name string) (string, string, bool) {
	parts := strings.Split(name, ".")
	for i := len(parts) - 1; i > 0; i-- {
		className := strings.Join(parts[:i], ".")
		if !strings.Contains(className, ".") && vm.currentClass != "" {
			if resolved, ok := vm.resolveNestedTypeInClassHierarchy(vm.currentClass, className); ok {
				return resolved, strings.Join(parts[i:], "."), true
			}
		}
		if resolved, ok := vm.resolveClassName(className); ok {
			return resolved, strings.Join(parts[i:], "."), true
		}
		if generated, ok := generatedPlatformTypeIndex[strings.ToLower(className)]; ok {
			return generated.Name, strings.Join(parts[i:], "."), true
		}
		if class, ok := vm.resolveEnumClass(className); ok {
			return class.Name, strings.Join(parts[i:], "."), true
		}
	}
	return "", "", false
}
func apexIdentifierStartsUpper(name string) bool {
	if name == "" {
		return false
	}
	first := name[0]
	return first >= 'A' && first <= 'Z'
}
func (vm *VM) typeForName(namespace, name string, explicitNamespace bool) Value {
	if strings.TrimSpace(name) == "" {
		return Null
	}
	if strings.HasPrefix(name, "System.") {
		systemName := strings.TrimPrefix(name, "System.")
		if isBuiltinTypeName(systemName) {
			return platformScalar("Type", "System."+systemName)
		}
	}
	if strings.TrimSpace(namespace) == "System" {
		systemName := strings.TrimPrefix(name, "System.")
		if isBuiltinTypeName(systemName) {
			return platformScalar("Type", "System."+systemName)
		}
		return Null
	}
	if namespace != "" {
		for _, candidate := range namespaceTypeNameCandidates(namespace, name) {
			if class, ok := vm.lookupClass(candidate); ok {
				return platformScalar("Type", typeForNameClassToken(namespace, class))
			}
		}
		return Null
	}
	if resolved, ok := vm.resolveClassName(name); ok {
		if class, ok := vm.lookupClass(resolved); ok {
			if !explicitNamespace && !strings.Contains(name, ".") && !typeForNameClassVisible(class) {
				return Null
			}
			return platformScalar("Type", vm.classTypeToken(class))
		}
		return platformScalar("Type", resolved)
	}
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(name); ok {
			return platformScalar("Type", canonical)
		}
	}
	if resolved, ok := vm.resolveTypeNameToken(name); ok {
		return platformScalar("Type", resolved)
	}
	if isBuiltinTypeName(name) || isGenericTypeName(name) || isCommonSObjectTypeName(name) {
		return platformScalar("Type", name)
	}
	return Null
}
func typeForNameClassVisible(class Class) bool {
	if !class.IsTest || strings.Contains(class.Name, ".") {
		return true
	}
	switch strings.ToLower(class.Access) {
	case "public", "global":
		return true
	default:
		return false
	}
}
func namespaceTypeNameCandidates(namespace, name string) []string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return nil
	}
	candidates := []string{namespace + "." + name}
	if strings.Contains(name, ".") {
		candidates = append(candidates, name)
	}
	return candidates
}
func typeForNameClassToken(namespace string, class Class) string {
	namespace = strings.TrimSpace(namespace)
	if class.Namespace == "" || namespace == "" || !strings.EqualFold(namespace, class.Namespace) {
		return class.Name
	}
	if prefix, _, ok := strings.Cut(class.Name, "."); ok && strings.EqualFold(prefix, class.Namespace) {
		return class.Name
	}
	return class.Namespace + "." + class.Name
}
func (vm *VM) classTypeToken(class Class) string {
	namespace := strings.TrimSpace(class.Namespace)
	if namespace == "" && vm.Org != nil {
		namespace = strings.TrimSpace(vm.Org.Namespace)
	}
	if namespace == "" {
		return class.Name
	}
	if prefix, _, ok := strings.Cut(class.Name, "."); ok && strings.EqualFold(prefix, namespace) {
		return class.Name
	}
	return namespace + "." + class.Name
}
func (vm *VM) resolveTypeNameToken(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	if strings.HasSuffix(name, "[]") {
		element, ok := vm.resolveTypeNameToken(strings.TrimSpace(strings.TrimSuffix(name, "[]")))
		if !ok {
			return "", false
		}
		return element + "[]", true
	}
	if args, ok := genericTypeArgs(name); ok {
		base, ok := genericBaseName(name)
		if !ok {
			return "", false
		}
		switch {
		case strings.EqualFold(base, "List"), strings.EqualFold(base, "Set"), strings.EqualFold(base, "Iterator"), strings.EqualFold(base, "Iterable"):
			if len(args) != 1 {
				return "", false
			}
			element, ok := vm.resolveTypeNameToken(args[0])
			if !ok {
				return "", false
			}
			return base + "<" + element + ">", true
		case strings.EqualFold(base, "Map"):
			if len(args) != 2 {
				return "", false
			}
			key, keyOK := vm.resolveTypeNameToken(args[0])
			value, valueOK := vm.resolveTypeNameToken(args[1])
			if !keyOK || !valueOK {
				return "", false
			}
			return base + "<" + key + "," + value + ">", true
		}
		return "", false
	}
	if resolved, ok := vm.resolveClassName(name); ok {
		return resolved, true
	}
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(name); ok {
			return canonical, true
		}
	}
	if isBuiltinTypeName(name) || isCommonSObjectTypeName(name) {
		return name, true
	}
	return "", false
}
func isBuiltinTypeName(name string) bool {
	if isBuiltinExceptionType(exceptionTypeName(name)) {
		return true
	}
	if strings.EqualFold(name, "sObject") {
		return true
	}
	switch name {
	case "Object", "String", "Boolean", "Integer", "Long", "Decimal", "Double", "Date", "Datetime", "Time", "TimeZone", "Blob", "Id", "Type", "URL", "JSONGenerator", "JSONParser", "JSONToken", "StatusCode", "ChildRelationship", "DescribeFieldResult", "DescribeSObjectResult", "DescribeTabResult", "DescribeTabSetResult", "PicklistEntry", "RecordTypeInfo", "XmlStreamReader", "XmlStreamWriter", "PageReference", "SelectOption", "LoggingLevel", "AccessType", "SObjectAccessDecision", "ApexPages.Severity", "ApexPages.StandardController", "ApexPages.StandardSetController", "RestContext", "RestRequest", "RestResponse", "Callable", "StubProvider", "InstallContext", "InstallHandler", "UninstallContext", "UninstallHandler", "Auth.JWT", "ConnectApi.UserSettings", "ConnectApi.TimeZone", "Metadata.Metadata", "Metadata.MetadataType", "Metadata.DeployContainer", "Metadata.CustomMetadata", "Metadata.CustomField", "Metadata.CustomObject", "Metadata.DeployCallback", "Metadata.DeployCallBack", "Metadata.DeployResult", "Metadata.DeployStatus", "Metadata.DeployDetails", "Metadata.DeployMessage", "Metadata.DeployCallbackContext", "Metadata.AsyncResult":
		return true
	default:
		return false
	}
}
func isGenericTypeName(name string) bool {
	open := strings.IndexByte(name, '<')
	if open <= 0 || !strings.HasSuffix(name, ">") {
		return false
	}
	base := name[:open]
	args, ok := genericTypeArgs(name)
	if !ok {
		return false
	}
	switch base {
	case "List", "Set":
		return len(args) == 1 && isTypeNameToken(args[0])
	case "Map":
		return len(args) == 2 && isTypeNameToken(args[0]) && isTypeNameToken(args[1])
	default:
		return false
	}
}
func isTypeNameToken(name string) bool {
	if strings.HasSuffix(name, "[]") {
		return isTypeNameToken(strings.TrimSpace(strings.TrimSuffix(name, "[]")))
	}
	return isBuiltinTypeName(name) || isGenericTypeName(name) || isCommonSObjectTypeName(name)
}
func isCommonSObjectTypeName(name string) bool {
	if storage.IsKnownStandardObject(name) {
		return true
	}
	for _, objectName := range standardSObjectPrefixes {
		if strings.EqualFold(name, objectName) {
			return true
		}
	}
	return false
}
func (vm *VM) resolveClassName(typeName string) (string, bool) {
	if isCommonSObjectTypeName(typeName) {
		return typeName, true
	}
	if !strings.Contains(typeName, ".") && vm.currentClass != "" {
		if resolved, ok := vm.resolveNestedTypeInClassHierarchy(vm.currentClass, typeName); ok {
			if namespace := strings.TrimSpace(vm.currentExecutionNamespace()); namespace != "" {
				if class, found := vm.lookupClassInNamespace(namespace, typeName); found && strings.EqualFold(resolved, class.Name) {
					return runtimeClassName(class), true
				}
			}
			return resolved, true
		}
		if namespace := strings.TrimSpace(vm.currentExecutionNamespace()); namespace != "" {
			if class, ok := vm.lookupClassInNamespace(namespace, typeName); ok {
				return runtimeClassName(class), true
			}
		}
	}
	if strings.Contains(typeName, ".") && vm.currentClass != "" {
		if namespace := strings.TrimSpace(vm.currentExecutionNamespace()); namespace != "" {
			if class, ok := vm.lookupClass(namespace + "." + typeName); ok {
				return runtimeClassName(class), true
			}
		}
	}
	if class, ok := vm.lookupClass(typeName); ok {
		if strings.Contains(typeName, ".") && class.Namespace != "" {
			return runtimeClassName(class), true
		}
		return class.Name, true
	}
	return "", false
}
func (vm *VM) lookupClass(typeName string) (Class, bool) {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return Class{}, false
	}
	if cached, ok := vm.classLookupNameCache[typeName]; ok {
		if !cached.OK {
			return Class{}, false
		}
		class, ok := vm.Classes[cached.Alias]
		if ok {
			return class, true
		}
		delete(vm.classLookupNameCache, typeName)
	}
	if class, ok := vm.Classes[typeName]; ok {
		vm.storeClassLookupNameCache(typeName, typeName, true)
		return class, true
	}
	if vm.sharedClassLookupKeys != nil {
		if key, ok := foldLookupStringMap(vm.sharedClassLookupKeys, typeName); ok {
			if class, ok := vm.Classes[key]; ok {
				vm.storeClassLookupNameCache(typeName, key, true)
				return class, true
			}
		}
		vm.storeClassLookupNameCache(typeName, "", false)
		return Class{}, false
	}
	if vm.classLookup == nil {
		vm.rebuildClassLookup()
	}
	if class, ok := foldLookupClassMap(vm.classLookup, typeName); ok {
		return class, true
	}
	return Class{}, false
}

func (vm *VM) storeClassLookupNameCache(typeName, alias string, ok bool) {
	if vm.classLookupNameCache == nil {
		vm.classLookupNameCache = make(map[string]classLookupNameResult)
	}
	vm.classLookupNameCache[typeName] = classLookupNameResult{Alias: alias, OK: ok}
}

// FreezeClassLookup builds the shared, immutable canonical-key -> live Classes
// key index so subsequent CloneRuntime calls can share it by pointer instead of
// rebuilding a per-clone classLookup. Call it once on a base machine after all
// class/method registration is complete and before cloning per-test runtimes.
// Any later registration on a clone transparently falls back via
// unshareClassLookup.
func (vm *VM) FreezeClassLookup() {
	if vm == nil {
		return
	}
	keys := make(map[string]string, len(vm.Classes)*2)
	ranks := make(map[string]int, len(vm.Classes)*2)
	nss := make(map[string]string, len(vm.Classes)*2)
	put := func(key, alias string, class Class, exact bool) {
		rank := classLookupKeyRank(class, exact)
		if cur, ok := ranks[key]; ok && !classLookupKeyWins(rank, class.Namespace, cur, nss[key]) {
			return
		}
		keys[key] = alias
		ranks[key] = rank
		nss[key] = class.Namespace
	}
	for alias, class := range vm.Classes {
		put(canonicalClassLookupKey(alias), alias, class, true)
		put(canonicalClassLookupKey(class.Name), alias, class, false)
		if class.Namespace != "" {
			put(canonicalClassLookupKey(class.Namespace+"."+class.Name), alias, class, true)
		}
	}
	vm.sharedClassLookupKeys = keys
	vm.sharedClassCopyPlan = buildClassCopyPlan(vm.Classes)
	vm.classNameSearchEntries()
	vm.rebuildTopLevelClassLookup()
	vm.classLookupNameCache = nil
	vm.classLookup = nil
}

// classLookupKeyRank scores a write into a canonical class-lookup index so that
// short-name collisions between a local (project) class and a managed-dependency
// class resolve to the local class, matching Salesforce, where an unqualified
// name in local code never binds to a packaged class. Local provenance dominates;
// an exact alias match outranks a bare class-name fallback within the same
// provenance. Higher wins.
func classLookupKeyRank(class Class, exact bool) int {
	rank := 0
	if !class.Dependency {
		rank += 2
	}
	if exact {
		rank++
	}
	return rank
}

// classLookupKeyWins reports whether a candidate write should replace the current
// owner of a canonical key. Equal ranks break deterministically on namespace so
// the frozen and rebuilt indexes are stable regardless of map iteration order.
func classLookupKeyWins(candidateRank int, candidateNS string, currentRank int, currentNS string) bool {
	if candidateRank != currentRank {
		return candidateRank > currentRank
	}
	return strings.ToLower(strings.TrimSpace(candidateNS)) < strings.ToLower(strings.TrimSpace(currentNS))
}

// unshareClassLookup reverts a VM from the shared frozen index back to a private
// legacy classLookup. Registration mutators call this so the shared base index
// is never modified; per-test clones never register, so they keep sharing.
func (vm *VM) unshareClassLookup() {
	if vm == nil || vm.sharedClassLookupKeys == nil {
		return
	}
	vm.sharedClassLookupKeys = nil
	vm.sharedClassCopyPlan = nil
	vm.rebuildClassLookup()
}
func (vm *VM) storeClassAliases(class Class) {
	vm.unshareClassLookup()
	if vm.Classes == nil {
		vm.Classes = make(map[string]Class)
	}
	if vm.classLookup == nil {
		vm.classLookup = make(map[string]Class)
	}
	if existing, exists := vm.Classes[class.Name]; !exists || shouldReplaceShortClassAlias(existing, class) {
		vm.Classes[class.Name] = class
	}
	vm.resetClassAccessCaches()
	vm.enumLookup = nil
	vm.enumSuffixLookup = nil
	vm.storeClassLookupAlias(class.Name, class)
	if class.Namespace != "" {
		qualified := runtimeClassName(class)
		vm.Classes[qualified] = class
		vm.storeClassLookupAlias(qualified, class)
	}
}
func shouldReplaceShortClassAlias(existing, incoming Class) bool {
	if strings.EqualFold(existing.Namespace, incoming.Namespace) {
		return true
	}
	// Keep local/project class on short-name collisions; dependency classes
	// remain available through explicit namespace-qualified aliases.
	if !existing.Dependency && incoming.Dependency {
		return false
	}
	if existing.Dependency && !incoming.Dependency {
		return true
	}
	// Stable tie-breaker for same provenance kind.
	return strings.Compare(strings.ToLower(strings.TrimSpace(incoming.Namespace)), strings.ToLower(strings.TrimSpace(existing.Namespace))) < 0
}
func (vm *VM) storeClassLookupAlias(name string, class Class) {
	if strings.TrimSpace(name) == "" {
		return
	}
	vm.classLookup[canonicalClassLookupKey(name)] = class
}
func (vm *VM) storeClassValue(class Class) {
	// Frozen fast path: static-field writeback updates the value of an
	// already-registered class. When the shared lookup index is frozen,
	// lookupClass resolves through vm.Classes by live key, so updating the
	// clone's existing alias entries in place is immediately visible without
	// rebuilding the entire (~2x len(Classes)) lookup index or thrashing the
	// access caches. Class structure (name/namespace/access) is unchanged, so
	// those caches remain valid. Only fall back to the rebuild path when a name
	// would be newly introduced.
	if vm.sharedClassLookupKeys != nil && vm.updateExistingClassValue(class) {
		return
	}
	vm.unshareClassLookup()
	if vm.Classes == nil {
		vm.Classes = make(map[string]Class)
	}
	if existing, exists := vm.Classes[class.Name]; !exists || shouldReplaceShortClassAlias(existing, class) {
		vm.Classes[class.Name] = class
	}
	vm.Classes[runtimeClassName(class)] = class
	if class.Namespace != "" && !strings.Contains(class.Name, ".") {
		vm.Classes[class.Namespace+"."+class.Name] = class
	}
}

// updateExistingClassValue updates a class value in place on the frozen lookup
// fast path. It returns false (so the caller unshares and rebuilds) if any
// target alias is not already present in the clone's Classes map, since adding
// a new entry would require updating the shared frozen lookup index.
func (vm *VM) updateExistingClassValue(class Class) bool {
	if vm.Classes == nil {
		return false
	}
	existing, exists := vm.Classes[class.Name]
	if !exists {
		return false
	}
	runtimeName := runtimeClassName(class)
	if _, ok := vm.Classes[runtimeName]; !ok {
		return false
	}
	hasNamespaceAlias := class.Namespace != "" && !strings.Contains(class.Name, ".")
	namespaceAlias := ""
	if hasNamespaceAlias {
		namespaceAlias = class.Namespace + "." + class.Name
		if _, ok := vm.Classes[namespaceAlias]; !ok {
			return false
		}
	}
	if shouldReplaceShortClassAlias(existing, class) {
		vm.Classes[class.Name] = class
	}
	vm.Classes[runtimeName] = class
	if hasNamespaceAlias {
		vm.Classes[namespaceAlias] = class
	}
	return true
}
func runtimeClassName(class Class) string {
	name := strings.TrimSpace(class.Name)
	namespace := strings.TrimSpace(class.Namespace)
	if name == "" || namespace == "" {
		return name
	}
	if hasTypePrefixFold(name, namespace) {
		return name
	}
	return namespace + "." + name
}

func (vm *VM) topLevelClassLookupIndex() map[string]topLevelClassLookup {
	if vm.topLevelClassLookup != nil {
		return vm.topLevelClassLookup
	}
	return vm.rebuildTopLevelClassLookup()
}

func (vm *VM) rebuildTopLevelClassLookup() map[string]topLevelClassLookup {
	index := make(map[string]topLevelClassLookup)
	for _, class := range vm.Classes {
		name := strings.TrimSpace(class.Name)
		if name == "" || strings.Contains(name, ".") {
			continue
		}
		nameKey := canonicalClassLookupKey(name)
		namespaceKey := canonicalClassLookupKey(class.Namespace)
		candidate := runtimeClassName(class)
		entry := index[nameKey]
		if entry.ByNamespace == nil {
			entry.ByNamespace = make(map[string]string)
		}
		if existing := entry.ByNamespace[namespaceKey]; existing == "" || strings.EqualFold(existing, candidate) {
			entry.ByNamespace[namespaceKey] = candidate
		}
		if entry.Unique == "" {
			entry.Unique = candidate
		} else if !strings.EqualFold(entry.Unique, candidate) {
			entry.Ambiguous = true
		}
		index[nameKey] = entry
	}
	vm.topLevelClassLookup = index
	return index
}

func (vm *VM) rebuildClassLookup() {
	vm.sharedClassLookupKeys = nil
	vm.sharedClassCopyPlan = nil
	vm.classLookupNameCache = nil
	vm.resetClassAccessCaches()
	vm.classLookup = make(map[string]Class, len(vm.Classes)*2)
	ranks := make(map[string]int, len(vm.Classes)*2)
	nss := make(map[string]string, len(vm.Classes)*2)
	put := func(name string, class Class, exact bool) {
		if strings.TrimSpace(name) == "" {
			return
		}
		key := canonicalClassLookupKey(name)
		rank := classLookupKeyRank(class, exact)
		if cur, ok := ranks[key]; ok && !classLookupKeyWins(rank, class.Namespace, cur, nss[key]) {
			return
		}
		vm.classLookup[key] = class
		ranks[key] = rank
		nss[key] = class.Namespace
	}
	for alias, class := range vm.Classes {
		put(alias, class, true)
		put(class.Name, class, false)
		if class.Namespace != "" {
			put(class.Namespace+"."+class.Name, class, true)
		}
	}
}
func (vm *VM) resetClassAccessCaches() {
	vm.namespaceClassLookup = make(map[string]map[string]namespaceClassLookup)
	vm.classNamespaceCache = make(map[string]string)
	vm.classForAccessCache = make(map[classForAccessKey]classForAccessLookup)
	vm.classLookupNameCache = nil
	vm.nestedTypeHierarchyCache = nil
	vm.topLevelTypeCache = nil
	vm.topLevelClassLookup = nil
	vm.classNameSearchCache = nil
}
func canonicalClassLookupKey(name string) string {
	// Apex identifiers are ASCII. Avoid the strings.ToLower allocation
	// when no folding is required (most lookups in steady state).
	trimmed := strings.TrimSpace(name)
	needsFold := false
	for i := 0; i < len(trimmed); i++ {
		if c := trimmed[i]; c >= 'A' && c <= 'Z' {
			needsFold = true
			break
		}
	}
	if !needsFold {
		return trimmed
	}
	buf := make([]byte, len(trimmed))
	for i := 0; i < len(trimmed); i++ {
		c := trimmed[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf[i] = c
	}
	return string(buf)
}

// foldLookupStringMap probes m with the case-folded form of name without
// allocating. It relies on the Go compiler optimization that map indexing with
// string([]byte) does not allocate when the conversion is inline at the index
// expression. ASCII fold matches canonicalClassLookupKey for Apex identifiers.
func foldLookupStringMap(m map[string]string, name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	needsFold := false
	for i := 0; i < len(trimmed); i++ {
		if c := trimmed[i]; c >= 'A' && c <= 'Z' {
			needsFold = true
			break
		}
	}
	if !needsFold {
		v, ok := m[trimmed]
		return v, ok
	}
	if len(trimmed) <= foldClassKeyBuf {
		var buf [foldClassKeyBuf]byte
		for i := 0; i < len(trimmed); i++ {
			c := trimmed[i]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			buf[i] = c
		}
		v, ok := m[string(buf[:len(trimmed)])]
		return v, ok
	}
	v, ok := m[canonicalClassLookupKey(name)]
	return v, ok
}

// foldLookupClassMap is foldLookupStringMap for the Class-valued classLookup.
func foldLookupClassMap(m map[string]Class, name string) (Class, bool) {
	trimmed := strings.TrimSpace(name)
	needsFold := false
	for i := 0; i < len(trimmed); i++ {
		if c := trimmed[i]; c >= 'A' && c <= 'Z' {
			needsFold = true
			break
		}
	}
	if !needsFold {
		v, ok := m[trimmed]
		return v, ok
	}
	if len(trimmed) <= foldClassKeyBuf {
		var buf [foldClassKeyBuf]byte
		for i := 0; i < len(trimmed); i++ {
			c := trimmed[i]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			buf[i] = c
		}
		v, ok := m[string(buf[:len(trimmed)])]
		return v, ok
	}
	v, ok := m[canonicalClassLookupKey(name)]
	return v, ok
}
