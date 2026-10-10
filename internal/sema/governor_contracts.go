package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// These native diagnostics apply only to the captured governor/logging
// surfaces. Project declarations with the same spelling retain their rules.
func semaGovernorType(typeName string, model *semaTypeMemberView) string {
	if semaProjectTypeShadowsPlatform(model, typeName) {
		return ""
	}
	name := semaCanonicalPlatformAlias(typeName)
	switch normalizeName(name) {
	case "limits":
		return "Limit"
	case "orglimits":
		return "System.OrgLimits"
	case "orglimit":
		return "System.OrgLimit"
	case "logginglevel":
		return "System.LoggingLevel"
	case "system":
		return "System"
	}
	return ""
}

func semaGovernorDiagnostic(typ typesys.TypeSymbol, code, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: code, Message: message, File: typ.File, Range: semaRange(source, start, end)}
}

func semaGovernorCallCandidate(receiverType, method string, model *semaTypeMemberView) bool {
	switch semaGovernorType(receiverType, model) {
	case "Limit":
		return semaGovernorRemovedGetter(method) || strings.EqualFold(method, "getQueries") || strings.EqualFold(method, "setLimitQueries")
	case "System.OrgLimits":
		return strings.EqualFold(method, "getAll") || strings.EqualFold(method, "getMap")
	case "System.OrgLimit":
		return strings.EqualFold(method, "setValue")
	case "System":
		return strings.EqualFold(method, "debug")
	}
	return false
}

func semaGovernorRemovedGetter(method string) bool {
	switch normalizeName(method) {
	case "getchildrelationshipsdescribes", "getlimitchildrelationshipsdescribes",
		"getfieldsetsdescribes", "getlimitfieldsetsdescribes",
		"getfieldsdescribes", "getlimitfieldsdescribes",
		"getpicklistdescribes", "getlimitpicklistdescribes",
		"getrecordtypesdescribes", "getlimitrecordtypesdescribes",
		"getscriptstatements", "getlimitscriptstatements":
		return true
	}
	return false
}

func semaGovernorCallMessage(version, receiverType, method string, argTypes []string, model *semaTypeMemberView, mode string) string {
	receiver := semaGovernorType(receiverType, model)
	var params [][]string
	switch receiver {
	case "Limit":
		// C001-C012: legacy describe/script counters were removed after 30.
		if semaGovernorRemovedGetter(method) && apexversion.AtLeast(version, 31) {
			return "Method was removed after version 30.0: " + method
		}
		switch normalizeName(method) {
		case "getqueries":
			params = [][]string{{}}
		case "setlimitqueries":
			// C014: no writable query-limit API exists.
		default:
			return ""
		}
	case "System.OrgLimits":
		if !strings.EqualFold(method, "getAll") && !strings.EqualFold(method, "getMap") {
			return ""
		}
		params = [][]string{{}}
	case "System.OrgLimit":
		if !strings.EqualFold(method, "setValue") {
			return ""
		}
		// C025: the snapshot has getters, not a usage setter.
	case "System":
		if !strings.EqualFold(method, "debug") || mode != "class" {
			return ""
		}
		params = [][]string{{"Object"}, {"LoggingLevel", "Object"}}
	default:
		return ""
	}
	if params != nil && semaArgsMatchAny(params, argTypes, model) {
		return ""
	}
	args := append([]string(nil), argTypes...)
	for i, name := range args {
		if strings.EqualFold(name, "null") {
			args[i] = "NULL"
		}
	}
	return fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(args, ", "), receiver)
}

func semaGovernorConstructorMessage(typeName string, argTypes []string, model *semaTypeMemberView) string {
	switch semaGovernorType(typeName, model) {
	case "Limit":
		// C017: Limits is a static receiver, not a constructible Apex type.
		return "Invalid type: " + typeName
	case "System.OrgLimit":
		// C019/C020: OrgLimit values come from OrgLimits, not constructors.
		return "Constructor not defined: [System.OrgLimit].<Constructor>(" + strings.Join(argTypes, ", ") + ")"
	}
	return ""
}

func semaGovernorDiagnosticType(typeName string, model *semaTypeMemberView) string {
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) > 0 {
		for i, arg := range args {
			args[i] = semaGovernorDiagnosticType(arg, model)
		}
		return base + "<" + strings.Join(args, ",") + ">"
	}
	if name := semaGovernorType(typeName, model); name == "System.OrgLimit" || name == "System.LoggingLevel" {
		return name
	}
	return typeName
}

func semaGovernorAssignmentMessage(targetType, valueType, receiverType string, model *semaTypeMemberView) string {
	// C029/C030 have an enum target. C015/C016/C021/C022/C026 have a
	// governor getter source; unrelated primitive assignments are unchanged.
	governor := semaGovernorType(targetType, model) == "System.LoggingLevel"
	switch semaGovernorType(receiverType, model) {
	case "Limit", "System.OrgLimit", "System.OrgLimits":
		governor = true
	}
	if !governor {
		return ""
	}
	return "Illegal assignment from " + semaGovernorDiagnosticType(valueType, model) + " to " + semaGovernorDiagnosticType(targetType, model)
}

func (a *Analyzer) semaIRGovernorAssignmentMessage(targetType, valueType string, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) string {
	receiverType := ""
	if expr.Kind == ir.ExprCall {
		if expr.Left != nil {
			receiverType = a.inferIRExprType(*expr.Left, scope, model, currentType)
		} else if receiver, _, ok := splitSemaMethodPath(expr.Callee); ok {
			receiverType = semaIRReceiverType(receiver, scope, model, currentType)
		}
	}
	return semaGovernorAssignmentMessage(targetType, valueType, receiverType, model)
}

// The text scanner diagnoses named local initializers before the IR checker.
// Format the same captured assignment at both entries without adding errors.
func semaGovernorLocalAssignmentDiagnostics(items []diagnostic.Diagnostic, currentType, body string, bodyOffset int, scopes semaScopeModel, model *semaTypeMemberView, matches [][]int) []diagnostic.Diagnostic {
	for i := range items {
		item := &items[i]
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
			bindings := scopes.flatAtCopy(value.start)
			targetType := resolveNestedTypeReference(model, currentType, strings.TrimSpace(body[match[2]:match[3]]))
			valueType := semaResolveConstructedExpressionType(model, currentType, value.text, bindings)
			receiverType := ""
			if receiver, _, _, ok := splitLastSemaCall(strings.TrimSpace(value.text)); ok {
				receiverType = semaTextReceiverType(receiver, bindings, model)
			}
			if message := semaGovernorAssignmentMessage(targetType, valueType, receiverType, model); message != "" {
				item.Message = message
			}
			break
		}
	}
	return items
}
