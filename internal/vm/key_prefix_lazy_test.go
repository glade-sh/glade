package vm

import (
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func lazyPrefixOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.OrgID = storage.DefaultOrgID
	org.EnableStandardSpreadSource()
	storage.EnsureUniqueKeyPrefixes(&org)
	org.RuntimeSchemaStamp = "lazy-stamp"
	return org
}

// Every prefix of the completed spread must resolve to the object the eager
// spread reports, whatever clones of the same generation installed before.
func TestKeyPrefixLookupOnLazySpreadMatchesEager(t *testing.T) {
	eager := storage.NewOrgState()
	eager.OrgID = storage.DefaultOrgID
	storage.EnsureStandardObjectSpread(&eager)
	storage.EnsureUniqueKeyPrefixes(&eager)
	eager.RuntimeSchemaStamp = "eager-stamp"
	want := map[string]string{}
	names := make([]string, 0, len(eager.Objects))
	for name := range eager.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		prefix := eager.Objects[name].Definition.KeyPrefix
		if _, taken := want[prefix]; !taken && prefix != "" {
			want[prefix] = name
		}
	}

	template := lazyPrefixOrg()
	// Two clones of one generation install different objects, one apiece, so
	// their object counts agree while their object sets differ.
	left, right := template.Clone(), template.Clone()
	var installed []string
	for _, name := range storage.KnownStandardObjectNames() {
		if _, held := left.Objects[name]; held {
			continue
		}
		if canonical, ok := storage.LazyStandardObjectName(&left, name); ok && canonical == name {
			installed = append(installed, name)
		}
		if len(installed) == 2 {
			break
		}
	}
	if len(installed) < 2 {
		t.Fatalf("found %d owed objects, want 2", len(installed))
	}
	if _, ok := storage.EnsureLazyStandardObject(&left, installed[0]); !ok {
		t.Fatalf("%s not installed", installed[0])
	}
	if _, ok := storage.EnsureLazyStandardObject(&right, installed[1]); !ok {
		t.Fatalf("%s not installed", installed[1])
	}
	if len(left.Objects) != len(right.Objects) {
		t.Fatalf("clones hold %d and %d objects, want equal counts", len(left.Objects), len(right.Objects))
	}
	lm, rm := New(nil), New(nil)
	lm.Org, rm.Org = &left, &right
	// Build any generation index from the left clone first.
	lm.orgObjectNameForKeyPrefix(left.Objects[installed[0]].Definition.KeyPrefix)
	for _, machine := range []*VM{rm, lm} {
		for prefix, name := range want {
			if got, ok := machine.sObjectNameForIDPrefix(prefix); !ok || got != name {
				t.Fatalf("prefix %q: lazy lookup got %q, %v; eager spread holds %q", prefix, got, ok, name)
			}
		}
	}
	// The completed org agrees too, and matches the eager prefixes object by object.
	full := template.Clone()
	storage.MaterializeStandardObjectSpread(&full)
	if len(full.Objects) != len(eager.Objects) {
		t.Fatalf("completed spread holds %d objects, eager %d", len(full.Objects), len(eager.Objects))
	}
	for name, state := range eager.Objects {
		if got := full.Objects[name].Definition.KeyPrefix; got != state.Definition.KeyPrefix {
			t.Fatalf("%s: completed spread prefix %q, eager %q", name, got, state.Definition.KeyPrefix)
		}
	}
	fm := New(nil)
	fm.Org = &full
	for prefix, name := range want {
		if got, ok := fm.orgObjectNameForKeyPrefix(prefix); !ok || got != name {
			t.Fatalf("prefix %q: completed lookup got %q, %v; want %q", prefix, got, ok, name)
		}
	}
}
