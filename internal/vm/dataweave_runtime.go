package vm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dataweave"
	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) dataWeaveCreateScript(args []Value) (Value, error) {
	namespace := vm.currentCallerNamespace()
	var name Value
	switch {
	case len(args) == 1:
		name = args[0]
	case len(args) == 2:
		name = args[1]
		if args[0].Kind == ValueString {
			namespace = args[0].Text
			if namespace == "" && vm.Org != nil {
				namespace = vm.Org.Namespace
			}
		} else if args[0].Kind != ValueNull {
			return Null, fmt.Errorf("DataWeave.Script.createScript expects namespace String")
		}
	default:
		return Null, fmt.Errorf("DataWeave.Script.createScript expects script name String")
	}
	if name.Kind != ValueString {
		return Null, fmt.Errorf("DataWeave.Script.createScript expects script name String")
	}
	source, err := vm.dataWeaveSource(namespace, name.Text)
	if err != nil {
		return Null, err
	}
	script := Object("DataWeave.Script")
	script.Fields["name"] = String(source.Name)
	script.Fields["__gladeSource"] = String(source.Content)
	script.Fields["__gladeAPIVersion"] = String(source.APIVersion)
	script.Fields["__gladeNamespace"] = String(source.Namespace)
	return script, nil
}

func (vm *VM) dataWeaveSource(namespace, name string) (storage.DataWeaveResourceMetadata, error) {
	var found *storage.DataWeaveResourceMetadata
	if vm.Org != nil {
		for _, source := range vm.Org.Metadata.DataWeaveResources {
			if !strings.EqualFold(source.Namespace, namespace) || !strings.EqualFold(source.Name, name) {
				continue
			}
			if found != nil {
				return storage.DataWeaveResourceMetadata{}, fmt.Errorf("ambiguous DataWeave resource %s.%s", namespace, name)
			}
			copy := source
			found = &copy
		}
	}
	if found == nil || found.ContentPath == "" {
		return storage.DataWeaveResourceMetadata{}, newExceptionError("NoDataFoundException", "Could not find DataWeave script "+name)
	}
	return *found, nil
}

func (vm *VM) executeDataWeaveSource(receiver, inputs Value) (Value, error) {
	name := dataWeaveScriptName(receiver)
	source, ok := receiver.Fields["__gladeSource"]
	api := receiver.Fields["__gladeAPIVersion"].Text
	namespace := receiver.Fields["__gladeNamespace"].Text
	if !ok {
		resource, err := vm.dataWeaveSource(vm.currentCallerNamespace(), name)
		if err != nil {
			return Null, err
		}
		source = String(resource.Content)
		api = resource.APIVersion
		namespace = resource.Namespace
	}
	request := dataweave.Request{Name: name, Source: source.Text, APIVersion: api, Inputs: make(map[string]dataweave.Input)}
	request.Modules = make(map[string]dataweave.Module)
	if vm.Org != nil {
		for _, resource := range vm.Org.Metadata.DataWeaveResources {
			if resource.ContentPath == "" || !strings.EqualFold(resource.Namespace, namespace) {
				continue
			}
			request.Modules[resource.Name] = dataweave.Module{Name: resource.Name, Namespace: resource.Namespace, APIVersion: resource.APIVersion, Source: resource.Content}
		}
	}

	for raw, value := range inputs.Map {
		key := mapStoredKey(inputs, raw)
		if key.Kind != ValueString {
			return Null, fmt.Errorf("DataWeave input names must be Strings")
		}
		switch {
		case value.Kind == ValueString:
			request.Inputs[key.Text] = dataweave.Input{Data: []byte(value.Text)}
		case value.Kind == ValueObject && strings.EqualFold(value.Type, "Blob"):
			request.Inputs[key.Text] = dataweave.Input{Data: []byte(blobText(value))}
		default:
			typed, err := vm.dataWeaveInputValue(value, make(map[uint64]bool), 0)
			if err != nil {
				return Null, err
			}
			request.Inputs[key.Text] = dataweave.Input{Typed: &typed}
		}
	}
	runtime, err := gladehome.DataWeaveRuntime()
	if err != nil {
		return Null, &dataweave.HostError{Kind: "toolchain", Detail: "DataWeave toolchain is unavailable; run glade toolchain install dataweave", Cause: err}
	}
	ctx := vm.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	output, err := runtime.Execute(ctx, request)
	if err != nil {
		var engineError *dataweave.EngineError
		if errors.As(err, &engineError) {
			return Null, newExceptionError("DataWeaveScriptException", engineError.Message)
		}
		return Null, err
	}
	if output.Typed == nil && !strings.EqualFold(output.Charset, "UTF-8") && !strings.EqualFold(output.Charset, "UTF8") {
		return Null, unsupportedCallError("DataWeave output charset " + output.Charset)
	}
	result := Object("DataWeave.Result")
	if output.Typed != nil {
		value, err := vm.dataWeaveOutputValue(*output.Typed, 0)
		if err != nil {
			return Null, err
		}
		result.Fields["value"] = value
		result.Fields["__gladeTypedResult"] = Bool(true)
	} else {
		result.Fields["valueAsString"] = String(string(output.Data))
		switch strings.ToLower(output.MIMEType) {
		case "application/json", "text/plain", "application/csv", "application/octet-stream":
			result.Fields["value"] = result.Fields["valueAsString"]
		}
	}
	result.Fields["mimeType"] = String(output.MIMEType)
	result.Fields["__gladeSourceDriven"] = Bool(true)
	return result, nil
}

func dataWeaveScriptName(receiver Value) string {
	if receiver.Kind != ValueObject {
		return ""
	}
	if _, value, ok := objectFieldValue(receiver, "name"); ok && value.Kind == ValueString {
		return value.Text
	}
	if strings.HasPrefix(receiver.Type, "DataWeaveScriptResource.") {
		return strings.TrimPrefix(receiver.Type, "DataWeaveScriptResource.")
	}
	return ""
}
