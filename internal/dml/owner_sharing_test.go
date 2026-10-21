package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func ownerRuleOrg() storage.OrgState {
	org := testOrg()
	org.Objects["OwnedBox__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "OwnedBox__c", KeyPrefix: "a10", SharingModel: "Private", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}, OwnerSharingRules: []storage.OwnerSharingRule{{Name: "OwnedReadGrant", SourceGroup: "OwnedOrigin", TargetGroup: "OwnedTarget", AccessLevel: "Read"}}}, Records: map[storage.ID]storage.Record{}}
	org.Objects["OwnedBox__Share"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "OwnedBox__Share", KeyPrefix: "a11", Metadata: map[string]string{"kind": "generatedShare", "parentObject": "OwnedBox__c"}}, Records: map[storage.ID]storage.Record{}}
	org.Objects["Group"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Group", KeyPrefix: "00G"}, Records: map[storage.ID]storage.Record{}}
	for id, name := range map[storage.ID]string{"00G000000000001": "OwnedOrigin", "00G000000000002": "OwnedTarget"} {
		org.Objects["Group"].Records[id] = storage.Record{Object: "Group", ID: id, Fields: map[string]storage.Value{"DeveloperName": storage.StringValue(name), "Type": storage.StringValue("Regular")}}
	}
	org.Objects["GroupMember"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "GroupMember", KeyPrefix: "011"}, Records: map[storage.ID]storage.Record{"011000000000001": {Object: "GroupMember", ID: "011000000000001", Fields: map[string]storage.Value{"GroupId": storage.IDValue("00G000000000001"), "UserOrGroupId": storage.IDValue("005000000000002")}}}}
	org.IDSequences["Group"] = 9
	org.IDSequences["GroupMember"] = 11
	org.IDSequences["UserRole"] = 13
	return org
}

