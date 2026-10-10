package vm

import (
	"fmt"
	"testing"
)

func TestExecExternalServiceCallbackRejectsUnregisteredURI(t *testing.T) {
	const endpoint = "https://example.test/callback"
	const message = "Callback uri is invalid: https://example.test/callback. Specify a valid callback uri."

	cases := []struct {
		name     string
		request  string
		withMock bool
	}{
		{name: "original endpoint only"},
		{name: "configured without registration", request: "request.setMethod('POST'); request.setHeader('Content-Type', 'application/json'); request.setBody('{\"owned\":true}');"},
		{name: "configured with HTTP mock", request: "request.setMethod('POST'); request.setHeader('Content-Type', 'application/json'); request.setBody('{\"owned\":true}');", withMock: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockSetup := ""
			mockAssertion := ""
			if tc.withMock {
				mockSetup = "Test.setMock(HttpCalloutMock.class, new CallbackMock());"
				mockAssertion = "System.assertEquals(0, CallbackMock.calls);"
			}
			program, err := CompileAnonymous(fmt.Sprintf(`
%s
HttpRequest request = new HttpRequest();
request.setEndpoint('%s');
%s
Test.startTest();
try {
  Test.getExternalService().sendCallback(request);
  System.assert(false, 'expected invalid callback URI');
} catch (Exception e) {
  System.assertEquals('System.InvalidParameterValueException', e.getTypeName());
  System.assertEquals('%s', e.getMessage());
  System.assertEquals(0, Limits.getCallouts());
}
Test.stopTest();
%s
`, mockSetup, endpoint, tc.request, message, mockAssertion))
			if err != nil {
				t.Fatal(err)
			}

			machine := New(nil)
			machine.EnableTestContext()
			if tc.withMock {
				respond, err := CompileAnonymous(`
CallbackMock.calls++;
HttpResponse response = new HttpResponse();
response.setStatusCode(201);
return response;
`)
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{
					Name:       "CallbackMock",
					Interfaces: []string{"HttpCalloutMock"},
					StaticFields: map[string]Field{
						"calls": {Name: "calls", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)},
					},
				}); err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{
					Name:       "CallbackMock.respond",
					ClassName:  "CallbackMock",
					ReturnType: "HttpResponse",
					Params:     []Param{{Name: "request", Type: "HttpRequest"}},
					Program:    respond,
				}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExecLocalExternalServiceCallbacksReturnSuccess(t *testing.T) {
	program, err := CompileAnonymous(`
HttpResponse callback = Test.getExternalService().sendCallback(new HttpRequest());
System.assertEquals(200, callback.getStatusCode());
HttpResponse asyncResponse = new TestAsyncHttp().executeHttpRequest(new HttpRequest());
System.assertEquals(200, asyncResponse.getStatusCode());
`)
	if err != nil {
		t.Fatal(err)
	}

	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
