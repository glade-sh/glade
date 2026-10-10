package vm

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/storage"
)

type runtimeClassNameCacheKey struct {
	Namespace string
	Name      string
}

type frozenClassLookup struct {
	generation uint64
	// keys maps canonical class names to live Classes keys. On a root
	// generation (base == nil) it is complete. On a layer it holds only the
	// keys whose result differs from base, where "" hides the base entry.
	keys map[string]string
	// base is the root generation a layer was registered on. written lists
	// the Classes keys a layer's runtime wrote since base, cumulatively.
	base    *frozenClassLookup
	written map[string]struct{}

	// Derived artifacts of this generation, shared like keys. A nil
	// searchEntries or copyPlan disables layering on this generation.
	searchEntries []classNameSearchEntry
	topLevel      map[string]topLevelClassLookup
	copyPlan      *classCopyPlan

	// contributors maps each root key to every Classes key that writes it
	// in a full build. It is built once, by the first registering clone.
	contributorsOnce  sync.Once
	contributorsReady atomic.Bool
	contributors      map[string][]string

	// namespaceAliases is built lazily, once per namespace, by the first clone
	// that queries it and then shared by every clone of this generation.
	namespaceMu      sync.RWMutex
	namespaceAliases map[string]map[string]namespaceClassAlias
}

type namespaceClassAlias struct {
	Alias string
	Name  string
	OK    bool
}

var (
	runtimeClassNameCacheMu sync.RWMutex
	runtimeClassNameCache   = make(map[runtimeClassNameCacheKey]string)
)

const (
	// Managed-package runtimes exercise tens of thousands of distinct raw
	// type-name spellings per worker. Keep the overlay above that working set
	// while retaining explicit memory bounds for pathological inputs.
	maxClassLookupNameCacheEntries = 64 << 10
	maxClassLookupNameCacheBytes   = 8 << 20
)

