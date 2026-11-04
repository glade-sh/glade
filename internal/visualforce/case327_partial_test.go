package visualforce

import (
	"reflect"
	"strings"
	"testing"
)

func TestCase327PartialResponseProjection(t *testing.T) {
	html := `<div id="f:target">first</div><div id="target">exact</div><div id="f:target">duplicate</div><span data-rerender="fallback">fallback</span><div id="outside" data-vf-poller="true" data-enabled="true" data-interval="5"></div>`
	for _, ids := range [][]string{nil, {" target ", "target", "fallback", "missing", " ", "f:target"}, {"outside"}} {
		got := NewPartialResponse(html, "state", ids)
		targets, messages := RenderPartialTargetsWithDiagnostics(html, ids)
		if !reflect.DeepEqual(got.Targets, targets) || !reflect.DeepEqual(got.Messages, messages) || !reflect.DeepEqual(got.Pollers, partialPollerStates(html)) || got.ViewState != "state" {
			t.Fatalf("partial response differs for %q: %#v", ids, got)
		}
		if len(ids) > 1 && (!strings.Contains(got.Targets["target"], "exact") || !strings.Contains(got.Targets["f:target"], "first")) {
			t.Fatalf("target precedence changed: %#v", got.Targets)
		}
		if got.Pollers["outside"] != (AjaxPollerState{Enabled: true, Interval: "5"}) {
			t.Fatalf("poller lost: %#v", got.Pollers)
		}
	}
}

func TestCase327PartialProjectionWorkload(t *testing.T) {
	html := strings.Repeat(`<div id="f:item"><span>value</span></div>`, 1000) + `<div id="poll" data-vf-poller="true" data-enabled="true" data-interval="5"></div>`
	for i := 0; i < 40; i++ {
		got := NewPartialResponse(html, "state", []string{"item", "missing"})
		if !strings.Contains(got.Targets["f:item"], "value") || !got.Pollers["poll"].Enabled || len(got.Messages) != 1 {
			t.Fatal("unexpected projection")
		}
	}
}
