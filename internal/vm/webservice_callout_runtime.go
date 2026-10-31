package vm

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	webServiceFutureLabelField        = "__webServiceCalloutLabel"
	webServiceFutureResponseType      = "__webServiceCalloutResponseType"
	webServiceFutureResponseNamespace = "__webServiceCalloutResponseNamespace"
	webServiceFutureResponseName      = "__webServiceCalloutResponseName"
	webServiceSOAPNamespace           = "http://schemas.xmlsoap.org/soap/envelope/"
)

func (vm *VM) webServiceCalloutBeginInvoke(args []Value, result *Result) (Value, error) {
	if len(args) != 5 {
		return Null, unsupportedCallError("WebServiceCallout.beginInvoke argument count")
	}
	for _, position := range []int{2, 3, 4} {
		if args[position].Kind == ValueNull {
			return Null, newExceptionError("NullPointerException", fmt.Sprintf("Argument %d cannot be null", position+1))
		}
	}
	if args[4].Kind != ValueList {
		return Null, unsupportedCallError("WebServiceCallout.beginInvoke option type")
	}
	if len(args[4].List) != 7 {
		return Null, newExceptionError("TypeException", fmt.Sprintf("Invalid info with length %d", len(args[4].List)))
	}
	for _, option := range args[4].List {
		if option.Kind != ValueString {
			return Null, unsupportedCallError("WebServiceCallout.beginInvoke non-String option")
		}
	}
	if args[0].Kind == ValueNull {
		return Null, newExceptionError("CalloutException", "Web service callout failed: Callout stub not found: null")
	}
	if args[0].Kind != ValueObject || args[3].Kind != ValueObject || !strings.EqualFold(args[3].Type, "Continuation") {
		return Null, unsupportedCallError("WebServiceCallout.beginInvoke stub or continuation type")
	}
	if args[2].Kind != ValueObject || !strings.EqualFold(args[2].Type, "Type") {
		return Null, unsupportedCallError("WebServiceCallout.beginInvoke future type token")
	}
	futureType := typeValueText(args[2])
	if !vm.typeMatches(futureType, "System.WebServiceCalloutFuture", make(map[string]bool)) {
		if strings.EqualFold(strings.TrimPrefix(futureType, "System."), "String") {
			return Null, newExceptionError("CalloutException", " Incompatible return type specified.")
		}
		return Null, unsupportedCallError("WebServiceCallout.beginInvoke incompatible future type")
	}
	if err := vm.checkCalloutAllowed(); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("callouts", 1); err != nil {
		return Null, err
	}
	appendTrace(result, "apex.callout.webservice", "apex.callout", map[string]any{"operation": "WebServiceCallout.beginInvoke"})
	if vm.testContext == nil {
		// API 62/67 anonymous R028 reaches typemap validation after consuming
		// a callout. Actual asynchronous transport remains a hosted boundary.
		if _, _, found := vm.lookupField(runtimeObjectType(args[0]), "ns_map_type_info"); !found {
			return Null, newExceptionError("CalloutException", "Web service callout failed: Unable to find namespace-typemap info. Please regenerate the stub")
		}
		return Null, unsupportedCallError("WebServiceCallout.beginInvoke real network transport")
	}

	// Native R002/R004/R005: beginInvoke does not call WebServiceMock and
	// registers an empty HttpRequest, even without an installed mock.
	label, _, _, handled, err := callContinuationMember(args[3], "addHttpRequest", []Value{newHttpRequest()})
	if err != nil {
		return Null, err
	}
	if !handled || label.Kind != ValueString {
		return Null, unsupportedCallError("WebServiceCallout.beginInvoke continuation registration")
	}
	future, err := vm.constructValueWithLiteral(futureType, nil, nil, result, false)
	if err != nil {
		return Null, err
	}
	if future.Kind != ValueObject {
		return Null, unsupportedCallError("WebServiceCallout.beginInvoke future construction")
	}
	future.Fields[webServiceFutureLabelField] = label
	future.Fields[webServiceFutureResponseType] = args[4].List[6]
	future.Fields[webServiceFutureResponseNamespace] = args[4].List[4]
	future.Fields[webServiceFutureResponseName] = args[4].List[5]
	return future, nil
}

