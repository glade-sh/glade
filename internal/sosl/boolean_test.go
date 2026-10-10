package sosl_test

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/sosl"
)

func TestBooleanSearchStructureAndFlatLeaves(t *testing.T) {
	for _, tc := range []struct{ text, root, left, right string }{
		{"Alpha OR Beta AND Gamma", "OR", "TERM", "AND"},
		{"(Alpha OR Beta) AND Gamma", "AND", "OR", "TERM"},
		{"Alpha AND (Beta OR Gamma)", "AND", "TERM", "OR"},
		{"Alpha Beta OR Gamma", "OR", "AND", "TERM"},
	} {
		t.Run(tc.text, func(t *testing.T) {
			q, err := sosl.Parse("FIND '" + tc.text + "' RETURNING Contact(Id)")
			if err != nil {
				t.Fatal(err)
			}
			e := q.Expression
			if e == nil || e.Operator != tc.root || e.Left == nil || e.Right == nil || e.Left.Operator != tc.left || e.Right.Operator != tc.right {
				t.Fatalf("expression=%#v", e)
			}
			if !reflect.DeepEqual(q.Terms, []sosl.SearchTerm{{Text: "Alpha"}, {Text: "Beta"}, {Text: "Gamma"}}) {
				t.Fatalf("terms=%#v", q.Terms)
			}
		})
	}
	q, err := sosl.Parse("FIND '(Alpha AND Beta) OR Gamma*' RETURNING Contact(Id)")
	if err != nil || len(q.Terms) != 3 || !q.Terms[2].Prefix || q.Terms[2].Text != "Gamma" {
		t.Fatalf("query=%#v err=%v", q, err)
	}
}

func TestBooleanSearchMalformedAndUnprovedOperators(t *testing.T) {
	// A35 controls L002-L012: these FIND expressions are accepted syntax.
	// Empty operands are runtime errors; unsupported index matching is separate.
	for _, text := range []string{"", "AND Alpha", "Alpha OR", "Alpha AND OR Beta", "()", "(Alpha", "Alpha)", "Alpha OR ()", `"Alpha Beta`} {
		if _, err := sosl.Parse("FIND '" + text + "' RETURNING Contact(Id)"); err != nil {
			t.Errorf("rejected native-accepted expression %q: %v", text, err)
		}
	}
	for _, text := range []string{"Alpha NOT Beta", "NOT Alpha"} {
		query, err := sosl.Parse("FIND '" + text + "' RETURNING Contact(Id)")
		if err != nil || query.MatchUnsupported == "" {
			t.Errorf("expression=%q query=%#v err=%v", text, query, err)
		}
	}
}

func TestQuotedSearchTermsRetainPhraseAndOperatorText(t *testing.T) {
	q, err := sosl.Parse(`FIND '"Alpha Beta" OR "AND"' RETURNING Contact(Id)`)
	if err != nil {
		t.Fatal(err)
	}
	if q.Expression == nil || q.Expression.Operator != "OR" || !reflect.DeepEqual(q.Terms, []sosl.SearchTerm{{Text: "Alpha Beta"}, {Text: "AND"}}) {
		t.Fatalf("query=%#v", q)
	}
}

func TestBareWildcardSearchOperandIsAccepted(t *testing.T) {
	q, err := sosl.Parse("FIND '* @group' RETURNING Contact(Id)")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(q.Terms, []sosl.SearchTerm{{Text: "", Prefix: true}, {Text: "@group"}}) {
		t.Fatalf("terms=%#v", q.Terms)
	}
}

func TestQuotedEmptySearchOperandRemainsRejected(t *testing.T) {
	// L013 rejects a double-quoted FIND literal; L014-L015 are runtime errors.
	if _, err := sosl.Parse(`FIND "" RETURNING Contact(Id)`); err == nil {
		t.Fatal("accepted a double-quoted FIND literal")
	}
	for _, text := range []string{`FIND '""' RETURNING Contact(Id)`, `FIND '* ""' RETURNING Contact(Id)`} {
		query, err := sosl.Parse(text)
		if err != nil || query.Expression == nil || query.Expression.Operator != "INVALID_SEARCH_TERM" {
			t.Errorf("query=%#v error=%v, want native runtime term rejection", query, err)
		}
	}
}
