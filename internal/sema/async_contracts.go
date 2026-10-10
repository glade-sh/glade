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
