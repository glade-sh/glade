package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
)

func TestHandleVisualforceRemotingDispatchesRemoteAction(t *testing.T) {
	srv := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  @RemoteAction
  public static String echo(String name) {
    return 'echo:' + name;
  }
}`)

	body := remotingEnvelopeWithViewState(t, srv, `[{"action":"AjaxController","method":"echo","data":["trail"],"type":"rpc","tid":3}]`)
	req := httptest.NewRequest(http.MethodPost, "/apex/Remote/remoting", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type = %q body=%s", got, rec.Body.String())
	}
	var responses []visualforce.RemotingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
		t.Fatalf("remoting response json: %v body=%s", err, rec.Body.String())
	}
	if len(responses) != 1 {
		t.Fatalf("responses = %#v", responses)
	}
	got := responses[0]
	if !got.Status || got.Action != "AjaxController" || got.Method != "echo" || got.Type != "rpc" || got.TID != 3 || got.Result != "echo:trail" {
		t.Fatalf("response = %#v", got)
	}
}

func TestHandleVisualforceRemotingConvertsDeclaredScalarsFromNativeRows(t *testing.T) {
	// The result and diagnostic texts are captured by V13's corresponding
	// r_invoke_* rows at API 59 and 67. Exercise dispatch, not just the decoder.
	srv := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  @RemoteAction public static String stringValue(String value) { return value; }
  @RemoteAction public static Boolean booleanValue(Boolean value) { return value; }
  @RemoteAction public static Integer integerValue(Integer value) { return value; }
  @RemoteAction public static Long longValue(Long value) { return value; }
  @RemoteAction public static Decimal decimalValue(Decimal value) { return value; }
  @RemoteAction public static Double doubleValue(Double value) { return value; }
  @RemoteAction public static Date dateValue(Date value) { return value; }
  @RemoteAction public static Datetime datetimeValue(Datetime value) { return value; }
  @RemoteAction public static Id idValue(Id value) { return value; }
}`)
	for _, row := range []struct {
		id, method, argument, result, message string
	}{
		{id: "r_invoke_string_text", method: "stringValue", argument: `"OWNED"`, result: `"OWNED"`},
		{id: "r_invoke_string_number", method: "stringValue", argument: `42`, result: `"42"`},
		{id: "r_invoke_string_boolean", method: "stringValue", argument: `true`, result: `"true"`},
		{id: "r_invoke_string_null", method: "stringValue", argument: `null`, result: `null`},
		{id: "r_invoke_boolean_string", method: "booleanValue", argument: `"true"`, result: `true`},
		{id: "r_invoke_boolean_false", method: "booleanValue", argument: `false`, result: `false`},
		{id: "r_invoke_integer_max", method: "integerValue", argument: `2147483647`, result: `2147483647`},
		{id: "r_invoke_integer_overflow", method: "integerValue", argument: `2147483648`, result: `null`, message: "Unable to convert number 2147483648 to Apex type Integer."},
		{id: "r_invoke_integer_text", method: "integerValue", argument: `"not-a-number"`, result: `null`, message: "Unable to convert number not-a-number to Apex type Integer."},
		{id: "r_invoke_long_string", method: "longValue", argument: `"1234567890123"`, result: `1234567890123`},
		{id: "r_invoke_decimal_string", method: "decimalValue", argument: `"12.50"`, result: `12.5`},
		{id: "r_invoke_double_text", method: "doubleValue", argument: `"NaN"`, result: `"NaN"`},
		{id: "r_invoke_date_leap", method: "dateValue", argument: `"2024-02-29"`, result: `null`, message: "Unable to convert date '2024-02-29' to Apex type Date."},
		{id: "r_invoke_date_null", method: "dateValue", argument: `null`, result: `null`},
		{id: "r_invoke_datetime_utc", method: "datetimeValue", argument: `"2024-02-29T12:34:56.000Z"`, result: `null`, message: "Unable to convert date '2024-02-29T12:34:56.000Z' to Apex type Datetime."},
		{id: "r_invoke_id_empty", method: "idValue", argument: `""`, result: `""`},
		{id: "r_invoke_id_invalid", method: "idValue", argument: `"invalid-owned-id"`, result: `null`, message: "Value 'invalid-owned-id' cannot be converted from String to Id."},
	} {
		t.Run(row.id, func(t *testing.T) {
			body := fmt.Sprintf(`[{"action":"AjaxController","method":%q,"data":[%s],"type":"rpc","tid":17}]`, row.method, row.argument)
			responses := postVisualforceRemotingWithViewState(t, srv, body)
			if len(responses) != 1 {
				t.Fatalf("responses = %#v", responses)
			}
			response := responses[0]
			if response.Status != (row.message == "") || response.Message != row.message {
				t.Fatalf("response = %#v, want message %q", response, row.message)
			}
			if response.Action != "AjaxController" || response.Method != row.method || response.Type != "rpc" || response.TID != 17 {
				t.Fatalf("response identity = %#v", response)
			}
			observed, err := json.Marshal(response.Result)
			if err != nil {
				t.Fatal(err)
			}
			if string(observed) != row.result {
				t.Fatalf("expected <%s> actual <%s>", row.result, observed)
			}
			if row.message != "" && (len(response.Errors) != 1 || response.Errors[0].Message != row.message) {
				t.Fatalf("errors = %#v, want %q", response.Errors, row.message)
			}
		})
	}
}

