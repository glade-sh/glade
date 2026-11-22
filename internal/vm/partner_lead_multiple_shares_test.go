package vm

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func partnerLeadMultipleShares(t testing.TB) *VM {
	t.Helper()
	machine, _ := partnerLeadChildVisibilityMachine(t)
	machine.currentMethod = Method{APIVersion: "67.0"}
	leads := machine.Org.Objects["Lead"]
	for id, row := range leads.Records {
		row.System.CreatedByID = row.System.OwnerID
		leads.PutRecord(id, row)
	}
	machine.Org.Objects["Lead"] = leads
	shares := machine.Org.Objects["LeadShare"]
	for i := 1; i <= 2; i++ {
		key := storage.ID(fmt.Sprintf("00T%012d", 500+i))
		shares.PutRecord(key, storage.Record{Object: "LeadShare", Fields: map[string]storage.Value{
			"RowCause": storage.StringValue("Manual"), "LeadAccessLevel": storage.StringValue("Read"),
			"LeadId":        storage.IDValue(storage.ID(fmt.Sprintf("00Q%012d", i))),
			"UserOrGroupId": storage.IDValue("005000000000002"),
		}})
	}
	machine.Org.Objects["LeadShare"] = shares
	return machine
}

func TestPartnerLeadMultipleSharesVisibleIDs(t *testing.T) {
	machine := partnerLeadMultipleShares(t)
	user := machine.currentUserID()
	for _, scoped := range []bool{false, true} {
		t.Run(fmt.Sprintf("scoped=%t", scoped), func(t *testing.T) {
			if scoped {
				defer machine.beginPartnerLeadShareScan()()
			}
			var ids []string
			for _, row := range machine.Org.Objects["Lead"].Records {
				if machine.userModeRecordVisible("Lead", row, user) {
					ids = append(ids, string(row.ID))
				}
			}
			slices.Sort(ids)
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(ids, ","))))
			if len(ids) != 200 || digest != "2f3002fc2ca62089b999e5711eb494ab1b321aad2967800716bf6cf614b29acc" {
				t.Fatalf("visible Lead IDs: count %d digest %s", len(ids), digest)
			}
		})
	}
}

func TestPartnerLeadMultipleSharesSkipPolicyWork(t *testing.T) {
	machine := partnerLeadMultipleShares(t)
	for _, scoped := range []bool{false, true} {
		t.Run(fmt.Sprintf("scoped=%t", scoped), func(t *testing.T) {
			if scoped {
				defer machine.beginPartnerLeadShareScan()()
			}
			allocs := testing.AllocsPerRun(20, func() {
				if machine.partnerLeadPrivateExternalDefault("Lead") {
					t.Fatal("multiple shares matched the single-share policy")
				}
			})
			if allocs != 0 {
				t.Fatalf("warm multiple-share exclusion allocated %.0f times per Lead, want zero", allocs)
			}
		})
	}
}

func TestPartnerLeadMultipleSharesTrackCurrentRows(t *testing.T) {
	machine := partnerLeadMultipleShares(t)
	org := machine.Org
	shares := org.Objects["LeadShare"]
	const key = "00T000000000502"
	second := shares.Records[key]
	check := func(stage string, want bool) {
		t.Helper()
		if got := machine.partnerLeadPrivateExternalDefault("Lead"); got != want {
			t.Fatalf("%s: private default = %v, want %v", stage, got, want)
		}
	}
	check("two map keys with empty record IDs", false)
	alias := storage.ObjectState{}
	alias.SetRecords(shares.Records)
	row := alias.Records[key]
	row.Fields["RowCause"] = storage.StringValue("Owner")
	alias.PutRecord(key, row)
	check("aliased owner cause", true)
	row.Fields["RowCause"] = storage.StringValue("Manual")
	check("aliased manual cause", false)
	second.System.IsDeleted = true
	shares.PutRecord(key, second)
	check("soft delete", true)
	second.System.IsDeleted = false
	shares.PutRecord(key, second)
	check("undelete", false)
	shares.DeleteRecord(key)
	check("hard delete", true)
	shares.PutRecord(key, second)
	check("insert", false)
	replacement := storage.ObjectState{Definition: shares.Definition}
	replacement.SetRecords(nil)
	org.Objects["LeadShare"] = replacement
	check("replace share map", true)
	org.Objects["LeadShare"] = shares
	check("restore share map", false)
}

func TestPartnerLeadMultipleSharesKeepLiveAccess(t *testing.T) {
	machine := partnerLeadMultipleShares(t)
	org := machine.Org
	leads := org.Objects["Lead"]
	const id = "00Q000000000003"
	row := leads.Records[id]
	check := func(stage string, want bool) {
		t.Helper()
		records := org.Objects["Lead"].Records
		current, exists := records[id]
		got := exists && machine.userModeRecordVisible("Lead", current, machine.currentUserID())
		if got != want {
			t.Fatalf("%s: visible = %v, want %v", stage, got, want)
		}
	}
	check("public model", true)
	leads.Definition.SharingModel = "Private"
	org.Objects["Lead"] = leads
	check("private model", false)
	row.System.OwnerID = storage.ID(machine.currentUserID())
	leads.PutRecord(id, row)
	check("current owner", true)
	leads.DeleteRecord(id)
	check("deleted Lead", false)
	row.System.OwnerID = "00G000000000001"
	leads.PutRecord(id, row)
	check("insert with queue owner", false)

	for _, name := range []string{"Group", "GroupMember"} {
		storage.EnsureStandardObject(org, name)
	}
	groups := org.Objects["Group"]
	groups.PutRecord(row.System.OwnerID, storage.Record{ID: row.System.OwnerID, Fields: map[string]storage.Value{"Type": storage.StringValue("Queue")}})
	org.Objects["Group"] = groups
	members := org.Objects["GroupMember"]
	const membership = "011000000000001"
	members.PutRecord(membership, storage.Record{ID: membership, Fields: map[string]storage.Value{"GroupId": storage.IDValue(row.System.OwnerID), "UserOrGroupId": storage.IDValue("005000000000003")}})
	org.Objects["GroupMember"] = members
	check("other queue member", false)
	prior := machine.testContext.CurrentUser
	machine.testContext.CurrentUser = Object("User")
	machine.testContext.CurrentUser.Fields["Id"] = String("005000000000003")
	machine.testContext.CurrentUser.Fields["ProfileId"] = String("00e000000000003")
	machine.testContext.CurrentUser.Fields["UserType"] = String("Standard")
	machine.testContext.RunAsDepth++
	check("nested runAs queue member", true)
	machine.testContext.CurrentUser.Fields["UserRoleId"] = String("00E000000000001")
	check("member with role", true)
	members.DeleteRecord(membership)
	check("removed member with role", false)
	machine.testContext.CurrentUser = prior
	machine.testContext.RunAsDepth--
	check("restored partner", false)
}
