package server

import (
	"net/http/httptest"
	"testing"

	"github.com/glade-sh/glade/internal/lwcshell"
)

func TestB11LWRFlowContext(t *testing.T) {
	route := lwcShellContextFromQuery(httptest.NewRequest("GET", "/lwc/preview/flow/Test?lwr=true&site=Example&siteId=site&networkId=network&guest=true&language=en-US", nil))
	ctx := mergeFlowRouteContext(lwcshell.PageContext{}, route)
	if !ctx.Community.LWR || ctx.Community.Site != "Example" || ctx.Community.SiteID != "site" || ctx.Community.NetworkID != "network" || !ctx.Community.Guest || ctx.Community.Language != "en-US" {
		t.Fatalf("flow context lost community data: %+v", ctx.Community)
	}
}

func TestB11LWRFlowGuestFalseOverridesPreset(t *testing.T) {
	route := lwcShellContextFromQuery(httptest.NewRequest("GET", "/lwc/preview/flow/Test?lwr=true&guest=false", nil))
	preset := lwcshell.PageContext{Community: lwcshell.CommunityContext{Guest: true}}
	if got := mergeFlowRouteContext(preset, route); got.Community.Guest {
		t.Fatal("explicit guest=false did not override the preset")
	}
}

func TestB11LWRFlowDifferentSiteDoesNotInheritPreset(t *testing.T) {
	preset := lwcshell.PageContext{Community: lwcshell.CommunityContext{Site: "Origin", SiteID: "origin-site", NetworkID: "origin-network", BasePath: "/origin", Guest: true}}
	route := lwcShellContextFromQuery(httptest.NewRequest("GET", "/lwc/preview/flow/Test?lwr=true&site=Destination", nil))
	got := mergeFlowRouteContext(preset, route).Community
	if got.Site != "Destination" || got.SiteID != "" || got.NetworkID != "" || got.BasePath != "" || got.Guest {
		t.Fatalf("cross-site flow inherited preset: %+v", got)
	}
}
