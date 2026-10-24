package sosl

import "testing"

func TestParseTemporalReturningRelativeLiteral(t *testing.T) {
	query, err := Parse("FIND 'Temporal' IN ALL FIELDS RETURNING Contact(Id WHERE CreatedDate = LAST_N_DAYS:1)")
	if err != nil {
		t.Fatal(err)
	}
	if len(query.Returning) != 1 || query.Returning[0].Where == nil || query.Returning[0].Where.Value != "LAST_N_DAYS:1" {
		t.Fatalf("relative date value changed: %#v", query)
	}
}

// Range members remain outside the supported IN-list grammar. Their native
// semantics are not covered by the scalar-filter capture.
func TestNumberedDateRangesDoNotWidenSOSLLists(t *testing.T) {
	for _, operator := range []string{"IN", "NOT IN"} {
		t.Run(operator, func(t *testing.T) {
			query := "FIND 'Temporal' RETURNING Contact(Id WHERE CreatedDate " + operator + " (LAST_N_DAYS:1))"
			if _, err := Parse(query); err == nil {
				t.Fatalf("uncaptured range-list grammar accepted: %s", query)
			}
		})
	}
}
