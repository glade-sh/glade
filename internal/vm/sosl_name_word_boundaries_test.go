package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sosl"
)

// Keep unmeasured cases on the matcher path that preceded the indexed captures.
func TestSOSLNameTokenUnmeasuredBoundaries(t *testing.T) {
	for _, tc := range []struct {
		term, text  string
		scope       sosl.SearchScope
		matched     bool
		unsupported bool
	}{
		{`"5551287"`, "+1 (415) 555-1287", sosl.SearchScopePhone, false, false},
		{`"1287"`, "+1 (415) 555-1287", sosl.SearchScopePhone, true, false},
		{"B*ta", "Baxeta", sosl.SearchScopeName, false, true},
		{"B?ta", "Betamax", sosl.SearchScopeName, false, true},
		{"B*ta", "Betta", sosl.SearchScopeName, true, false},
	} {
		t.Run(tc.term+"_"+tc.text, func(t *testing.T) {
			q, err := sosl.Parse("FIND {" + tc.term + "} IN " + string(tc.scope) + " RETURNING Contact(Id)")
			if err != nil {
				t.Fatal(err)
			}
			matched, err := soslFieldTextMatches(tc.text, q.Expression.Term, tc.scope)
			if matched != tc.matched || (err != nil) != tc.unsupported || err != nil && !strings.Contains(err.Error(), "SOSL wildcard index matching") {
				t.Errorf("got (%v, %v), want match=%v unsupported=%v", matched, err, tc.matched, tc.unsupported)
			}
		})
	}
}