var nextClassLookupGeneration atomic.Uint64

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
	if vm.currentTrigger && len(vm.activeTriggerNamespaces) > 0 {
		return strings.TrimSpace(vm.activeTriggerNamespaces[len(vm.activeTriggerNamespaces)-1])
	}
	if vm.currentMethod.SourceContextBound {
		return strings.TrimSpace(vm.currentMethod.Namespace)
	}
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
	if classesByShort, ok := vm.frozenNamespaceClassLookup(nsKey); ok {
		vm.namespaceClassLookup[nsKey] = classesByShort
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

// frozenNamespaceClassLookup returns this VM's per-namespace short-name table
// using the alias table shared by every clone of one frozen generation. The
// shared table holds live Classes keys only; Class values are read from this
// VM's own Classes map when the namespace is first queried, exactly when the
// unshared path below would snapshot them. Registration drops the frozen
// generation, so a changed class set never reads a stale table.
func (vm *VM) frozenNamespaceClassLookup(nsKey string) (map[string]namespaceClassLookup, bool) {
	frozen := vm.frozenClassLookup
	if frozen == nil {
		return nil, false
	}
	frozen.namespaceMu.RLock()
	aliases, ok := frozen.namespaceAliases[nsKey]
	frozen.namespaceMu.RUnlock()
	if !ok {
		aliases = vm.buildFrozenNamespaceClassAliases(frozen, nsKey)
		frozen.namespaceMu.Lock()
		if frozen.namespaceAliases == nil {
			frozen.namespaceAliases = make(map[string]map[string]namespaceClassAlias)
		}
		if existing, exists := frozen.namespaceAliases[nsKey]; exists {
			aliases = existing
		} else {
			frozen.namespaceAliases[nsKey] = aliases
		}
		frozen.namespaceMu.Unlock()
	}
	classesByShort := make(map[string]namespaceClassLookup, len(aliases))
	for key, entry := range aliases {
		if !entry.OK {
			classesByShort[key] = namespaceClassLookup{}
			continue
		}
		class, exists := vm.Classes[entry.Alias]
		if !exists {
			return nil, false
		}
		classesByShort[key] = namespaceClassLookup{Class: class, OK: true}
	}
	return classesByShort, true
}

// buildFrozenNamespaceClassAliases mirrors the unshared table build in
// lookupClassInNamespace, recording the frozen alias each lookupClass call
// would resolve instead of the Class value.
func (vm *VM) buildFrozenNamespaceClassAliases(frozen *frozenClassLookup, nsKey string) map[string]namespaceClassAlias {
	aliases := make(map[string]namespaceClassAlias)
	for _, entry := range vm.classNameSearchEntries() {
		typeName := strings.TrimSpace(entry.Name)
		if typeName == "" {
			continue
		}
		alias, ok := frozen.resolve(typeName)
		if !ok {
			continue
		}
		class, ok := vm.Classes[alias]
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(class.Namespace), nsKey) {
			continue
		}
		if !strings.Contains(class.Name, ".") {
			key := strings.ToLower(strings.TrimSpace(class.Name))
			aliases[key] = namespaceClassAlias{Alias: alias, Name: class.Name, OK: true}
			continue
		}
		key := strings.ToLower(shortTypeName(class.Name))
		if existing, exists := aliases[key]; exists {
			if existing.OK && strings.Contains(existing.Name, ".") && !strings.EqualFold(existing.Name, class.Name) {
				aliases[key] = namespaceClassAlias{}
			}
			continue
		}
		aliases[key] = namespaceClassAlias{Alias: alias, Name: class.Name, OK: true}
	}
	return aliases
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
		if generated, ok := generatedPlatformTypes()[strings.ToLower(className)]; ok {
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

// typeTokenName gives each resolved record a stable qualified identity, including
// records nested in generic Type tokens. Class declarations retain their identity.
func (vm *VM) typeTokenName(name string) string {
	// SObject is the record base type, not a concrete schema declaration.
	if strings.EqualFold(canonicalRuntimePlatformType(name), "SObject") {
		return "SObject"
	}
	if record, ok := vm.explicitSchemaRecordType(name); ok {
		if strings.EqualFold(record, "SObject") {
			return "SObject"
		}
		return "Schema." + record
	}
	if strings.HasSuffix(name, "[]") {
		return vm.typeTokenName(strings.TrimSuffix(name, "[]")) + "[]"
	}
	if args, ok := genericTypeArgs(name); ok {
		base, _ := genericBaseName(name)
		for i := range args {
			args[i] = vm.typeTokenName(args[i])
		}
		return base + "<" + strings.Join(args, ",") + ">"
	}
	if _, class := vm.lookupClass(name); class {
		return name
	}
	if record, ok := vm.resolveObjectName(name); ok {
		return "Schema." + record
	}
	if vm.isSObjectLikeType(name) {
		return "Schema." + name
	}
	return name
}

// Record identity is private metadata so the existing Type display and scalar
// payload stay unchanged. A later class registration cannot redirect the token.
const reflectionTypeIdentityField = "__gladeTypeIdentity"

func (vm *VM) markReflectionTypeToken(value Value, name string) Value {
	identity := vm.typeTokenName(name)
	if identity != typeValueText(value) {
		if value.Fields == nil {
			value.Fields = make(map[string]Value)
		}
		value.Fields[reflectionTypeIdentityField] = String(identity)
	}
	return value
}

func (vm *VM) reflectionTypeToken(name string) Value {
	display := name
	if record, ok := vm.explicitSchemaRecordType(name); ok {
		display = record
	}
	return vm.markReflectionTypeToken(platformScalar("Type", display), name)
}

func (vm *VM) typeForName(namespace, name string, explicitNamespace bool) Value {
	// R195: forName does not trim a type-name argument.
	if name == "" || name != strings.TrimSpace(name) {
		return Null
	}
	// R185/R187: qualified schema records and System interfaces are visible
	// through the single-name overload; System scalar aliases are not.
	if namespace == "" && hasPrefixFold(name, "Schema.") {
		if record, ok := vm.explicitSchemaRecordType(name); ok {
			return vm.reflectionTypeToken("Schema." + record)
		}
	}
	if namespace == "" && strings.EqualFold(name, "System.Callable") {
		return vm.reflectionTypeToken("System.Callable")
	}
	// The single-name overload prefers a schema record even when a project
	// class has the same name. Empty/null explicit namespaces keep class lookup.
	if namespace == "" && !explicitNamespace {
		if record, ok := vm.resolveObjectName(name); ok {
			return vm.reflectionTypeToken("Schema." + record)
		}
		if record, ok := storage.ResolveKnownStandardObjectName(name); ok {
			return vm.reflectionTypeToken("Schema." + record)
		}
	}
	if namespace != "" {
		if resolved, ok := generatedPlatformTypeForName(namespace, name); ok {
			return vm.reflectionTypeToken(resolved)
		}
		// Salesforce exposes concrete System exception types through the
		// two-argument overload, but the abstract Exception base type is not
		// returned from Type.forName('System', 'Exception').
		if strings.EqualFold(namespace, "System") &&
			!strings.EqualFold(name, "Exception") && isBuiltinExceptionType(name) {
			return vm.reflectionTypeToken("System." + exceptionTypeName(name))
		}
		for _, candidate := range namespaceTypeNameCandidates(namespace, name) {
			if class, ok := vm.lookupClass(candidate); ok && !methodHasModifier(class.Modifiers, AnonymousClassModifier) {
				return vm.reflectionTypeToken(typeForNameClassToken(namespace, class))
			}
		}
		if objectName, ok := vm.localNamespaceSObjectTypeForName(namespace, name); ok {
			return vm.reflectionTypeToken(objectName)
		}
		return Null
	}
	if resolved, ok := vm.resolveClassName(name); ok {
		if class, ok := vm.lookupClass(resolved); ok && !methodHasModifier(class.Modifiers, AnonymousClassModifier) {
			if !explicitNamespace && !strings.Contains(name, ".") && !typeForNameClassVisible(class) {
				return Null
			}
			return vm.reflectionTypeToken(vm.classTypeToken(class))
		}
		if _, class := vm.lookupClass(resolved); !class {
			return vm.reflectionTypeToken(resolved)
		}
	}
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(name); ok {
			return vm.reflectionTypeToken("Schema." + canonical)
		}
	}
	if hasPrefixFold(name, "System.") {
		return Null
	}
	if resolved, ok := vm.resolveTypeNameToken(name); ok {
		if class, exists := vm.lookupClass(resolved); exists && methodHasModifier(class.Modifiers, AnonymousClassModifier) {
			return Null
		}
		return vm.reflectionTypeToken(resolved)
	}
	if isBuiltinTypeName(name) || isGenericTypeName(name) || isCommonSObjectTypeName(name) {
		return vm.reflectionTypeToken(name)
	}
	return Null
}
func (vm *VM) localNamespaceSObjectTypeForName(namespace, name string) (string, bool) {
	if vm == nil || vm.Org == nil {
		return "", false
	}
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return "", false
	}
	candidates := []string{name}
	if strings.Contains(name, ".") {
		_, rest, _ := strings.Cut(name, ".")
		if rest != "" {
			candidates = append(candidates, rest)
		}
	}
	for _, candidate := range candidates {
		prefixed := storage.NamespaceTokenName(namespace, candidate)
		for _, objectCandidate := range []string{candidate, prefixed} {
			if objectName, ok := vm.resolveObjectName(objectCandidate); ok && sObjectBelongsToNamespace(objectName, namespace) {
				return objectName, true
			}
		}
	}
	return "", false
}
func sObjectBelongsToNamespace(objectName, namespace string) bool {
	prefix := strings.TrimSpace(namespace) + "__"
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(objectName)), strings.ToLower(prefix))
}
func generatedPlatformTypeForName(namespace, name string) (string, bool) {
	name = strings.TrimSpace(name)
	namespace = strings.TrimSpace(namespace)
	if name == "" {
		return "", false
	}
	candidates := []string{name}
	if namespace != "" {
		qualifiedName := name
		if prefix, rest, ok := strings.Cut(name, "."); !ok || !strings.EqualFold(prefix, namespace) {
			qualifiedName = namespace + "." + name
		} else {
			qualifiedName = namespace + "." + rest
		}
		candidates = []string{qualifiedName}
	}
	for _, candidate := range candidates {
		if generated, ok := generatedPlatformTypes()[strings.ToLower(candidate)]; ok {
			return generated.Name, true
		}
		if alias, ok := platformShortTypeAlias(candidate); ok {
			return alias, true
		}
	}
	return "", false
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
	if resolved, ok := generatedPlatformTypeForName("", name); ok {
		return resolved, true
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
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if commonSObjectTypeNameLookup()[strings.ToLower(name)] {
		return true
	}
	return storage.IsKnownStandardObject(name)
}

// explicitSchemaRecordType resolves record qualification without consulting
// user-class aliases. Keep Schema in type tokens until record allocation, where
// the canonical object name and classInstance=false carry the same provenance.
func (vm *VM) explicitSchemaRecordType(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	if !hasPrefixFold(typeName, "Schema.") {
		return "", false
	}
	record := typeName[len("Schema."):]
	if canonical, ok := vm.resolveObjectName(record); ok {
		return canonical, true
	}
	// Suffix-shaped names alone do not establish that a record exists.
	if isCommonSObjectTypeName(record) {
		return record, true
	}
	return "", false
}

func (vm *VM) resolveClassName(typeName string) (string, bool) {
	if record, ok := vm.explicitSchemaRecordType(typeName); ok {
		return "Schema." + record, true
	}
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
		if cached.Generation == vm.classLookupGeneration {
			vm.recordClassLookupHit()
			if !cached.OK {
				return Class{}, false
			}
			class, ok := vm.Classes[cached.Alias]
			if ok {
				return class, true
			}
		}
		vm.deleteClassLookupNameCache(typeName)
	}
	vm.recordClassLookupMiss()
	if frozen := vm.frozenClassLookup; frozen != nil {
		if alias, ok := frozen.resolve(typeName); ok {
			if class, ok := vm.Classes[alias]; ok {
				vm.storeClassLookupNameCache(typeName, alias, true)
				return class, true
			}
		}
		vm.storeClassLookupNameCache(typeName, "", false)
		return Class{}, false
	}
	if class, ok := vm.Classes[typeName]; ok {
		vm.storeClassLookupNameCache(typeName, typeName, true)
		return class, true
	}
	if overlay := vm.classOverlay; overlay != nil {
		if class, ok := overlay.lookup(vm.Classes, typeName); ok {
			if alias := vm.liveClassAlias(class); alias != "" {
				vm.storeClassLookupNameCache(typeName, alias, true)
			}
			return class, true
		}
		vm.storeClassLookupNameCache(typeName, "", false)
		return Class{}, false
	}
	if vm.classLookup == nil {
		vm.rebuildClassLookup()
	}
	if class, ok := foldLookupClassMap(vm.classLookup, typeName); ok {
		if alias := vm.liveClassAlias(class); alias != "" {
			vm.storeClassLookupNameCache(typeName, alias, true)
		}
		return class, true
	}
	vm.storeClassLookupNameCache(typeName, "", false)
	return Class{}, false
}

