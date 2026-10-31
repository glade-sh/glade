package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestVisualforceOnDemandRemoting(t *testing.T) {
	host := newVisualforceRemotingFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController { @RemoteAction public static String echo(String name) { return 'echo:' + name; } }`)
	if host.Index == nil {
		t.Fatal("fixture has no project index")
	}
	host.SetProjectRuntime(*host.Index, nil, nil)
	responses := postVisualforceRemotingWithViewState(t, host, `[{"action":"AjaxController","method":"echo","data":["trail"],"type":"rpc","tid":3}]`)
	if len(responses) != 1 || !responses[0].Status || responses[0].Result != "echo:trail" {
		t.Fatalf("responses = %#v", responses)
	}
}

func TestVisualforceOnDemandLookup(t *testing.T) {
	host := newVisualforceLookupRouteServer(t, vfAccessOwnerID, "Owned User")
	sch, err := schema.LoadProject(host.Source.Project)
	if err != nil {
		t.Fatal(err)
	}
	host.SetProjectRuntime(typesys.Build(host.Source.Project, sch), nil, nil)
	response := requestVisualforceLookupRoute(host, "/record/User/"+string(vfAccessOwnerID), "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Owned User") {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}