func TestHandleVisualforceRemotingReturnsNativeApexExceptionMessages(t *testing.T) {
	// V13 r_error_throw/divide_zero/bad_integer/null_argument/null_deref capture these
	// messages at API 59 and 67, including the null result and failed status.
	srv := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  public class OwnedException extends Exception {}
  @RemoteAction public static String throwMessage() { throw new OwnedException('V13_OWNED_EXCEPTION'); }
  @RemoteAction public static Integer divideZero() { Integer x=0; return 7/x; }
  @RemoteAction public static Integer badInteger() { return Integer.valueOf('V13_NOT_NUMBER'); }
  @RemoteAction public static String nullArgument(String value) { if(value==null)throw new OwnedException('V13_NULL_VALUE'); return value; }
  @RemoteAction public static Integer nullDeref() { String x=null; return x.length(); }
}`)
	for _, row := range []struct {
		id, method, arguments, message string
	}{
		{"r_error_throw", "throwMessage", `[]`, "V13_OWNED_EXCEPTION"},
		{"r_error_divide_zero", "divideZero", `[]`, "Divide by 0"},
		{"r_error_bad_integer", "badInteger", `[]`, "Invalid integer: V13_NOT_NUMBER"},
		{"r_error_null_argument", "nullArgument", `[null]`, "V13_NULL_VALUE"},
		{"r_error_null_deref", "nullDeref", `[]`, "Attempt to de-reference a null object"},
	} {
		t.Run(row.id, func(t *testing.T) {
			body := fmt.Sprintf(`[{"action":"AjaxController","method":%q,"data":%s,"type":"rpc","tid":23}]`, row.method, row.arguments)
			responses := postVisualforceRemotingWithViewState(t, srv, body)
			if len(responses) != 1 {
				t.Fatalf("responses = %#v", responses)
			}
			response := responses[0]
			if response.Status || response.Result != nil || response.Message != row.message {
				t.Fatalf("response = %#v, want message <%s> and raw null result", response, row.message)
			}
			if response.Action != "AjaxController" || response.Method != row.method || response.Type != "rpc" || response.TID != 23 {
				t.Fatalf("response identity = %#v", response)
			}
			if len(response.Errors) != 1 || response.Errors[0].Message != row.message {
				t.Fatalf("errors = %#v, want message <%s>", response.Errors, row.message)
			}
		})
	}
}

func TestHandleVisualforceRemotingReturnsNativeCollectionAndObjectShapes(t *testing.T) {
	// These are V13's captured r_invoke_liststring_*, mapstringstring_*,
	// account_* and owneddto_* rows, including their empty and null controls.
	srv := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  public class OwnedDTO { public String text; public Integer count; }
  @RemoteAction public static List<String> listValue(List<String> value) { return value; }
  @RemoteAction public static Map<String,String> mapValue(Map<String,String> value) { return value; }
  @RemoteAction public static Account accountValue(Account value) { return value; }
  @RemoteAction public static OwnedDTO dtoValue(OwnedDTO value) { return value; }
}`)
	for _, row := range []struct {
		id, method, argument, result string
	}{
		{"r_invoke_liststring_two", "listValue", `["A","B"]`, `["A","B"]`},
		{"r_invoke_liststring_empty", "listValue", `[]`, `[]`},
		{"r_invoke_liststring_null", "listValue", `null`, `null`},
		{"r_invoke_liststring_null_item", "listValue", `["A",null]`, `["A"]`},
		{"r_invoke_liststring_numbers", "listValue", `[1,2]`, `["1","2"]`},
		{"r_invoke_mapstringstring_normal", "mapValue", `{"b":"B","a":"A"}`, `{"a":"A","b":"B"}`},
		{"r_invoke_mapstringstring_empty", "mapValue", `{}`, `{}`},
		{"r_invoke_mapstringstring_null", "mapValue", `null`, `null`},
		{"r_invoke_mapstringstring_null_value", "mapValue", `{"a":null}`, `{}`},
		{"r_invoke_account_name", "accountValue", `{"Name":"V13_OWNED"}`, `{"Name":"V13_OWNED"}`},
		{"r_invoke_account_empty", "accountValue", `{}`, `{}`},
		{"r_invoke_account_null", "accountValue", `null`, `null`},
		{"r_invoke_account_phone", "accountValue", `{"Name":"V13_OWNED","Phone":"5550100"}`, `{"Name":"V13_OWNED","Phone":"5550100"}`},
		{"r_invoke_owneddto_normal", "dtoValue", `{"text":"V13_OWNED","count":7}`, `{"count":7,"text":"V13_OWNED"}`},
		{"r_invoke_owneddto_empty", "dtoValue", `{}`, `{}`},
		{"r_invoke_owneddto_null", "dtoValue", `null`, `null`},
		{"r_invoke_owneddto_conversion", "dtoValue", `{"text":123,"count":"7"}`, `{"count":7,"text":"123"}`},
	} {
		t.Run(row.id, func(t *testing.T) {
			body := fmt.Sprintf(`[{"action":"AjaxController","method":%q,"data":[%s],"type":"rpc","tid":29}]`, row.method, row.argument)
			responses := postVisualforceRemotingWithViewState(t, srv, body)
			if len(responses) != 1 || !responses[0].Status {
				t.Fatalf("responses = %#v", responses)
			}
			response := responses[0]
			if response.Action != "AjaxController" || response.Method != row.method || response.Type != "rpc" || response.TID != 29 {
				t.Fatalf("response identity = %#v", response)
			}
			observed, err := json.Marshal(response.Result)
			if err != nil {
				t.Fatal(err)
			}
			if string(observed) != row.result {
				t.Fatalf("expected <%s> actual <%s>", row.result, observed)
			}
		})
	}
}

