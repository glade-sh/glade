package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func semaQueryLocatorCall(receiverType, method string) bool {
	switch normalizeName(semaCanonicalPlatformAlias(receiverType)) {
	case "database.cursor", "database.querylocator", "database.querylocatoriterator":
		return true
	case "database":
		switch normalizeName(method) {
		case "getquerylocator", "getquerylocatorwithbinds", "getcursor", "getcursorwithbinds":
			return true
		}
	}
	return false
}

// C012-C038 and R136 capture the closed query-handle method surface. Keep this
// check ahead of generated Object overloads and permissive platform fallbacks.
func semaQueryLocatorCallDiagnostic(typ typesys.TypeSymbol, receiverType, method string, args []string, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if !semaQueryLocatorCall(receiverType, method) || semaProjectTypeShadowsPlatform(model, receiverType) {
		return diagnostic.Diagnostic{}, false
	}
	name := semaCanonicalPlatformAlias(receiverType)
	var params [][]string
	switch normalizeName(name) {
	case "database":
		name = "Database"
		switch normalizeName(method) {
		case "getquerylocator":
			method = "getQueryLocator"
			params = [][]string{{"String"}, {"List<SObject>"}, {"String", "Object"}, {"List<SObject>", "Object"}}
		case "getcursor":
			method = "getCursor"
			params = [][]string{{"String"}, {"String", "Object"}}
		case "getquerylocatorwithbinds":
			method = "getQueryLocatorWithBinds"
			params = [][]string{{"String", "Map", "Object"}}
		case "getcursorwithbinds":
			method = "getCursorWithBinds"
			params = [][]string{{"String", "Map", "Object"}}
		default:
			return diagnostic.Diagnostic{}, false
		}
	case "database.cursor":
		name = "Database.Cursor"
		switch normalizeName(method) {
		case "fetch":
			method, params = "fetch", [][]string{{"Integer", "Integer"}}
		case "getnumrecords":
			method, params = "getNumRecords", [][]string{{}}
		case "hashcode", "equals", "tostring", "clone":
			return diagnostic.Diagnostic{}, false
		}
	case "database.querylocator":
		name = "Database.QueryLocator"
		switch normalizeName(method) {
		case "iterator":
			method, params = "iterator", [][]string{{}}
		case "getquery":
			method, params = "getQuery", [][]string{{}}
		case "hashcode", "equals", "tostring", "clone", "querymore":
			return diagnostic.Diagnostic{}, false
		}
	case "database.querylocatoriterator":
		name = "Database.QueryLocatorIterator"
		switch normalizeName(method) {
		case "hasnext":
			method, params = "hasNext", [][]string{{}}
		case "next":
			method, params = "next", [][]string{{}}
		case "hashcode", "equals", "tostring", "clone":
			return diagnostic.Diagnostic{}, false
		}
	default:
		return diagnostic.Diagnostic{}, false
	}
	message := ""
	if name == "Database" && method == "getQueryLocator" && len(args) == 1 && strings.EqualFold(args[0], "null") {
		message = "Ambiguous method signature: void getQueryLocator(NULL)"
	} else {
		for _, candidate := range params {
			if len(candidate) != len(args) {
				continue
			}
			matches := true
			for i, arg := range args {
				if arg == "" || strings.EqualFold(arg, "null") {
					continue
				}
				actual := semaCanonicalPlatformAlias(arg)
				switch candidate[i] {
				case "Integer":
					// Ranges are Integer-only; ordinary numeric widening cannot
					// admit the Long/Decimal overloads rejected by C016-C018.
					matches = matches && strings.EqualFold(actual, "Integer")
				default:
					matches = matches && semaArgsMatchAny([][]string{{candidate[i]}}, []string{arg}, model)
				}
			}
			if matches {
				return diagnostic.Diagnostic{}, false
			}
		}
		display := make([]string, len(args))
		for i, arg := range args {
			display[i] = semaCanonicalPlatformAlias(arg)
			if strings.EqualFold(arg, "null") {
				display[i] = "NULL"
			}
		}
		message = "Method does not exist or incorrect signature: void " + method + "(" + strings.Join(display, ", ") + ") from the type " + name
	}
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA009",
		Message:  message,
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}, true
}
