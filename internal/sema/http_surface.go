package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// These API 62/67 diagnostics come from captured HTTP rejection cases. Keep
// them at the platform boundary so generated symbols and chained calls agree.
func semaHTTPReceiver(typeName string) string {
	typeName = semaCanonicalPlatformAlias(typeName)
	for _, name := range []string{"Http", "HttpRequest", "HttpResponse", "RestRequest", "RestResponse", "Continuation", "WebServiceCallout"} {
		if strings.EqualFold(typeName, name) {
			return name
		}
	}
	return ""
}

func semaHTTPInvisibleType(typeName string) string {
	for _, name := range []string{"commercetax.TaxEngineContext", "functions.Function"} {
		if strings.EqualFold(strings.TrimSpace(typeName), name) {
			return name
		}
	}
	return ""
}

func semaHTTPReferencedInvisibleType(reference string, model *semaTypeMemberView) string {
	if strings.HasSuffix(strings.ToLower(reference), ".class") {
		reference = reference[:len(reference)-len(".class")]
	}
	if semaProjectTypeShadowsPlatform(model, reference) {
		return ""
	}
	return semaHTTPInvisibleType(reference)
}

func semaHTTPFieldName(path string, assignment bool) string {
	receiver, field, ok := splitSemaMethodPath(path)
	if !ok {
		return ""
	}
	receiver = semaHTTPReceiver(receiver)
	if !assignment && receiver == "Continuation" && strings.EqualFold(field, "state") {
		return "System.Continuation.state"
	}
	if assignment && receiver == "RestRequest" {
		for _, name := range []string{"headers", "params"} {
			if strings.EqualFold(field, name) {
				return "System.RestRequest." + name
			}
		}
	}
	return ""
}

func semaHTTPDiagnostic(typ typesys.TypeSymbol, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA028", Message: message, File: typ.File, Range: semaRange(source, start, end)}
}

// C029/C030/C032: render the resolved platform mock requirement, including
// its return type and parameters. Project interfaces retain their diagnostics.
func semaHTTPRequiredMethodMessage(className string, requirement methodRequirement, model *semaTypeMemberView) (string, bool) {
	if requirement.sourceKind != "interface" {
		return "", false
	}
	owner := semaCanonicalPlatformAlias(requirement.owner)
	method := ""
	switch owner {
	case "HttpCalloutMock":
		method = "respond"
	case "WebServiceMock":
		method = "doInvoke"
	default:
		return "", false
	}
	members, ok := model.lookup(normalizeName(requirement.owner))
	if !ok || !members.platform || !strings.EqualFold(requirement.member.Name, method) {
		return "", false
	}
	displayType := func(typeName string) string {
		typeName = semaCanonicalPlatformAlias(typeName)
		if typeName == "HttpRequest" || typeName == "HttpResponse" {
			return "System." + typeName
		}
		return typeName
	}
	parameters := make([]string, len(requirement.member.Parameters))
	for i, parameter := range requirement.member.Parameters {
		parameters[i] = displayType(parameter.Type)
	}
	return fmt.Sprintf("Class %s must implement the method: %s System.%s.%s(%s)", className, displayType(requirement.member.Type), owner, method, strings.Join(parameters, ", ")), true
}

// The legacy known-type checker calls generated, invisible providers unknown.
// They remain rejected; expose the native visibility reason at the body boundary
// without changing known-type resolution or other families' diagnostic text.
func semaHTTPVisibilityDiagnostics(diagnostics []diagnostic.Diagnostic, model *semaTypeMemberView) []diagnostic.Diagnostic {
	for i := range diagnostics {
		if diagnostics[i].Code != "GLADESEMA006" {
			continue
		}
		for _, name := range []string{"commercetax.TaxEngineContext", "functions.Function"} {
			if semaProjectTypeShadowsPlatform(model, name) {
				continue
			}
			message := diagnostics[i].Message
			if strings.EqualFold(message, "Invalid type: "+name) || strings.HasSuffix(strings.ToLower(message), strings.ToLower(fmt.Sprintf("%q", name))) {
				diagnostics[i].Message = "Type is not visible: " + name
			}
		}
	}
	return diagnostics
}

func semaHTTPArgNames(argTypes []string) string {
	names := make([]string, len(argTypes))
	for i, name := range argTypes {
		name = semaCanonicalPlatformAlias(name)
		if strings.EqualFold(name, "null") {
			name = "NULL"
		}
		names[i] = name
	}
	return strings.Join(names, ", ")
}