func TestHandleVisualforceRemotingRejectsMissingViewState(t *testing.T) {
	srv := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  @RemoteAction
  public static String echo(String name) {
    return 'echo:' + name;
  }
}`)

	responses := postVisualforceRemoting(t, srv, `[{"action":"AjaxController","method":"echo","data":["trail"],"type":"rpc","tid":3}]`)
	if len(responses) != 1 {
		t.Fatalf("responses = %#v", responses)
	}
	got := responses[0]
	if got.Status || got.TID != 3 || !strings.Contains(got.Message, "missing Visualforce view state") {
		t.Fatalf("response = %#v", got)
	}
}

func TestHandleVisualforceRemotingAcceptsBrowserManagerEnvelope(t *testing.T) {
	srv := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"><apex:form /></apex:page>`, `public class AjaxController {
  @RemoteAction
  public static String echo(String name) {
    return 'echo:' + name;
  }
}`)
	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Remote", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("initial status = %d body=%s", first.Code, first.Body.String())
	}
	envelope := fmt.Sprintf(`[{"action":"AjaxController","method":"echo","data":["trail"],"type":"rpc","tid":1,"ctx":{"page":"/apex/Remote","viewState":%q,"csrf":%q}}]`,
		extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName()),
		extractHTMLInput(first.Body.String(), "__vf_csrf"),
	)

	responses := postVisualforceRemoting(t, srv, envelope)
	if len(responses) != 1 {
		t.Fatalf("responses = %#v", responses)
	}
	got := responses[0]
	if !got.Status || got.Action != "AjaxController" || got.Method != "echo" || got.Type != "rpc" || got.TID != 1 || got.Result != "echo:trail" {
		t.Fatalf("response = %#v", got)
	}
}

