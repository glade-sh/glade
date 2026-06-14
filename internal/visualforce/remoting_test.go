package visualforce

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("%q contains %q", haystack, needle)
	}
}

func TestRemotingDiscoveryAndEnvelopeDispatch(t *testing.T) {
	page := Page{Controller: "RemoteController", Extensions: []string{"AuditExtension"}}
	idx := typesys.Index{Types: []typesys.TypeSymbol{
		{Name: "RemoteController", Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationMethod, Name: "echo", Modifiers: []string{"public", "static", "@RemoteAction"}},
			{Kind: apexast.DeclarationMethod, Name: "helper", Modifiers: []string{"public", "static"}},
		}},
		{Name: "AuditExtension", Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationMethod, Name: "stamp", Modifiers: []string{"global", "static", "RemoteAction"}},
		}},
		{Name: "OtherController", Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationMethod, Name: "hidden", Modifiers: []string{"public", "static", "RemoteAction"}},
		}},
	}}

	metadata, err := BuildRemotingMetadataFromIndex(page, idx)
	if err != nil {
		t.Fatalf("BuildRemotingMetadataFromIndex err = %v", err)
	}
	if len(metadata.Actions) != 2 || metadata.Actions[0].Action != "AuditExtension.stamp" || metadata.Actions[1].Action != "RemoteController.echo" {
		t.Fatalf("actions = %#v, want controller and extension remote actions", metadata.Actions)
	}

	responses := DispatchRemotingRequests(metadata, []RemotingRequest{{
		Action: `{!$RemoteAction.RemoteController.echo}`,
		Type:   "rpc",
		TID:    7,
		Data:   []json.RawMessage{json.RawMessage(`"x"`)},
	}}, func(invocation RemotingInvocation) (any, error) {
		if invocation.Action.Action != "RemoteController.echo" || len(invocation.Arguments) != 1 || string(invocation.Arguments[0]) != `"x"` {
			t.Fatalf("invocation = %#v", invocation)
		}
		return "echo:x", nil
	})

	if len(responses) != 1 {
		t.Fatalf("responses = %#v", responses)
	}
	response := responses[0]
	if !response.Status || response.Result != "echo:x" || response.Action != "RemoteController" || response.Method != "echo" || response.Type != "rpc" || response.TID != 7 {
		t.Fatalf("response = %#v", response)
	}
}

func TestRemotingExposureRequiresRemoteActionStaticPublicOrGlobal(t *testing.T) {
	method := RemoteActionMethod{
		ClassName:   "RemoteController",
		MethodName:  "echo",
		Annotations: []string{"RemoteAction"},
		Modifiers:   []string{"public", "static"},
	}
	if err := ValidateRemoteActionExposure(method); err != nil {
		t.Fatalf("valid remote action rejected: %v", err)
	}

	method.Modifiers = []string{"public"}
	if err := ValidateRemoteActionExposure(method); err == nil || !strings.Contains(err.Error(), "static") {
		t.Fatalf("err = %v, want static diagnostic", err)
	}

	method.Modifiers = []string{"private", "static"}
	if err := ValidateRemoteActionExposure(method); err == nil || !strings.Contains(err.Error(), "public or global") {
		t.Fatalf("err = %v, want visibility diagnostic", err)
	}
}

func TestRemotingMetadataExposesPageControllersAndExtensions(t *testing.T) {
	page := Page{Controller: "RemoteController", Extensions: []string{"AuditExtension"}}
	actions := []RemoteActionMethod{
		{ClassName: "RemoteController", MethodName: "echo", Annotations: []string{"RemoteAction"}, Modifiers: []string{"public", "static"}},
		{ClassName: "AuditExtension", MethodName: "stamp", Annotations: []string{"RemoteAction"}, Modifiers: []string{"global", "static"}},
		{ClassName: "OtherController", MethodName: "hidden", Annotations: []string{"RemoteAction"}, Modifiers: []string{"public", "static"}},
	}

	metadata, err := BuildRemotingMetadata(page, actions)
	if err != nil {
		t.Fatalf("BuildRemotingMetadata err = %v", err)
	}
	if len(metadata.Actions) != 2 {
		t.Fatalf("actions = %#v, want controller and extension actions", metadata.Actions)
	}
	script := RenderRemotingMetadataScript(metadata)
	assertContains(t, script, `Visualforce.remoting.Manager.invokeAction`)
	assertContains(t, script, `RemoteController.echo`)
	assertContains(t, script, `AuditExtension.stamp`)
	assertNotContains(t, script, `OtherController.hidden`)
}

func TestRemotingMetadataScriptInstallsLocalBrowserManager(t *testing.T) {
	metadata := RemotingMetadata{Actions: []RemoteActionDescriptor{{
		ClassName:  "RemoteController",
		MethodName: "echo",
		Action:     "RemoteController.echo",
	}}}

	script := RenderRemotingMetadataScript(metadata)
	assertNotContains(t, script, `throw new Error("Visualforce remoting dispatch is not bound")`)
	assertContains(t, script, `fetch(window.location.pathname.replace(/\/$/,"")+"/remoting"`)
	assertContains(t, script, `"Content-Type":"application/json"`)
	assertContains(t, script, `body:JSON.stringify([request])`)
	assertContains(t, script, `ctx:{page:window.location.pathname,viewState:read("`+ViewStateFormFieldName()+`"),csrf:read("__vf_csrf")}`)
	assertContains(t, script, `var isOptions=function(value){return value&&typeof value=="object"&&!Array.isArray(value)&&("escape" in value||"timeout" in value||"buffer" in value||"abortable" in value);}`)
	assertContains(t, script, `if(callback&&values.length&&isOptions(values[values.length-1])){values.pop();}`)
	assertContains(t, script, `tid:Visualforce.remoting.Manager._tid++`)
	assertContains(t, script, `status:!!(response&&response.status)`)
	assertContains(t, script, `callback(response?response.result:null,event)`)
	assertContains(t, script, `return response;`)
	assertContains(t, script, `RemoteController.echo=function()`)
}

func TestRemotingRequestLimitAndTimeoutBounds(t *testing.T) {
	if err := ValidateRemotingRequest(make([]byte, MaxVisualforceRemotingRequestBytes+1)); err == nil {
		t.Fatal("oversized remoting request was accepted")
	}
	if timeout, err := NormalizeRemotingTimeout(0); err != nil || timeout != DefaultVisualforceRemotingTimeout {
		t.Fatalf("default timeout = %s, %v", timeout, err)
	}
	if _, err := NormalizeRemotingTimeout(MaxVisualforceRemotingTimeout + time.Millisecond); err == nil || !strings.Contains(err.Error(), "120s") {
		t.Fatalf("err = %v, want max timeout diagnostic", err)
	}
}
