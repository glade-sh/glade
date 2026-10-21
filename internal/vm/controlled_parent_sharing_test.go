package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Controlled-by-parent read access follows the master-detail metadata, rather
// than the detail's stored owner or an unrelated public lookup target.
func TestControlledParentPublicReadSharing(t *testing.T) {
	for _, model := range []string{"ReadWrite", "ReadOnly"} {
		t.Run(model, func(t *testing.T) {
			machine, detail := controlledParentReadFixture(t, model)
			if !machine.userModeRecordVisible(detail.Object, detail, machine.currentUserID()) {
				t.Fatal("another owner's detail is hidden despite its public master")
			}
			projected := detail
			projected.Fields = map[string]storage.Value{}
			if !machine.userModeRecordVisible(projected.Object, projected, machine.currentUserID()) {
				t.Fatal("omitting the master field from SELECT changed detail visibility")
			}
			if machine.soqlObjectHasPublicReadSharing(detail.Object) {
				t.Fatal("controlled-by-parent became an unconditional object grant")
			}
		})
	}
}

// These controls preserve the boundary of the direct, single-master public
// read case; they do not assert broader parent-sharing or write semantics.
func TestControlledParentPublicReadSharingBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*VM, *storage.Record)
	}{
		{"private master", func(machine *VM, _ *storage.Record) {
			state := machine.Org.Objects["OwnedMaster__c"]
			state.Definition.SharingModel = "Private"
			machine.Org.Objects["OwnedMaster__c"] = state
		}},
		{"unrelated public lookup", func(machine *VM, detail *storage.Record) {
			state := machine.Org.Objects[detail.Object]
			field := state.Definition.Fields["Master__c"]
			field.MasterDetail = false
			state.Definition.Fields["Master__c"] = field
			machine.Org.Objects[detail.Object] = state
		}},
		{"private master with public lookup", func(machine *VM, detail *storage.Record) {
			master := machine.Org.Objects["OwnedMaster__c"]
			master.Definition.SharingModel = "Private"
			machine.Org.Objects["OwnedMaster__c"] = master
			machine.Org.Objects["OwnedPublic__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "OwnedPublic__c", SharingModel: "ReadWrite"},
				Records:    map[storage.ID]storage.Record{"a02000000000001": {ID: "a02000000000001", Object: "OwnedPublic__c"}},
			}
			state := machine.Org.Objects[detail.Object]
			state.Definition.Fields["Other__c"] = storage.Field{APIName: "Other__c", Type: storage.FieldReference, ReferenceTo: []string{"OwnedPublic__c"}}
			machine.Org.Objects[detail.Object] = state
			detail.Fields["Other__c"] = storage.IDValue("a02000000000001")
		}},
		{"missing master record", func(machine *VM, _ *storage.Record) {
			delete(machine.Org.Objects["OwnedMaster__c"].Records, "a00000000000001")
		}},
		{"deleted master record", func(machine *VM, _ *storage.Record) {
			state := machine.Org.Objects["OwnedMaster__c"]
			master := state.Records["a00000000000001"]
			master.System.IsDeleted = true
			state.Records[master.ID] = master
		}},
		{"missing master foreign key", func(_ *VM, detail *storage.Record) {
			delete(detail.Fields, "Master__c")
		}},
		{"multiple masters", func(machine *VM, detail *storage.Record) {
			state := machine.Org.Objects[detail.Object]
			field := state.Definition.Fields["Master__c"]
			field.APIName = "SecondMaster__c"
			state.Definition.Fields[field.APIName] = field
			machine.Org.Objects[detail.Object] = state
			detail.Fields[field.APIName] = storage.IDValue("a00000000000001")
		}},
		{"controlled parent chain", func(machine *VM, _ *storage.Record) {
			state := machine.Org.Objects["OwnedMaster__c"]
			state.Definition.SharingModel = "ControlledByParent"
			state.Definition.Fields["Ancestor__c"] = storage.Field{APIName: "Ancestor__c", Type: storage.FieldReference, MasterDetail: true, ReferenceTo: []string{"OwnedAncestor__c"}}
			master := state.Records["a00000000000001"]
			master.Fields["Ancestor__c"] = storage.IDValue("a02000000000001")
			state.Records[master.ID] = master
			machine.Org.Objects["OwnedMaster__c"] = state
			machine.Org.Objects["OwnedAncestor__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "OwnedAncestor__c", SharingModel: "ReadWrite"},
				Records:    map[storage.ID]storage.Record{"a02000000000001": {ID: "a02000000000001", Object: "OwnedAncestor__c"}},
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			machine, detail := controlledParentReadFixture(t, "ReadWrite")
			tc.mutate(machine, &detail)
			machine.Org.Objects[detail.Object].Records[detail.ID] = detail
			if machine.userModeRecordVisible(detail.Object, detail, machine.currentUserID()) {
				t.Fatal("uncaptured parent shape widened another user's read access")
			}
			if !machine.userModeRecordVisible(detail.Object, detail, string(detail.System.OwnerID)) {
				t.Fatal("existing detail-owner visibility changed")
			}
		})
	}
}

func controlledParentReadFixture(t *testing.T, masterModel string) (*VM, storage.Record) {
	t.Helper()
	org := storage.NewOrgState()
	master := storage.Record{ID: "a00000000000001", Object: "OwnedMaster__c", Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: "005000000000001"}}
	org.Objects[master.Object] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: master.Object, SharingModel: masterModel, Fields: map[string]storage.Field{}},
		Records:    map[storage.ID]storage.Record{master.ID: master},
	}
	detail := storage.Record{ID: "a01000000000001", Object: "OwnedDetail__c", Fields: map[string]storage.Value{"Master__c": storage.IDValue(master.ID)}, System: storage.SystemFields{OwnerID: master.System.OwnerID}}
	org.Objects[detail.Object] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: detail.Object, SharingModel: "ControlledByParent", Fields: map[string]storage.Field{
			"Master__c": {APIName: "Master__c", Type: storage.FieldReference, MasterDetail: true, ReferenceTo: []string{master.Object}},
		}},
		Records: map[storage.ID]storage.Record{detail.ID: detail},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000002", Object: "User"})
	if machine.currentUserBypassesRecordSharing() || machine.currentUserObjectRecordGrant(detail.Object, "PermissionsViewAllRecords", "PermissionsModifyAllRecords") {
		t.Fatal("fixture actor bypasses sharing")
	}
	return machine, detail
}