func TestHandleVisualforceRemotingReturnsEnvelopeForMissingMethod(t *testing.T) {
	srv := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  @RemoteAction
  public static String echo(String name) {
    return 'echo:' + name;
  }
}`)

	responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"missing","data":[],"type":"rpc","tid":4}]`)
	if len(responses) != 1 {
		t.Fatalf("responses = %#v", responses)
	}
	got := responses[0]
	if got.Status || got.Action != "AjaxController" || got.Method != "missing" || got.TID != 4 || !strings.Contains(got.Message, "action not found") {
		t.Fatalf("response = %#v", got)
	}
}

func TestHandleVisualforceRemotingRejectsInstanceRemoteActionController(t *testing.T) {
	// Native e_annotation_instance at API 59/67 rejects the class and its page;
	// an instance RemoteAction cannot reach the remoting request handler.
	srv := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  @RemoteAction
  public String echo() {
    return 'echo';
  }
}`)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/Remote", nil))
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var errors []struct {
		Code    string `json:"errorCode"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errors); err != nil {
		t.Fatalf("page rejection json: %v body=%s", err, rec.Body.String())
	}
	if len(errors) != 1 || errors[0].Code != "UNSUPPORTED_FEATURE" || errors[0].Message != "Apex class 'AjaxController' does not exist" {
		t.Fatalf("page rejection = %#v", errors)
	}
}

func TestHandleVisualforceRemotingAcceptsObjectAndArrayParameters(t *testing.T) {
	srv := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  @RemoteAction
  public static String inspect(Map<String, Object> payload, List<Object> values) {
    return String.valueOf(payload.get('name')) + ':' + String.valueOf(values.size());
  }
}`)

	responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"inspect","data":[{"name":"trail"},["one","two"]],"type":"rpc","tid":6}]`)
	if len(responses) != 1 {
		t.Fatalf("responses = %#v", responses)
	}
	got := responses[0]
	if !got.Status || got.Action != "AjaxController" || got.Method != "inspect" || got.TID != 6 || got.Result != "trail:2" {
		t.Fatalf("response = %#v", got)
	}
}