func (vm *VM) liveClassAlias(class Class) string {
	for _, alias := range []string{runtimeClassName(class), class.Name} {
		candidate, ok := vm.Classes[alias]
		if !ok {
			continue
		}
		if strings.EqualFold(candidate.Name, class.Name) &&
			strings.EqualFold(candidate.Namespace, class.Namespace) &&
			candidate.Dependency == class.Dependency {
			return alias
		}
	}
	return ""
}

func (vm *VM) storeClassLookupNameCache(typeName, alias string, ok bool) {
	// A fresh, unfrozen VM has no reusable class generation. Retaining lookup
	// results there only adds per-request allocations. Registration or a
	// post-freeze structural change assigns a private generation; frozen
	// runtimes resolve through their shared immutable lookup above.
	if vm.classLookupGeneration == 0 {
		return
	}
	retainedBytes := len(typeName) + len(alias)
	if retainedBytes > maxClassLookupNameCacheBytes {
		return
	}
	if vm.classLookupNameCache == nil {
		vm.classLookupNameCache = make(map[string]classLookupNameResult)
	}
	if existing, exists := vm.classLookupNameCache[typeName]; exists {
		vm.classLookupNameBytes -= len(typeName) + len(existing.Alias)
	} else {
		for len(vm.classLookupNameCache) >= maxClassLookupNameCacheEntries ||
			vm.classLookupNameBytes+retainedBytes > maxClassLookupNameCacheBytes {
			if len(vm.classLookupNameOrder) == 0 {
				break
			}
			oldest := vm.classLookupNameOrder[0]
			vm.classLookupNameOrder[0] = ""
			vm.classLookupNameOrder = vm.classLookupNameOrder[1:]
			if _, exists := vm.classLookupNameCache[oldest]; exists {
				vm.deleteClassLookupNameCache(oldest)
				vm.recordClassLookupEviction()
			}
		}
		vm.classLookupNameOrder = append(vm.classLookupNameOrder, typeName)
	}
	vm.classLookupNameCache[typeName] = classLookupNameResult{
		Alias:      alias,
		Generation: vm.classLookupGeneration,
		OK:         ok,
	}
	vm.classLookupNameBytes += retainedBytes
	vm.updateClassLookupNameCacheGauges()
}

