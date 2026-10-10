package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

const visualforceControllerAssignmentCode = "GLADESEMA_VF018"

// IsVisualforceControllerDiagnostic identifies captured controller compile
// failures. The VF loader can reject the dependent page without treating
// unrelated, partially modeled Apex diagnostics as failures.
func IsVisualforceControllerDiagnostic(diag diagnostic.Diagnostic) bool {
	if diag.Severity != diagnostic.Error {
		return false
	}
	if diag.Code == "GLADESEMA_VF001" || diag.Code == visualforceControllerAssignmentCode {
		return true
	}
	// Captured class failures make the referenced page's controller
	// unavailable. Keep this list confined to the measured compiler errors.
	switch diag.Code {
	case "GLADESEMA032":
		if diag.Message == "transient is not allowed on methods" || diag.Message == "transient is not allowed on classes" {
			return true
		}
	case "GLADESEMA027":
		if strings.HasPrefix(diag.Message, "Final members can only be assigned in their declaration, init blocks, or constructors: ") {
			return true
		}
	case "GLADESEMA002":
		if strings.HasPrefix(diag.NativeMessage, "Invalid type: ") {
			return true
		}
	case "GLADESEMA018":
		if diag.NativeMessage == "Illegal assignment from String to Integer" {
			return true
		}
	case "APEXPARSE001":
		if diag.Message == "Invalid type: transient" {
			return true
		}
	}
	return diag.Code == "GLADESEMA023" && strings.HasPrefix(diag.Message, "Method does not exist or incorrect signature: void ") &&
		(strings.HasSuffix(diag.Message, " from the type System.PageReference") || strings.HasSuffix(diag.Message, " from the type Map<String,String>") ||
			strings.HasSuffix(diag.Message, " from the type ApexPages.StandardController") ||
			strings.HasSuffix(diag.Message, " from the type ApexPages.StandardSetController") || strings.HasSuffix(diag.Message, " from the type System.SelectOption"))
}

