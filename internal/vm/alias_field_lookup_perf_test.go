package vm

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func aliasFieldLookupFixture() Value {
	row := Object("Sample__c")
	row.Fields["Target__c"] = Int(7)
	for i := 0; i < 64; i++ {
		row.Fields[fmt.Sprintf("Field%03d__c", i)] = Int(int64(i))
	}
	markExplicitSObjectField(&row, "Field000__c")
	return row
}

func TestExplicitAliasLookupAvoidsUnrelatedFieldAllocations(t *testing.T) {
	row := aliasFieldLookupFixture()
	allocations := testing.AllocsPerRun(100, func() {
		name, value, ok := explicitAliasObjectFieldValue(row, "Target__c")
		if ok || name != "" || value.Kind != ValueNull {
			t.Fatalf("unexpected alias: %q %#v %v", name, value, ok)
		}
	})
	if allocations != 0 {
		t.Fatalf("lookup allocated %.0f times for unrelated fields; want 0", allocations)
	}
}

func TestAliasFieldLookupPreservesMutationAndNamespace(t *testing.T) {
	row := aliasFieldLookupFixture()
	row.Fields["pkg__Target__c"] = Int(12)
	markExplicitSObjectField(&row, "pkg__Target__c")
	name, value, ok := objectFieldValue(row, "Target__c")
	if !ok || name != "pkg__Target__c" || value.Int != 12 {
		t.Fatalf("explicit alias = %q %#v %v", name, value, ok)
	}
	setExplicitSObjectField(&row, "Target__c", Null)
	name, value, ok = objectFieldValue(row, "Target__c")
	if !ok || name != "Target__c" || value.Kind != ValueNull {
		t.Fatalf("mutated explicit null = %q %#v %v", name, value, ok)
	}
	if name, _, ok := explicitAliasObjectFieldValue(row, "Unrelated__c"); ok {
		t.Fatalf("unrelated field matched %q", name)
	}
}

func TestExplicitAliasLookupMatchesBaselineNeighbors(t *testing.T) {
	for _, tc := range []struct {
		name, candidate, query string
		explicit               bool
		value                  Value
	}{
		{"qualified", "pkg__Target__c", "Target__c", true, Int(12)},
		{"case folded", "tArGeT__c", "Target__c", true, String("value")},
		{"explicit null", "pkg__Target__c", "Target__c", true, Null},
		{"not explicit", "pkg__Target__c", "Target__c", false, Int(12)},
		{"different field", "pkg__Other__c", "Target__c", true, Int(12)},
		{"exact field", "Target__c", "Target__c", true, Int(12)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := aliasFieldLookupFixture()
			row.Fields[tc.candidate] = tc.value
			if tc.explicit {
				markExplicitSObjectField(&row, tc.candidate)
			}
			wantName, wantValue, wantOK := baselineExplicitAliasObjectFieldValue(row, tc.query)
			gotName, gotValue, gotOK := explicitAliasObjectFieldValue(row, tc.query)
			if gotName != wantName || gotOK != wantOK || !reflect.DeepEqual(gotValue, wantValue) {
				t.Fatalf("lookup = %q %#v %v, baseline = %q %#v %v", gotName, gotValue, gotOK, wantName, wantValue, wantOK)
			}
		})
	}
}

// This is the lookup at origin/main, retained as a differential oracle.
func baselineExplicitAliasObjectFieldValue(object Value, name string) (string, Value, bool) {
	stripped := storage.StripAnyNamespaceToken(name)
	for candidate, value := range object.Fields {
		if candidate == name || !isExplicitSObjectField(object, candidate) {
			continue
		}
		if strings.EqualFold(candidate, name) || strings.EqualFold(storage.StripAnyNamespaceToken(candidate), stripped) {
			return candidate, value, true
		}
	}
	return "", Null, false
}

func BenchmarkAliasFieldLookupWideRecord(b *testing.B) {
	row := aliasFieldLookupFixture()
	b.ReportAllocs()
	for range b.N {
		name, value, ok := objectFieldValue(row, "Target__c")
		if !ok || name != "Target__c" || value.Kind != ValueInt || value.Int != 7 {
			b.Fatalf("lookup = %q %#v %v", name, value, ok)
		}
	}
}
