package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHiddenSpreadAccessInvalidatesVirtualSchemaCache(t *testing.T) {
	org := storage.NewOrgState()
	org.RuntimeSchemaStamp = "fixed"
	org.StandardSpreadHiddenAccess = map[string][]string{"Hidden__c": {"First__c"}}
	query, err := Parse("SELECT DurableId FROM UserFieldAccess")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	check := func(want ...storage.ID) {
		t.Helper()
		result, err := ExecuteWithCache(org, query, cache)
		if err != nil {
			t.Fatal(err)
		}
		got := make(map[storage.ID]bool, len(result.Records))
		for _, record := range result.Records {
			got[record.ID] = true
		}
		if len(got) != len(want) {
			t.Fatalf("record count %d, want %d", len(got), len(want))
		}
		for _, id := range want {
			if !got[id] {
				t.Errorf("missing record %s", id)
			}
		}
	}
	check("Hidden__c.First__c")
	org.StandardSpreadHiddenAccess["Hidden__c"][0] = "Other__c"
	check("Hidden__c.Other__c")
	fields := org.StandardSpreadHiddenAccess["Hidden__c"]
	delete(org.StandardSpreadHiddenAccess, "Hidden__c")
	org.StandardSpreadHiddenAccess["Another__c"] = fields
	check("Another__c.Other__c")
	org.StandardSpreadHiddenAccess["Another__c"] = append(fields, "Extra__c")
	check("Another__c.Other__c", "Another__c.Extra__c")
	org.StandardSpreadHiddenAccess = nil
	check()
}