func semaHTTPMethodDiagnostic(typ typesys.TypeSymbol, receiver, method string, argTypes []string, model *semaTypeMemberView, mode string, start, end int, source string) (diagnostic.Diagnostic, bool) {
	if semaProjectTypeShadowsPlatform(model, receiver) {
		return diagnostic.Diagnostic{}, false
	}
	receiver = semaHTTPReceiver(receiver)
	if receiver == "" {
		return diagnostic.Diagnostic{}, false
	}
	canonicalMethod := ""
	for _, name := range []string{"getHeaderKeys", "getTimeout", "setTimeout", "setBody", "setBodyAsBlob", "setHeader", "setStatusCode", "send", "getHeader", "getParameter", "addHttpRequest", "invoke"} {
		if strings.EqualFold(method, name) {
			canonicalMethod = name
			break
		}
	}
	if canonicalMethod == "" {
		return diagnostic.Diagnostic{}, false
	}
	rejected := semaAPI67RejectedPlatformCallAtVersion(typ.EffectiveAPIVersion, receiver, method, mode) || semaAPI67RejectedPlatformCallArgs(typ.EffectiveAPIVersion, receiver, method, argTypes)
	if !rejected {
		sig, ok := semaPlatformMethodSignatureForMode(model, receiver, method, mode)
		if !ok && receiver == "Continuation" && canonicalMethod == "addHttpRequest" && mode == "class" {
			// C045/S013-S015: resolve the instance signature before deciding
			// whether a static call has bad arguments or a static-context error.
			sig, ok = semaPlatformMethodSignatureFor(model, receiver, method)
			if ok && semaArgsMatchAny(sig.params, argTypes, model) {
				return semaHTTPDiagnostic(typ, "Non static method cannot be referenced from a static context: String System.Continuation.addHttpRequest(System.HttpRequest)", start, end, source), true
			}
		}
		if !ok || semaArgsMatchAny(sig.params, argTypes, model) {
			return diagnostic.Diagnostic{}, false
		}
	}
	displayReceiver := "System." + receiver
	if receiver == "WebServiceCallout" {
		displayReceiver = receiver // SOAP diagnostics use the unqualified native type.
	}
	message := fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", canonicalMethod, semaHTTPArgNames(argTypes), displayReceiver)
	return semaHTTPDiagnostic(typ, message, start, end, source), true
}

// A receiver-side HTTP call must be checked even when an outer
// call (for example size()) has already inferred a usable generated return type.
// R224: cast operands must also pass the existing variable-visibility check.
// Other receiver calls retain the HTTP-only method filter.
func (a *Analyzer) checkIRHTTPReceiverCalls(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	diagnostics, _ := a.checkIRHTTPReceiverCallsWithType(typ, member, expr, scope, pos, bodyOffset, source, model, constructability, false)
	return diagnostics
}

// Infer a receiver after checking its calls, so a fluent chain is evaluated from
// the inside out instead of repeatedly inferring every preceding call.
func (a *Analyzer) checkIRHTTPReceiverCallsWithType(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol, inferType bool) ([]diagnostic.Diagnostic, string) {
	var diagnostics []diagnostic.Diagnostic
	ordinaryCall := expr.Kind == ir.ExprCall && !strings.HasPrefix(expr.Callee, "__")
	inferLeft := ordinaryCall
	if !inferType && expr.Left != nil {
		// These forms cannot yield an HTTP receiver, and their inference does
		// not hydrate the member model. Keep traversing their receiver calls.
		switch expr.Left.Kind {
		case ir.ExprSOQL:
			inferLeft = false
		case ir.ExprUnary:
			if expr.Left.Operator == "!" {
				inferLeft = false
			}
		case ir.ExprLiteral:
			value := strings.TrimSpace(expr.Left.Value)
			if strings.HasPrefix(value, "'") || semaKeywordLiteralType(value) != "" {
				inferLeft = false
			}
		}
	}
	receiverType := ""
	if expr.Left != nil {
		var childDiagnostics []diagnostic.Diagnostic
		childDiagnostics, receiverType = a.checkIRHTTPReceiverCallsWithType(typ, member, *expr.Left, scope, pos, bodyOffset, source, model, constructability, inferLeft)
		diagnostics = append(diagnostics, childDiagnostics...)
	}
	httpReceiver := false
	if expr.Kind == ir.ExprCall {
		if strings.HasPrefix(expr.Callee, "__cast:") {
			for _, arg := range expr.Args {
				diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, arg, &scope, pos, bodyOffset, source, model, constructability)...)
			}
		} else if ordinaryCall {
			if expr.Left == nil {
				if receiver, _, ok := splitSemaMethodPath(expr.Callee); ok {
					receiverType = semaIRReceiverType(receiver, scope, model, typ.Name)
				}
			}
			httpReceiver = semaHTTPReceiver(receiverType) != ""
			if httpReceiver && !semaProjectTypeShadowsPlatform(model, receiverType) {
				diagnostics = append(diagnostics, a.checkIRCall(typ, member, expr, scope, pos, bodyOffset, source, model, constructability)...)
			}
		}
	}
	if !inferType {
		return diagnostics, ""
	}
	if ordinaryCall && expr.Left != nil && !httpReceiver && !strings.HasPrefix(expr.Callee, "new:") && !strings.HasPrefix(expr.Callee, "newlit:") {
		// No model lookup occurs between the child's inference and this use.
		return diagnostics, a.inferIRCallTypeWithReceiver(expr, receiverType, scope, model, typ.Name)
	}
	// HTTP checks and their project-shadow lookup can hydrate the model. Infer
	// again afterwards, as before; special expressions also retain their path.
	return diagnostics, a.inferIRExprType(expr, scope, model, typ.Name)
}
