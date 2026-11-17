package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func mergeTestEngine(locale string, setLocale bool) *Engine {
	org := testOrg()
	fields := map[string]storage.Value{}
	if setLocale {
		fields["LocaleSidKey"] = storage.StringValue(locale)
	}
	id := storage.ID(storage.DefaultUserID)
	org.Objects["User"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "User"}, Records: map[storage.ID]storage.Record{id: {ID: id, Object: "User", Fields: fields}}}
	engine := NewEngine(&org)
	engine.UserID = id
	return &engine
}

// A typed flow merge reads in the user's locale only for a captured locale. Any
// other locale (a bare language such as de or it, ru_RU, pt_PT), an explicit
// blank and a missing field keep the original text: flowMergedText reports false
// and the caller runs the unrewritten LiteralValue. The default running user
// stores en_US explicitly, so a blank is not a captured shape. Literal text is
// copied as is, including a newline, a tab and an escaped ampersand.
func TestFlowTypedMergeFallsBackForUncapturedLocales(t *testing.T) {
	parts := []storage.FlowMergePart{
		{Text: "n=\n\t&amp; \""},
		{Ref: "Num", Kind: "number", Scale: 3},
		{Text: " "},
		{Ref: "When", Kind: "date"},
		{Text: " "},
		{Ref: "Flag"},
	}
	for _, test := range []struct {
		name, locale string
		set          bool
		want         string
		ok           bool
	}{
		{"en_US", "en_US", true, "n=\n\t&amp; \"1,234,567.891 January 2, 2026 true", true},
		{"de_DE", "de_DE", true, "n=\n\t&amp; \"1.234.567,891 2. January 2026 true", true},
		{"it_IT", "it_IT", true, "n=\n\t&amp; \"1.234.567,891 2 January 2026 true", true},
		{"sv_SE", "sv_SE", true, "n=\n\t&amp; \"1 234 567,891 2 January 2026 true", true},
		{"it", "it", true, "", false},
		{"de", "de", true, "", false},
		{"ru_RU", "ru_RU", true, "", false},
		{"pt_PT", "pt_PT", true, "", false},
		{"blank", "", true, "", false},
		{"missing", "", false, "", false},
	} {
		engine := mergeTestEngine(test.locale, test.set)
		frame := newFlowFrame()
		frame.scalars["num"] = storage.DecimalValue("1234567.891")
		frame.scalars["when"] = storage.DateValue("2026-01-02")
		frame.scalars["flag"] = storage.BooleanValue(true)
		got, ok := engine.flowMergedText(parts, frame)
		if ok != test.ok || got != test.want {
			t.Fatalf("%s: got %q ok=%v, want %q ok=%v", test.name, got, ok, test.want, test.ok)
		}
		// flowFrameValue runs the unrewritten literal for an uncaptured locale.
		value, ok := engine.flowFrameValue(storage.FlowAssignment{Target: "Out", LiteralValue: "n={!Num}", Merge: parts}, frame)
		want := test.want
		if !test.ok {
			want = "n={!Num}"
		}
		if !ok || value.String != want {
			t.Fatalf("%s: flowFrameValue %q ok=%v, want %q", test.name, value.String, ok, want)
		}
	}
}
