package soql

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type case332OldRelationshipCache struct {
	mu      sync.Mutex
	entries map[string]childRelationshipResolution
}

func (c *case332OldRelationshipCache) resolve(org storage.OrgState, parent storage.ObjectDefinition, relationship string) (string, storage.Relationship, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]childRelationshipResolution)
	}
	key := strings.ToLower(org.Namespace) + "\x00" + strings.ToLower(parent.APIName) + "\x00" + strings.ToLower(relationship)
	if v, ok := c.entries[key]; ok {
		return v.childObjectName, v.relation, v.ok
	}
	name, rel, ok := childRelationship(org, parent, relationship)
	c.entries[key] = childRelationshipResolution{childObjectName: name, relation: rel, ok: ok}
	return name, rel, ok
}

func TestCase332RelationshipCacheAgainstOriginal(t *testing.T) {
	org := storage.NewOrgState()
	parent := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}
	org.Objects["Account"] = storage.ObjectState{Definition: parent}
	org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}, Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account", ChildRelationship: "Contacts"}}}}
	for _, names := range [][]string{{"Contacts", "contacts", "Missing", "missing", "ns__Contacts"}, {"Missing", "missing", "contacts", "Contacts", "ns__Contacts"}} {
		old := &case332OldRelationshipCache{}
		candidate := newChildRelationshipQueryCache(NewExecutionCache())
		for _, name := range names {
			wantName, wantRel, wantOK := old.resolve(org, parent, name)
			gotName, gotRel, gotOK := childRelationshipCached(org, parent, name, candidate)
			if gotName != wantName || gotOK != wantOK || !reflect.DeepEqual(gotRel, wantRel) {
				t.Fatalf("%q: got %q %#v %v, want %q %#v %v", name, gotName, gotRel, gotOK, wantName, wantRel, wantOK)
			}
		}
	}
}