func visualforceControllerDiagnostic(typ typesys.TypeSymbol, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA_VF001",
		Message:  message,
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func semaVisualforcePlatformType(model *semaTypeMemberView, typeName string) bool {
	if semaProjectTypeShadowsPlatform(model, typeName) {
		return false
	}
	canonical := semaCanonicalPlatformAlias(typeName)
	if strings.HasPrefix(strings.ToLower(canonical), "apexpages.") && semaProjectTypeShadowsPlatform(model, "ApexPages") {
		return false
	}
	members, ok := model.lookup(normalizeName(canonical))
	return ok && members.platform
}

// Native c_diagnostic_unknown_severity rejects a missing enum constant. The
// five declared constants are exercised by the native message_* controls.
func semaVisualforceSeverityDiagnostic(typ typesys.TypeSymbol, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, start int, source string) (diagnostic.Diagnostic, bool) {
	path, ok := semaIRExprTypeReceiverPath(expr)
	if !ok {
		return diagnostic.Diagnostic{}, false
	}
	dot := strings.LastIndexByte(path, '.')
	if dot < 0 || !strings.EqualFold(semaCanonicalPlatformAlias(path[:dot]), "ApexPages.Severity") {
		return diagnostic.Diagnostic{}, false
	}
	root, _, _ := strings.Cut(path, ".")
	if scope.hasNonFieldBinding(root) || !semaVisualforcePlatformType(model, path[:dot]) {
		return diagnostic.Diagnostic{}, false
	}
	field := path[dot+1:]
	if strings.EqualFold(field, "class") {
		return diagnostic.Diagnostic{}, false
	}
	if _, exists := semaResolveFieldPath(model, "ApexPages.Severity", field); exists {
		return diagnostic.Diagnostic{}, false
	}
	return visualforceControllerDiagnostic(typ, "Variable does not exist: "+field, start, start+max(1, len(path)), source), true
}

// Captured Visualforce platform constructor failures enter this path; ordinary
// constructors, overload selection and visibility use existing code.
func semaVisualforceConstructorDiagnostic(typ typesys.TypeSymbol, typeName string, argTypes []string, model *semaTypeMemberView, start int, source string) (diagnostic.Diagnostic, bool) {
	canonical := semaCanonicalPlatformAlias(typeName)
	if !strings.EqualFold(canonical, "PageReference") && !strings.EqualFold(canonical, "ApexPages.Message") && !strings.EqualFold(canonical, "ApexPages.StandardController") && !strings.EqualFold(canonical, "ApexPages.StandardSetController") && !strings.EqualFold(canonical, "SelectOption") {
		return diagnostic.Diagnostic{}, false
	}
	if !semaVisualforcePlatformType(model, typeName) || semaArgTypesContainUnknown(argTypes) {
		return diagnostic.Diagnostic{}, false
	}
	message := ""
	switch strings.ToLower(canonical) {
	case "pagereference":
		if len(argTypes) == 1 && strings.EqualFold(argTypes[0], "null") {
			message = "Ambiguous method signature: void <init>(NULL)"
		}
	case "apexpages.message":
		if params, ok := semaPlatformConstructorSignatures(canonical); ok && !semaArgsMatchAny(params, argTypes, model) {
			message = "Constructor not defined: [ApexPages.Message].<Constructor>(" + semaVisualforceArgumentTypes(argTypes) + ")"
		}
	case "apexpages.standardcontroller":
		if params, ok := semaPlatformConstructorSignatures(canonical); ok && !semaArgsMatchAny(params, argTypes, model) {
			message = "Constructor not defined: [ApexPages.StandardController].<Constructor>(" + semaVisualforceArgumentTypes(argTypes) + ")"
		}
	case "selectoption":
		if params, ok := semaPlatformConstructorSignatures(canonical); ok && !semaArgsMatchAny(params, argTypes, model) {
			message = "Constructor not defined: [System.SelectOption].<Constructor>(" + semaVisualforceArgumentTypes(argTypes) + ")"
		}
	case "apexpages.standardsetcontroller":
		if params, ok := semaPlatformConstructorSignatures(canonical); ok && !semaArgsMatchAny(params, argTypes, model) {
			message = "Constructor not defined: [ApexPages.StandardSetController].<Constructor>(" + semaVisualforceArgumentTypes(argTypes) + ")"
		}
	}
	if message == "" {
		return diagnostic.Diagnostic{}, false
	}
	return visualforceControllerDiagnostic(typ, message, start, start+max(1, len(typeName)), source), true
}

func (a *Analyzer) checkIRVisualforceCall(typ typesys.TypeSymbol, expr ir.Expr, receiverType, method, receiverMode string, scope irSemaScope, model *semaTypeMemberView, start int, source string) (diagnostic.Diagnostic, bool) {
	canonical := semaCanonicalPlatformAlias(receiverType)
	pageReference := strings.EqualFold(canonical, "PageReference") && semaVisualforcePlatformType(model, receiverType)
	standardController := strings.EqualFold(canonical, "ApexPages.StandardController") && semaVisualforcePlatformType(model, receiverType)
	selectOption := strings.EqualFold(canonical, "SelectOption") && semaVisualforcePlatformType(model, receiverType)
	standardSetController := strings.EqualFold(canonical, "ApexPages.StandardSetController") && semaVisualforcePlatformType(model, receiverType)
	parameterMap := strings.EqualFold(canonical, "Map<String,String>") && strings.EqualFold(method, "put") &&
		a.irVisualforceParameterMap(expr.Left, scope, model, typ.Name)
	if !pageReference && !standardController && !standardSetController && !parameterMap && !selectOption {
		return diagnostic.Diagnostic{}, false
	}
	argTypes := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
	if semaArgTypesContainUnknown(argTypes) {
		return diagnostic.Diagnostic{}, false
	}
	target := "System.PageReference"
	if selectOption {
		target = "System.SelectOption"
	}
	if parameterMap {
		sig, ok := semaCollectionMethodSignature(receiverType, method)
		if !ok || semaArgsMatchAny(sig.params, argTypes, model) {
			return diagnostic.Diagnostic{}, false
		}
		target = "Map<String,String>"
	} else {
		sig, known := semaPlatformMethodSignatureForMode(model, receiverType, method, receiverMode)
		if standardSetController && known && strings.EqualFold(method, "setSelected") {
			// Native controller calls reject List<Integer>, while Account, Contact and
			// SObject lists remain valid. Idea controllers accept List<Object>.
			sig.params = [][]string{{"List<SObject>"}}
		}
		if known && semaArgsMatchAny(sig.params, argTypes, model) {
			return diagnostic.Diagnostic{}, false
		}
		// Preserve existing handling of other declared platform methods, including
		// the getContent/getContentAsPDF boundary.
		if !known && len(resolveMemberMethods(model, receiverType, method)) != 0 {
			return diagnostic.Diagnostic{}, false
		}
		if standardController {
			if !known {
				return diagnostic.Diagnostic{}, false
			}
			target = "ApexPages.StandardController"
		}
		if standardSetController {
			if !known {
				return diagnostic.Diagnostic{}, false
			}
			target = "ApexPages.StandardSetController"
		}
	}
	message := fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, semaVisualforceArgumentTypes(argTypes), target)
	diag := visualforceControllerDiagnostic(typ, message, start, start+max(1, len(method)), source)
	// Retain the call diagnostic code so the existing IR/text duplicate removal
	// keeps the resolved IR message instead of a second text-scanner message.
	diag.Code = "GLADESEMA023"
	return diag, true
}

