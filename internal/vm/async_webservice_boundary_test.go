package vm

import (
	"strings"
	"testing"
)

// The native oracle covers flat xsd:string responses. Other schemas must stay
// explicit unsupported boundaries rather than acquire guessed scalar coercions.
func TestAsyncWebserviceUncapturedResponseSchemasRemainUnsupported(t *testing.T) {
	for _, fieldType := range []string{"Boolean", "Integer", "Long", "Decimal", "Double", "Object"} {
		t.Run(fieldType, func(t *testing.T) {
			machine := New(nil)
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{Name: "SyntheticReply", Fields: map[string]Field{
				"value":                 {Name: "value", Type: fieldType},
				"apex_schema_type_info": {Name: "apex_schema_type_info", Type: "List<String>", InitialValue: List(String("urn:synthetic"), String("true"), String("false"))},
				"field_order_type_info": {Name: "field_order_type_info", Type: "List<String>", InitialValue: List(String("value"))},
				"value_type_info":       {Name: "value_type_info", Type: "List<String>", InitialValue: List(String("value"), String("urn:synthetic"), String("xsd:string"), String("0"), String("1"), String("false"))},
			}}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterClass(Class{Name: "SyntheticFuture", SuperClass: "System.WebServiceCalloutFuture"}); err != nil {
				t.Fatal(err)
			}
			future := Object("SyntheticFuture")
			future.Fields[webServiceFutureLabelField] = String("synthetic")
			future.Fields[webServiceFutureResponseType] = String("SyntheticReply")
			future.Fields["__webServiceCalloutResponseNamespace"] = String("urn:synthetic")
			future.Fields["__webServiceCalloutResponseName"] = String("Reply")
			response := Object("HttpResponse")
			response.Fields["body"] = String(`<SOAP:Envelope xmlns:SOAP="http://schemas.xmlsoap.org/soap/envelope/"><SOAP:Body><m:Reply xmlns:m="urn:synthetic"><m:value>1</m:value></m:Reply></SOAP:Body></SOAP:Envelope>`)
			machine.testContext.ContinuationResponses["synthetic"] = response
			_, err := machine.webServiceCalloutEndInvoke([]Value{future}, &Result{})
			if err == nil || !strings.Contains(err.Error(), "unsupported") {
				t.Fatalf("uncaptured %s response should be unsupported, got %v", fieldType, err)
			}
		})
	}
}