func TestHandleVisualforceRemotingFailureEnvelopeKeepsStableShape(t *testing.T) {
	srv := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  @RemoteAction
  public static String echo(String name) {
    return 'echo:' + name;
  }
}`)

	responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"missing","data":[],"type":"rpc","tid":9}]`)
	if len(responses) != 1 {
		t.Fatalf("responses = %#v", responses)
	}
	body, err := json.Marshal(responses[0])
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"action", "method", "type", "tid", "status", "message", "errors"} {
		if _, ok := envelope[key]; !ok {
			t.Fatalf("envelope missing %q: %s", key, body)
		}
	}
	errorsValue, ok := envelope["errors"].([]any)
	if !ok || len(errorsValue) != 1 {
		t.Fatalf("errors = %#v body=%s", envelope["errors"], body)
	}
	errorObject, ok := errorsValue[0].(map[string]any)
	if !ok || errorObject["message"] != envelope["message"] {
		t.Fatalf("error object = %#v envelope=%#v", errorsValue[0], envelope)
	}
	if envelope["action"] != "AjaxController" || envelope["method"] != "missing" || envelope["type"] != "rpc" || envelope["tid"] != float64(9) || envelope["status"] != false {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestHandleVisualforceRemoteObjectsDispatchesCRUD(t *testing.T) {
	srv := newVisualforceRemotingFixtureServer(t, "RemoteObjects.page", `<apex:page>
  <apex:remoteObjects>
    <apex:remoteObjectModel name="Account" fields="Id,Name"/>
  </apex:remoteObjects>
</apex:page>`, "")
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.KeyPrefix = "001"
	account.Definition.Fields["Id"] = storage.Field{APIName: "Id", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString, Required: true, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Records = map[storage.ID]storage.Record{}
	org.Objects["Account"] = account
	srv.Org = &org
	configureVisualforceTestPrincipal(t, srv)

	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/RemoteObjects", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("initial status = %d body=%s", first.Code, first.Body.String())
	}
	body := fmt.Sprintf(`{"operation":"create","objectName":"Account","fields":{"Name":"Acme"},"viewState":%q,"csrf":%q}`,
		extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName()),
		extractHTMLInput(first.Body.String(), "__vf_csrf"),
	)
	req := httptest.NewRequest(http.MethodPost, "/apex/RemoteObjects/remoteObjects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var result visualforce.RemoteObjectCRUDResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("remote objects response json: %v body=%s", err, rec.Body.String())
	}
	if !result.Success || len(result.IDs) != 1 || !strings.HasPrefix(result.IDs[0], "001") {
		t.Fatalf("result = %#v", result)
	}
	if len(srv.Org.Objects["Account"].Records) != 1 {
		t.Fatalf("records = %#v", srv.Org.Objects["Account"].Records)
	}
}

func TestVisualforceRemoteObjectsRejectsEditedPageExpressionBeforeCRUD(t *testing.T) {
	// Native expression/diagnostic: V02 e_broken_operator at API59/API67 in
	// the native expression capture. Valid view state must not
	// let the local CRUD route bypass this rejection after a page edit.
	const markup = `<apex:page>
  <apex:remoteObjects>
    <apex:remoteObjectModel name="Account" fields="Id,Name"/>
  </apex:remoteObjects>
  <apex:outputText value="{!1}"/>
</apex:page>`
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			srv := newVisualforceRemotingFixtureServer(t, "RemoteObjects.page", markup, "")
			pageFile := filepath.Join(srv.Source.Project.Root, "force-app/main/default/pages/RemoteObjects.page")
			writeServerTestFile(t, pageFile+"-meta.xml", fmt.Sprintf(`<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><label>RemoteObjects</label></ApexPage>`, api))
			org := testOrg()
			account := org.Objects["Account"]
			account.Definition.KeyPrefix = "001"
			account.Definition.Fields["Id"] = storage.Field{APIName: "Id", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
			account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString, Required: true, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
			account.Records = map[storage.ID]storage.Record{}
			org.Objects["Account"] = account
			srv.Org = &org
			configureVisualforceTestPrincipal(t, srv)

			first := httptest.NewRecorder()
			srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/RemoteObjects", nil))
			if first.Code != http.StatusOK {
				t.Fatalf("initial status = %d body=%s", first.Code, first.Body.String())
			}
			viewState := extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName())
			csrf := extractHTMLInput(first.Body.String(), "__vf_csrf")
			if viewState == "" || csrf == "" {
				t.Fatalf("initial page did not produce view state and CSRF: %s", first.Body.String())
			}
			writeServerTestFile(t, pageFile, strings.Replace(markup, "{!1}", "{!1 +}", 1))
			body := fmt.Sprintf(`{"operation":"create","objectName":"Account","fields":{"Name":"Acme"},"viewState":%q,"csrf":%q}`, viewState, csrf)
			req := httptest.NewRequest(http.MethodPost, "/apex/RemoteObjects/remoteObjects", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			var result visualforce.RemoteObjectCRUDResult
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatalf("remote objects response json: %v body=%s", err, rec.Body.String())
			}
			if result.Success || len(result.Errors) != 1 || result.Errors[0].StatusCode != "UNSUPPORTED_FEATURE" || result.Errors[0].Message != "Syntax error.  Found 'end of formula'" {
				t.Fatalf("result = %#v; want the exact native e_broken_operator rejection", result)
			}
			if len(result.IDs) != 0 || len(srv.Org.Objects["Account"].Records) != 0 {
				t.Fatalf("rejected expression dispatched CRUD: result=%#v records=%#v", result, srv.Org.Objects["Account"].Records)
			}
		})
	}
}

