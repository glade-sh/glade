package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/trace"
)

type UIInvocationResult struct {
	Framework    string         `json:"framework"`
	ClassName    string         `json:"className"`
	MethodName   string         `json:"methodName"`
	Success      bool           `json:"success"`
	ReturnValue  any            `json:"returnValue,omitempty"`
	PageMessages []any          `json:"pageMessages,omitempty"`
	Error        *UIActionError `json:"error,omitempty"`
	Trace        []trace.Event  `json:"trace,omitempty"`
}

type UIActionError struct {
	Code    string `json:"code,omitempty"`
	Type    string `json:"type,omitempty"`
	Message string `json:"message"`
	cause   error
}

// ExceptionTypeName qualifies known platform exceptions while preserving the
// declared names of custom exceptions, whether top-level or nested.
func (err *UIActionError) ExceptionTypeName() string {
	if err == nil {
		return ""
	}
	return exceptionQualifiedTypeName(err.Type)
}

func (vm *VM) InvokeAuraAction(className, methodName string, params map[string]any) (UIInvocationResult, error) {
	return vm.invokeUIAction("aura", className, methodName, params)
}

func (vm *VM) InvokeLWCMethod(className, methodName string, params map[string]any) (UIInvocationResult, error) {
	return vm.invokeUIAction("lwc", className, methodName, params)
}

func (vm *VM) InvokeVisualforceAction(className, methodName, pageURL string, params map[string]string) (UIInvocationResult, error) {
	out := UIInvocationResult{Framework: "visualforce", ClassName: className, MethodName: methodName}
	if strings.TrimSpace(className) == "" || strings.TrimSpace(methodName) == "" {
		out.Success = false
		out.Error = &UIActionError{Type: "UnsupportedFeature", Message: "Visualforce action requires controller class and method"}
		return out, nil
	}
	method, ok := vm.resolveInstanceMethod(className, methodName)
	if !ok || method.IsStatic || len(method.Params) != 0 {
		out.Success = false
		out.Error = &UIActionError{Type: "UnsupportedFeature", Message: fmt.Sprintf("no instance Visualforce action %s.%s accepts zero arguments", className, methodName)}
		return out, nil
	}
	if strings.TrimSpace(pageURL) == "" {
		pageURL = "/apex/current"
	}
	vm.pageMessages = nil
	vm.currentPage = vm.newPageReference(pageURL)
	if len(params) > 0 {
		mergeCurrentPageStringParams(vm.currentPage, params)
	}
	result := &Result{TraceFormat: trace.FormatChromeTraceEvent, traceEnabled: true}
	appendVisualforceTrace(result, "current_page", map[string]any{
		"className":  className,
		"methodName": methodName,
		"page":       tracePageReference(vm.currentPage),
	})
	constructStart, constructStartedAt := traceSpanStart(result)
	appendVisualforceTrace(result, "controller.construct.start", map[string]any{
		"className": className,
	})
	controller, err := vm.constructValue(className, nil, nil, result)
	if err != nil {
		out.Success = false
		out.Error = uiInvocationError(err)
		appendVisualforceTrace(result, "controller.construct.error", map[string]any{
			"className": className,
			"error":     out.Error.Message,
			"errorType": out.Error.Type,
		})
		appendDurationTrace(result, "apex.visualforce.controller.construct", "apex.visualforce", constructStart, traceDurationSince(constructStartedAt), map[string]any{
			"className": className,
			"error":     out.Error.Message,
			"errorType": out.Error.Type,
		})
		out.Trace = result.Trace
		return out, nil
	}
	appendVisualforceTrace(result, "controller.construct.complete", map[string]any{
		"className": className,
	})
	appendDurationTrace(result, "apex.visualforce.controller.construct", "apex.visualforce", constructStart, traceDurationSince(constructStartedAt), map[string]any{
		"className": className,
	})
	actionStart, actionStartedAt := traceSpanStart(result)
	appendVisualforceTrace(result, "action.invoke", map[string]any{
		"className":  className,
		"methodName": methodName,
		"page":       tracePageReference(vm.currentPage),
	})
	value, err := vm.callMethodWithReceiver(method, controller, nil, result)
	out.PageMessages = jsonListFromValues(vm.pageMessages)
	if err != nil {
		out.Success = false
		out.Error = uiInvocationError(err)
		appendVisualforceTrace(result, "action.error", map[string]any{
			"className":         className,
			"methodName":        methodName,
			"error":             out.Error.Message,
			"errorType":         out.Error.Type,
			"pageMessageCount":  len(out.PageMessages),
			"pageMessages":      out.PageMessages,
			"currentPage":       tracePageReference(vm.currentPage),
			"controllerCreated": true,
		})
		appendDurationTrace(result, "apex.visualforce.action", "apex.visualforce", actionStart, traceDurationSince(actionStartedAt), map[string]any{
			"className":        className,
			"methodName":       methodName,
			"error":            out.Error.Message,
			"errorType":        out.Error.Type,
			"pageMessageCount": len(out.PageMessages),
		})
		out.Trace = result.Trace
		return out, nil
	}
	out.Success = true
	out.ReturnValue = plainUIJSON(jsonFromValue(value, false))
	completeArgs := map[string]any{
		"className":        className,
		"methodName":       methodName,
		"returnValue":      out.ReturnValue,
		"pageMessageCount": len(out.PageMessages),
		"currentPage":      tracePageReference(vm.currentPage),
	}
	if value.Kind == ValueObject && value.Type == "PageReference" {
		completeArgs["pageReference"] = tracePageReference(value)
	}
	if len(out.PageMessages) > 0 {
		completeArgs["pageMessages"] = out.PageMessages
	}
	appendVisualforceTrace(result, "action.complete", completeArgs)
	appendDurationTrace(result, "apex.visualforce.action", "apex.visualforce", actionStart, traceDurationSince(actionStartedAt), completeArgs)
	out.Trace = result.Trace
	return out, nil
}

