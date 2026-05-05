package vm

import (
	"errors"
	"fmt"
	"strings"
)

// NewRestRequestValue creates the local RestRequest shape used by RestContext.
func NewRestRequestValue() Value {
	return newRestRequest()
}

// NewRestResponseValue creates the local RestResponse shape used by RestContext.
func NewRestResponseValue() Value {
	return newRestResponse()
}

// NewBlobValue creates a local Blob scalar backed by text bytes.
func NewBlobValue(value string) Value {
	return platformScalar("Blob", value)
}

// NewStringMapValue creates a typed Map<String,String> value.
func NewStringMapValue(values map[string]string) Value {
	out := typedMap("Map<String,String>")
	for key, value := range values {
		out.Map[mapKey(String(key))] = String(value)
	}
	return out
}

// StringMapEntries returns String-keyed entries from a VM map value.
func StringMapEntries(value Value) map[string]string {
	out := map[string]string{}
	if value.Kind != ValueMap {
		return out
	}
	for key, rawValue := range StringValueMapEntries(value) {
		out[key] = rawValue.String()
	}
	return out
}

// StringValueMapEntries returns String-keyed VM values without flattening them.
func StringValueMapEntries(value Value) map[string]Value {
	out := map[string]Value{}
	if value.Kind != ValueMap {
		return out
	}
	for rawKey, rawValue := range value.Map {
		key := valueFromMapKey(rawKey)
		if key.Kind != ValueString {
			continue
		}
		out[key.Text] = rawValue
	}
	return out
}

// SetRestContext installs request/response values for RestContext static fields.
func (vm *VM) SetRestContext(request, response Value) error {
	if request.Kind != ValueNull && (request.Kind != ValueObject || request.Type != "RestRequest") {
		return fmt.Errorf("RestContext.request expects RestRequest")
	}
	if response.Kind != ValueNull && (response.Kind != ValueObject || response.Type != "RestResponse") {
		return fmt.Errorf("RestContext.response expects RestResponse")
	}
	vm.restRequest = request
	vm.restResponse = response
	return nil
}

func (vm *VM) RestRequest() Value {
	if vm.restRequest.Kind == "" {
		return Null
	}
	return vm.restRequest
}

func (vm *VM) RestResponse() Value {
	if vm.restResponse.Kind == "" || vm.restResponse.Kind == ValueNull {
		vm.restResponse = newRestResponse()
	}
	return vm.restResponse
}

// CallStatic invokes a registered static Apex method by fully qualified name.
func (vm *VM) CallStatic(name string, args []Value) (Value, error) {
	dot := strings.LastIndex(name, ".")
	if dot <= 0 || dot == len(name)-1 {
		return Null, fmt.Errorf("static method name must be Class.method")
	}
	className := name[:dot]
	methodName := name[dot+1:]
	method, ok, ambiguous := vm.resolveStaticMethodForArgs(className, methodName, args)
	if ambiguous {
		return Null, vm.ambiguousOverloadError(name, args)
	}
	if !ok {
		return Null, fmt.Errorf("unknown static method %q", name)
	}
	if err := vm.ensureClassInitialized(method.ClassName); err != nil {
		return Null, err
	}
	value, err := vm.callMethod(method, args, &Result{})
	if err != nil {
		var thrown *apexThrowError
		if errors.As(err, &thrown) {
			if len(thrown.stack) == 0 {
				thrown.stack = vm.rawStackFrames()
			}
			return Null, runtimeError(thrown.value, thrown.stack)
		}
	}
	return value, err
}