func (vm *VM) deleteClassLookupNameCache(typeName string) {
	existing, ok := vm.classLookupNameCache[typeName]
	if !ok {
		return
	}
	delete(vm.classLookupNameCache, typeName)
	vm.classLookupNameBytes -= len(typeName) + len(existing.Alias)
	if vm.classLookupNameBytes < 0 {
		vm.classLookupNameBytes = 0
	}
	vm.updateClassLookupNameCacheGauges()
}

func (vm *VM) resetClassLookupNameCache() {
	vm.classLookupNameCache = nil
	vm.classLookupNameOrder = nil
	vm.classLookupNameBytes = 0
	vm.classLookupNameStats = classLookupNameCacheStats{}
}

func (vm *VM) recordClassLookupHit() {
	vm.classLookupNameStats.Hits++
	if vm.classLookupPerf != nil {
		vm.classLookupPerf.hits.Add(1)
	}
}

func (vm *VM) recordClassLookupMiss() {
	vm.classLookupNameStats.Misses++
	if vm.classLookupPerf != nil {
		vm.classLookupPerf.misses.Add(1)
	}
}

func (vm *VM) recordClassLookupEviction() {
	vm.classLookupNameStats.Evictions++
	if vm.classLookupPerf != nil {
		vm.classLookupPerf.evictions.Add(1)
	}
}

