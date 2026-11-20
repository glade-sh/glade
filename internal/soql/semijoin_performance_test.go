package soql

import (
	"fmt"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func scaleJoinAccountID(i int) storage.ID {
	return storage.ID(fmt.Sprintf("001%012d", i+1))
}

func scaleJoinOrg(n int, mode string) (storage.OrgState, map[storage.ID]bool) {
	org := storage.NewOrgState()
	parents := make(map[storage.ID]storage.Record, n)
	children := make(map[storage.ID]storage.Record, n)
	matched := make(map[storage.ID]bool)
	for i := range n {
		id := scaleJoinAccountID(i)
		parents[id] = storage.Record{Object: "Account", ID: id}
	}
	for i := range n {
		parentIndex := 0
		switch mode {
		case "distinct":
			// Two child rows for each even parent, no rows for odd parents.
			parentIndex = 2 * (i % ((n + 1) / 2))
		case "empty":
			continue
		case "fanout":
		default:
			panic("unknown fixture")
		}
		id := storage.ID(fmt.Sprintf("003%012d", i+1))
		parentID := scaleJoinAccountID(parentIndex)
		children[id] = storage.Record{Object: "Contact", ID: id,
			Fields: map[string]storage.Value{"AccountId": storage.IDValue(parentID)}}
		matched[parentID] = true
	}
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account"}, Records: parents}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
			"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		}}, Records: children}
	return org, matched
}

func scaleJoinQuery(anti bool) string {
	op := "IN"
	if anti {
		op = "NOT IN"
	}
	return "SELECT Id FROM Account WHERE Id " + op + " (SELECT AccountId FROM Contact)"
}

func checkScaleJoin(t testing.TB, result Result, n int, matched map[storage.ID]bool, anti bool) {
	t.Helper()
	expected := make([]storage.ID, 0)
	for i := range n {
		id := scaleJoinAccountID(i)
		if matched[id] != anti {
			expected = append(expected, id)
		}
	}
	observed := make([]storage.ID, len(result.Records))
	for i, row := range result.Records {
		observed[i] = row.ID
	}
	if result.Rows != len(expected) || result.AggregateInputRows != nil || !slices.Equal(expected, observed) {
		t.Fatalf("rows=%d, first IDs %v, expected %d with first IDs %v",
			result.Rows, observed[:min(3, len(observed))], len(expected), expected[:min(3, len(expected))])
	}
}

func TestSemiJoinPerformanceResults(t *testing.T) {
	for _, n := range []int{3, 64} {
		for _, mode := range []string{"empty", "fanout", "distinct"} {
			for _, anti := range []bool{false, true} {
				org, matched := scaleJoinOrg(n, mode)
				query, err := Parse(scaleJoinQuery(anti))
				if err != nil {
					t.Fatal(err)
				}
				cache := NewExecutionCache()
				for phase := 0; phase < 2; phase++ {
					result, err := ExecuteWithCache(org, query, cache)
					if err != nil {
						t.Fatal(err)
					}
					checkScaleJoin(t, result, n, matched, anti)
					if phase == 0 {
						obj := org.Objects["Contact"]
						parent := scaleJoinAccountID(1)
						id := storage.ID("003000000900001")
						obj.PutRecord(id, storage.Record{Object: "Contact", ID: id, Fields: map[string]storage.Value{"AccountId": storage.IDValue(parent)}})
						nullID := storage.ID("003000000900002")
						obj.PutRecord(nullID, storage.Record{Object: "Contact", ID: nullID, Fields: map[string]storage.Value{"AccountId": storage.NullValue()}})
						org.Objects["Contact"] = obj
						matched[parent] = true
					}
				}
			}
		}
	}
}

func BenchmarkSemiJoin(b *testing.B) {
	for _, mode := range []string{"fanout", "distinct"} {
		for _, anti := range []bool{false, true} {
			op := "in"
			if anti {
				op = "notin"
			}
			for _, n := range []int{8, 250, 500, 1000, 2000, 4000} {
				b.Run(fmt.Sprintf("%s/%s/N%d", mode, op, n), func(b *testing.B) {
					org, matched := scaleJoinOrg(n, mode)
					query, err := Parse(scaleJoinQuery(anti))
					if err != nil {
						b.Fatal(err)
					}
					cache := NewExecutionCache()
					var last Result
					b.ReportAllocs()
					for b.Loop() {
						last, err = ExecuteWithCache(org, query, cache)
						if err != nil {
							b.Fatal(err)
						}
					}
					checkScaleJoin(b, last, n, matched, anti)
				})
			}
		}
	}
}

func TestSemiJoinIDEqualityResults(t *testing.T) {
	ids := []storage.ID{"00100000000AbCd", "00100000000AbCdAAA", "00100000000AbCdBBB", "00100000000aBCd", "001000000000099"}
	for _, kind := range []string{"id", "string"} {
		for _, anti := range []bool{false, true} {
			org := storage.NewOrgState()
			parents := make(map[storage.ID]storage.Record)
			children := make(map[storage.ID]storage.Record)
			for _, id := range ids {
				parents[id] = storage.Record{Object: "Account", ID: id}
			}
			var values []storage.Value
			for i := 0; i < 20; i++ {
				id := storage.ID(fmt.Sprintf("003%012d", i+1))
				value := storage.IDValue(ids[i%2])
				if kind == "string" {
					value = storage.StringValue(string(ids[i%2]))
				}
				values = append(values, idFieldValue(value))
				children[id] = storage.Record{Object: "Contact", ID: id, Fields: map[string]storage.Value{"AccountId": value}}
			}
			org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account"}, Records: parents}
			org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}}, Records: children}
			query, err := Parse(scaleJoinQuery(anti))
			if err != nil {
				t.Fatal(err)
			}
			result, err := ExecuteWithCache(org, query, nil)
			if err != nil {
				t.Fatal(err)
			}
			var want []storage.ID
			for _, id := range ids {
				found := false
				for _, value := range values {
					if equalValues(storage.IDValue(id), value) {
						found = true
						break
					}
				}
				if found != anti {
					want = append(want, id)
				}
			}
			slices.Sort(want)
			got := make([]storage.ID, 0, len(result.Records))
			for _, row := range result.Records {
				got = append(got, row.ID)
			}
			if !slices.Equal(got, want) || result.Rows != len(want) {
				t.Fatalf("kind=%s anti=%v got %v want %v", kind, anti, got, want)
			}
		}
	}
}
