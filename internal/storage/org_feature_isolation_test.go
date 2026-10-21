package storage

import (
	"reflect"
	"testing"
)

func TestOrgShapeHelpersKeepFrozenDefinitionsPrivate(t *testing.T) {
	for _, tc := range []struct{ feature, object string }{
		{"Sites", "Site"}, {"Communities", "Network"},
		{"ContactsToMultipleAccounts", "AccountContactRelation"},
		{"PlatformCache", "PlatformCachePartition"}, {"AddCustomApps:1", "CustomApplication"},
	} {
		t.Run(tc.feature, func(t *testing.T) {
			org := NewOrgState()
			org.Objects[tc.object] = ObjectState{Definition: ObjectDefinition{
				APIName: tc.object, Fields: map[string]Field{"Id": {APIName: "Id", Type: FieldID}},
			}}
			before := org.Objects[tc.object].Definition.Clone()
			clone := org.CloneRuntimeFrozenDefinition()
			ApplyOrgShape(&clone, []string{tc.feature})
			if !reflect.DeepEqual(org.Objects[tc.object].Definition, before) {
				t.Fatal("feature helper mutated the frozen parent definition")
			}
			if len(clone.Objects[tc.object].Definition.Fields) <= len(before.Fields) {
				t.Fatal("clone did not receive the feature fields")
			}
		})
	}
}

func TestRepeatedPersonAccountShapeKeepsFrozenRelationBackingPrivate(t *testing.T) {
	org := NewOrgState()
	ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	n := len(account.Definition.Relations)
	backing := make([]Relationship, n+1)
	copy(backing, account.Definition.Relations)
	backing[n] = Relationship{Field: "Reserved"}
	account.Definition.Relations = backing[:n]
	org.Objects["Account"] = account
	clone := org.CloneRuntimeFrozenDefinition()
	ApplyOrgShape(&clone, []string{"PersonAccounts"})
	if backing[n].Field != "Reserved" {
		t.Fatal("reapplying the feature mutated the frozen parent's relation backing array")
	}
}
