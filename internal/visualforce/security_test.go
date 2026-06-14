package visualforce

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

func TestEscapeVisualforceOutputEscapesByDefault(t *testing.T) {
	got := EscapeVisualforceOutput(`<script>alert("x")</script>`, true)
	want := `&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;`
	if got != want {
		t.Fatalf("escaped output = %q, want %q", got, want)
	}
}

func TestEscapeVisualforceOutputAllowsRawWhenEscapeFalse(t *testing.T) {
	raw := `<b data-x="1">raw</b>`
	got := EscapeVisualforceOutput(raw, false)
	if got != raw {
		t.Fatalf("raw output = %q, want %q", got, raw)
	}
}

func TestRenderOutputPanelEscapesMergeFieldTextButKeepsLiteralHTML(t *testing.T) {
	markup := `<apex:page><apex:outputPanel><em>literal</em>{!payload}</apex:outputPanel></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	controller := vm.Object("SecurityController")
	controller.Fields["payload"] = vm.String(`<img src=x onerror=alert(1)> & "quoted"`)

	rendered, err := RenderMarkupTree(tree, &RenderContext{
		Expression: &ExpressionContext{Controller: controller},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(rendered, `<em>literal</em>`) {
		t.Fatalf("rendered missing literal HTML = %q", rendered)
	}
	if strings.Contains(rendered, `<img src=x onerror=alert(1)>`) {
		t.Fatalf("rendered unescaped merge field HTML = %q", rendered)
	}
	if !strings.Contains(rendered, `&lt;img src=x onerror=alert(1)&gt; &amp; &#34;quoted&#34;`) {
		t.Fatalf("rendered = %q, want escaped merge field text", rendered)
	}
}

func TestRenderVisualforceTextDoesNotReevaluateReplacementText(t *testing.T) {
	controller := vm.Object("SecurityController")
	controller.Fields["payload"] = vm.String("{!secret}")
	controller.Fields["secret"] = vm.String(`<script>alert(1)</script>`)

	got, err := RenderVisualforceText("body={!payload}; secret={!secret}", &ExpressionContext{Controller: controller})
	if err != nil {
		t.Fatal(err)
	}
	if got != "body={!secret}; secret=&lt;script&gt;alert(1)&lt;/script&gt;" {
		t.Fatalf("text = %q", got)
	}
}

func TestRenderScriptTextPreservesJavaScriptEscapedMergeField(t *testing.T) {
	markup := `<apex:page><script>window.payload = JSON.parse('{!JSENCODE(payload)}');</script><div>{!payload}</div></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	controller := vm.Object("SecurityController")
	controller.Fields["payload"] = vm.String(`{"visitor":{"id":"system@example.invalid-00D000000000001EAA"}}`)

	rendered, err := RenderMarkupTree(tree, &RenderContext{
		Expression: &ExpressionContext{Controller: controller},
	})
	if err != nil {
		t.Fatal(err)
	}

	wantScript := `JSON.parse('{\"visitor\":{\"id\":\"system@example.invalid-00D000000000001EAA\"}}')`
	if !strings.Contains(rendered, wantScript) {
		t.Fatalf("rendered script = %q, want %q", rendered, wantScript)
	}
	if strings.Contains(rendered, `\&#34;`) || strings.Contains(rendered, `&quot;`) {
		t.Fatalf("rendered script contains HTML entities: %q", rendered)
	}
	if !strings.Contains(rendered, `{&#34;visitor&#34;:{&#34;id&#34;:&#34;system@example.invalid-00D000000000001EAA&#34;}}`) {
		t.Fatalf("rendered HTML text did not stay escaped: %q", rendered)
	}
}

