package soql

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestIDPointQueryResults(t *testing.T) {
	const id = "a0B00000000aCdx"
	ext := storage.ID18(id)
	other := id + "ZZZ"
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	obj := storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "IndexProbe__c", KeyPrefix: "a0B",
		Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString}}}}
	add := func(key, storedID, name string, deleted bool) {
		obj.PutRecord(storage.ID(key), storage.Record{ID: storage.ID(storedID), Object: "IndexProbe__c",
			Fields: map[string]storage.Value{"Name": storage.StringValue(name)}, System: storage.SystemFields{IsDeleted: deleted}})
	}
	add(id, id, "A", false)
	add(ext, ext, "B", false)
	add(other, other, "C", false)
	add(strings.ToLower(id), strings.ToLower(id), "D", false)
	add("opaque-old-key", id, "E", false)
	add("soft-deleted-key", ext, "F", true)
	add("detached-old-key", "a0B000000000002", "G", false)
	org.Objects["IndexProbe__c"] = obj
	names := func(xs string) []string {
		if xs == "" {
			return []string{}
		}
		return strings.Fields(xs)
	}
	for _, tc := range []struct {
		name      string
		v         storage.Value
		want      string
		all, deny bool
		limit     int
	}{
		{"typed15", storage.IDValue(id), "A B C E", false, false, 0},
		{"typed18", storage.IDValue(storage.ID(ext)), "A B E", false, false, 0},
		{"typed-different-suffix", storage.IDValue(storage.ID(other)), "A C E", false, false, 0},
		{"typed-case", storage.IDValue(storage.ID(strings.ToLower(id))), "D", false, false, 0},
		{"string-case-collisions", storage.StringValue(strings.ToLower(id)), "A B C D E", false, false, 0},
		{"all-rows", storage.IDValue(id), "A B C E F", true, false, 0},
		{"permission-before-window", storage.StringValue(id), "C D", false, true, 2},
		{"miss", storage.IDValue("a0B000000000009"), "", false, false, 0},
		{"null", storage.NullValue(), "", false, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := Query{Object: "IndexProbe__c", Fields: []string{"Name", "Id"}, Where: &Condition{Field: "Id", Op: "=", Value: tc.v},
				Order: []OrderSpec{{Field: "Name"}}, AllRows: tc.all}
			if tc.limit > 0 {
				q.HasLimit = true
				q.Limit = tc.limit
			}
			ctx := ExecutionContext{}
			if tc.deny {
				ctx.WindowRecordVisible = func(_ string, r storage.Record) bool { n := r.Fields["Name"].String; return n != "A" && n != "B" }
			}
			r, err := ExecuteWithCacheAndContext(org, q, nil, ctx)
			if err != nil {
				t.Fatal(err)
			}
			actual := make([]string, 0, len(r.Records))
			for _, rec := range r.Records {
				actual = append(actual, rec.Fields["Name"].String)
			}
			if !reflect.DeepEqual(actual, names(tc.want)) || r.Rows != len(actual) {
				t.Fatalf("%s: got %v rows=%d, want %q", tc.name, actual, r.Rows, tc.want)
			}
		})
	}
	ordinary := []*Condition{
		{Field: "Name", Op: "=", Value: storage.StringValue("A")},
		{Field: "Id", Op: "=", Value: storage.IntegerValue(2)},
		{Field: "Id", Op: "=", Value: storage.NullValue()},
		{Field: "Id", Op: "=", Value: storage.StringValue("short")},
		{Not: true, Field: "Id", Op: "=", Value: storage.IDValue(id)},
		{Field: "Owner.Id", Op: "=", Value: storage.IDValue(id)},
		{Field: "Id", Op: "IN", Values: []storage.Value{storage.IDValue(id)}},
		{And: []Condition{{Field: "Name", Op: "=", Value: storage.StringValue("A")}, {Field: "Id", Op: "=", Value: storage.IDValue(id)}}},
	}
	for _, w := range ordinary {
		got := candidateRecordIDs(obj, w, false)
		if len(got) != len(obj.Records) {
			t.Fatalf("fallback altered candidate set: %+v => %v", w, got)
		}
	}
}

func BenchmarkIDPointQueries(b *testing.B) {
	org := storage.NewOrgState()
	records := make(map[storage.ID]storage.Record, 3000)
	for i := 0; i < 3000; i++ {
		id := storage.ID(fmt.Sprintf("a0B%012d", i+1))
		records[id] = storage.Record{ID: id, Object: "Item__c", Fields: map[string]storage.Value{"Name": storage.StringValue("item")}}
	}
	org.Objects["Item__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Item__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: records}
	queries := make([]Query, 72)
	for i := range queries {
		queries[i] = Query{Object: "Item__c", Fields: []string{"Id"}, Where: &Condition{Field: "Id", Op: "=", Value: storage.IDValue(storage.ID(fmt.Sprintf("a0B%012d", i+1)))}}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, q := range queries {
			result, err := ExecuteWithCache(org, q, nil)
			if err != nil || result.Rows != 1 || result.Records[0].ID != q.Where.Value.ID {
				b.Fatalf("unexpected result: %+v %v", result, err)
			}
		}
	}
}
