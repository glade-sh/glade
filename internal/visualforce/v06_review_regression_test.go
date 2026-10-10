package visualforce_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

func v06PreserveBodyFormulaSource(t *testing.T) {
	// Preserve the pre-existing RenderVisualforceText expression path: quotes
	// and a closing brace inside a quoted value remain formula source. Literal
	// escaping is already backed by composition_body_escaped_runtime; these
	// additional variants assert preservation, not new native observations.
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "quoted-string", body: `{!'owned'}`, want: `<span>owned</span>`},
		{name: "mixed-literals-and-formulas", body: `before &amp; &lt; {!'owned'} {!'}'} {!label} &quot; &#39; after`, want: `<span>before &amp; &lt; owned } &lt;em&gt;bound&lt;/em&gt; &#34; &#39; after</span>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := v06RenderReviewComponent(t, "ReviewBody", `<apex:component><apex:componentBody/></apex:component>`,
				`<c:ReviewBody>`+tc.body+`</c:ReviewBody>`, map[string]vm.Value{"label": vm.String("<em>bound</em>")})
			if got != tc.want {
				t.Fatalf("body rendering: expected <%s> actual <%s>", tc.want, got)
			}
		})
	}
}

func v06PreserveScopeBeforeAttribute(t *testing.T) {
	// Preserve resolveVisualforceIdentifier's scope-before-Variables lookup,
	// including a scoped null and case-insensitive names. The component's own
	// attribute must become visible again after renderApexRepeat pops its frame.
	component := `<apex:component><apex:attribute name="value" type="String" description="Outer value"/><apex:attribute name="items" type="String[]" description="Repeat values"/><apex:outputText value="{!value}"/>|<apex:repeat value="{!items}" var="VaLuE"><apex:outputText value="{!value}"/>;</apex:repeat>|<apex:outputText value="{!value}"/></apex:component>`
	got := v06RenderReviewComponent(t, "ReviewScope", component, `<c:ReviewScope value="component" items="{!rows}"/>`,
		map[string]vm.Value{"rows": vm.List(vm.String("first"), vm.String("second"), vm.Null)})
	want := `<span>component|first;second;;|component</span>`
	if got != want {
		t.Fatalf("repeat scope: expected <%s> actual <%s>", want, got)
	}
}

func v06RenderReviewComponent(t *testing.T, name, component, invocation string, variables map[string]vm.Value) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"packageDirectories":[{"path":"force-app","default":true}]}`,
		"force-app/main/default/components/" + name + ".component": component,
	}
	for path, source := range files {
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	// This exercises the existing render-only contract. Native source admission
	// and all captured answers still run separately in the same conformance test.
	index, err := visualforce.LoadProjectForRender(p)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := visualforce.ParseMarkupTree(invocation)
	if err != nil {
		t.Fatal(err)
	}
	got, err := visualforce.RenderMarkupTree(tree, &visualforce.RenderContext{
		VM:         vm.New(nil),
		Project:    p,
		VFIndex:    &index,
		Expression: &visualforce.ExpressionContext{Variables: variables},
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}
