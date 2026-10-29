package storage

import (
	"fmt"
	"testing"
)

func TestObjectNameCacheFollowsDeclaredGeneration(t *testing.T) {
	for _, boundary := range []string{"clear", "runtime", "mutable"} {
		for _, change := range []string{"positive", "negative", "preferred"} {
			t.Run(boundary+"/"+change, func(t *testing.T) {
				org := NewOrgState()
				org.Objects["Owned__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Owned__c", Fields: map[string]Field{"Name": {APIName: "Name"}}}}
				org.Objects["pkg__Owned__c"] = ObjectState{Definition: ObjectDefinition{APIName: "pkg__Owned__c", Fields: map[string]Field{}}}
				name := "owned__c"
				if change == "negative" {
					name = "fresh__c"
				}
				if change == "preferred" {
					name = "Owned__c"
				}
				beforeName, beforeOK := ResolveObjectName(org, name)
				sibling := org.Clone()
				if change == "preferred" {
					state := org.Objects["pkg__Owned__c"]
					state.Definition = state.Definition.Clone()
					state.Definition.Fields["Name"] = Field{APIName: "Name"}
					state.Definition.Fields["Extra__c"] = Field{APIName: "Extra__c"}
					org.Objects["pkg__Owned__c"] = state
				} else {
					delete(org.Objects, "Owned__c")
					delete(org.Objects, "pkg__Owned__c")
					org.Objects["Fresh__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Fresh__c"}}
					org.Objects["Carry__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Carry__c"}}
				}
				switch boundary {
				case "clear":
					org.ClearRuntimeSchemaStamp()
				case "runtime":
					org.InvalidateRuntimeGeneration()
				case "mutable":
					object := "Fresh__c"
					if change == "preferred" {
						object = "Owned__c"
					}
					if _, ok := EnsureMutableObjectDefinition(&org, object); !ok {
						t.Fatal("missing mutation target")
					}
				}
				fresh := org.Clone()
				// Clone can carry the same index, so explicitly cold means no
				// cached table or name result, not another alias of stale state.
				fresh.objectNameCache = nil
				wantName, wantOK := ResolveObjectName(fresh, name)
				if wantName == beforeName && wantOK == beforeOK {
					t.Fatal("unchanged control")
				}
				if gotName, gotOK := ResolveObjectName(org, name); gotName != wantName || gotOK != wantOK {
					t.Fatalf("warm=%s/%v cold=%s/%v", gotName, gotOK, wantName, wantOK)
				}
				if gotName, gotOK := ResolveObjectName(sibling, name); gotName != beforeName || gotOK != beforeOK {
					t.Fatalf("sibling changed: %s/%v", gotName, gotOK)
				}
			})
		}
	}
}

func TestDeleteObjectOwnsNameCacheInvalidation(t *testing.T) {
	for _, present := range []bool{false, true} {
		t.Run(fmt.Sprint(present), func(t *testing.T) {
			org := NewOrgState()
			org.Objects["Owned__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Owned__c"}}
			index := objectNameIndexFor(org)
			generation := org.SchemaGeneration()
			name := "Missing__c"
			if present {
				name = "Owned__c"
			}
			DeleteObject(&org, name)
			if (org.SchemaGeneration() != generation) != present {
				t.Fatal("deletion did not track actual schema mutation")
			}
			if !present && objectNameIndexFor(org) != index {
				t.Fatal("missing-object no-op discarded preparation")
			}
		})
	}
}

func TestObjectNameCacheRejectsForeignObjectMaps(t *testing.T) {
	for _, forkFirst := range []bool{false, true} {
		for _, folded := range []bool{false, true} {
			t.Run(fmt.Sprintf("fork-first=%v/folded=%v", forkFirst, folded), func(t *testing.T) {
				org := NewOrgState()
				org.Objects["Source__c"] = ObjectState{}
				index := objectNameIndexFor(org)
				fork := org
				fork.Objects = map[string]ObjectState{"Other__c": {}}
				names := []string{"Source__c", "Other__c"}
				if folded {
					names = []string{"source__c", "other__c"}
				}
				owners := []OrgState{org, fork}
				if forkFirst {
					owners = []OrgState{fork, org}
				}
				for _, owner := range owners {
					for _, name := range names {
						cold := owner
						cold.objectNameCache = nil
						want, wantOK := ResolveObjectName(cold, name)
						if got, gotOK := ResolveObjectName(owner, name); got != want || gotOK != wantOK {
							t.Fatalf("name=%s cached=%s/%v cold=%s/%v", name, got, gotOK, want, wantOK)
						}
					}
				}
				if objectNameIndexFor(org) != index {
					t.Fatal("foreign map replaced the owner's immutable name preparation")
				}
			})
		}
	}
}
