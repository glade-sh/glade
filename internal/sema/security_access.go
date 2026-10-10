package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/ir"
)

// C005/C006 and NC001: the generated security symbol is an interface, with
// evaluate(SObject) as its contract. Preserve an explicitly declared shadow.
func semaApplySecurityInterfaceOverlay(model map[string]typeMembers, platform *semaTypeMemberModel) {
	const name = "TxnSecurity.EventCondition"
	if current, ok := model[normalizeName(name)]; ok && !current.platform {
		return
	}
	members, ok := semaPlatformTypeMembers(platform, name)
	if !ok {
		return
	}
	members.name = name
	members.kind = apexast.DeclarationInterface
	members.constructors = nil
	members.constructorsAuthoritative = true
	model[normalizeName(name)] = members
}

// The security surface has closed signatures even on dotted platform receivers.
// These checks precede the permissive fallback used for external dependencies.
func (a *Analyzer) securityAccessCallMessage(expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) string {
	if expr.Left != nil {
		if message := a.securityAccessExpressionMessage(*expr.Left, scope, model, currentType); message != "" {
			return message
		}
	}
	if strings.HasPrefix(expr.Callee, "new:") {
		if len(expr.Args)+len(expr.NamedArgs) != 0 {
			return ""
		}
		typeName := resolveNestedTypeReference(model, currentType, strings.TrimPrefix(expr.Callee, "new:"))
		if strings.EqualFold(typeName, "TxnSecurity.EventCondition") && semaDMLPlatformType(model, typeName) {
			return "Type cannot be constructed: TxnSecurity.EventCondition"
		}
		if strings.EqualFold(typeName, "TxnSecurity.Event") && semaDMLPlatformType(model, typeName) {
			return "Constructor not defined: [TxnSecurity.Event].<Constructor>()"
		}
		return ""
	}
	receiver, method, qualified := splitSemaMethodPath(expr.Callee)
	if !qualified && expr.Left != nil {
		method = expr.Callee
	} else if qualified {
		if message := a.securityAccessFieldMessage(receiver, scope, model); message != "" {
			return message
		}
	}
	// Only these methods have closed security signatures. Check receiver-side
	// diagnostics above, but avoid inferring unrelated fluent call chains.
	switch normalizeName(method) {
	case "name", "ordinal", "values", "valueof", "query", "insert", "update", "delete", "upsert":
	default:
		// Match the EqualFold checks below, including Unicode fold equivalents.
		if !strings.EqualFold(method, "stripInaccessible") && !strings.EqualFold(method, "execute") && !strings.EqualFold(method, "log") {
			return ""
		}
	}
	if !qualified && expr.Left != nil {
		receiver = a.inferIRExprType(*expr.Left, scope, model, currentType)
	} else if qualified {
		if valueType, ok := scope.lookup(receiver); ok {
			receiver = valueType
		} else if fieldType := inferSemaArgTypeWithModel(receiver, scope.flatCopy(), model); fieldType != "" &&
			!semaDMLPlatformType(model, receiver) {
			receiver = fieldType
		}
	}
	// N056-N063: a lexically declared enum takes precedence over an
	// unqualified platform spelling; explicit System qualification stays intact.
	receiver = resolveNestedTypeReference(model, currentType, receiver)
	receiver = semaCanonicalPlatformAlias(receiver)
	switch normalizeName(receiver) {
	case "accesslevel", "security", "database", "txnsecurity.eventcondition", "userprovisioning.userprovisioninglog":
	default:
		return ""
	}
	if !semaDMLPlatformType(model, receiver) {
		return ""
	}
	argTypes := irCallArgTypes(a, expr.Args, scope, model, currentType)
	rejected := false
	nativeType := receiver
	switch normalizeName(receiver) {
	case "accesslevel":
		nativeType = "System.AccessLevel"
		switch normalizeName(method) {
		case "name", "ordinal", "values", "valueof":
			rejected = true // AccessLevel is a class, not an enum (R075-R084).
		}
	case "security":
		nativeType = "System.Security"
		if strings.EqualFold(method, "stripInaccessible") {
			rejected = !semaArgsMatchAny([][]string{{"AccessType", "List<SObject>"}, {"AccessType", "List<SObject>", "Boolean"}, {"AccessType", "List<SObject>", "Boolean", "Id"}}, argTypes, model)
		}
	case "database":
		switch normalizeName(method) {
		case "query":
			rejected = !semaArgsMatchAny([][]string{{"String"}, {"String", "AccessLevel"}}, argTypes, model)
		case "insert", "update", "delete", "upsert":
			if len(argTypes) == 2 && strings.EqualFold(argTypes[1], "null") {
				return fmt.Sprintf("Ambiguous method signature: void %s(%s, NULL)", method, argTypes[0])
			}
		}
	case "txnsecurity.eventcondition":
		rejected = strings.EqualFold(method, "execute")
	case "userprovisioning.userprovisioninglog":
		if strings.EqualFold(method, "log") {
			rejected = !semaArgsMatchAny([][]string{{"Id", "String"}, {"Id", "String", "String"}}, argTypes, model)
		}
	}
	if !rejected {
		return ""
	}
	for i, argType := range argTypes {
		if strings.EqualFold(argType, "null") {
			argTypes[i] = "NULL"
		} else if strings.EqualFold(semaCanonicalPlatformAlias(argType), "AccessType") {
			argTypes[i] = "System.AccessType"
		}
	}
	return fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(argTypes, ", "), nativeType)
}