func (vm *VM) webServiceCalloutEndInvoke(args []Value, result *Result) (Value, error) {
	if len(args) != 1 {
		return Null, unsupportedCallError("WebServiceCallout.endInvoke argument count")
	}
	if args[0].Kind == ValueNull {
		return Null, newExceptionError("NullPointerException", "Argument 1 cannot be null")
	}
	if args[0].Kind != ValueObject || !vm.evalInstanceOf(args[0], "System.WebServiceCalloutFuture").Bool {
		return Null, unsupportedCallError("WebServiceCallout.endInvoke future type")
	}
	label, hasLabel := args[0].Fields[webServiceFutureLabelField]
	if !hasLabel {
		return Null, nil // R027: a directly constructed future has no operation.
	}
	if vm.testContext == nil {
		return Null, unsupportedCallError("WebServiceCallout.endInvoke real network transport")
	}
	responseType := args[0].Fields[webServiceFutureResponseType]
	responseNamespace := args[0].Fields[webServiceFutureResponseNamespace]
	responseName := args[0].Fields[webServiceFutureResponseName]
	if label.Kind != ValueString || responseType.Kind != ValueString || responseNamespace.Kind != ValueString || responseName.Kind != ValueString {
		return Null, unsupportedCallError("WebServiceCallout.endInvoke future metadata")
	}
	response, ok := vm.testContext.ContinuationResponses[label.Text]
	if !ok || response.Kind == ValueNull {
		return Null, newExceptionError("NullPointerException", "Attempt to de-reference a null object")
	}
	if response.Kind != ValueObject || !strings.EqualFold(response.Type, "HttpResponse") {
		return Null, unsupportedCallError("WebServiceCallout.endInvoke continuation response type")
	}
	return vm.decodeWebServiceResponse(httpBodyString(response).Text, responseType.Text,
		xml.Name{Space: responseNamespace.Text, Local: responseName.Text}, result)
}

// The oracle covers flat WSDL2Apex xsd:string responses only. Reject other
// schemas explicitly instead of deriving scalar or nested-object coercions.
func (vm *VM) decodeWebServiceResponse(body, responseType string, wrapper xml.Name, result *Result) (Value, error) {
	class, found := vm.lookupClass(responseType)
	if !found || len(class.Constructors) != 0 || class.SuperClass != "" && !strings.EqualFold(class.SuperClass, "Object") && !strings.EqualFold(class.SuperClass, "System.Object") {
		return Null, unsupportedCallError("WebServiceCallout.endInvoke response constructor or inheritance")
	}
	response, err := vm.constructValueWithLiteral(responseType, nil, nil, result, false)
	if err != nil {
		return Null, err
	}
	if response.Kind != ValueObject {
		return Null, unsupportedCallError("WebServiceCallout.endInvoke response schema")
	}
	fields, err := vm.webServiceStringResponseFields(response, wrapper.Space)
	if err != nil {
		return Null, err
	}
	values, err := webServiceSOAPStringValues(body, wrapper, fields)
	if err != nil {
		return Null, err
	}
	for name, storageName := range fields {
		value, found := values[name]
		if !found {
			value = Null
		}
		response.Fields[storageName] = value
	}
	return response, nil
}

func (vm *VM) webServiceStringResponseFields(response Value, namespace string) (map[xml.Name]string, error) {
	unsupported := func() (map[xml.Name]string, error) {
		return nil, unsupportedCallError("WebServiceCallout.endInvoke response schema (only flat xsd:string fields are captured)")
	}
	read := func(name string) (Value, bool) {
		field, owner, found := vm.lookupField(runtimeObjectType(response), name)
		if !found || !plainInstanceField(field) {
			return Null, false
		}
		_, value, found := vm.instanceFieldValue(response, owner, field)
		return value, found
	}
	schema, found := read("apex_schema_type_info")
	if !found || schema.Kind != ValueList || len(schema.List) != 3 || schema.List[0].Kind != ValueString || schema.List[0].Text != namespace ||
		schema.List[1].Kind != ValueString || schema.List[1].Text != "true" || schema.List[2].Kind != ValueString || schema.List[2].Text != "false" {
		return unsupported()
	}
	order, found := read("field_order_type_info")
	if !found || order.Kind != ValueList || len(order.List) == 0 {
		return unsupported()
	}
	fields := make(map[xml.Name]string, len(order.List))
	for _, item := range order.List {
		if item.Kind != ValueString || item.Text == "" {
			return unsupported()
		}
		field, owner, found := vm.lookupField(runtimeObjectType(response), item.Text)
		if !found || !plainInstanceField(field) || !strings.EqualFold(strings.TrimPrefix(field.Type, "System."), "String") ||
			field.InitializerLine != 0 || field.InitialValue.Kind != "" && field.InitialValue.Kind != ValueNull {
			return unsupported()
		}
		storageName, _, found := vm.instanceFieldValue(response, owner, field)
		if !found {
			return unsupported()
		}
		info, found := read(item.Text + "_type_info")
		if !found || info.Kind != ValueList || len(info.List) != 6 {
			return unsupported()
		}
		for _, part := range info.List {
			if part.Kind != ValueString {
				return unsupported()
			}
		}
		if info.List[0].Text == "" || info.List[1].Text != namespace || info.List[2].Text != "xsd:string" ||
			info.List[3].Text != "0" || info.List[4].Text != "1" || info.List[5].Text != "false" {
			return unsupported()
		}
		name := xml.Name{Space: info.List[1].Text, Local: info.List[0].Text}
		if _, duplicate := fields[name]; duplicate {
			return unsupported()
		}
		fields[name] = storageName
	}
	return fields, nil
}

