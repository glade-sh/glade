package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/trace"
)

type UIInvocationResult struct {
	Framework   string         `json:"framework"`
	ClassName   string         `json:"className"`
	MethodName  string         `json:"methodName"`
	Success     bool           `json:"success"`
	ReturnValue any            `json:"returnValue,omitempty"`
	Error       *UIActionError `json:"error,omitempty"`
	Trace       []trace.Event  `json:"trace,omitempty"`
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
	result := &Result{TraceFormat: trace.FormatChromeTraceEvent}
	value, err := vm.callMethod(method, args, result)
	out.Trace = result.Trace
	if err != nil {
		out.Success = false
		out.Error = uiInvocationError(err)
		return out, nil
	}
	out.Success = true
	out.ReturnValue = jsonFromValue(value, false)
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

func uiMethodSignature(method Method) string {
	parts := make([]string, 0, len(method.Params))
	for _, param := range method.Params {
		parts = append(parts, param.Name+":"+param.Type)
	}
	return method.Name + "(" + strings.Join(parts, ",") + ")"
}

func (r UIInvocationResult) JSON() ([]byte, error) {
	return json.Marshal(r)
}
