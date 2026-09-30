package sosl_test

import (
	"errors"
	"github.com/glade-sh/glade/internal/sosl"
	"reflect"
	"testing"
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
	for _, text := range []string{"", "AND Alpha", "Alpha OR", "Alpha AND OR Beta", "()", "(Alpha", "Alpha)", "Alpha OR ()", `"Alpha Beta`} {
		if _, err := sosl.Parse("FIND '" + text + "' RETURNING Contact(Id)"); err == nil {
			t.Errorf("accepted malformed expression %q", text)
		}
	}
	for _, text := range []string{"Alpha NOT Beta", "NOT Alpha"} {
		_, err := sosl.Parse("FIND '" + text + "' RETURNING Contact(Id)")
		var unsupported *sosl.UnsupportedFeatureError
		if !errors.As(err, &unsupported) {
			t.Errorf("expression=%q err=%v", text, err)
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
	for _, query := range []string{`FIND "" RETURNING Contact(Id)`, `FIND '""' RETURNING Contact(Id)`, `FIND '* ""' RETURNING Contact(Id)`} {
		if _, err := sosl.Parse(query); err == nil {
			t.Errorf("accepted an empty quoted search operand in %q", query)
		}
	}
}