func TestPrivateOwnerRuleShareLifecycle(t *testing.T) {
	org := ownerRuleOrg()
	engine := NewEngine(&org)
	inserted := engine.Insert([]storage.Record{{Object: "OwnedBox__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Owned")}}})
	if !inserted[0].Success {
		t.Fatal(inserted)
	}
	id := inserted[0].ID
	assertRows := func(owner storage.ID, rule bool) {
		t.Helper()
		owners, rules := 0, 0
		for _, row := range org.Objects["OwnedBox__Share"].Records {
			if row.Fields["ParentId"].ID != id {
				t.Fatal("wrong share parent")
			}
			switch row.Fields["RowCause"].String {
			case "Owner":
				owners++
				if row.Fields["UserOrGroupId"].ID != owner || row.Fields["AccessLevel"].String != "All" {
					t.Fatal("wrong Owner row", row)
				}
			case "Rule":
				rules++
				if row.Fields["UserOrGroupId"].ID != "00G000000000002" || row.Fields["AccessLevel"].String != "Read" {
					t.Fatal("wrong Rule row", row)
				}
			default:
				t.Fatal("unexpected share", row)
			}
		}
		if owners != 1 || rules != map[bool]int{false: 0, true: 1}[rule] {
			t.Fatalf("owner/rule count=%d/%d", owners, rules)
		}
	}
	assertRows(storage.DefaultUserID, false)
	frozen := org.CloneRuntimeFrozenShared()
	before := frozen.Objects["OwnedBox__Share"].Clone()
	share := org.Objects["OwnedBox__Share"]
	share.Indexes = map[string]storage.IndexSet{"cached": {}}
	org.Objects["OwnedBox__Share"] = share
	result := engine.Update([]storage.Record{{Object: "OwnedBox__c", ID: id, Fields: map[string]storage.Value{"OwnerId": storage.IDValue("005000000000002")}}})
	if !result[0].Success {
		t.Fatal(result)
	}
	assertRows("005000000000002", true)
	if org.Objects["OwnedBox__Share"].Indexes != nil {
		t.Fatal("stale share candidate indexes retained")
	}
	if !reflect.DeepEqual(frozen.Objects["OwnedBox__Share"].Records, before.Records) {
		t.Fatal("share mutation escaped COW snapshot")
	}
	sequences := ownerShareSequencesCopy(org.IDSequences)
	noop := engine.Update([]storage.Record{{Object: "OwnedBox__c", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Renamed")}}})
	if !noop[0].Success || !reflect.DeepEqual(sequences, org.IDSequences) {
		t.Fatal("unchanged owner changed IDs", noop)
	}
	assertRows("005000000000002", true)
	result = engine.Update([]storage.Record{{Object: "OwnedBox__c", ID: id, Fields: map[string]storage.Value{"OwnerId": storage.IDValue(storage.DefaultUserID)}}})
	if !result[0].Success {
		t.Fatal(result)
	}
	assertRows(storage.DefaultUserID, false)
	for name, want := range map[string]uint64{"Group": 9, "GroupMember": 11, "UserRole": 13} {
		if org.IDSequences[name] != want {
			t.Fatalf("unrelated %s counter changed", name)
		}
	}
}

func ownerShareSequencesCopy(values map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}

func TestPrivateOwnerSharesRollbackAndAbsentRuleControls(t *testing.T) {
	for _, journal := range []bool{false, true} {
		org := ownerRuleOrg()
		engine := NewEngine(&org)
		if journal {
			engine.IsolationJournal = storage.NewIsolationJournal(&org)
		}
		inserted := engine.Insert([]storage.Record{{Object: "OwnedBox__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Owned")}}})
		if !inserted[0].Success {
			t.Fatal(inserted)
		}
		before := org.Clone()
		rollback := engine.beginRollbackPoint(true)
		changed := engine.Update([]storage.Record{{Object: "OwnedBox__c", ID: inserted[0].ID, Fields: map[string]storage.Value{"OwnerId": storage.IDValue("005000000000002")}}})
		if !changed[0].Success {
			t.Fatal(changed)
		}
		if err := engine.restoreRollbackPoint(rollback); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
			t.Fatal("rollback lost counters", journal)
		}
		for name, state := range before.Objects {
			after := org.Objects[name]
			if !reflect.DeepEqual(state.Records, after.Records) || !reflect.DeepEqual(state.Definition, after.Definition) {
				t.Fatalf("rollback lost %s records or definition (journal=%v)", name, journal)
			}
		}
	}
	for _, shape := range []string{"absent rule", "nonPrivate", "nonGenerated", "missing target", "no member"} {
		t.Run(shape, func(t *testing.T) {
			org := ownerRuleOrg()
			state := org.Objects["OwnedBox__c"]
			switch shape {
			case "absent rule":
				state.Definition.OwnerSharingRules = nil
			case "nonPrivate":
				state.Definition.SharingModel = "ReadWrite"
			case "nonGenerated":
				s := org.Objects["OwnedBox__Share"]
				s.Definition.Metadata = nil
				org.Objects["OwnedBox__Share"] = s
			case "missing target":
				delete(org.Objects["Group"].Records, "00G000000000002")
			case "no member":
				delete(org.Objects["GroupMember"].Records, "011000000000001")
			}
			org.Objects["OwnedBox__c"] = state
			engine := NewEngine(&org)
			engine.UserID = "005000000000002"
			r := engine.Insert([]storage.Record{{Object: "OwnedBox__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Control")}}})
			if !r[0].Success {
				t.Fatal(r)
			}
			want := 1
			if shape == "nonPrivate" || shape == "nonGenerated" {
				want = 0
			}
			if got := len(org.Objects["OwnedBox__Share"].Records); got != want {
				t.Fatalf("share count=%d want %d", got, want)
			}
			for _, share := range org.Objects["OwnedBox__Share"].Records {
				if share.Fields["RowCause"].String != "Owner" {
					t.Fatal("unsupported shape fabricated Rule", share)
				}
			}
		})
	}
}