func (vm *VM) InvokeVisualforceActionOnController(controller Value, className, methodName, pageURL string, params map[string]string) (Value, Value, UIInvocationResult, error) {
	vm.registerSObjectAliasRecord(controller)
	out := UIInvocationResult{Framework: "visualforce", ClassName: className, MethodName: methodName}
	if strings.TrimSpace(className) == "" || strings.TrimSpace(methodName) == "" {
		out.Success = false
		out.Error = &UIActionError{Type: "UnsupportedFeature", Message: "Visualforce action requires controller class and method"}
		return Null, controller, out, nil
	}
	if strings.TrimSpace(pageURL) == "" {
		pageURL = "/apex/current"
	}
	// A bound action shares its request's messages with constructors and setters.
	vm.currentPage = vm.newPageReference(pageURL)
	if len(params) > 0 {
		mergeCurrentPageStringParams(vm.currentPage, params)
	}
	result := &Result{TraceFormat: trace.FormatChromeTraceEvent, traceEnabled: true}
	if strings.EqualFold(className, "ApexPages.StandardController") || strings.EqualFold(className, "ApexPages.StandardSetController") {
		appendVisualforceTrace(result, "action.invoke", map[string]any{
			"className":  className,
			"methodName": methodName,
			"page":       tracePageReference(vm.currentPage),
			"bound":      true,
		})
		var value Value
		var updated Value
		var handled bool
		var err error
		if strings.EqualFold(className, "ApexPages.StandardSetController") {
			value, updated, _, handled, err = vm.callStandardSetControllerMember(controller, methodName, nil, result)
		} else {
			value, updated, _, handled, err = vm.callStandardControllerMember(controller, methodName, nil, result)
		}
		out.PageMessages = jsonListFromValues(vm.pageMessages)
		if err != nil {
			out.Success = false
			out.Error = uiInvocationError(err)
			out.Trace = result.Trace
			return value, updated, out, nil
		}
		if !handled {
			out.Success = false
			out.Error = &UIActionError{Type: "UnsupportedFeature", Message: fmt.Sprintf("no standard Visualforce action %s accepts zero arguments", methodName)}
			out.Trace = result.Trace
			return Null, controller, out, nil
		}
		out.Success = true
		out.ReturnValue = plainUIJSON(jsonFromValue(value, false))
		appendVisualforceTrace(result, "action.complete", map[string]any{
			"className":        className,
			"methodName":       methodName,
			"returnValue":      out.ReturnValue,
			"pageMessageCount": len(out.PageMessages),
			"pageMessages":     out.PageMessages,
			"currentPage":      tracePageReference(vm.currentPage),
			"bound":            true,
		})
		out.Trace = result.Trace
		return value, updated, out, nil
	}
	method, ok := vm.resolveInstanceMethod(className, methodName)
	if !ok || method.IsStatic || len(method.Params) != 0 {
		out.Success = false
		out.Error = &UIActionError{Type: "UnsupportedFeature", Message: fmt.Sprintf("no instance Visualforce action %s.%s accepts zero arguments", className, methodName)}
		return Null, controller, out, nil
	}
	if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
		out.Success = false
		out.Error = uiInvocationError(err)
		return Null, controller, out, nil
	}
	appendVisualforceTrace(result, "action.invoke", map[string]any{
		"className":  className,
		"methodName": methodName,
		"page":       tracePageReference(vm.currentPage),
		"bound":      true,
	})
	value, err := vm.callMethodWithReceiver(method, controller, nil, result)
	out.PageMessages = jsonListFromValues(vm.pageMessages)
	if err != nil {
		out.Success = false
		out.Error = uiInvocationError(err)
		appendVisualforceTrace(result, "action.error", map[string]any{
			"className":        className,
			"methodName":       methodName,
			"error":            out.Error.Message,
			"errorType":        out.Error.Type,
			"pageMessageCount": len(out.PageMessages),
			"pageMessages":     out.PageMessages,
			"currentPage":      tracePageReference(vm.currentPage),
			"bound":            true,
		})
		out.Trace = result.Trace
		return value, controller, out, nil
	}
	out.Success = true
	out.ReturnValue = plainUIJSON(jsonFromValue(value, false))
	appendVisualforceTrace(result, "action.complete", map[string]any{
		"className":        className,
		"methodName":       methodName,
		"returnValue":      out.ReturnValue,
		"pageMessageCount": len(out.PageMessages),
		"pageMessages":     out.PageMessages,
		"currentPage":      tracePageReference(vm.currentPage),
		"bound":            true,
	})
	out.Trace = result.Trace
	return value, controller, out, nil
}

