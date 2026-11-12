package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A lazy org installs an object with the prefix the unique assignment gives
// it. Most catalog objects have no catalog prefix, so a lookup by the prefix a
// complete spread holds must find the cold object, as the eager spread does.
func TestLazySpreadPrefixLookupFindsReassignedColdObjects(t *testing.T) {
	eager := storage.NewOrgState()
	eager.OrgID = storage.DefaultOrgID
	storage.EnsureStandardObjectSpread(&eager)
	storage.EnsureUniqueKeyPrefixes(&eager)
	lazy := storage.NewOrgState()
	lazy.OrgID = storage.DefaultOrgID
	lazy.EnableStandardSpreadSource()
	storage.EnsureUniqueKeyPrefixes(&lazy)

	eagerVM, lazyVM := New(nil), New(nil)
	eagerVM.Org, lazyVM.Org = &eager, &lazy
	checked := 0
	for name, state := range eager.Objects {
		prefix := state.Definition.KeyPrefix
		if prefix == "" || storage.StandardKeyPrefix(name) == prefix {
			continue
		}
		if _, held := lazy.Objects[name]; held {
			continue
		}
		want, wantOK := eagerVM.sObjectNameForIDPrefix(prefix)
		got, gotOK := lazyVM.sObjectNameForIDPrefix(prefix)
		if got != want || gotOK != wantOK {
			t.Fatalf("prefix %q: lazy lookup got %q, %v; eager spread holds %q, %v", prefix, got, gotOK, want, wantOK)
		}
		id := prefix + "000000000001AAA"
		wantID, wantIDOK := eagerVM.sObjectNameForID(id)
		gotID, gotIDOK := lazyVM.sObjectNameForID(id)
		if gotID != wantID || gotIDOK != wantIDOK {
			t.Fatalf("id %q: lazy lookup got %q, %v; eager spread holds %q, %v", id, gotID, gotIDOK, wantID, wantIDOK)
		}
		if checked++; checked >= 40 {
			break
		}
	}
	if checked == 0 {
		t.Fatal("no cold object with a reassigned prefix to check")
	}
}
