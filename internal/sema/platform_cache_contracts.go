package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// Cache diagnostics use the platform namespace spelling captured at
// API 62 and 67. Source-defined types with the same name keep their own rules.
func semaPlatformCacheTypeName(typeName string, model *semaTypeMemberView) (string, bool) {
	var name string
	switch normalizeName(typeName) {
	case "cache.org":
		name = "cache.Org"
	case "cache.session":
		name = "cache.Session"
	case "cache.orgpartition":
		name = "cache.OrgPartition"
	case "cache.sessionpartition":
		name = "cache.SessionPartition"
	case "cache.cachebuilder":
		name = "cache.CacheBuilder"
	default:
		return "", false
	}
	if members, ok := model.lookup(normalizeName(typeName)); !ok || !members.platform {
		return "", false
	}
	return name, true
}

func semaPlatformCacheDiagnosticType(typeName string, model *semaTypeMemberView) string {
	if name, ok := semaPlatformCacheTypeName(typeName, model); ok {
		return name
	}
	canonical := semaCanonicalPlatformAlias(typeName)
	if strings.EqualFold(canonical, "Type") {
		return "System.Type"
	}
	return canonical
}

func semaPlatformCacheDiagnostic(typ typesys.TypeSymbol, code, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     code,
		Message:  message,
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func semaPlatformCacheAssignmentMessage(targetType, valueType string, model *semaTypeMemberView) string {
	return "Illegal assignment from " + semaPlatformCacheDiagnosticType(valueType, model) + " to " + semaPlatformCacheDiagnosticType(targetType, model)
}

func semaTextPlatformCacheAssignmentMessage(targetType, valueType, value string, scope map[string]string, model *semaTypeMemberView) (string, bool) {
	_, targetCache := semaPlatformCacheTypeName(targetType, model)
	_, valueCache := semaPlatformCacheTypeName(valueType, model)
	if !targetCache && !valueCache {
		// C015/C016 have primitive target/result types. Identify the direct
		// Cache call rather than treating every void/Object assignment as Cache.
		receiver, _, _, ok := splitLastSemaCall(strings.TrimSpace(value))
		if !ok {
			return "", false
		}
		if _, cache := semaPlatformCacheTypeName(semaTextReceiverType(receiver, scope, model), model); !cache {
			return "", false
		}
	}
	return semaPlatformCacheAssignmentMessage(targetType, valueType, model), true
}

func semaPlatformCacheConstructorDiagnostic(typ typesys.TypeSymbol, typeName string, argc int, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	name, ok := semaPlatformCacheTypeName(typeName, model)
	if !ok {
		return diagnostic.Diagnostic{}, false
	}
	// C017: the generated stub's constructor must not make this interface
	// constructible. C001/C002: partition zero-argument constructors are private.
	if name == "cache.CacheBuilder" && semaAPI67RejectedPlatformConstructor(typeName) {
		return semaPlatformCacheDiagnostic(typ, "GLADESEMA015", "Type cannot be constructed: "+name, start, end, source), true
	}
	if argc == 0 && (name == "cache.OrgPartition" || name == "cache.SessionPartition") {
		return semaPlatformCacheDiagnostic(typ, "GLADESEMA011", "Method is not visible: void "+name+".<init>()", start, end, source), true
	}
	return diagnostic.Diagnostic{}, false
}

func semaPlatformCacheCallDiagnostic(typ typesys.TypeSymbol, receiverType, method string, argTypes []string, receiverMode string, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	name, ok := semaPlatformCacheTypeName(receiverType, model)
	if !ok || (name != "cache.Org" && name != "cache.Session") {
		return diagnostic.Diagnostic{}, false
	}
	// Only the captured Cache method surfaces change diagnostic formatting;
	// overload acceptance still comes from the shared platform signatures.
	switch normalizeName(method) {
	case "getpartition":
		method = "getPartition"
	case "get", "put", "contains", "remove":
		method = normalizeName(method)
	default:
		return diagnostic.Diagnostic{}, false
	}
	if semaAPI67RejectedPlatformCallArgs(typ.EffectiveAPIVersion, receiverType, method, argTypes) {
		// N019/N021: preserve the legacy overload at API 54 and below.
		return semaPlatformCacheDiagnostic(typ, "GLADESEMA009", "Method was removed after version 54.0: "+method, start, end, source), true
	}
	sig, ok := semaPlatformMethodSignatureForMode(model, receiverType, method, receiverMode)
	if !ok || semaArgsMatchAny(sig.params, argTypes, model) {
		return diagnostic.Diagnostic{}, false
	}
	// C005-C014/C019/C020: report the actual argument types, including the
	// System.Type name used for builder class literals.
	displayTypes := make([]string, len(argTypes))
	for i, argType := range argTypes {
		displayTypes[i] = semaPlatformCacheDiagnosticType(argType, model)
	}
	message := "Method does not exist or incorrect signature: void " + method + "(" + strings.Join(displayTypes, ", ") + ") from the type " + name
	return semaPlatformCacheDiagnostic(typ, "GLADESEMA009", message, start, end, source), true
}

func (a *Analyzer) semaIRPlatformCacheAssignment(typ typesys.TypeSymbol, targetType, valueType string, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView) bool {
	// C003/C004/C018: assignments involving a Cache partition or interface.
	if _, ok := semaPlatformCacheTypeName(targetType, model); ok {
		return true
	}
	if _, ok := semaPlatformCacheTypeName(valueType, model); ok {
		return true
	}
	// C015/C016: the Cache call's void/Object result has no Cache type name.
	if expr.Kind != ir.ExprCall {
		return false
	}
	if expr.Left != nil {
		receiverType := a.inferIRExprType(*expr.Left, scope, model, typ.Name)
		if _, ok := semaPlatformCacheTypeName(receiverType, model); ok {
			return true
		}
	}
	if receiver, _, ok := splitSemaMethodPath(expr.Callee); ok {
		receiverType := semaTextReceiverType(receiver, scope.flatCopy(), model)
		_, ok := semaPlatformCacheTypeName(receiverType, model)
		return ok
	}
	return false
}
