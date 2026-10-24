package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// These diagnostics are scoped to the captured async platform surface.
// Callers preserve project type shadowing and the source position of the row.
// S001-S004: dotted project types can shadow platform names. Use the symbol
// selected by normal type resolution rather than the short-name shadow guard.
func semaAsyncPlatformType(model *semaTypeMemberView, resolvedTypeName string) bool {
	members, _, ok := semaLookupTypeMembers(model, resolvedTypeName)
	// C022: a captured unavailable platform type may have no member symbol.
	// Only a resolved project symbol suppresses these native restrictions.
	return !ok || members.platform
}

func semaAsyncTypeMessage(typeName string) string {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "queueableduplicatesignature.builder", "system.batchablecontextimpl":
		return "Invalid type: " + typeName
	}
	return ""
}

func semaAsyncConstructorMessage(typeName string) string {
	if message := semaAsyncTypeMessage(typeName); message != "" {
		return message
	}
	name := semaCanonicalPlatformAlias(typeName)
	switch strings.ToLower(name) {
	case "queueablecontext", "schedulablecontext", "finalizercontext":
		return "Type cannot be constructed: System." + name
	case "database.batchablecontext":
		return "Type cannot be constructed: Database.BatchableContext"
	case "queueablecontextimpl":
		return "Constructor not defined: [System.QueueableContextImpl].<Constructor>()"
	case "schedulablecontextimpl":
		return "Constructor not defined: [System.SchedulableContextImpl].<Constructor>()"
	case "finalizercontextimpl":
		return "Method is not visible: void System.FinalizerContextImpl.<init>()"
	}
	return ""
}

func semaAsyncCallCandidate(receiverType, method string) bool {
	receiver := semaCanonicalPlatformAlias(receiverType)
	return strings.EqualFold(receiver, "AsyncOptions") && strings.EqualFold(method, "getMaximumQueueableStackDepth") ||
		strings.EqualFold(receiver, "FlexQueue") && (strings.EqualFold(method, "moveJobBefore") || strings.EqualFold(method, "moveJobAfter")) ||
		strings.EqualFold(receiver, "Database") && strings.EqualFold(method, "executeBatch")
}

func semaAsyncCallMessage(receiverType, method string, argTypes []string) string {
	receiver := semaCanonicalPlatformAlias(receiverType)
	if strings.EqualFold(receiver, "AsyncOptions") && strings.EqualFold(method, "getMaximumQueueableStackDepth") {
		receiver = "System.AsyncOptions"
	} else if strings.EqualFold(receiver, "FlexQueue") && (strings.EqualFold(method, "moveJobBefore") || strings.EqualFold(method, "moveJobAfter")) {
		receiver = "FlexQueue"
	} else if strings.EqualFold(receiver, "Database") && strings.EqualFold(method, "executeBatch") && len(argTypes) == 2 && strings.EqualFold(argTypes[1], "Decimal") {
		receiver = "Database"
	} else {
		return ""
	}
	args := append([]string(nil), argTypes...)
	for i, arg := range args {
		if strings.EqualFold(arg, "null") {
			args[i] = "NULL"
		}
	}
	return fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(args, ", "), receiver)
}

func semaFinalizerCallMessage(receiver, method string, argTypes []string, model *semaTypeMemberView) string {
	if !strings.EqualFold(semaCanonicalPlatformAlias(receiver), "Finalizer") ||
		!semaAsyncPlatformType(model, receiver) || len(resolveMemberMethods(model, receiver, method)) != 0 {
		return ""
	}
	return fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type System.Finalizer", method, strings.Join(argTypes, ", "))
}

// Q001: generic Batchable signature de-duplication must leave real callback
// overload ambiguity rejected, with the captured native diagnostic text.
func semaAsyncBatchableAmbiguityMessage(receiverType, method string, argTypes []string, model *semaTypeMemberView) string {
	if !strings.EqualFold(method, "execute") || len(argTypes) != 2 ||
		!strings.EqualFold(argTypes[0], "null") || !strings.EqualFold(argTypes[1], "null") {
		return ""
	}
	batchable, ok := model.lookup(normalizeName("Database.Batchable"))
	if !ok || !batchable.platform || !semaTypeMatches(model, receiverType, "Database.Batchable", make(map[string]bool)) {
		return ""
	}
	return fmt.Sprintf("Ambiguous method signature: void %s(NULL, NULL)", method)
}

func semaAsyncFieldMessage(path string) string {
	receiver, field, ok := splitSemaMethodPath(path)
	if ok && strings.EqualFold(semaCanonicalPlatformAlias(receiver), "ParentJobResult") && !strings.EqualFold(field, "SUCCESS") && !strings.EqualFold(field, "UNHANDLED_EXCEPTION") {
		return "Variable does not exist: " + field
	}
	return ""
}

