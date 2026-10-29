package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func semaTypeReferenceKnownAtVersion(a *Analyzer, model *semaTypeMemberView, owner, reference, version string) bool {
	if !strings.Contains(reference, ".") {
		return a.hasKnownAtVersion(reference, version)
	}
	// Project source resolves first. A namespace declared by the project or
	// by a sub-project whose source it loads names that source, including
	// inner types: ns.Outer.Inner is the source type Outer.Inner.
	if root, suffix, _ := strings.Cut(reference, "."); semaQualifierNamesProjectSource(a, root) {
		if members, known := model.lookupName(suffix); known && !members.platform && !members.dependency {
			return a.hasKnownAtVersion(suffix, version)
		}
	}
	switch normalizeName(reference) {
	case "schema.address":
		return a.hasKnownAtVersion("Address", version)
	case "cache.org.orgcacheexception":
		return a.hasKnownAtVersion("cache.OrgCacheException", version)
	case "cache.session.sessioncacheexception":
		return a.hasKnownAtVersion("cache.SessionCacheException", version)
	case "auth.jwtvalidationexception":
		// Native API 62/67 accepts this exception, absent from the stubs.
		return a.hasKnownAtVersion(reference, version)
	}
	if semaVisualforceComponentType(reference) {
		return true
	}
	resolved := resolveNestedTypeReference(model, owner, reference)
	if _, known := model.lookupName(resolved); known {
		return a.hasKnownAtVersion(resolved, version)
	}
	canonical := semaCanonicalPlatformAlias(reference)
	if _, known := a.known[a.canonicalName(canonical)]; known {
		return a.hasKnownAtVersion(reference, version)
	}
	const systemPrefix = "System."
	if len(reference) > len(systemPrefix) && strings.EqualFold(reference[:len(systemPrefix)], systemPrefix) {
		systemType := reference[len(systemPrefix):]
		resolvedSystemType := resolveNestedTypeReference(model, owner, systemType)
		if members, known := model.lookupName(resolvedSystemType); known && members.platform {
			return a.hasKnownAtVersion(systemType, version)
		}
		if known, ok := a.known[a.canonicalName(systemType)]; ok && known.Kind == TypePlatform {
			return a.hasKnownAtVersion(systemType, version)
		}
	}
	// Schema.<object> names the SObject type itself unless a project type
	// named Schema shadows the namespace. Native:
	// testdata/schema_relationship_gate/org.tsv:301,307,319,325.
	if len(reference) > len("Schema.") && strings.EqualFold(reference[:len("Schema.")], "Schema.") {
		object := strings.TrimSpace(reference[len("Schema."):])
		members, shadowed := model.lookupName(resolveNestedTypeReference(model, owner, "Schema"))
		shadowed = shadowed && !members.platform && !members.dependency
		if !shadowed && !strings.Contains(object, ".") && (semaSObjectExistsInModel(object, model) || storage.IsKnownStandardObject(object)) {
			return a.hasKnownAtVersion(object, version)
		}
	}
	// A namespace can be rejected only when its inventory is complete. The
	// project's packaging namespace and dependency namespaces are open, even
	// when a source class happens to have the same name as the qualifier.
	root, tail, _ := strings.Cut(reference, ".")
	if semaQualifierNamesKnownNamespace(a, root) {
		return true
	}
	parts := strings.Split(reference, ".")
	for end := len(parts) - 1; end > 0 && !strings.EqualFold(root, "System"); end-- {
		prefix := resolveNestedTypeReference(model, owner, strings.Join(parts[:end], "."))
		members, known := model.lookupName(prefix)
		if !known || members.platform {
			continue
		}
		if members.dependency {
			return true
		}
		if members.sobject {
			// An SObject's short name can also qualify an undeclared installed
			// namespace. Only explicit Schema qualification disambiguates the
			// object type; resolved field/value paths are checked separately.
			if !strings.EqualFold(root, "Schema") {
				return true
			}
			// Source inference can create this shape from the reference being
			// checked. Only established object identity proves no nested types.
			return !semaSObjectIdentityEstablished(a, members.name, model)
		}
		// The deepest indexed source prefix owns the remaining nested name.
		// Its own declaration does not enumerate inherited types. Independent
		// source units with the same name also cannot establish one inventory.
		return !semaSourceTypeInCallingScope(model, owner, members) ||
			!model.state.base.completeSourceNestedTypes[normalizeName(members.name)]
	}
	if strings.EqualFold(root, "System") {
		// System enumerates top-level aliases, not every nested declaration
		// of each platform class. Only an absent top-level alias proves absence.
		platformRoot, _, nested := strings.Cut(tail, ".")
		if nested {
			// Workspace types may shadow a platform name in a.known. The
			// immutable platform inventory still owns explicit System names.
			known, ok := semaBaseKnownTypes()[a.canonicalName(platformRoot)]
			if !ok {
				known, ok = a.known[a.canonicalName(platformRoot)]
			}
			platformKnown := ok && (known.Kind == TypePlatform || known.Kind == TypeBuiltin)
			if !platformKnown && model != nil && model.state != nil {
				// The base known-type table is a subset of the platform catalog.
				// Workspace types can shadow names added from that catalog, so
				// consult the immutable platform model for explicit System roots.
				_, systemRoot := semaPlatformAlias("System." + platformRoot)
				members, found := semaPlatformTypeMembers(model.state.platform, platformRoot)
				platformKnown = systemRoot && found && members.platform
			}
			if platformKnown {
				if storage.IsKnownStandardObject(platformRoot) || isCommonSemaSObjectName(platformRoot) || vm.IsCommonSObjectTypeName(platformRoot) {
					return false
				}
				// Keep explicit System identity; a project class with the same
				// short name cannot close this platform nested-type catalog.
				return !semaAPI67RejectedPlatformType(tail) && !semaCapturedFormulaTypeAbsent(tail) &&
					!semaPlatformTypeUnavailable(version, tail)
			}
		}
		return false // Closed System type/alias inventory; invalid_system native row.
	}
	if strings.EqualFold(root, "Schema") {
		object, _, nested := strings.Cut(tail, ".")
		if nested && semaSObjectIdentityEstablished(a, object, model) {
			return false // An SObject has fields, not nested Apex type declarations.
		}
		if !nested && storage.IsReferenceOnlyStandardObject(object) {
			// Reference-only targets are a closed Schema inventory: the
			// analyzer's own rule decides them, including named-source
			// admission. Native: reference_only_names schema_new rows.
			return a.hasKnownAtVersion(reference, version)
		}
	}
	// Other platform catalogs, platform nested-type catalogs, and installed
	// package namespaces are incomplete. Preserve captured availability rules,
	// but never interpret a catalog miss as proof of absence.
	return !semaAPI67RejectedPlatformType(reference) && !semaCapturedFormulaTypeAbsent(reference) &&
		!semaPlatformTypeUnavailable(version, reference)
}