// Native diagnostics format rejected assignments from
// StandardSetController calls, and reject the dependent Visualforce page.
// Other ApexPages types and ordinary return assignments keep their own rules.
func (a *Analyzer) semaVisualforceSetControllerAssignment(typ typesys.TypeSymbol, targetType, valueType string, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, start, end int, source string) (diagnostic.Diagnostic, bool) {
	if expr.Kind != ir.ExprCall {
		return diagnostic.Diagnostic{}, false
	}
	receiverType := ""
	if expr.Left != nil {
		receiverType = a.inferIRExprType(*expr.Left, scope, model, typ.Name)
	} else if receiver, _, ok := splitSemaMethodPath(expr.Callee); ok {
		receiverType = semaIRReceiverType(receiver, scope, model, typ.Name)
	}
	message, rejected := semaVisualforceSetControllerAssignmentMessage(receiverType, targetType, valueType, model)
	if !rejected {
		return diagnostic.Diagnostic{}, false
	}
	return visualforceControllerDiagnostic(typ, message, start, end, source), true
}

func semaVisualforceSetControllerAssignmentMessage(receiverType, targetType, valueType string, model *semaTypeMemberView) (string, bool) {
	if !strings.EqualFold(semaCanonicalPlatformAlias(receiverType), "ApexPages.StandardSetController") || !semaVisualforcePlatformType(model, receiverType) {
		return "", false
	}
	if strings.EqualFold(semaCanonicalPlatformAlias(valueType), "PageReference") {
		valueType = "System.PageReference"
	}
	message := "Illegal assignment from " + valueType + " to " + semaCanonicalPlatformAlias(targetType)
	return message, true
}

// Local initializers are checked by both the text and IR passes. Format the
// same StandardSetController rejection at both entries so deduplication retains
// one native diagnostic without hiding unrelated assignment failures.
func semaVisualforceLocalAssignmentDiagnostics(diagnostics []diagnostic.Diagnostic, currentType, body string, bodyOffset int, scopes semaScopeModel, model *semaTypeMemberView, matches [][]int) []diagnostic.Diagnostic {
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
			base, calls, ok := splitSemaCallChain(value.text)
			if !ok || len(calls) != 1 {
				break
			}
			bindings := scopes.flatAtCopy(value.start)
			receiverType := semaTextReceiverType(base, bindings, model)
			targetType := resolveNestedTypeReference(model, currentType, strings.TrimSpace(body[match[2]:match[3]]))
			valueType := semaResolveConstructedExpressionType(model, currentType, value.text, bindings)
			if message, rejected := semaVisualforceSetControllerAssignmentMessage(receiverType, targetType, valueType, model); rejected {
				item.Code, item.Message = "GLADESEMA_VF001", message
			}
			break
		}
	}
	return diagnostics
}

// Native diagnostics report absent fields on the direct getRecord() result.
// Known fields and ordinary generic SObject receivers keep existing handling.
func (a *Analyzer) semaVisualforceSetControllerField(typ typesys.TypeSymbol, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, start int, source string) (diagnostic.Diagnostic, bool) {
	if expr.Left == nil || expr.Left.Kind != ir.ExprCall {
		return diagnostic.Diagnostic{}, false
	}
	field := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(expr.Callee, "__assignField:"), "__safe_field:"), "__field:")
	if field == expr.Callee || semaAnyKnownField(model, field) {
		return diagnostic.Diagnostic{}, false
	}
	call := *expr.Left
	method, receiverType := strings.TrimPrefix(call.Callee, "__safe_call:"), ""
	if receiver, name, ok := splitSemaMethodPath(method); ok {
		method = name
		receiverType = semaIRReceiverType(receiver, scope, model, typ.Name)
	}
	if call.Left != nil {
		receiverType = a.inferIRExprType(*call.Left, scope, model, typ.Name)
	}
	if len(call.Args) != 0 || !strings.EqualFold(method, "getRecord") || !strings.EqualFold(semaCanonicalPlatformAlias(receiverType), "ApexPages.StandardSetController") || !semaVisualforcePlatformType(model, receiverType) {
		return diagnostic.Diagnostic{}, false
	}
	return visualforceControllerDiagnostic(typ, "Variable does not exist: "+field, start, start+max(1, len(field)), source), true
}