func TestHandleVisualforceRemoteObjectsRejectsMissingViewState(t *testing.T) {
	srv := newVisualforceRemotingFixtureServer(t, "RemoteObjects.page", `<apex:page>
  <apex:remoteObjects>
    <apex:remoteObjectModel name="Account" fields="Id,Name"/>
  </apex:remoteObjects>
</apex:page>`, "")
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.KeyPrefix = "001"
	account.Definition.Fields["Id"] = storage.Field{APIName: "Id", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString, Required: true, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Records = map[storage.ID]storage.Record{}
	org.Objects["Account"] = account
	srv.Org = &org
	configureVisualforceTestPrincipal(t, srv)

	req := httptest.NewRequest(http.MethodPost, "/apex/RemoteObjects/remoteObjects", strings.NewReader(`{"operation":"create","objectName":"Account","fields":{"Name":"Acme"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var result visualforce.RemoteObjectCRUDResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("remote objects response json: %v body=%s", err, rec.Body.String())
	}
	if result.Success || len(result.Errors) == 0 || !strings.Contains(result.Errors[0].Message, "missing Visualforce view state") {
		t.Fatalf("result = %#v", result)
	}
	if len(srv.Org.Objects["Account"].Records) != 0 {
		t.Fatalf("records = %#v", srv.Org.Objects["Account"].Records)
	}
}

func TestHandleVisualforceRemotingRejectsOversizedBodyBeforeDecode(t *testing.T) {
	srv := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  @RemoteAction
  public static String echo(String name) {
    return 'echo:' + name;
  }
}`)
	body := strings.Repeat("x", visualforce.MaxVisualforceRemotingRequestBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/apex/Remote/remoting", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK || !strings.Contains(rec.Body.String(), "request body too large") {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func newVisualforceRemotingFixtureServer(t *testing.T, pageName, pageMarkup, controllerSource string) *Server {
	t.Helper()
	srv := newVisualforceFixtureServer(t, pageName, pageMarkup, controllerSource)
	configureVisualforceTestPrincipal(t, srv)
	return srv
}

func postVisualforceRemoting(t *testing.T, srv *Server, body string) []visualforce.RemotingResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/apex/Remote/remoting", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var responses []visualforce.RemotingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
		t.Fatalf("remoting response json: %v body=%s", err, rec.Body.String())
	}
	return responses
}

func postVisualforceRemotingWithViewState(t *testing.T, srv *Server, body string) []visualforce.RemotingResponse {
	t.Helper()
	return postVisualforceRemoting(t, srv, remotingEnvelopeWithViewState(t, srv, body))
}

func remotingEnvelopeWithViewState(t *testing.T, srv *Server, body string) string {
	t.Helper()
	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Remote", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("initial status = %d body=%s", first.Code, first.Body.String())
	}
	var requests []map[string]any
	if err := json.Unmarshal([]byte(body), &requests); err != nil {
		t.Fatalf("request json: %v", err)
	}
	ctx := map[string]any{
		"page":      "/apex/Remote",
		"viewState": extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName()),
		"csrf":      extractHTMLInput(first.Body.String(), "__vf_csrf"),
	}
	for i := range requests {
		requests[i]["ctx"] = ctx
	}
	data, err := json.Marshal(requests)
	if err != nil {
		t.Fatalf("request json marshal: %v", err)
	}
	return string(data)
}