func mergeCurrentPageStringParams(page Value, params map[string]string) {
	if len(params) == 0 || page.Kind != ValueObject {
		return
	}
	pageParams, ok := page.Fields["parameters"]
	if !ok || pageParams.Kind != ValueMap {
		pageParams = typedMap("Map<String,String>")
		page.Fields["parameters"] = pageParams
	}
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pageParams.Map[mapKey(String(name))] = String(params[name])
		if pageParams.MapKeys != nil {
			pageParams.MapKeys[mapKey(String(name))] = String(name)
		}
	}
	page.Fields["parameters"] = pageParams
}

func (vm *VM) invokeUIAction(framework, className, methodName string, params map[string]any) (UIInvocationResult, error) {
	out := UIInvocationResult{Framework: framework, ClassName: className, MethodName: methodName}
	callerEntrySharing := vm.entrySharingMode
	vm.entrySharingMode = "with sharing"
	defer func() { vm.entrySharingMode = callerEntrySharing }()
	lwc := framework == "lwc"
	method, args, err := vm.resolveUIAction(className, methodName, params, lwc)
	if err != nil {
		var diagnostic *UIActionDiagnosticError
		if errors.As(err, &diagnostic) {
			out.Success = false
			out.Error = &UIActionError{Code: diagnostic.Code, Type: diagnostic.Type, Message: diagnostic.Message}
			return out, nil
		}
		out.Success = false
		out.Error = &UIActionError{Type: "UnsupportedFeature", Message: err.Error()}
		return out, nil
	}
	if err := vm.ensureClassInitialized(method.ClassName); err != nil {
		out.Success = false
		out.Error = uiInvocationError(err)
		return out, nil
	}
	result := &Result{TraceFormat: trace.FormatChromeTraceEvent, traceEnabled: true}
	value, err := vm.callMethod(method, args, result)
	out.Trace = result.Trace
	if err != nil {
		out.Success = false
		out.Error = uiInvocationError(err)
		return out, nil
	}
	out.Success = true
	if lwc {
		out.ReturnValue = vm.lwcJSONFromValue(value)
	} else {
		out.ReturnValue = plainUIJSON(jsonFromValue(value, false))
	}
	return out, nil
}

