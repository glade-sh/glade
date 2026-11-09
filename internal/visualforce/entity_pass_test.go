package visualforce

import (
	"strings"
	"testing"
)

func TestProtectNativeEntitiesKeepsAttributeAndExpressionText(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"attribute named", `<span title="a &amp; b &quot;c&quot; &nbsp;">x</span>`, `<span title="a &amp;#38; b &amp;#34;c&amp;#34; &amp;#160;">x</span>`},
		{"attribute numeric stays", `<span title="&#38; &#x26;">x</span>`, `<span title="&amp;#38; &amp;#x26;">x</span>`},
		{"attribute unknown and bare", `<span title="&foo; &amp b & c">x</span>`, `<span title="&amp;foo; &amp;amp b &amp; c">x</span>`},
		{"attribute case matters", `<span title="&AMP; &Eacute;">x</span>`, `<span title="&amp;AMP; &amp;#201;">x</span>`},
		{"single quoted", `<span title='&lt;i&gt;'>x</span>`, `<span title='&amp;#60;i&amp;#62;'>x</span>`},
		{"expression in attribute", `<apex:outputText value="{!'a&amp;b'}"/>`, `<apex:outputText value="{!'a&amp;#38;b'}"/>`},
		{"expression in text", `{!'a&lt;b'} &lt; c`, `{!'a&amp;#60;b'} &lt; c`},
		{"text decodes normally", `a &amp; b &nbsp; &copy;`, `a &amp; b &nbsp; &copy;`},
		{"text lang and rang use HTML 4", `&lang;x&rang;`, `&#9001;x&#9002;`},
		{"comment untouched", `<!-- &amp; --><b title="&amp;">x</b>`, `<!-- &amp; --><b title="&amp;#38;">x</b>`},
		{"script body untouched", `<script>a = "&amp;";</script><b title="&amp;">x</b>`, `<script>a = "&amp;";</script><b title="&amp;#38;">x</b>`},
		{"no ampersand", `<apex:page><b>x</b></apex:page>`, `<apex:page><b>x</b></apex:page>`},
		{"unclosed expression is text", `{! a &amp; <b title="&amp;">x</b>`, `{! a &amp; <b title="&amp;#38;">x</b>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := protectNativeEntities(c.source); got != c.want {
				t.Fatalf("protectNativeEntities(%q)\n got %q\nwant %q", c.source, got, c.want)
			}
		})
	}
}

func TestTextEntityErrorOnlyInMarkupText(t *testing.T) {
	for _, c := range []struct{ name, source, want string }{
		{"declared", `a &amp; &nbsp; &hellip; &#65; &#x41;`, ""},
		{"unknown", `a &foo; b`, `The entity "foo" was referenced, but not declared.`},
		{"html5 name", `&check;`, `The entity "check" was referenced, but not declared.`},
		{"no semicolon", `a &amp b`, `The reference to entity "amp" must end with the ';' delimiter.`},
		{"bare", `a & b`, `The entity name must immediately follow the '&' in the entity reference.`},
		{"upper x", `&#X26;`, `A decimal representation must immediately follow the "&#" in a character reference.`},
		{"bad hex", `&#xZZ;`, `A hexadecimal representation must immediately follow the "&#x" in a character reference.`},
		{"numeric no semicolon", `&#38 x`, `The character reference must end with the ';' delimiter.`},
		{"attribute not checked", `<b title="a &foo; & &amp b">x</b>`, ""},
		{"expression not checked", `{!'a &foo; & b'}`, ""},
		{"comment not checked", `<!-- & -->x`, ""},
		{"script not checked", `<script>if (a && b) {}</script>`, ""},
		{"error after tag", `<b title="x">a & b</b>`, `The entity name must immediately follow the '&' in the entity reference.`},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := textEntityError(c.source)
			switch {
			case c.want == "" && err != nil:
				t.Fatalf("unexpected error %v", err)
			case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
				t.Fatalf("error=%v, want %q", err, c.want)
			}
		})
	}
}

func TestNativeEntityTableIsHTML4PlusApos(t *testing.T) {
	if got := len(nativeAttributeEntities); got != 253 {
		t.Fatalf("entity names=%d, want 252 HTML 4 names plus apos", got)
	}
	for name, code := range map[string]int{"amp": 38, "apos": 39, "nbsp": 160, "euro": 8364, "lang": 9001} {
		if nativeAttributeEntities[name] != code {
			t.Fatalf("&%s; = %d, want %d", name, nativeAttributeEntities[name], code)
		}
	}
}

func TestIndexFoldASCII(t *testing.T) {
	for _, c := range []struct {
		s, lower string
		want     int
	}{
		{"abc</script>", "</script", 3},
		{"abc</SCRIPT>", "</script", 3},
		{"abc</ScRiPt>", "</script", 3},
		{"abc< /script>", "</script", -1},
		{"</scrip", "</script", -1},
		{"", "</script", -1},
	} {
		if got := indexFoldASCII(c.s, c.lower); got != c.want {
			t.Fatalf("indexFoldASCII(%q, %q) = %d, want %d", c.s, c.lower, got, c.want)
		}
	}
}

// Both scanners search for each closing script or style tag in place. A page with
// thousands of script tags used to copy and lower-case the rest of the source for
// every tag, which is quadratic.
func TestEntityScannersHandleManyScriptTags(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 4000; i++ {
		b.WriteString(`<SCRIPT>var a = "&amp;";</SCRIPT><style>.a{}</STYLE><b title="&amp;">x</b>`)
	}
	source := b.String()
	got := protectNativeEntities(source)
	if want := strings.Count(source, `title="&amp;"`); strings.Count(got, `title="&amp;#38;"`) != want {
		t.Fatalf("attribute rewrites = %d, want %d", strings.Count(got, `title="&amp;#38;"`), want)
	}
	if strings.Count(got, `"&amp;";`) != 4000 {
		t.Fatalf("script bodies were rewritten")
	}
	if err := textEntityError(source); err != nil {
		t.Fatalf("textEntityError = %v", err)
	}
}
