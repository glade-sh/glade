package sema

import (
	"github.com/glade-sh/glade/internal/schema"
	"testing"
)

func TestLookupFilterSchemaCloneIsolation(t *testing.T) {
	original := schema.Field{FilteredLookupInfo: schema.FilteredLookupInfo{FilterItems: []schema.LookupFilterItem{{Field: "Account.Name", Operation: "equals", Value: "before"}}}}
	cloned := semaCloneSchemaField(original)
	cloned.FilteredLookupInfo.FilterItems[0].Value = "after"
	if original.FilteredLookupInfo.FilterItems[0].Value != "before" {
		t.Fatal("semantic field copy mutated original criteria")
	}
}
