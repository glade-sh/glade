package lwcbrowser

import (
	"strings"
	"testing"
)

func TestBootstrapHTMLDefinesLightningStub(t *testing.T) {
	html := BootstrapHTML(PageConfig{})
	if !strings.Contains(html, "window.$Lightning") {
		t.Fatalf("missing $Lightning stub in bootstrap:\n%s", html)
	}
	if !strings.Contains(html, "__gladeLightningPending") {
		t.Fatalf("missing pending queue in bootstrap:\n%s", html)
	}
	if strings.Index(html, "window.$Lightning") >= strings.Index(html, `type="module"`) {
		t.Fatalf("expected synchronous stub before module script:\n%s", html)
	}
}

func TestBootstrapHTMLIncludesLocalLWCImportMap(t *testing.T) {
	html := BootstrapHTML(PageConfig{
		Namespace: "verifiable",
		Manifest: Manifest{
			Modules: map[string]ModuleEntry{
				"verifiable:landing": {
					URL: "/lightning/modules/verifiable/landing/landing.js",
					Tag: "verifiable-landing",
				},
			},
		},
	})
	if !strings.Contains(html, `"c/landing":"/lightning/modules/verifiable/landing/landing.js"`) {
		t.Fatalf("missing c/landing import map entry:\n%s", html)
	}
	if !strings.Contains(html, `"verifiable/landing":"/lightning/modules/verifiable/landing/landing.js"`) {
		t.Fatalf("missing verifiable/landing import map entry:\n%s", html)
	}
}

func TestBootstrapHTMLIncludesOutAppDependencies(t *testing.T) {
	html := BootstrapHTML(PageConfig{
		Namespace: "c",
		OutApps:   []string{"c:lightningout"},
		OutAppDependencies: map[string][]string{
			"c:lightningout": {"c:apexwirehost", "c:recordwirehost"},
		},
		Manifest: Manifest{
			Modules: map[string]ModuleEntry{
				"c:apexwirehost": {
					URL: "/lightning/modules/c/apexWireHost/apexWireHost.js",
					Tag: "c-apex-wire-host",
				},
			},
		},
	})
	if !strings.Contains(html, `"outAppDependencies":{"c:lightningout":["c:apexwirehost","c:recordwirehost"]}`) {
		t.Fatalf("missing outAppDependencies in bootstrap:\n%s", html)
	}
}

func TestBootstrapHTMLIncludesPageReference(t *testing.T) {
	html := BootstrapHTML(PageConfig{
		PageReference: map[string]any{
			"type": "standard__recordPage",
			"attributes": map[string]any{
				"recordId":      "001000000000001AAA",
				"objectApiName": "Account",
				"actionName":    "view",
			},
		},
	})
	if !strings.Contains(html, `"pageReference"`) || !strings.Contains(html, `"standard__recordPage"`) {
		t.Fatalf("missing page reference in bootstrap:\n%s", html)
	}
}
