package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestNormalizeStoredRichTextMeasuredMatrix(t *testing.T) {
	field := storage.Field{APIName: "Description__c", Type: storage.FieldString, DisplayType: "RICHTEXTAREA"}
	for _, test := range []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Plain café & text", "Plain café &amp; text"},
		{"whitespace", "  first\nsecond  ", "first\nsecond"},
		{"entities", "&lt;b&gt;encoded&lt;/b&gt; &amp; &#169;", "&lt;b&gt;encoded&lt;/b&gt; &amp; &copy;"},
		{"format", "<p>Hello <b>bold</b> <i>italic</i><br/>end</p>", "<p>Hello <b>bold</b> <i>italic</i><br>end</p>"},
		{"list", "<ul><li>one</li><li>two</li></ul>", "<ul><li>one</li><li>two</li></ul>"},
		{"script", "before<script>alert(1)</script>after", "beforeafter"},
		{"events", `<p onclick="alert(1)">text</p><img src=x onerror=alert(document.cookie)>`, `<p>text</p><img src=""></img>`},
		{"safeimage", `<img src="https://example.invalid/image.png" alt="Owned" width="10">`, `<img src="https://example.invalid/image.png" alt="Owned" width="10"></img>`},
		{"links", `<a href="https://example.invalid/path" title="Owned">safe</a><a href="javascript:alert(1)">bad</a>`, `<a href="https://example.invalid/path" title="Owned" target="_blank">safe</a><a href="" target="_blank">bad</a>`},
		{"encodedurl", `<a href="java&#115;cript:alert(1)">encoded</a>`, `<a href="" target="_blank">encoded</a>`},
		{"style", `<span style="color:red;font-weight:bold" class="owned" id="owned">styled</span>`, `<span class="owned" id="owned" style="color: red; font-weight: bold;">styled</span>`},
		{"caseunknown", `<DIV OnMouseOver="alert(1)"><custom-owned>text</custom-owned></DIV>`, `<div>text</div>`},
		{"malformed", `<p>one<b>two</p>three<!-- comment -->`, `<p>one<b>two</b></p><b>three</b>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := storage.StringValue(test.in)
			got := normalizeStoredFieldValue(field, caller)
			if got.String != test.want {
				t.Fatalf("stored = %q, want %q", got.String, test.want)
			}
			if caller.String != test.in {
				t.Fatalf("caller changed to %q", caller.String)
			}
		})
	}
}

func TestNormalizeStoredRichTextLeavesPlainTextUnchanged(t *testing.T) {
	rich := storage.Field{APIName: "Description__c", Type: storage.FieldString, DisplayType: "RICHTEXTAREA"}
	plain := storage.Field{APIName: "Plain__c", Type: storage.FieldString, DisplayType: "TEXTAREA"}
	unsafe := `<img src=x onerror=alert(document.cookie)>`
	if got := normalizeStoredFieldValue(rich, storage.StringValue(unsafe)).String; got == unsafe {
		t.Fatalf("rich text did not normalize unsafe content: %q", got)
	}
	if got := normalizeStoredFieldValue(plain, storage.StringValue(unsafe)).String; got != unsafe {
		t.Fatalf("plain text changed to %q", got)
	}
}

func TestRichTextBoundaryLinksDoNotBroadenImageSources(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{`<a href="/owned/path">relative</a>`, `<a href="/owned/path" target="_blank">relative</a>`},
		{`<a href="javascript:alert(1)">unsafe</a>`, `<a href="" target="_blank">unsafe</a>`},
		{`<img src="mailto:owned@example.invalid">`, `<img src=""></img>`},
		{`<img src="http://example.invalid/image.png">`, `<img src=""></img>`},
	} {
		if got := sanitizeRichText(test.input); got != test.want {
			t.Fatalf("%q => %q, want %q", test.input, got, test.want)
		}
	}
}