func semaAsyncDiagnostic(typ typesys.TypeSymbol, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA028", Message: message, File: typ.File, Range: semaRange(source, start, end)}
}

func semaAsyncLocalTypeDiagnostics(items []diagnostic.Diagnostic, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	for i := range items {
		item := &items[i]
		if item.Code != "GLADESEMA006" || item.Range == nil {
			continue
		}
		start, end := item.Range.Start.Offset, item.Range.End.Offset
		if start < 0 || end < start || end > len(source) {
			continue
		}
		typeName := strings.TrimSpace(source[start:end])
		if native := semaAsyncTypeMessage(typeName); native != "" && !semaProjectTypeShadowsPlatform(model, typeName) {
			item.Message = native
		}
	}
	return items
}

func semaAsyncRequirementMessage(typ typesys.TypeSymbol, requirement methodRequirement) string {
	owner := semaCanonicalPlatformAlias(requirement.owner)
	context := ""
	switch strings.ToLower(owner) {
	case "queueable":
		context = "QueueableContext"
	case "schedulable":
		context = "SchedulableContext"
	case "finalizer":
		context = "FinalizerContext"
	default:
		return ""
	}
	if requirement.sourceKind != "interface" || !strings.EqualFold(requirement.member.Name, "execute") {
		return ""
	}
	name := typ.LocalName
	if name == "" {
		name = typ.Name
	}
	return fmt.Sprintf("Class %s must implement the method: void System.%s.execute(System.%s)", name, owner, context)
}

// Native async interface contracts accept a static callback declared by the
// implementing class, but an inherited static callback does not implement it.
func semaAsyncInheritedStaticOnly(model *semaTypeMemberView, typ typesys.TypeSymbol, requirement methodRequirement) bool {
	owner := semaCanonicalPlatformAlias(requirement.owner)
	if requirement.sourceKind != "interface" || (!strings.EqualFold(owner, "Queueable") && !strings.EqualFold(owner, "Schedulable")) || !semaAsyncPlatformType(model, requirement.owner) {
		return false
	}
	required := semaNormalizeMemberTypes(model, typ.Name, semaCloneMemberSymbol(requirement.member))
	found := false
	for current := typ.Name; current != ""; {
		members, ok := model.lookup(normalizeName(current))
		if !ok {
			break
		}
		for _, candidate := range members.methods[normalizeName(required.Name)] {
			candidate = semaNormalizeMemberTypes(model, members.name, semaCloneMemberSymbol(candidate))
			if semaQualifiedSignatureConflict(model, candidate, required) || (!sameSemaSignature(candidate, required) && !semaOverrideCompatibleSignature(candidate, required, model)) || !semaInterfaceReturnCompatible(candidate, required, model) || hasModifier(candidate.Modifiers, "abstract") {
				continue
			}
			if strings.EqualFold(current, typ.Name) || !hasModifier(candidate.Modifiers, "static") {
				return false
			}
			found = true
		}
		current = members.superClass
	}
	return found
}

// semaBatchableFromPlatformSuperclass reports whether a platform superclass of
// typeName declares a Database.Batchable interface.
func semaBatchableFromPlatformSuperclass(model *semaTypeMemberView, typeName string) bool {
	members, ok := model.lookup(normalizeName(typeName))
	if !ok {
		return false
	}
	seen := map[string]bool{normalizeName(typeName): true}
	for current := members.superClass; current != "" && !seen[normalizeName(current)]; {
		seen[normalizeName(current)] = true
		ancestor, ok := model.lookup(normalizeName(current))
		if !ok {
			return false
		}
		if ancestor.platform {
			for _, iface := range ancestor.interfaces {
				if semaDatabaseBatchableInterface(iface) {
					return true
				}
			}
		}
		current = ancestor.superClass
	}
	return false
}

func semaBatchableContractInterfaces(model *semaTypeMemberView, typeName string) []string {
	var interfaces []string
	seenTypes, seenInterfaces := map[string]bool{}, map[string]bool{}
	for current := typeName; current != "" && !seenTypes[normalizeName(current)]; {
		seenTypes[normalizeName(current)] = true
		members, ok := model.lookup(normalizeName(current))
		if !ok {
			break
		}
		// Concrete platform classes supply their callbacks. An abstract
		// platform superclass can leave callbacks for its subclass to supply.
		if members.platform && !hasModifier(members.modifiers, "abstract") && !strings.EqualFold(current, typeName) {
			break
		}
		for _, iface := range members.interfaces {
			if !seenInterfaces[normalizeName(iface)] && semaDatabaseBatchableInterface(iface) {
				seenInterfaces[normalizeName(iface)] = true
				interfaces = append(interfaces, iface)
			}
		}
		current = members.superClass
	}
	return interfaces
}
