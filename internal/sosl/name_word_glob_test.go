package sosl

import "testing"

func TestNameWordGlobKeepsEscapeAndQuotedBoundaries(t *testing.T) {
	for _, tc := range []struct {
		term string
		glob bool
	}{
		{"B?ta", true}, {"B*ta", true}, {"B??ta", true},
		{"Alpha?", false}, {"?Alpha", false}, {"B?ta*", false},
		{`B\*t?a`, false}, {`"B?ta"`, false},
	} {
		query, err := Parse("FIND {" + tc.term + "} IN NAME FIELDS RETURNING Contact(Id)")
		if err != nil || len(query.Terms) != 1 {
			t.Fatalf("parse %q: %+v, %v", tc.term, query.Terms, err)
		}
		if got := query.Terms[0].NameWordGlob; got != tc.glob {
			t.Errorf("name word glob %q = %t, want %t", tc.term, got, tc.glob)
		}
	}
}
