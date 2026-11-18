package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func likeCondition(field, pattern string) *Condition {
	return &Condition{Field: field, Op: "LIKE", Value: storage.StringValue(pattern)}
}

func TestDefaultParentOrderShapes(t *testing.T) {
	tests := []struct {
		name   string
		query  Query
		object string
		want   int
	}{
		{"account name prefix", Query{Where: likeCondition("Name", "GLA%")}, "Account", 1},
		{"account qualified", Query{Where: likeCondition("Account.Name", "GLA%")}, "Account", 1},
		{"account and list keeps base", Query{Where: &Condition{And: []Condition{{Field: "Site", Op: "=", Value: storage.StringValue("x")}, *likeCondition("Name", "A%")}}}, "Account", 0},
		{"single-element and keeps base", Query{Where: &Condition{And: []Condition{*likeCondition("Name", "A%")}}}, "Account", 0},
		{"unfiltered keeps base", Query{}, "Account", 0},
		{"leading wildcard keeps base", Query{Where: likeCondition("Name", "%GLA")}, "Account", 0},
		{"or keeps base", Query{Where: &Condition{Or: []Condition{*likeCondition("Name", "A%"), *likeCondition("Name", "B%")}}}, "Account", 0},
		{"other field keeps base", Query{Where: likeCondition("Site", "A%")}, "Account", 0},
		{"other object keeps base", Query{Where: likeCondition("Name", "A%")}, "Opportunity", 0},
		{"explicit order wins", Query{Where: likeCondition("Name", "A%"), Order: []OrderSpec{{Field: "Site", Desc: true}}}, "Account", 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			order := defaultParentOrder(test.query, test.object)
			if len(order) != test.want {
				t.Fatalf("order = %v; want %d specs", order, test.want)
			}
			if test.name == "explicit order wins" && order[0].Field != "Site" {
				t.Fatalf("explicit order replaced: %v", order)
			}
			if test.name == "account name prefix" && (order[0].Field != "Name" || order[0].Desc) {
				t.Fatalf("order = %v; want Name ascending", order)
			}
		})
	}
}

func TestDefaultChildOrderShapes(t *testing.T) {
	if order := defaultChildOrder(Query{Where: likeCondition("LastName", "GLC%")}, "Contact"); len(order) != 1 || order[0].Field != "LastName" || !order[0].Desc {
		t.Fatalf("order = %v; want LastName descending", order)
	}
	if order := defaultChildOrder(Query{Where: likeCondition("Name", "X%")}, "Case"); len(order) != 0 {
		t.Fatalf("Case Name child order = %v; want base", order)
	}
	if order := defaultChildOrder(Query{Where: likeCondition("Name", "X%")}, "Contact"); len(order) != 0 {
		t.Fatalf("Contact Name child order = %v; want base", order)
	}
	if order := defaultChildOrder(Query{Where: likeCondition("LastName", "X%")}, "Lead"); len(order) != 0 {
		t.Fatalf("Lead LastName child order = %v; want base", order)
	}
	and := &Condition{And: []Condition{{Field: "Email", Op: "=", Value: storage.StringValue("x")}, *likeCondition("LastName", "X%")}}
	if order := defaultChildOrder(Query{Where: and}, "Contact"); len(order) != 0 {
		t.Fatalf("compound child order = %v; want base", order)
	}
	if order := defaultChildOrder(Query{}, "Contact"); len(order) != 0 {
		t.Fatalf("unfiltered child order = %v; want base", order)
	}
	if order := defaultChildOrder(Query{Where: likeCondition("LastName", "%C")}, "Contact"); len(order) != 0 {
		t.Fatalf("leading wildcard child order = %v; want base", order)
	}
}

// BenchmarkDefaultParentOrderBase measures the per-query check on the shapes
// that keep base order (no ORDER BY, no indexed name prefix).
func BenchmarkDefaultParentOrderBase(b *testing.B) {
	query := Query{Where: &Condition{And: []Condition{{Field: "Site", Op: "=", Value: storage.StringValue("x")}, {Field: "Industry", Op: "=", Value: storage.StringValue("y")}}}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = defaultParentOrder(query, "Account")
	}
}
