package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// Messaging C001-C022 capture the native rejection text at both source APIs.
// Restrict these contracts to the resolved platform type: a source declaration
// that shadows Messaging or RichMessaging keeps its ordinary semantics.
func semaMessagingTypeName(typeName string, model *semaTypeMemberView) string {
	typeName = semaCanonicalPlatformAlias(typeName)
	lower := strings.ToLower(typeName)
	if lower != "messaging" && !strings.HasPrefix(lower, "messaging.") && !strings.HasPrefix(lower, "richmessaging.") {
		return ""
	}
	for _, name := range []string{
		"Messaging", "Messaging.SingleEmailMessage", "Messaging.MassEmailMessage",
		"Messaging.EmailFileAttachment", "Messaging.SendEmailResult", "Messaging.SendEmailError",
		"Messaging.InboundEmail", "Messaging.InboundEmailHandler", "Messaging.InboundEmailResult",
		"Messaging.CustomNotification", "RichMessaging.AuthRequestHandler",
		"RichMessaging.MessageDefinitionInputParameter", "RichMessaging.TimeSlotOption",
	} {
		if strings.EqualFold(typeName, name) {
			if members, _, found := semaLookupTypeMembers(model, typeName); found && !members.platform {
				return ""
			}
			return name
		}
	}
	return ""
}

func semaMessagingDiagnostic(typ typesys.TypeSymbol, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA035", Message: message, File: typ.File, Range: semaRange(source, start, end)}
}

func semaMessagingInvisibleType(typeName, api string, model *semaTypeMemberView) string {
	for _, name := range []string{"RichMessaging.ProcessFormHandler", "RichMessaging.CatalogSection"} {
		if strings.EqualFold(typeName, name) && semaPlatformTypeUnavailable(api, name) {
			if members, _, found := semaLookupTypeMembers(model, typeName); found && !members.platform {
				return ""
			}
			return name
		}
	}
	return ""
}

func semaMessagingVisibilityDiagnostics(diagnostics []diagnostic.Diagnostic, api string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	for i := range diagnostics {
		// C019/R261/R262 at the floor: both missing-type and version-gate
		// diagnostics report the captured native visibility error.
		if diagnostics[i].Code != "GLADESEMA006" && diagnostics[i].Code != "GLADESEMA008" && diagnostics[i].Code != "GLADESEMA028" {
			continue
		}
		for _, name := range []string{"RichMessaging.ProcessFormHandler", "RichMessaging.CatalogSection"} {
			if semaMessagingInvisibleType(name, api, model) != "" && strings.Contains(strings.ToLower(diagnostics[i].Message), strings.ToLower(name)) {
				diagnostics[i].Message = "Type is not visible: " + name
			}
		}
	}
	return diagnostics
}

func semaMessagingMethodDiagnostic(typ typesys.TypeSymbol, receiver, method string, argTypes []string, model *semaTypeMemberView, start, end int, source string) (diagnostic.Diagnostic, bool) {
	receiver = semaMessagingTypeName(receiver, model)
	var params [][]string
	canonicalMethod := ""
	switch receiver {
	case "Messaging":
		switch normalizeName(method) {
		case "sendemail":
			canonicalMethod = "sendEmail"
			params = [][]string{{"List<Messaging.Email>"}, {"List<Messaging.Email>", "Boolean"}, {"List<Messaging.Email>", "Messaging.SendEmailOptions"}}
		case "renderstoredemailtemplate":
			canonicalMethod = "renderStoredEmailTemplate"
			params = [][]string{{"Id", "Id", "Id"}, {"Id", "Id", "Id", "Messaging.AttachmentRetrievalOption"}, {"Id", "Id", "Id", "Messaging.AttachmentRetrievalOption", "Boolean"}}
		}
	case "Messaging.SingleEmailMessage":
		switch normalizeName(method) {
		case "settoaddresses", "setccaddresses", "setbccaddresses":
			params = [][]string{{"List<String>"}}
		case "setsaveasactivity":
			params = [][]string{{"Boolean"}}
		case "settargetobjectid":
			params = [][]string{{"Id"}}
		case "setfileattachments":
			params = [][]string{{"List<Messaging.EmailFileAttachment>"}}
		}
	case "Messaging.MassEmailMessage":
		if strings.EqualFold(method, "setPlainTextBody") {
			canonicalMethod = "setPlainTextBody" // not a MassEmailMessage member
			params = [][]string{}
		}
	case "Messaging.EmailFileAttachment":
		if strings.EqualFold(method, "setBody") {
			canonicalMethod = "setBody"
			params = [][]string{{"Blob"}}
		}
	case "Messaging.CustomNotification":
		if strings.EqualFold(method, "send") {
			canonicalMethod = "send"
			params = [][]string{{"Set<String>"}}
		}
	}
	if params == nil || semaArgsMatchAny(params, argTypes, model) {
		return diagnostic.Diagnostic{}, false
	}
	if canonicalMethod == "" {
		for _, name := range []string{"setToAddresses", "setCcAddresses", "setBccAddresses", "setSaveAsActivity", "setTargetObjectId", "setFileAttachments"} {
			if strings.EqualFold(method, name) {
				canonicalMethod = name
				break
			}
		}
	}
	names := append([]string(nil), argTypes...)
	for i, name := range names {
		if strings.EqualFold(name, "null") {
			names[i] = "NULL"
		}
	}
	message := fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", canonicalMethod, strings.Join(names, ", "), receiver)
	return semaMessagingDiagnostic(typ, message, start, end, source), true
}

func semaMessagingConstructorDiagnostic(typ typesys.TypeSymbol, typeName string, argTypes []string, model *semaTypeMemberView, start, end int, source string) (diagnostic.Diagnostic, bool) {
	if hidden := semaMessagingInvisibleType(typeName, typ.EffectiveAPIVersion, model); hidden != "" {
		return semaMessagingDiagnostic(typ, "Type is not visible: "+hidden, start, end, source), true
	}
	name := semaMessagingTypeName(typeName, model)
	switch name {
	case "Messaging.SendEmailResult", "Messaging.SendEmailError", "Messaging.InboundEmailHandler", "RichMessaging.AuthRequestHandler":
		return semaMessagingDiagnostic(typ, "Type cannot be constructed: "+name, start, end, source), true
	case "RichMessaging.TimeSlotOption":
		if !semaArgsMatchAny([][]string{{"Datetime", "Integer"}}, argTypes, model) {
			return semaMessagingDiagnostic(typ, "Constructor not defined: ["+name+"].<Constructor>("+strings.Join(argTypes, ", ")+")", start, end, source), true
		}
	}
	return diagnostic.Diagnostic{}, false
}

func semaMessagingAssignmentMessage(targetType, valueType, receiverType string, model *semaTypeMemberView) string {
	for _, name := range []string{targetType, valueType, receiverType} {
		if semaMessagingTypeName(name, model) != "" {
			return "Illegal assignment from " + valueType + " to " + targetType
		}
	}
	return ""
}

func semaMessagingLocalAssignmentDiagnostics(diagnostics []diagnostic.Diagnostic, currentType, body string, bodyOffset int, scopes semaScopeModel, model *semaTypeMemberView, matches [][]int) []diagnostic.Diagnostic {
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
			if message := semaMessagingAssignmentMessage(targetType, valueType, "", model); message != "" {
				item.Message = message
			}
			break
		}
	}
	return diagnostics
}