func (a *Analyzer) securityAccessExpressionMessage(expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) string {
	switch expr.Kind {
	case ir.ExprVariable:
		return a.securityAccessFieldMessage(expr.Name, scope, model)
	case ir.ExprCall:
		return a.securityAccessCallMessage(expr, scope, model, currentType)
	}
	return ""
}

func (a *Analyzer) securityAccessFieldMessage(name string, scope irSemaScope, model *semaTypeMemberView) string {
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return ""
	}
	// N039-N047: class literals use ordinary type resolution, rather than the
	// C011/C012/C015 closed static-token checks below.
	if strings.EqualFold(parts[len(parts)-1], "class") {
		return ""
	}
	// Resolve value bindings before interpreting a receiver as a platform type.
	// Fields and properties shadow tokens just as locals and parameters do.
	if _, bound := scope.lookup(parts[0]); bound {
		return ""
	}
	// N056/N058/N062: resolve enclosing types before closing the platform
	// token surface. The ordinary member checker handles the declared type.
	currentType, _ := scope.lookup(semaCurrentTypeScopeKey)
	resolvedRoot := resolveNestedTypeReference(model, currentType, parts[0])
	if !strings.EqualFold(resolvedRoot, parts[0]) && !semaDMLPlatformType(model, resolvedRoot) {
		return ""
	}
	if len(parts) > 2 && strings.EqualFold(parts[0], "System") {
		parts = parts[1:]
	}
	if len(parts) == 2 && semaDMLPlatformType(model, parts[0]) {
		var values []string
		switch normalizeName(parts[0]) {
		case "accesslevel":
			values = []string{"USER_MODE", "SYSTEM_MODE"}
		case "accesstype":
			values = []string{"READABLE", "CREATABLE", "UPDATABLE", "UPSERTABLE"}
		default:
			return ""
		}
		for _, value := range values {
			if strings.EqualFold(parts[1], value) {
				return ""
			}
		}
		return "Variable does not exist: " + parts[1]
	}
	if strings.EqualFold(parts[0], "Schema") {
		parts = parts[1:]
	}
	if len(parts) == 3 && strings.EqualFold(parts[1], "RowCause") {
		members, _, ok := semaLookupTypeMembers(model, parts[0])
		if !ok || !members.sobject {
			return ""
		}
		// C016 rejects an undeclared custom reason. Leave the existing
		// generated system-reason surface unchanged.
		if !strings.HasSuffix(normalizeName(parts[2]), "__c") {
			return ""
		}
		reasonName := parts[2]
		if local, ok := semaProjectLocalAPIName(a.namespace, reasonName); ok {
			reasonName = local
		}
		for _, object := range a.queryDeclaredObjects {
			share, generated := semaShareObjectForSchemaObject(object)
			if !generated {
				continue
			}
			shareName := share.Name
			if local, ok := semaProjectLocalAPIName(a.namespace, shareName); ok {
				shareName = local
			}
			if strings.EqualFold(parts[0], share.Name) || strings.EqualFold(parts[0], shareName) {
				for _, reason := range object.SharingReasons {
					if local, ok := semaProjectLocalAPIName(a.namespace, reason); ok {
						reason = local
					}
					if strings.EqualFold(reason, reasonName) {
						return ""
					}
				}
				return "Variable does not exist: " + parts[2]
			}
		}
		return ""
	}
	// C015: a declared custom object has a closed static field-token surface.
	if len(parts) == 2 && strings.HasSuffix(normalizeName(parts[0]), "__c") {
		if members, _, ok := semaLookupTypeMembers(model, parts[0]); ok && members.sobject && !members.partialSObject {
			if _, ok := semaResolveFieldFromMembers(model, members, normalizeName(parts[1]), parts[1], map[string]bool{}); !ok {
				return "Variable does not exist: " + strings.Join(parts, ".")
			}
		}
	}
	return ""
}