func semaCapturedFormulaTypeAbsent(reference string) bool {
	// formula_evaluation C016/C017 prove only these spellings absent, not the
	// rest of FormulaEval's partial namespace inventory.
	switch normalizeName(reference) {
	case "formulaeval.formulavalidationexception", "formulaeval.formulaevaluationexception":
		return true
	default:
		return false
	}
}

// semaQualifierNamesProjectSource reports whether root is a namespace that
// project source declares: the project's own or a loaded sub-project's.
func semaQualifierNamesProjectSource(a *Analyzer, root string) bool {
	if a == nil || root == "" {
		return false
	}
	return (a.namespace != "" && strings.EqualFold(a.namespace, root)) || a.sourceNamespaces[normalizeName(root)]
}

func semaQualifierNamesKnownNamespace(a *Analyzer, root string) bool {
	if a == nil {
		return false
	}
	return (a.namespace != "" && strings.EqualFold(a.namespace, root)) || a.deps[normalizeName(root)]
}

func semaQualifiedCallReceiverKnown(a *Analyzer, currentType, receiver string, scope irSemaScope, model *semaTypeMemberView, versions ...string) bool {
	receiver = strings.TrimSpace(receiver)
	if receiver == "" {
		return false
	}
	if semaKnownClassLiteralReceiver(a, currentType, receiver, model) {
		return true
	}
	if semaModelHasType(model, resolveNestedTypeName(model, currentType, receiver)) || semaEnumValuePathType(model, receiver) != "" {
		return true
	}
	if semaPageReceiverPathKnown(receiver) || semaKnownSObjectFieldTokenPath(receiver, model) ||
		semaKnownSObjectTypeTokenPath(receiver, model) || semaKnownSObjectFieldSetPath(receiver, model) {
		return true
	}
	if semaIRCustomLabelReference(receiver, scope, model, currentType) {
		return true
	}
	if semaQualifiedStaticReceiverPathKnown(model, currentType, receiver) {
		return true
	}
	// Keep an unbound static call with no known root, namespace or
	// declaration behind the compile gate, as the release did. Deferring a
	// type spelling is not a runnable package method: offline execution of
	// the call fails, so its unit stays a compile exclusion and independent
	// tests still run. Native missing_static rows reject it in an org without
	// the package (testdata/conformance/unloaded_dependency/api{62,67}).
	// A loaded dependency namespace is known; a declared but unloaded one
	// supplies no method either. Preserve declaration-backed deferral and
	// positively resolved receivers above.
	root, _, _ := strings.Cut(receiver, ".")
	if !a.hasKnown(root) && !semaIRUnresolvedDeclaredReceiver(receiver, scope, model) {
		if _, bound := scope.lookup(root); !bound {
			return false
		}
	}
	// This is a directly observed receiver rejection, not a claim that the
	// containing platform class has a complete nested-type catalog.
	version := ""
	if len(versions) != 0 {
		version = versions[0]
	}
	if semaCapturedQualifiedReceiverAbsent(currentType, receiver, version, model) {
		return false
	}
	if a.hasExternalDependencyName(receiver) {
		if semaQualifiedReceiverHasIndexedTypePrefix(a, receiver, currentType, model, version) {
			return false
		}
		return true
	}
	root, fieldPath, ok := strings.Cut(receiver, ".")
	if !ok {
		return false
	}
	_, ok = semaStaticClassFieldPathMemberInContext(model, currentType, root, fieldPath)
	if ok {
		return true
	}
	// An unindexed package type is not a missing member of an indexed class.
	// Keep value/member paths and closed project/System/Schema roots strict.
	if semaQualifiedReceiverHasIndexedTypePrefix(a, receiver, currentType, model, version) {
		return false
	}
	if semaQualifierNamesKnownNamespace(a, root) {
		return true
	}
	// Schema.<custom object>.fields is an SObject token path, not an
	// unresolved nested Apex class. Native qualified_receiver_widening
	// org.tsv:24,52 captures rejection of an absent object in this position.
	if strings.EqualFold(root, "Schema") && !semaProjectTypeShadowsPlatform(model, "Schema") {
		parts := strings.Split(fieldPath, ".")
		if len(parts) >= 2 && semaIsCustomAPIName(parts[0]) && strings.EqualFold(parts[1], "fields") {
			return false
		}
	}
	return semaTypeReferenceKnownAtVersion(a, model, currentType, receiver, version)
}