func TestRenderOutputTextEscapeFalseKeepsExpressionRawButEscapesLiteralMarkup(t *testing.T) {
	markup := `<apex:page><apex:outputText escape="false" value="{!payload}"/><apex:outputText escape="false" value="&lt;safe&gt;"/></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	controller := vm.Object("SecurityController")
	controller.Fields["payload"] = vm.String(`<strong>raw</strong>`)

	rendered, err := RenderMarkupTree(tree, &RenderContext{
		Expression: &ExpressionContext{Controller: controller},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, `<strong>raw</strong>`) {
		t.Fatalf("rendered = %q, want raw expression output", rendered)
	}
	if strings.Contains(rendered, `<safe>`) || !strings.Contains(rendered, `&lt;safe&gt;`) {
		t.Fatalf("rendered = %q, want literal markup encoded as text", rendered)
	}
}

func TestVisualforceCSPHeaderValueForCSPHeaderTrue(t *testing.T) {
	got := VisualforceCSPHeaderValue()
	want := "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'self'; base-uri 'self'"
	if got != want {
		t.Fatalf("CSP header = %q, want %q", got, want)
	}
}

func TestEscapeVisualforceJavaScriptStringEscapesScriptSensitiveBytes(t *testing.T) {
	got := EscapeVisualforceJavaScriptString("one\"two\n</script>\u2028")
	want := `one\"two\n<\/script>\u2028`
	if got != want {
		t.Fatalf("escaped JavaScript = %q, want %q", got, want)
	}
}

func TestVisualforcePageHeaderOptionsApplyCSPNoCacheAndDownloadName(t *testing.T) {
	options, err := VisualforcePageHeaderOptionsFromMarkup(`<apex:page cspHeader="true" cache="false" contentType="application/vnd.ms-excel#contacts.xls"></apex:page>`)
	if err != nil {
		t.Fatalf("VisualforcePageHeaderOptionsFromMarkup err = %v", err)
	}
	header := http.Header{}
	options.Apply(header, time.Date(2026, time.June, 14, 12, 0, 0, 0, time.UTC))

	if got := header.Get("Content-Security-Policy"); got != VisualforceCSPHeaderValue() {
		t.Fatalf("CSP = %q, want %q", got, VisualforceCSPHeaderValue())
	}
	if got := header.Get("Cache-Control"); got != "no-store, no-cache, must-revalidate, max-age=0" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := header.Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma = %q", got)
	}
	if got := header.Get("Expires"); got != "0" {
		t.Fatalf("Expires = %q", got)
	}
	if got := header.Get("Content-Type"); got != "application/vnd.ms-excel" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := header.Get("Content-Disposition"); got != `attachment; filename="contacts.xls"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
}

func TestVisualforcePageHeaderOptionsDoNotApplyCSPWhenAttributeIsAbsent(t *testing.T) {
	options, err := VisualforcePageHeaderOptionsFromMarkup(`<apex:page></apex:page>`)
	if err != nil {
		t.Fatalf("VisualforcePageHeaderOptionsFromMarkup err = %v", err)
	}
	header := http.Header{}
	options.Apply(header, time.Date(2026, time.June, 14, 12, 0, 0, 0, time.UTC))
	if got := header.Get("Content-Security-Policy"); got != "" {
		t.Fatalf("CSP = %q, want empty", got)
	}
}

func TestVisualforcePageHeaderOptionsApplyCacheExpiresMaxAge(t *testing.T) {
	options, err := VisualforcePageHeaderOptionsFromMarkup(`<apex:page cache="true" expires="120" contentType="text/csv"></apex:page>`)
	if err != nil {
		t.Fatalf("VisualforcePageHeaderOptionsFromMarkup err = %v", err)
	}
	header := http.Header{}
	options.Apply(header, time.Date(2026, time.June, 14, 12, 0, 0, 0, time.UTC))

	if got := header.Get("Cache-Control"); got != "public, max-age=120" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := header.Get("Expires"); got != "Sun, 14 Jun 2026 12:02:00 GMT" {
		t.Fatalf("Expires = %q", got)
	}
	if got := header.Get("Content-Type"); got != "text/csv" {
		t.Fatalf("Content-Type = %q", got)
	}
}