func (vm *VM) resolveUIAction(className, methodName string, params map[string]any, lwc bool) (Method, []Value, error) {
	if strings.TrimSpace(className) == "" || strings.TrimSpace(methodName) == "" {
		return Method{}, nil, fmt.Errorf("UI action requires class and method")
	}
	candidates := append([]Method(nil), vm.registeredOverloads(className+"."+methodName)...)
	if len(candidates) == 0 {
		candidates = append(candidates, vm.registeredFolded(strings.ToLower(className+"."+methodName))...)
	}
	sort.SliceStable(candidates, func(i, j int) bool { return uiMethodSignature(candidates[i]) < uiMethodSignature(candidates[j]) })
	var matchedMethod Method
	var matchedArgs []Value
	matched := 0
	matchedSignatures := map[string]bool{}
	var parameterErr error
	for _, method := range candidates {
		if !method.IsStatic || !methodHasAuraEnabled(method.Modifiers) || (!lwc && len(method.Params) != len(params)) {
			continue
		}
		args := make([]Value, 0, len(method.Params))
		ok := true
		for _, param := range method.Params {
			raw, exists := params[param.Name]
			if !exists && !lwc {
				ok = false
				break
			}
			typeName := vm.resolveTypeNameInClass(method.ClassName, param.Type)
			var value Value
			var err error
			if lwc {
				value, err = vm.lwcParameterValue(typeName, raw)
			} else {
				value, err = vm.typedValueFromJSON(typeName, raw, false)
			}
			if err != nil {
				if lwc {
					parameterErr = &UIActionDiagnosticError{Type: "InvalidActionParameter", Message: fmt.Sprintf("Value provided is invalid for action parameter '%s' of type '%s'", param.Name, param.Type)}
					var actionErr *UIActionDiagnosticError
					if errors.As(err, &actionErr) {
						parameterErr = actionErr
					}
				}
				ok = false
				break
			}
			args = append(args, value)
		}
		if ok {
			signature := uiMethodSignature(method)
			if matchedSignatures[signature] {
				continue
			}
			matchedSignatures[signature] = true
			matched++
			if matched > 1 {
				return Method{}, nil, &UIActionDiagnosticError{
					Code:    "GLADELWC013",
					Type:    "UnsupportedFeature",
					Message: "overloaded AuraEnabled method unsupported",
				}
			}
			matchedMethod = method
			matchedArgs = args
		}
	}
	if matched == 1 {
		return matchedMethod, matchedArgs, nil
	}
	if parameterErr != nil {
		return Method{}, nil, parameterErr
	}
	return Method{}, nil, fmt.Errorf("no static @AuraEnabled method %s.%s accepts parameters %s", className, methodName, sortedParamNames(params))
}

func (vm *VM) lwcParameterValue(typeName string, raw any) (Value, error) {
	// Native null List<String> parameters arrive as an empty collection; the
	// captured null List<Account> and Map<String,String> parameters stay null.
	if raw == nil && strings.EqualFold(strings.Join(strings.Fields(typeName), ""), "List<String>") {
		return typedList(typeName), nil
	}
	if strings.EqualFold(typeName, "Datetime") {
		if text, ok := raw.(string); ok && !strings.HasSuffix(text, "Z") {
			return Null, fmt.Errorf("LWC Datetime parameter requires UTC JSON")
		}
	}
	if fields, ok := raw.(map[string]any); ok && vm.isSObjectLikeType(typeName) && !strings.EqualFold(typeName, "SObject") {
		if actual, ok := fields["sobjectType"].(string); ok && !strings.EqualFold(typeName, actual) {
			// Preserve the entity mismatch as an explicit local boundary. The
			// native U# identifiers in r_serial_account_wrong_type are hosted
			// identities, not local schema identifiers.
			return Null, &UIActionDiagnosticError{Type: "InvalidActionParameter", Message: fmt.Sprintf("Mismatched entity type. Expected %s but received %s", typeName, actual)}
		}
	}
	return vm.typedValueFromJSON(typeName, raw, false)
}

// The Apex action transport has its own serializer. JSON.serialize and the
// Aura/Visualforce paths retain their existing representation. Native action rows
// omit null collection entries and fields, expose only @AuraEnabled DTO members,
// emit millisecond UTC Datetimes, and retain high-precision Decimals as strings.
func (vm *VM) lwcJSONFromValue(value Value) any {
	switch value.Kind {
	case ValueDecimal:
		if !isFloatBackedDecimal(value) {
			text := decimalDisplayText(value)
			digits := strings.TrimLeft(strings.ReplaceAll(strings.TrimPrefix(text, "-"), ".", ""), "0")
			if len(digits) > 15 {
				return text
			}
		}
		return jsonFromValue(value, false)
	case ValueList, ValueSet:
		items := value.List
		if value.Kind == ValueSet {
			items = value.Set
		}
		out := make([]any, 0, len(items))
		for _, item := range items {
			if item.Kind != ValueNull {
				out = append(out, vm.lwcJSONFromValue(item))
			}
		}
		return out
	case ValueMap:
		out := make(map[string]any, len(value.Map))
		for key, item := range value.Map {
			if item.Kind != ValueNull {
				out[mapStoredKey(value, key).String()] = vm.lwcJSONFromValue(item)
			}
		}
		return out
	case ValueObject:
		if strings.EqualFold(value.Type, "Datetime") {
			if timestamp, err := parsePlatformDatetime(value); err == nil {
				return timestamp.UTC().Format("2006-01-02T15:04:05.000Z")
			}
		}
		if scalar, ok := jsonPlatformScalarFromValue(value); ok {
			return scalar
		}
		out := make(map[string]any)
		if _, ok := vm.lookupClass(value.Type); ok {
			for _, name := range vm.jsonSerializableFieldNames(value.Type) {
				field, owner, ok := vm.lookupField(value.Type, name)
				if !ok || field.Static || !methodHasAuraEnabled(field.Modifiers) {
					continue
				}
				_, item, present := objectFieldValue(value, name)
				if field.Getter != nil {
					getterValue, err := vm.callGetter(vm.getterOwner(owner, field), field, value)
					if err != nil {
						continue
					}
					item, present = getterValue, true
				}
				if present && item.Kind != ValueNull {
					out[name] = vm.lwcJSONFromValue(item)
				}
			}
			return out
		}
		sobject := vm.isSObjectLikeType(value.Type)
		for name, item := range value.Fields {
			if item.Kind == ValueNull {
				continue
			}
			// Native r_return_account/r_return_accounts omit constructor
			// defaults. Explicit assignments clear this marker, retaining zero
			// and false values without changing DTO or map serialization.
			if sobject && (isInternalSObjectField(name) || isDefaultedSObjectField(value, name) || name == "attributes" || name == "sobjectType") {
				continue
			}
			out[name] = vm.lwcJSONFromValue(item)
		}
		return out
	default:
		return jsonFromValue(value, false)
	}
}

