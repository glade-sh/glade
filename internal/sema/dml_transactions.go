package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// Qualified names can resolve to project-owned nested types. Apply
// DML platform restrictions only to the symbol selected by normal resolution.
func semaDMLPlatformType(model *semaTypeMemberView, typeName string) bool {
	members, _, ok := semaLookupTypeMembers(model, typeName)
	return ok && members.platform
}

// Database DTOs have a closed public field surface even though their captured
// symbols are dependencies.
func semaDMLDTOType(typeName string) bool {
	switch normalizeName(semaCanonicalPlatformAlias(typeName)) {
	case "database.dmloptions", "database.assignmentruleheader", "database.duplicateruleheader", "database.emailheader", "database.localeoptions",
		"database.saveresult", "database.upsertresult", "database.deleteresult", "database.undeleteresult", "database.mergeresult", "database.error", "database.leadconvert", "database.leadconvertresult":
		return true
	}
	return false
}

func semaDMLMissingField(model *semaTypeMemberView, receiverType, path string) string {
	for _, field := range strings.Split(path, ".") {
		if !semaDMLDTOType(receiverType) || !semaDMLPlatformType(model, receiverType) {
			return ""
		}
		target, ok := semaResolveFieldPath(model, receiverType, field)
		if !ok {
			return field
		}
		receiverType = target.member.Type
	}
	return ""
}

func semaDMLAssignmentMessage(targetType, valueType, receiverType string, model *semaTypeMemberView) string {
	for _, typeName := range []string{targetType, valueType, receiverType} {
		if semaDMLDTOType(typeName) && semaDMLPlatformType(model, typeName) {
			return "Illegal assignment from " + valueType + " to " + targetType
		}
	}
	return ""
}

// Named bodies also collect local-initializer diagnostics before
// the IR checks. Format the same rejected DML assignment at that entry point.
func semaDMLLocalAssignmentDiagnostics(diagnostics []diagnostic.Diagnostic, currentType, body string, bodyOffset int, scopes semaScopeModel, model *semaTypeMemberView, matches [][]int) []diagnostic.Diagnostic {
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
			valueType := semaResolveConstructedExpressionType(model, currentType, value.text, scopes.flatAtCopy(value.start))
			if message := semaDMLAssignmentMessage(targetType, valueType, "", model); message != "" {
				item.Message = message
			}
			break
		}
	}
	return diagnostics
}

func semaDMLDiagnostic(typ typesys.TypeSymbol, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA034", Message: message, File: typ.File, Range: semaRange(source, start, end)}
}

func semaDMLNonconstructibleType(typeName string) string {
	typeName = semaCanonicalPlatformAlias(typeName)
	if strings.EqualFold(typeName, "Savepoint") || strings.EqualFold(typeName, "System.Savepoint") {
		return "System.Savepoint"
	}
	// Other constructor surfaces are unchanged.
	for _, name := range []string{"Database.SaveResult", "Database.UpsertResult", "Database.Error"} {
		if strings.EqualFold(typeName, name) {
			return name
		}
	}
	return ""
}

func (a *Analyzer) semaDMLCallMessage(receiverType, method string, args []ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) string {
	canonicalReceiverType := semaCanonicalPlatformAlias(receiverType)
	switch normalizeName(canonicalReceiverType) {
	case "database", "database.leadconvert", "datacloud.findduplicates":
	default:
		return ""
	}
	if !semaDMLPlatformType(model, receiverType) {
		return ""
	}
	receiverType = canonicalReceiverType
	argTypes := irCallArgTypes(a, args, scope, model, currentType)
	rejected := false
	switch normalizeName(receiverType) {
	case "database":
		switch normalizeName(method) {
		case "insert", "insertasync", "insertimmediate":
			// The native boundary depends on the
			// static event type. Erased SObject inserts remain valid (W005/W006).
			if len(argTypes) > 0 && semaPlatformEventType(semaDMLObjectType(argTypes[0]), model) {
				if strings.EqualFold(method, "insert") {
					return "Argument must be of internal sObject type. use insertAsync() or insertImmediate() instead"
				}
				return "Argument must be of virtual sObject type."
			}
		case "rollback", "releasesavepoint":
			rejected = !semaArgsMatchAny([][]string{{"Savepoint"}}, argTypes, model)
		case "upsert":
			// The external-field overloads
			// require SObject inputs even though the older fallback accepts Object.
			if semaDatabaseUpsertObjectFieldCall(receiverType, method, argTypes, model) {
				rejected = true
				for i, argType := range argTypes {
					if strings.EqualFold(semaCanonicalPlatformAlias(argType), "AccessLevel") {
						argTypes[i] = "System.AccessLevel"
					}
				}
				break
			}
			// Generic SObject operands defer
			// selector capability checks to runtime, where the record type is known.
			if len(args) < 2 || args[1].Kind != ir.ExprVariable || !strings.EqualFold(semaCanonicalPlatformAlias(argTypes[1]), "Schema.SObjectField") {
				break
			}
			if strings.EqualFold(semaDMLObjectType(semaCanonicalAssignableType(argTypes[0])), "SObject") {
				break
			}
			if _, _, qualified := strings.Cut(args[1].Name, "."); qualified && !a.semaUpsertSelectorAllowed(argTypes[0], args[1].Name) {
				_, field, _ := strings.Cut(args[1].Name, ".")
				return "Invalid field for upsert, must be an External Id custom or standard indexed field: " + field
			}
		}
	case "database.leadconvert":
		switch normalizeName(method) {
		case "getdonotcreateopportunity", "getoverwriteleadsource", "getsendnotificationemail":
			// R213-R215/R217-R225: native Boolean accessors use is, not get.
			rejected = true
		case "setleadid":
			rejected = !semaArgsMatchAny([][]string{{"Id"}}, argTypes, model)
		case "setdonotcreateopportunity":
			rejected = !semaArgsMatchAny([][]string{{"Boolean"}}, argTypes, model)
		}
	case "datacloud.findduplicates":
		if strings.EqualFold(method, "findDuplicates") {
			rejected = !semaArgsMatchAny([][]string{{"List<SObject>"}}, argTypes, model)
		}
	}
	if rejected {
		return fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(argTypes, ", "), receiverType)
	}
	return ""
}