func webServiceSOAPStringValues(body string, wrapper xml.Name, fields map[xml.Name]string) (map[xml.Name]Value, error) {
	if body == "" {
		return nil, newExceptionError("CalloutException", "IO Exception: input contained no data")
	}
	decoder := xml.NewDecoder(strings.NewReader(body))
	unsupported := func() error { return unsupportedCallError("WebServiceCallout.endInvoke SOAP response shape") }
	next := func() (xml.Token, error) {
		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			if text, ok := token.(xml.CharData); ok && strings.TrimSpace(string(text)) == "" {
				continue
			}
			return token, nil
		}
	}
	unexpected := func(want, got xml.Name) error {
		return newExceptionError("CalloutException", fmt.Sprintf("Web service callout failed: Unexpected element. Parser was expecting element '%s:%s' but found '%s:%s'", want.Space, want.Local, got.Space, got.Local))
	}
	for position, expected := range []xml.Name{{Space: webServiceSOAPNamespace, Local: "Envelope"}, {Space: webServiceSOAPNamespace, Local: "Body"}, wrapper} {
		token, err := next()
		if err != nil {
			return nil, unsupported()
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			return nil, unsupported()
		}
		if start.Name != expected {
			if position == 0 || position == 2 {
				return nil, unexpected(expected, start.Name)
			}
			return nil, unsupported()
		}
		for _, attr := range start.Attr {
			if !webServiceXMLNamespaceAttribute(attr) {
				return nil, unsupported()
			}
		}
	}
	values := make(map[xml.Name]Value, len(fields))
	for {
		token, err := next()
		if err != nil {
			return nil, unsupported()
		}
		if end, ok := token.(xml.EndElement); ok && end.Name == wrapper {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			return nil, unsupported()
		}
		if _, known := fields[start.Name]; !known {
			name := start.Name.Local
			if start.Name.Space != wrapper.Space {
				name = start.Name.Space + "=" + name
			}
			return nil, newExceptionError("CalloutException", "Web service callout failed: Unable to parse callout response. Apex type not found for element "+name)
		}
		nilValue := false
		for _, attr := range start.Attr {
			if webServiceXMLNamespaceAttribute(attr) {
				continue
			}
			if attr.Name.Space != "http://www.w3.org/2001/XMLSchema-instance" || attr.Name.Local != "nil" || attr.Value != "true" || nilValue {
				return nil, unsupported()
			}
			nilValue = true
		}
		var text strings.Builder
		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, unsupported()
			}
			switch token := token.(type) {
			case xml.CharData:
				text.Write(token)
			case xml.EndElement:
				if token.Name != start.Name || nilValue && text.Len() != 0 {
					return nil, unsupported()
				}
				if nilValue {
					values[start.Name] = Null
				} else {
					values[start.Name] = String(text.String())
				}
				goto fieldComplete
			default:
				return nil, unsupported()
			}
		}
	fieldComplete:
	}
	for _, expected := range []xml.Name{{Space: webServiceSOAPNamespace, Local: "Body"}, {Space: webServiceSOAPNamespace, Local: "Envelope"}} {
		token, err := next()
		end, ok := token.(xml.EndElement)
		if err != nil || !ok || end.Name != expected {
			return nil, unsupported()
		}
	}
	if _, err := next(); err != io.EOF {
		return nil, unsupported()
	}
	return values, nil
}

func webServiceXMLNamespaceAttribute(attr xml.Attr) bool {
	return attr.Name.Space == "xmlns" || attr.Name.Space == "" && attr.Name.Local == "xmlns"
}
