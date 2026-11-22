package vm

import (
	"fmt"
	"testing"
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
