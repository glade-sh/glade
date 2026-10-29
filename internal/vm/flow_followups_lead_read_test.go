package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// flow_followups/org.tsv:63,125 observe one fact: a Standard User who created
// a Lead still reads it after assignment moves it to an empty queue, at both
// source APIs. No row has a reader who neither owns nor created the Lead, so
// the read grant is scoped to that creator and queue state. Every other
// omitted-model Lead read behaves as it did before the follow-up change.
func TestFlowFollowupsLeadReadSharing(t *testing.T) {
	const (
		actor      storage.ID = "005000000000002"
		other      storage.ID = "005000000000001"
		emptyQueue storage.ID = "00G000000000001"
		fullQueue  storage.ID = "00G000000000002"
		regular    storage.ID = "00G000000000003"
	)
	for _, api := range []string{"62.0", "67.0"} {
		for _, userType := range []string{"Standard", "Guest", "CspLitePortal", ""} {
			for _, model := range []string{"", "Private", "Read"} {
				for _, owner := range []storage.ID{actor, other, emptyQueue, fullQueue, regular} {
					for _, creator := range []storage.ID{actor, other} {
						name := api + "/" + userType + "/" + model + "/" + string(owner) + "/by-" + string(creator)
						t.Run(name, func(t *testing.T) {
							org := storage.NewOrgState()
							org.APIVersion = api
							storage.EnsureStandardObject(&org, "Lead")
							storage.EnsureStandardObject(&org, "Group")
							storage.EnsureStandardObject(&org, "GroupMember")
							state := org.Objects["Lead"]
							state.Definition.SharingModel = model
							org.Objects["Lead"] = state
							groups := org.Objects["Group"]
							groups.Records = map[storage.ID]storage.Record{}
							for id, groupType := range map[storage.ID]string{emptyQueue: "Queue", fullQueue: "Queue", regular: "Regular"} {
								groups.Records[id] = storage.Record{ID: id, Object: "Group", Fields: map[string]storage.Value{"Type": storage.StringValue(groupType)}}
							}
							org.Objects["Group"] = groups
							members := org.Objects["GroupMember"]
							members.Records = map[storage.ID]storage.Record{"011000000000001": {ID: "011000000000001", Object: "GroupMember", Fields: map[string]storage.Value{
								"GroupId": storage.StringValue(string(fullQueue)), "UserOrGroupId": storage.StringValue(string(other)),
							}}}
							org.Objects["GroupMember"] = members
							machine := New(nil)
							machine.SetOrg(&org)
							machine.SetCurrentUser(storage.Record{ID: actor, Object: "User", Fields: map[string]storage.Value{
								"UserType": storage.StringValue(userType),
							}})
							if machine.currentUserBypassesRecordSharing() || machine.currentUserObjectRecordGrant("Lead", "PermissionsViewAllRecords", "PermissionsModifyAllRecords") {
								t.Fatal("fixture actor bypasses record sharing")
							}
							lead := storage.Record{ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{
								OwnerID: owner, CreatedByID: creator,
							}}
							// Read is an explicit public grant; no other model or
							// user type widens the shared-object path.
							if got, want := machine.soqlObjectHasPublicReadSharing("Lead"), model == "Read"; got != want {
								t.Errorf("object public read=%t, want %t", got, want)
							}
							// Base behavior: public Read, or the owner. The one added
							// grant is the Standard creator reading from an empty queue.
							want := model == "Read" || owner == actor ||
								(model == "" && userType == "Standard" && creator == actor && owner == emptyQueue)
							if got := machine.userModeRecordVisible("Lead", lead, string(actor)); got != want {
								t.Errorf("record visible=%t, want %t", got, want)
							}
							if model != "Read" {
								query := soql.Query{Object: "Lead", SecurityMode: "USER_MODE"}
								if got := machine.soqlSharingUserID(query); got != string(actor) {
									t.Errorf("sharing user %q, want the reading user", got)
								}
							}
							if owner != actor {
								for _, operation := range []string{"update", "upsert", "delete", "undelete"} {
									if machine.currentUserCanWriteRecord("Lead", lead, string(actor), operation) {
										t.Errorf("read grant allowed another owner's %s", operation)
									}
								}
							}
						})
					}
				}
			}
		}
	}
}