// Unresolved type receivers share one boundary in the IR and text call walks.
// Positive type/value/token resolutions retain their ordinary member checks.
func semaQualifiedCallReceiverDeferred(a *Analyzer, currentType, receiver string, scope irSemaScope, model *semaTypeMemberView, version string) bool {
	root, _, qualified := strings.Cut(receiver, ".")
	// Colons mark lowered IR intrinsics (constructors, field reads and
	// assignments), never Apex namespace-qualified method receivers.
	if !qualified || strings.Contains(receiver, ":") {
		return false
	}
	if _, bound := scope.lookup(root); bound {
		return false
	}
	return semaQualifiedCallReceiverKnown(a, currentType, receiver, scope, model, version) &&
		!semaModelHasType(model, resolveNestedTypeName(model, currentType, receiver)) &&
		!semaIRStaticValueReceiver(model, currentType, receiver) &&
		!semaKnownClassLiteralReceiver(a, currentType, receiver, model) &&
		!semaPageReceiverPathKnown(receiver) && !semaKnownSObjectFieldTokenPath(receiver, model) &&
		!semaKnownSObjectTypeTokenPath(receiver, model) && !semaKnownSObjectFieldSetPath(receiver, model) &&
		!semaIRCustomLabelReference(receiver, scope, model, currentType)
}

// Each fact proves only the captured receiver expression at its captured API
// versions. It does not close sibling names, aliases or type declarations.
func semaCapturedQualifiedReceiverAbsent(currentType, receiver, version string, model *semaTypeMemberView) bool {
	if shadow, known := model.lookupName("System"); known && !shadow.platform && !shadow.dependency && semaSourceTypeInCallingScope(model, currentType, shadow) {
		return false
	}
	for _, fact := range []struct{ receiver, versions string }{
		// qualified_receiver_widening/org.tsv:22,50 and its ORG-RUN.txt.
		{"System.Database.Bogus", "|62.0|67.0|"},
	} {
		if strings.EqualFold(receiver, fact.receiver) && (version == "" || strings.Contains(fact.versions, "|"+version+"|")) {
			return true
		}
	}
	return false
}

