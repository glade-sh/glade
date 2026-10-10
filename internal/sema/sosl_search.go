package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// C026/C027 distinguish the two Search return types. Keep this diagnostic
// adapter confined to their platform calls, including project shadow checks.
func semaSearchAssignmentMessage(targetType, valueType, expr string, model *semaTypeMemberView) (string, bool) {
	call := strings.TrimSpace(expr)
	open := strings.IndexByte(call, '(')
	if open < 0 {
		return "", false
	}
	name := strings.TrimSpace(call[:open])
	dot := strings.LastIndexByte(name, '.')
	if dot < 0 {
		return "", false
	}
	receiver, method := name[:dot], name[dot+1:]
	if !strings.EqualFold(semaCanonicalPlatformAlias(receiver), "Search") {
		return "", false
	}
	if target, ok := model.lookup(normalizeName(receiver)); !ok || !target.platform {
		return "", false
	}
	query := strings.EqualFold(method, "query") && strings.EqualFold(valueType, "List<List<SObject>>") && strings.EqualFold(targetType, "Search.SearchResults")
	find := strings.EqualFold(method, "find") && strings.EqualFold(valueType, "Search.SearchResults") && strings.EqualFold(targetType, "List<List<SObject>>")
	if !query && !find {
		return "", false
	}
	return fmt.Sprintf("Illegal assignment from %s to %s", valueType, targetType), true
}

func semaIRSearchAssignmentMessage(targetType, valueType string, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView) (string, bool) {
	if expr.Kind != ir.ExprCall || semaIRCallReceiverMode(expr, scope, model) != "class" {
		return "", false
	}
	callee := expr.Callee
	if expr.Left != nil {
		receiver, ok := semaIRExprTypeReceiverPath(*expr.Left)
		if !ok {
			return "", false
		}
		callee = receiver + "." + callee
	}
	return semaSearchAssignmentMessage(targetType, valueType, callee+"(", model)
}

// Search's access and suggestion-options boundaries are distinct: options
// accept Object and validate at runtime, while Boolean/Integer access levels
// do not name an overload (C022-C024).
func semaSearchCallDiagnostic(typ typesys.TypeSymbol, receiver, method string, args []string, mode string, start int, source string, model *semaTypeMemberView) (*diagnostic.Diagnostic, bool) {
	canonicalReceiver := semaCanonicalPlatformAlias(receiver)
	if !strings.EqualFold(canonicalReceiver, "Search") && !strings.EqualFold(canonicalReceiver, "Search.SuggestionOption") {
		return nil, false
	}
	if target, ok := model.lookup(normalizeName(receiver)); !ok || !target.platform {
		return nil, false
	}
	receiver = canonicalReceiver
	nullable := func(value, want string) bool {
		return value == "" || strings.EqualFold(value, "null") || strings.EqualFold(value, want)
	}
	access := func(value string) bool { return nullable(value, "AccessLevel") || strings.EqualFold(value, "Object") }
	valid, handled := false, false
	if strings.EqualFold(receiver, "Search") && mode == "class" {
		switch strings.ToLower(method) {
		case "query", "find":
			handled = true
			valid = (len(args) == 1 || len(args) == 2) && nullable(args[0], "String")
			if valid && len(args) == 2 {
				valid = access(args[1])
			}
		case "suggest":
			handled = true
			valid = (len(args) == 3 || len(args) == 4) && nullable(args[0], "String") && nullable(args[1], "String")
			if valid && len(args) == 4 {
				valid = access(args[3])
			}
		}
	} else if strings.EqualFold(receiver, "Search.SuggestionOption") && strings.EqualFold(method, "setLimit") && len(args) == 1 && strings.EqualFold(args[0], "String") {
		handled = true
	}
	if !handled || valid {
		return nil, handled
	}
	for i, arg := range args {
		if strings.EqualFold(arg, "null") {
			args[i] = "NULL"
		}
	}
	d := diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA009", Message: fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(args, ", "), receiver), File: typ.File, Range: semaRange(source, start, start+max(1, len(method)))}
	return &d, true
}

func semaSearchConstructorDiagnostic(typ typesys.TypeSymbol, name string, args []ir.Expr, start int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if len(args) != 0 {
		return diagnostic.Diagnostic{}, false
	}
	resolvedName := name
	name = strings.TrimPrefix(name, "System.")
	switch name {
	case "Search.SearchResult", "Search.SearchResults", "Search.SuggestionResult", "Search.SuggestionResults":
	default:
		return diagnostic.Diagnostic{}, false
	}
	// C029-C033 platform contracts do not apply to the source-owned nested
	// Search classes accepted by native controls S001-S005.
	if target, ok := model.lookup(normalizeName(resolvedName)); !ok || !target.platform {
		return diagnostic.Diagnostic{}, false
	}
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA015", Message: "Constructor not defined: [" + name + "].<Constructor>()", File: typ.File, Range: semaRange(source, start, start+len(name))}, true
}
