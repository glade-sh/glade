package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestLookupFilterDescribeCloneIsolation(t *testing.T) {
	original := storage.FilteredLookupInfo{FilterItems: []storage.LookupFilterItem{{Field: "Account.Name", Operation: "equals", Value: "before"}}}
	cloned := cloneFilteredLookupInfo(original)
	cloned.FilterItems[0].Value = "after"
	if original.FilterItems[0].Value != "before" {
		t.Fatal("describe copy mutated original criteria")
	}
}