// This diagnostic formatting applies to the direct getParameters() result,
// not to an unrelated Map<String,String> receiver.
func (a *Analyzer) irVisualforceParameterMap(expr *ir.Expr, scope irSemaScope, model *semaTypeMemberView, owner string) bool {
	if expr == nil || expr.Kind != ir.ExprCall {
		return false
	}
	method := strings.TrimPrefix(expr.Callee, "__safe_call:")
	if dot := strings.LastIndexByte(method, '.'); dot >= 0 {
		method = method[dot+1:]
	}
	if !strings.EqualFold(method, "getParameters") {
		return false
	}
	receiver := ""
	if expr.Left != nil {
		receiver = a.inferIRExprType(*expr.Left, scope, model, owner)
	} else if path, _, ok := splitSemaMethodPath(expr.Callee); ok {
		receiver = semaIRReceiverType(path, scope, model, owner)
	}
	return strings.EqualFold(semaCanonicalPlatformAlias(receiver), "PageReference") && semaVisualforcePlatformType(model, receiver)
}

func semaVisualforceArgumentTypes(argTypes []string) string {
	types := make([]string, len(argTypes))
	for i, argType := range argTypes {
		if strings.EqualFold(argType, "null") {
			types[i] = "NULL"
		} else {
			types[i] = semaCanonicalPlatformAlias(argType)
		}
	}
	return strings.Join(types, ", ")
}

// Only direct, resolved StandardController calls use these captured assignment
// diagnostics. Other SObject, Id, void and PageReference expressions retain
// their existing type checks and diagnostic formatting.
func (a *Analyzer) irStandardControllerCall(expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, owner string) (string, bool) {
	if expr.Kind != ir.ExprCall || len(expr.Args) != 0 || len(expr.NamedArgs) != 0 {
		return "", false
	}
	method := strings.TrimPrefix(expr.Callee, "__safe_call:")
	receiver := ""
	if expr.Left != nil {
		receiver = a.inferIRExprType(*expr.Left, scope, model, owner)
	} else if path, name, ok := splitSemaMethodPath(method); ok {
		receiver = semaIRReceiverType(path, scope, model, owner)
		method = name
	}
	if !strings.EqualFold(semaCanonicalPlatformAlias(receiver), "ApexPages.StandardController") ||
		!semaVisualforcePlatformType(model, receiver) || semaIRCallReceiverMode(expr, scope, model) == "class" {
		return "", false
	}
	return strings.ToLower(method), true
}

func standardControllerAssignmentMessage(targetType, valueType, method string) (string, bool) {
	switch method {
	case "getrecord", "getid", "reset", "save", "cancel", "edit", "delete", "view":
	default:
		return "", false
	}
	valueType = semaCanonicalPlatformAlias(valueType)
	if method == "getid" {
		// Native e_getId_cast reports String, although the local signature is Id.
		valueType = "String"
	} else if strings.EqualFold(valueType, "PageReference") {
		valueType = "System.PageReference"
	}
	return "Illegal assignment from " + valueType + " to " + semaCanonicalPlatformAlias(targetType), true
}

func semaTextStandardControllerAssignmentMessage(targetType, valueType, value string, scope map[string]string, model *semaTypeMemberView) (string, bool) {
	receiver, method, args, ok := splitLastSemaCall(strings.TrimSpace(value))
	if !ok || len(args) != 0 || semaTextReceiverExprLooksLikeType(receiver, scope, model) {
		return "", false
	}
	receiverType := semaTextReceiverType(receiver, scope, model)
	if !strings.EqualFold(semaCanonicalPlatformAlias(receiverType), "ApexPages.StandardController") || !semaVisualforcePlatformType(model, receiverType) {
		return "", false
	}
	return standardControllerAssignmentMessage(targetType, valueType, strings.ToLower(method))
}

// The getRecord result is the declared SObject type, not a statically typed
// Account. Captured e_getRecord_member rejects a missing member on that result;
// explicitly cast records continue through the ordinary SObject field rules.
func (a *Analyzer) irStandardControllerRecordMemberDiagnostic(typ typesys.TypeSymbol, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, start int, source string) (diagnostic.Diagnostic, bool) {
	if expr.Kind != ir.ExprCall || expr.Left == nil ||
		(!strings.HasPrefix(expr.Callee, "__field:") && !strings.HasPrefix(expr.Callee, "__safe_field:")) {
		return diagnostic.Diagnostic{}, false
	}
	method, standard := a.irStandardControllerCall(*expr.Left, scope, model, typ.Name)
	if !standard || method != "getrecord" {
		return diagnostic.Diagnostic{}, false
	}
	field := strings.TrimPrefix(strings.TrimPrefix(expr.Callee, "__safe_field:"), "__field:")
	if _, exists := semaResolveFieldPath(model, "SObject", field); exists {
		return diagnostic.Diagnostic{}, false
	}
	return visualforceControllerDiagnostic(typ, "Variable does not exist: "+field, start, start+max(1, len(field)), source), true
}
