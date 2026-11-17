package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A typed flow merge reads in the user's locale only for a captured locale.
// Any other locale (a bare language such as de or it, ru_RU, pt_PT) keeps the original text.
func TestFlowTypedMergeFallsBackForUncapturedLocales(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}
	field := storage.Field{APIName: "X__c", Type: storage.FieldString}
	record := storage.Record{Object: "Account", Fields: map[string]storage.Value{}}
	formula := `GLADELOCALEMERGE("n={!Num}", "n=" & GLADENUMBERTEXT(1234567.891, 3) & " " & GLADEDATETEXT(DATE(2026, 1, 2)))`
	for _, test := range []struct{ locale, want string }{
		{"en_US", "n=1,234,567.891 January 2, 2026"},
		{"de_DE", "n=1.234.567,891 2. January 2026"},
		{"it_IT", "n=1.234.567,891 2 January 2026"},
		{"sv_SE", "n=1\u00a0234\u00a0567,891 2 January 2026"},
		{"it", "n={!Num}"},
		{"de", "n={!Num}"},
		{"ru_RU", "n={!Num}"},
		{"pt_PT", "n={!Num}"},
		{"", "n={!Num}"},
		{"\x00missing", "n={!Num}"},
	} {
		org := testOrg()
		fields := map[string]storage.Value{}
		// "" is an explicit blank; the \x00 name leaves the field out. The default
		// running user stores en_US, so neither is a captured shape.
		if test.locale != "\x00missing" {
			fields["LocaleSidKey"] = storage.StringValue(test.locale)
		}
		id := storage.ID(storage.DefaultUserID)
		org.Objects["User"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "User"}, Records: map[storage.ID]storage.Record{id: {ID: id, Object: "User", Fields: fields}}}
		value, _, ok := EvaluateRecordFormulaValueInOrgWithOptions(formula, field, &org, definition, record, FormulaEvaluationOptions{FlowContext: true, UserID: id})
		if !ok || value.String != test.want {
			t.Fatalf("locale %q: got %q ok=%v, want %q", test.locale, value.String, ok, test.want)
		}
	}
}

// A rendering that cannot evaluate must not break an uncaptured locale: the
// original text of the old path is returned without evaluating the rendering.
func TestFlowTypedMergeEvaluatesOnlyTheChosenBranch(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}
	field := storage.Field{APIName: "X__c", Type: storage.FieldString}
	record := storage.Record{Object: "Account", Fields: map[string]storage.Value{}}
	formula := `GLADELOCALEMERGE("n={!Num}", "n=" & GLADENUMBERTEXT("", 3))`
	for _, test := range []struct {
		locale string
		want   string
		ok     bool
	}{
		{"ru_RU", "n={!Num}", true},
		{"de", "n={!Num}", true},
		{"en_US", "", false},
	} {
		org := testOrg()
		id := storage.ID(storage.DefaultUserID)
		org.Objects["User"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "User"}, Records: map[storage.ID]storage.Record{id: {ID: id, Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(test.locale)}}}}
		value, _, ok := EvaluateRecordFormulaValueInOrgWithOptions(formula, field, &org, definition, record, FormulaEvaluationOptions{FlowContext: true, UserID: id})
		if ok != test.ok || (ok && value.String != test.want) {
			t.Fatalf("locale %q: got %q ok=%v, want %q ok=%v", test.locale, value.String, ok, test.want, test.ok)
		}
	}
}