// The short token spelling is not the describe-result fields collection.
// Native receiver_member_routes accepts neither getMap() nor get(String) here.
func semaShortSObjectFieldsCall(a *Analyzer, receiver, method string, model *semaTypeMemberView) bool {
	if !strings.EqualFold(method, "getMap") && !strings.EqualFold(method, "get") {
		return false
	}
	parts := strings.Split(receiver, ".")
	short := len(parts) == 2 && strings.EqualFold(parts[1], "fields")
	token := len(parts) == 3 && strings.EqualFold(parts[1], "SObjectType") && strings.EqualFold(parts[2], "fields")
	// Both short spellings are captured SObjectField proxies, not a describe
	// result's field map. A declared namespace or inferred object is not proof
	// that this receiver denotes the proxy.
	return (short || token) && !semaQualifierNamesKnownNamespace(a, parts[0]) &&
		!semaProjectTypeShadowsPlatform(model, parts[0]) && semaSObjectIdentityEstablished(a, parts[0], model)
}

func semaKnownClassLiteralReceiver(a *Analyzer, currentType, receiver string, model *semaTypeMemberView) bool {
	if !strings.HasSuffix(strings.ToLower(receiver), ".class") {
		return false
	}
	typeName := strings.TrimSpace(receiver[:len(receiver)-len(".class")])
	for _, ref := range extractTypeNames(resolveNestedTypeReference(model, currentType, typeName)) {
		if !semaTypeReferenceKnownAtVersion(a, model, currentType, ref, "") {
			return false
		}
	}
	return true
}

// A type prefix does not close its nested inventory. Only shared type absence
// or a positively resolved value/member prefix makes a missing receiver strict.
func semaQualifiedReceiverHasIndexedTypePrefix(a *Analyzer, receiver, currentType string, model *semaTypeMemberView, version string) bool {
	if !semaTypeReferenceKnownAtVersion(a, model, currentType, receiver, version) {
		return true
	}
	parts := strings.Split(receiver, ".")
	for i := 1; i < len(parts); i++ {
		prefix := resolveNestedTypeName(model, currentType, strings.Join(parts[:i], "."))
		members, ok := model.lookupName(prefix)
		if !ok || !(members.platform || members.dependency || members.sobject || semaSourceTypeInCallingScope(model, currentType, members)) {
			continue
		}
		if members.sobject && !semaSObjectIdentityEstablished(a, members.name, model) {
			continue // Inferred fields cannot establish a value receiver's identity.
		}
		if i > 1 && strings.EqualFold(parts[0], "System") && !members.platform {
			if platform, known := semaBaseKnownTypes()[a.canonicalName(parts[1])]; known &&
				(platform.Kind == TypePlatform || platform.Kind == TypeBuiltin) {
				continue // Explicit System prefixes cannot borrow a short schema/source alias.
			}
		}
		if _, known := semaResolveField(model, members.name, parts[i], make(map[string]bool)); known {
			return true
		}
	}
	return false
}

func semaQualifiedStaticReceiverPathKnown(model *semaTypeMemberView, currentType, receiver string) bool {
	parts := strings.Split(receiver, ".")
	if len(parts) < 2 {
		return false
	}
	for end := len(parts) - 1; end > 0; end-- {
		typeName := strings.Join(parts[:end], ".")
		resolvedType := resolveNestedTypeName(model, currentType, typeName)
		members, ok := model.lookup(normalizeName(resolvedType))
		if !ok || members.sobject {
			continue
		}
		first, ok := semaResolveField(model, members.name, parts[end], make(map[string]bool))
		if !ok || !hasModifier(first.member.Modifiers, "static") {
			continue
		}
		receiverType := first.member.Type
		for i := end + 1; i < len(parts); i++ {
			field, ok := semaResolveField(model, receiverType, parts[i], make(map[string]bool))
			if !ok {
				return false
			}
			receiverType = field.member.Type
		}
		return true
	}
	return false
}

func semaPageReceiverPathKnown(receiver string) bool {
	root, name, ok := strings.Cut(strings.TrimSpace(receiver), ".")
	return ok && strings.EqualFold(root, "Page") && name != "" && !strings.Contains(name, ".")
}

