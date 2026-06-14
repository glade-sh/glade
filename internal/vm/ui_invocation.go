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
	Type    string `json:"type,omitempty"`
	Message string `json:"message"`
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
	out := UIInvocationResult{Framework: "visualforce", ClassName: className, MethodName: methodName}
	if strings.TrimSpace(className) == "" || strings.TrimSpace(methodName) == "" {
		out.Success = false
		out.Error = &UIActionError{Type: "UnsupportedFeature", Message: "Visualforce action requires controller class and method"}
		return Null, controller, out, nil
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
	method, args, err := vm.resolveUIAction(className, methodName, params)
	if err != nil {
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
	out.ReturnValue = plainUIJSON(jsonFromValue(value, false))
	return out, nil
}

func (vm *VM) resolveUIAction(className, methodName string, params map[string]any) (Method, []Value, error) {
	if strings.TrimSpace(className) == "" || strings.TrimSpace(methodName) == "" {
		return Method{}, nil, fmt.Errorf("UI action requires class and method")
	}
	candidates := append([]Method(nil), vm.MethodOverloads[className+"."+methodName]...)
	if len(candidates) == 0 {
		candidates = append(candidates, vm.MethodFolded[strings.ToLower(className+"."+methodName)]...)
	}
	sort.SliceStable(candidates, func(i, j int) bool { return uiMethodSignature(candidates[i]) < uiMethodSignature(candidates[j]) })
	for _, method := range candidates {
		if !method.IsStatic || !methodHasAuraEnabled(method.Modifiers) || len(method.Params) != len(params) {
			continue
		}
		args := make([]Value, 0, len(method.Params))
		ok := true
		for _, param := range method.Params {
			raw, exists := params[param.Name]
			if !exists {
				ok = false
				break
			}
			value, err := vm.typedValueFromJSON(vm.resolveTypeNameInClass(method.ClassName, param.Type), raw, false)
			if err != nil {
				ok = false
				break
			}
			args = append(args, value)
		}
		if ok {
			return method, args, nil
		}
	}
	return Method{}, nil, fmt.Errorf("no static @AuraEnabled method %s.%s accepts parameters %s", className, methodName, sortedParamNames(params))
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
			return &UIActionError{Type: runtimeErr.Type, Message: runtimeErr.Message}
		}
	}
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) {
		return &UIActionError{Type: runtimeErr.Type, Message: runtimeErr.Message}
	}
	return &UIActionError{Type: "RuntimeError", Message: err.Error()}
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
