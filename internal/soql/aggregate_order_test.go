package soql

import "testing"

func TestParseSelectedAggregateOrderExpression(t *testing.T) {
	for _, order := range []string{"COUNT_DISTINCT(Id)", "count_distinct ( id ) DESC NULLS LAST"} {
		q, err := Parse("SELECT COUNT_DISTINCT(Id), Account.Name FROM Contact GROUP BY Account.Name ORDER BY " + order)
		if err != nil {
			t.Fatal(err)
		}
		if len(q.Order) != 1 || q.Order[0].Field != "expr0" || q.OrderBy != "expr0" || !q.Order[0].RewrittenAggregate {
			t.Fatalf("aggregate order not bound to selected output: %+v", q.Order)
		}
		if order[0] == 'c' && (!q.Order[0].Desc || q.Order[0].Nulls != "LAST") {
			t.Fatalf("order modifiers changed: %+v", q.Order)
		}
	}
}

func TestParseAggregateOrderPreservesOtherSelectors(t *testing.T) {
	for _, order := range []string{"Account.Name", "total", "expr0", "COUNT_DISTINCT(LastName)"} {
		q, err := Parse("SELECT COUNT_DISTINCT(Id) total, Account.Name FROM Contact GROUP BY Account.Name ORDER BY " + order)
		if err != nil {
			t.Fatal(err)
		}
		if q.Order[0].Field != order || q.Order[0].RewrittenAggregate {
			t.Fatalf("unrelated selector %q rewritten as %q", order, q.Order[0].Field)
		}
	}
}
