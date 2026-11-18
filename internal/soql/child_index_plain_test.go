package soql

import (
	"reflect"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The hoisted field resolution must build the same index as the generic
// recordValue route for every record shape the index can meet.
func TestChildRelationshipIndexPlainFieldMatchesRecordValueRoute(t *testing.T) {
	org := childIndexBenchOrg(3, 2)
	obj := org.Objects["Contact"]
	a0, a1 := storage.ID("001000000000000"), storage.ID("001000000000001")
	obj.Records["003x1"] = storage.Record{ID: "003x1", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.StringValue(string(a0))}}
	obj.Records["003x2"] = storage.Record{ID: "003x2", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.NullValue()}}
	obj.Records["003x3"] = storage.Record{ID: "003x3", Object: "Contact"}
	obj.Records["003x4"] = storage.Record{ID: "003x4", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(a1 + "AAA")}}
	deleted := storage.Record{ID: "003x5", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(a0)}}
	deleted.System.IsDeleted = true
	obj.Records["003x5"] = deleted
	cleared := storage.Record{ID: "003x6", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(a0)}}
	cleared.ExplicitNulls = map[string]bool{"AccountId": true}
	obj.Records["003x6"] = cleared
	org.Objects["Contact"] = obj

	for _, field := range []string{"AccountId", "accountid"} {
		if _, plain := childRelationshipPlainReferenceField(org, obj.Definition, field); !plain {
			t.Fatalf("%s should take the plain route", field)
		}
		got := childRelationshipIndex(org, "Contact", obj, field, nil)
		want := make(map[storage.ID][]string)
		for id, record := range obj.Records {
			if record.System.IsDeleted {
				continue
			}
			value, ok := recordValue(org, obj.Definition, record, field)
			if !ok {
				continue
			}
			parentID := idFromValue(value)
			if parentID == "" {
				continue
			}
			parentID = storage.IDIndexKey(parentID)
			want[parentID] = append(want[parentID], string(id))
		}
		for key := range want {
			sort.Strings(want[key])
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: index differs\n got %v\nwant %v", field, got, want)
		}
	}
	for _, field := range []string{"Id", "OwnerId", "Account.Id", "Missing__c"} {
		if _, plain := childRelationshipPlainReferenceField(org, obj.Definition, field); plain {
			t.Fatalf("%s must keep the generic route", field)
		}
	}
}