type UIActionDiagnosticError struct {
	Code    string
	Type    string
	Message string
}

func (err *UIActionDiagnosticError) Error() string {
	if err == nil {
		return ""
	}
	if err.Code == "" {
		return err.Message
	}
	return err.Code + " " + err.Message
}

func methodHasAuraEnabled(modifiers []string) bool {
	for _, modifier := range modifiers {
		normalized := strings.TrimPrefix(strings.TrimSpace(modifier), "@")
		if strings.EqualFold(normalized, "AuraEnabled") || strings.HasPrefix(strings.ToLower(normalized), "auraenabled(") {
			return true
		}
	}
	return false
}

func uiInvocationError(err error) *UIActionError {
	var thrown *apexThrowError
	if errors.As(err, &thrown) {
		runtime := runtimeError(thrown.value, thrown.stack)
		if runtimeErr, ok := runtime.(*RuntimeError); ok {
			return &UIActionError{Type: runtimeErr.Type, Message: runtimeErr.Message, cause: runtimeErr}
		}
	}
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) {
		return &UIActionError{Type: runtimeErr.Type, Message: runtimeErr.Message, cause: runtimeErr}
	}
	return &UIActionError{Type: "RuntimeError", Message: err.Error(), cause: err}
}

func sortedParamNames(params map[string]any) []string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func jsonListFromValues(values []Value) []any {
	if len(values) == 0 {
		return nil
	}
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, plainUIJSON(jsonFromValue(value, false)))
	}
	return out
}

func plainUIJSON(value any) any {
	switch typed := value.(type) {
	case orderedJSONObject:
		out := make(map[string]any, len(typed))
		for _, field := range typed {
			if field.name == "attributes" {
				continue
			}
			out[field.name] = plainUIJSON(field.value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = plainUIJSON(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if key == "attributes" {
				continue
			}
			out[key] = plainUIJSON(item)
		}
		return out
	default:
		return value
	}
}

func uiMethodSignature(method Method) string {
	parts := make([]string, 0, len(method.Params))
	for _, param := range method.Params {
		parts = append(parts, param.Name+":"+param.Type)
	}
	return method.Name + "(" + strings.Join(parts, ",") + ")"
}

func appendVisualforceTrace(result *Result, name string, args map[string]any) {
	appendTrace(result, "apex.visualforce."+name, "apex.visualforce", args)
}

func tracePageReference(value Value) map[string]any {
	out := map[string]any{}
	if value.Kind != ValueObject || value.Type != "PageReference" {
		return out
	}
	if url, ok := value.Fields["url"]; ok && url.Kind == ValueString {
		out["url"] = url.Text
	}
	if redirect, ok := value.Fields["redirect"]; ok && redirect.Kind == ValueBool {
		out["redirect"] = redirect.Bool
	}
	if params, ok := value.Fields["parameters"]; ok && params.Kind == ValueMap {
		out["parameters"] = jsonFromValue(params, false)
	}
	if headers, ok := value.Fields["headers"]; ok && headers.Kind == ValueMap && len(headers.Map) > 0 {
		out["headers"] = jsonFromValue(headers, false)
	}
	return out
}

func (r UIInvocationResult) JSON() ([]byte, error) {
	return json.Marshal(r)
}
