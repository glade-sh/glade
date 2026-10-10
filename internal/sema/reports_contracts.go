package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// Reports/Wave diagnostics use the native namespace spelling. A source type
// shadowing either platform namespace keeps the source-defined type rules.
func semaReportsTypeName(typeName string, model *semaTypeMemberView) (string, bool) {
	key := normalizeName(typeName)
	if !strings.HasPrefix(key, "reports.") && !strings.HasPrefix(key, "wave.") {
		return "", false
	}
	members, ok := model.lookup(key)
	if !ok || !members.platform {
		return "", false
	}
	namespace, _, _ := strings.Cut(key, ".")
	name := members.name
	if strings.HasPrefix(normalizeName(name), namespace+".") {
		name = name[len(namespace)+1:]
	}
	return namespace + "." + name, true
}

func semaReportsDiagnosticType(typeName string, model *semaTypeMemberView) string {
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) != 0 {
		for i, arg := range args {
			args[i] = semaReportsDiagnosticType(arg, model)
		}
		return semaCanonicalPlatformAlias(base) + "<" + strings.Join(args, ",") + ">"
	}
	if name, ok := semaReportsTypeName(typeName, model); ok {
		return name
	}
	return semaCanonicalPlatformAlias(typeName)
}

func semaReportsRelatedType(typeName string, model *semaTypeMemberView) bool {
	if _, ok := semaReportsTypeName(typeName, model); ok {
		return true
	}
	_, args := semaGenericBaseAndArgs(typeName)
	for _, arg := range args {
		if semaReportsRelatedType(arg, model) {
			return true
		}
	}
	return false
}

func semaReportsDiagnostic(typ typesys.TypeSymbol, code, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: code, Message: message, File: typ.File, Range: semaRange(source, start, end)}
}

// C027-C037 and K048-K052: enforce the catalog signature before the general
// dependency-type fallback, which otherwise accepts absent platform methods.
func semaReportsCallDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, argTypes []string, receiverMode string, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	name, reports := semaReportsTypeName(receiverType, model)
	if !reports || semaArgTypesContainUnknown(argTypes) {
		return diagnostic.Diagnostic{}, false
	}
	candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, receiverType, method), receiverMode)
	if len(candidates) != 0 {
		method = candidates[0].member.Name
	}
	if sig, ok := semaPlatformMethodSignatureForMode(model, receiverType, method, receiverMode); ok && semaArgsMatchAny(sig.params, argTypes, model) {
		if candidate, matched, _ := bestResolvedMemberByArgTypes(candidates, argTypes, model); matched {
			if _, blocked := checkSemaMemberAccess(typ, member, method, candidate, start, end, source, model); blocked {
				params := make([]string, len(candidate.member.Parameters))
				for i, param := range candidate.member.Parameters {
					params[i] = semaReportsDiagnosticType(param.Type, model)
				}
				message := "Method is not visible: " + candidate.member.Type + " " + name + "." + candidate.member.Name + "(" + strings.Join(params, ", ") + ")"
				return semaReportsDiagnostic(typ, "GLADESEMA010", message, start, end, source), true
			}
		}
		return diagnostic.Diagnostic{}, false
	}
	// Keep the existing class/instance access diagnostic when the signature is
	// valid for the other receiver mode; this does not change lifecycle rules.
	if _, matched, _ := bestResolvedMemberByArgTypes(candidates, argTypes, model); matched {
		return diagnostic.Diagnostic{}, false
	}
	displayTypes := make([]string, len(argTypes))
	for i, argType := range argTypes {
		displayTypes[i] = semaReportsDiagnosticType(argType, model)
	}
	message := "Method does not exist or incorrect signature: void " + method + "(" + strings.Join(displayTypes, ", ") + ") from the type " + name
	return semaReportsDiagnostic(typ, "GLADESEMA009", message, start, end, source), true
}

func semaReportsAssignmentMessage(targetType, valueType string, model *semaTypeMemberView) (string, bool) {
	if !semaReportsRelatedType(targetType, model) && !semaReportsRelatedType(valueType, model) {
		return "", false
	}
	return "Illegal assignment from " + semaReportsDiagnosticType(valueType, model) + " to " + semaReportsDiagnosticType(targetType, model), true
}

func semaReportsCastMessage(targetType, valueType string, model *semaTypeMemberView) (string, bool) {
	if !semaReportsRelatedType(targetType, model) && !semaReportsRelatedType(valueType, model) {
		return "", false
	}
	return "Incompatible types since an instance of " + semaReportsDiagnosticType(valueType, model) + " is never an instance of " + semaReportsDiagnosticType(targetType, model), true
}