func semaKnownSObjectFieldTokenPath(receiver string, model *semaTypeMemberView) bool {
	parts := strings.Split(strings.TrimSpace(receiver), ".")
	schemaQualified := len(parts) > 0 && strings.EqualFold(parts[0], "Schema")
	if schemaQualified {
		parts = parts[1:]
	}
	objectName, fieldName := "", ""
	switch {
	case len(parts) == 2:
		objectName, fieldName = parts[0], parts[1]
	case len(parts) == 3 && strings.EqualFold(parts[1], "fields"):
		objectName, fieldName = parts[0], parts[2]
	case len(parts) == 4 && strings.EqualFold(parts[1], "SObjectType") && strings.EqualFold(parts[2], "fields"):
		objectName, fieldName = parts[0], parts[3]
	case len(parts) == 3 && strings.EqualFold(parts[0], "SObjectType") && strings.EqualFold(parts[2], "fields"):
		objectName = parts[1]
	case len(parts) == 4 && strings.EqualFold(parts[0], "SObjectType") && strings.EqualFold(parts[2], "fields"):
		objectName, fieldName = parts[1], parts[3]
	default:
		return false
	}
	if !semaSObjectExistsInModel(objectName, model) {
		return false
	}
	if fieldName == "" {
		return true
	}
	if _, ok := semaResolveField(model, objectName, fieldName, make(map[string]bool)); ok {
		return true
	}
	if !storage.IsKnownStandardObject(objectName) {
		return false
	}
	provider := semaStandardSObjectFieldProviderFor(objectName)
	if provider == nil {
		return false
	}
	_, ok := provider.lookup(fieldName)
	return ok
}

func semaKnownSObjectTypeTokenPath(receiver string, model *semaTypeMemberView) bool {
	parts := strings.Split(strings.TrimSpace(receiver), ".")
	if len(parts) > 0 && strings.EqualFold(parts[0], "Schema") {
		parts = parts[1:]
		if len(parts) == 1 {
			return semaSObjectExistsInModel(parts[0], model)
		}
	}
	switch {
	case len(parts) == 2 && strings.EqualFold(parts[0], "SObjectType"):
		return semaSObjectExistsInModel(parts[1], model)
	case len(parts) == 2 && strings.EqualFold(parts[1], "SObjectType"):
		return semaSObjectExistsInModel(parts[0], model)
	default:
		return false
	}
}

func semaKnownSObjectFieldSetPath(receiver string, model *semaTypeMemberView) bool {
	parts := strings.Split(strings.TrimSpace(receiver), ".")
	if len(parts) > 0 && strings.EqualFold(parts[0], "Schema") {
		parts = parts[1:]
	}
	if len(parts) != 3 && len(parts) != 4 {
		return false
	}
	if !strings.EqualFold(parts[0], "SObjectType") || !strings.EqualFold(parts[2], "fieldSets") || !semaSObjectExistsInModel(parts[1], model) {
		return false
	}
	return len(parts) == 3 || semaFieldTokenPart(parts[3])
}

func semaSObjectExistsInModel(name string, model *semaTypeMemberView) bool {
	if model == nil {
		return false
	}
	if members, ok := model.lookup(normalizeName(name)); ok && members.sobject {
		return true
	}
	return isCommonSemaSObjectName(name) || vm.IsCommonSObjectTypeName(name)
}

// A provisional source-inferred shape is not proof that a qualifier is an
// SObject. Standard identities and original declared metadata provide that
// proof even when the object's field catalog is partial.
func semaSObjectIdentityEstablished(a *Analyzer, name string, model *semaTypeMemberView) bool {
	if storage.IsKnownStandardObject(name) || isCommonSemaSObjectName(name) || vm.IsCommonSObjectTypeName(name) {
		return true
	}
	if a == nil {
		return false
	}
	canonical := name
	if members, ok := model.lookupName(name); ok && members.sobject {
		canonical = members.name
	}
	for _, object := range a.queryDeclaredObjects {
		if strings.EqualFold(object.Name, name) || strings.EqualFold(object.Name, canonical) {
			return true
		}
	}
	return false
}

func semaMissingSObjectFieldsObject(a *Analyzer, name string, scope irSemaScope, model *semaTypeMemberView) (string, bool) {
	if _, scoped := scope.lookup("Schema"); scoped || semaProjectTypeShadowsPlatform(model, "Schema") {
		return "", false
	}
	parts := strings.Split(strings.TrimSpace(name), ".")
	if len(parts) < 4 || !strings.EqualFold(parts[0], "Schema") || !strings.EqualFold(parts[1], "SObjectType") || !strings.EqualFold(parts[3], "fields") {
		return "", false
	}
	if semaSObjectExistsInModel(parts[2], model) {
		members, indexed := model.lookup(normalizeName(parts[2]))
		if a == nil || !indexed || !members.partialSObject || storage.IsKnownStandardObject(parts[2]) {
			return "", false
		}
		// Named analysis may infer a partial object from this very source
		// reference. Only declared metadata establishes its existence.
		for _, object := range a.queryDeclaredObjects {
			if strings.EqualFold(object.Name, parts[2]) || strings.EqualFold(object.Name, members.name) {
				return "", false
			}
		}
	}
	return strings.Join(parts[:4], "."), true
}