func (vm *VM) updateClassLookupNameCacheGauges() {
	vm.classLookupNameStats.Entries = len(vm.classLookupNameCache)
	vm.classLookupNameStats.RetainedBytes = vm.classLookupNameBytes
	vm.classLookupPerf.recordGauge(len(vm.classLookupNameCache), vm.classLookupNameBytes)
}

// FreezeClassLookup binds the immutable canonical-key -> live Classes key
// results to one runtime generation. Subsequent CloneRuntime calls share that
// exact artifact by pointer instead of rebuilding a per-clone classLookup.
// Later registration invalidates the frozen artifact and starts a private
// generation with a bounded result overlay.
//
// After registrations on a frozen clone, freezing publishes a layer over the
// root generation in O(registered names); see class_lookup_overlay.go.
func (vm *VM) FreezeClassLookup() {
	if vm == nil {
		return
	}
	if overlay := vm.classOverlay; overlay != nil {
		vm.classOverlay = nil
		if vm.freezeClassLookupOverlay(overlay) {
			return
		}
	}
	vm.classLookupBuilds++
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
	generation := nextClassLookupGeneration.Add(1)
	frozen := &frozenClassLookup{generation: generation, keys: keys}
	vm.frozenClassLookup = frozen
	vm.sharedClassCopyPlan = buildClassCopyPlan(vm.Classes)
	vm.classMapWritten = false
	vm.classValuesWritten = false
	vm.classLookupGeneration = generation
	// A search cache missing names (one that predates an unregistered alias
	// write) is kept as before but never extended by a layer.
	if entries := vm.classNameSearchEntries(); len(entries) == len(vm.Classes) {
		frozen.searchEntries = entries
	}
	frozen.topLevel = vm.rebuildTopLevelClassLookup()
	frozen.copyPlan = vm.sharedClassCopyPlan
	vm.resetClassLookupNameCache()
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

// unshareClassLookup invalidates the frozen generation for this VM and rebuilds
// the structural index privately. Registration mutators call this so the shared
// base generation is never modified; ordinary per-test clones keep sharing it.
func (vm *VM) unshareClassLookup() {
	if vm == nil || vm.frozenClassLookup == nil {
		return
	}
	vm.frozenClassLookup = nil
	vm.sharedClassCopyPlan = nil
	vm.rebuildClassLookup()
}
func (vm *VM) storeClassAliases(class Class) {
	vm.prepareClassMapWrite()
	overlay := vm.beginClassLookupOverlay()
	if overlay == nil {
		vm.unshareClassLookup()
	}
	if vm.Classes == nil {
		vm.Classes = make(map[string]Class)
	}
	if overlay == nil && vm.classLookup == nil {
		vm.classLookup = make(map[string]Class)
	}
	if existing, exists := vm.Classes[class.Name]; !exists || shouldReplaceShortClassAlias(existing, class) {
		vm.writeRegisteredClass(overlay, class.Name, class)
	}
	vm.classLookupGeneration = nextClassLookupGeneration.Add(1)
	vm.resetClassAccessCaches()
	vm.enumLookup = nil
	vm.enumSuffixLookup = nil
	vm.storeRegisteredClassLookupAlias(overlay, class.Name, class)
	if class.Namespace != "" {
		qualified := runtimeClassName(class)
		vm.writeRegisteredClass(overlay, qualified, class)
		vm.storeRegisteredClassLookupAlias(overlay, qualified, class)
	}
}

func (vm *VM) writeRegisteredClass(overlay *classLookupOverlay, alias string, class Class) {
	// Frozen clones can already have a static index before registration.
	// Replacing or adding static fields makes that index stale.
	previous := vm.Classes[alias]
	if len(previous.StaticFields) != 0 || len(class.StaticFields) != 0 {
		vm.invalidateStaticValueRefs()
	}
	if overlay != nil {
		overlay.recordWrite(vm.Classes, alias, class)
	}
	vm.Classes[alias] = class
}

func (vm *VM) storeRegisteredClassLookupAlias(overlay *classLookupOverlay, name string, class Class) {
	if overlay != nil {
		overlay.storeAlias(name, class)
		return
	}
	vm.storeClassLookupAlias(name, class)
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
	vm.prepareClassMapWrite()
	vm.classValuesWritten = true
	if vm.frozenClassLookup != nil && vm.updateExistingClassValue(class) {
		return
	}
	vm.unshareClassLookup()
	if vm.Classes == nil {
		vm.Classes = make(map[string]Class)
	}
	if existing, exists := vm.Classes[class.Name]; !exists || shouldReplaceShortClassAlias(existing, class) {
		vm.writeClassValue(class.Name, class)
	}
	vm.writeClassValue(runtimeClassName(class), class)
	if class.Namespace != "" && !strings.Contains(class.Name, ".") {
		vm.writeClassValue(class.Namespace+"."+class.Name, class)
	}
}

// writeClassValue writes a class value outside registration, keeping the
// pending overlay's view of the values its lookups were taken from.
func (vm *VM) writeClassValue(alias string, class Class) {
	if vm.classOverlay != nil {
		vm.classOverlay.recordPrior(vm.Classes, alias)
	}
	vm.Classes[alias] = class
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
	key := runtimeClassNameCacheKey{Namespace: namespace, Name: name}
	runtimeClassNameCacheMu.RLock()
	cached, ok := runtimeClassNameCache[key]
	runtimeClassNameCacheMu.RUnlock()
	if ok {
		return cached
	}
	qualified := namespace + "." + name
	runtimeClassNameCacheMu.Lock()
	if cached, ok := runtimeClassNameCache[key]; ok {
		runtimeClassNameCacheMu.Unlock()
		return cached
	}
	runtimeClassNameCache[key] = qualified
	runtimeClassNameCacheMu.Unlock()
	return qualified
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
	vm.classLookupBuilds++
	vm.frozenClassLookup = nil
	vm.classOverlay = nil
	vm.sharedClassCopyPlan = nil
	vm.classLookupGeneration = nextClassLookupGeneration.Add(1)
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
	vm.resetClassLookupNameCache()
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
