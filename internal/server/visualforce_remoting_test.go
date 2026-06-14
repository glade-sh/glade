package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
)

func TestHandleVisualforceRemotingDispatchesRemoteAction(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
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

func TestHandleVisualforceRemotingRejectsMissingViewState(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
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
	srv := newVisualforceFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"><apex:form /></apex:page>`, `public class AjaxController {
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
	srv := newVisualforceFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
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

func TestHandleVisualforceRemotingReturnsEnvelopeForUnsupportedRemoteAction(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  @RemoteAction
  public String echo() {
    return 'echo';
  }
}`)

	responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"echo","data":[],"type":"rpc","tid":5}]`)
	if len(responses) != 1 {
		t.Fatalf("responses = %#v", responses)
	}
	got := responses[0]
	if got.Status || got.Action != "AjaxController" || got.Method != "echo" || got.TID != 5 || !strings.Contains(got.Message, "must be static") {
		t.Fatalf("response = %#v", got)
	}
}

func TestHandleVisualforceRemotingAcceptsObjectAndArrayParameters(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
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
	srv := newVisualforceFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
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
	srv := newVisualforceFixtureServer(t, "RemoteObjects.page", `<apex:page>
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

func TestHandleVisualforceRemoteObjectsRejectsMissingViewState(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "RemoteObjects.page", `<apex:page>
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
	srv := newVisualforceFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
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
