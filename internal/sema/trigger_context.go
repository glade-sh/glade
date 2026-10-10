package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

func semaTriggerContextField(name string, scope irSemaScope, model *semaTypeMemberView) (string, bool) {
	root, field, qualified := strings.Cut(name, ".")
	if !qualified || !strings.EqualFold(root, "Trigger") || strings.ContainsAny(field, ".[?") {
		return "", false
	}
	if strings.EqualFold(field, "class") {
		return "", false
	}
	if _, bound := scope.lookup(root); bound || semaProjectTypeShadowsPlatform(model, "Trigger") {
		return "", false
	}
	return field, true
}

func semaKnownTriggerContextField(field string) bool {
	switch strings.ToLower(field) {
	case "new", "old", "newmap", "oldmap", "isexecuting", "isbefore", "isafter", "isinsert", "isupdate", "isdelete", "isundelete", "operationtype", "size":
		return true
	default:
		return false
	}
}

func semaUnavailableTriggerOperation(name string, scope irSemaScope, model *semaTypeMemberView) bool {
	root, field, qualified := strings.Cut(name, ".")
	if !qualified {
		return false
	}
	if _, bound := scope.lookup(root); bound {
		return false
	}
	if strings.EqualFold(root, "System") {
		root, field, qualified = strings.Cut(field, ".")
		if !qualified || semaProjectTypeShadowsPlatform(model, "System") {
			return false
		}
	}
	// The valid seven-event enum has no before-undelete member.
	return strings.EqualFold(root, "TriggerOperation") && strings.EqualFold(field, "BEFORE_UNDELETE") && !semaProjectTypeShadowsPlatform(model, "TriggerOperation")
}

func semaTriggerAssignmentMessage(targetType, valueType string, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView) string {
	triggerValue := false
	if expr.Kind == ir.ExprVariable {
		field, platform := semaTriggerContextField(expr.Name, scope, model)
		triggerValue = platform && semaKnownTriggerContextField(field)
	}
	for _, name := range []string{targetType, valueType} {
		if strings.EqualFold(semaCanonicalAssignableType(name), "TriggerOperation") && !semaProjectTypeShadowsPlatform(model, "TriggerOperation") {
			triggerValue = true
		}
	}
	if !triggerValue {
		return ""
	}
	qualify := func(name string) string {
		if strings.EqualFold(semaCanonicalAssignableType(name), "TriggerOperation") {
			return "System.TriggerOperation"
		}
		return name
	}
	// Retain native type spelling and exact diagnostic text.
	return "Illegal assignment from " + qualify(valueType) + " to " + qualify(targetType)
}

func semaTriggerLocalAssignmentDiagnostics(diagnostics []diagnostic.Diagnostic, currentType, body string, bodyOffset int, scopes semaScopeModel, model *semaTypeMemberView, matches [][]int) []diagnostic.Diagnostic {
	for i := range diagnostics {
		item := &diagnostics[i]
		if item.Code != "GLADESEMA018" || item.Range == nil {
			continue
		}
		for _, match := range matches {
			if match[1] == 0 || body[match[1]-1] != '=' {
				continue
			}
			value := trimSemaArg(body, match[1], semaLocalInitializerEnd(body, match[1]))
			if item.Range.Start.Offset != bodyOffset+value.start || item.Range.End.Offset != bodyOffset+value.end {
				continue
			}
			targetType := resolveNestedTypeReference(model, currentType, strings.TrimSpace(body[match[2]:match[3]]))
			flat := scopes.flatAtCopy(value.start)
			valueType := semaResolveConstructedExpressionType(model, currentType, value.text, flat)
			scope := newIRSemaScope(flat)
			if message := semaTriggerAssignmentMessage(targetType, valueType, ir.Expr{Kind: ir.ExprVariable, Name: value.text}, scope, model); message != "" {
				item.Message = message
			}
			break
		}
	}
	return diagnostics
}

func semaTriggerDiagnostic(typ typesys.TypeSymbol, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA021", Message: message, File: typ.File, Range: semaRange(source, start, end)}
}
