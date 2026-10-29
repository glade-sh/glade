package dml

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The AR rows in flow_followups/org.tsv capture these formulas under original,
// nested runAs and restored execution users at API 62.0 and 67.0. Replay the same
// observations through both assignment engine entry points.
func TestAssignmentFormulaExecutingUserContext(t *testing.T) {
	const oracle = "../apextest/testdata/conformance/flow_followups/org.tsv"
	raw, err := os.ReadFile(oracle)
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[string]map[string]any)
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		row := strings.SplitN(line, "\t", 4)
		if len(row) != 4 {
			t.Fatalf("invalid native row: %s", line)
		}
		if row[1] != "istest" || !strings.HasPrefix(row[2], "AR_") {
			continue
		}
		var observation map[string]any
		key := row[0] + "/" + row[2]
		if _, exists := want[key]; exists || json.Unmarshal([]byte(row[3]), &observation) != nil {
			t.Fatalf("invalid or duplicate native assignment row: %s", key)
		}
		want[key] = observation
	}
	const secondAdmin storage.ID = "005000000000071"
	const standardUser storage.ID = "005000000000072"
	cases := []struct {
		name, native, company string
		user, owner           storage.ID
	}{
		{"original_matching_owner", "AR_original_matching_owner", "FF_USER", storage.DefaultUserID, storage.DefaultUserID},
		{"original_nonmatching_owner", "AR_original_nonmatching_owner", "FF_USER", storage.DefaultUserID, secondAdmin},
		{"original_profile_admin", "AR_original_profile_admin", "FF_PROFILE", storage.DefaultUserID, storage.DefaultUserID},
		{"runAs_matching_owner", "AR_runAs_matching_owner", "FF_USER", secondAdmin, secondAdmin},
		{"runAs_nonmatching_owner", "AR_runAs_nonmatching_owner", "FF_USER", secondAdmin, storage.DefaultUserID},
		{"runAs_nonmatching_company", "AR_runAs_nonmatching_company", "FF_OTHER", secondAdmin, secondAdmin},
		{"runAs_profile_standard", "AR_runAs_profile_standard", "FF_PROFILE", standardUser, standardUser},
		{"runAs_profile_rejection", "AR_runAs_profile_rejection", "FF_OTHER", standardUser, standardUser},
		{"restored_matching_owner", "AR_restored_matching_owner", "FF_USER", storage.DefaultUserID, storage.DefaultUserID},
		{"restored_profile_admin", "AR_restored_profile_admin", "FF_PROFILE", storage.DefaultUserID, storage.DefaultUserID},
		{"implicit_default_user", "AR_original_matching_owner", "FF_USER", "", storage.DefaultUserID},
		{"implicit_default_profile", "AR_original_profile_admin", "FF_PROFILE", "", storage.DefaultUserID},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"owner_resolver", "apply_assignment"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				storage.EnsureStandardObject(&org, "Lead")
				storage.EnsureStandardObject(&org, "Group")
				storage.EnsureDeterministicPlatformData(&org)
				var standardProfile storage.ID
				for id, profile := range org.Objects["Profile"].Records {
					if profile.Fields["Name"].String == "Standard User" {
						standardProfile = id
					}
				}
				if standardProfile == "" {
					t.Fatal("missing Standard User fixture profile")
				}
				users := org.Objects["User"]
				for _, id := range []storage.ID{secondAdmin, standardUser} {
					user := users.Records[storage.DefaultUserID].Clone()
					user.ID = id
					user.Fields["Id"] = storage.IDValue(id)
					if id == standardUser {
						user.Fields["ProfileId"] = storage.IDValue(standardProfile)
					}
					users.Records[id] = user
				}
				org.Objects["User"] = users
				groups := org.Objects["Group"]
				if groups.Records == nil {
					groups.Records = make(map[storage.ID]storage.Record)
				}
				for i, name := range []string{"FF_UserQueue", "FF_ProfileQueue"} {
					id := []storage.ID{"00G000000000071", "00G000000000072"}[i]
					groups.Records[id] = storage.Record{Object: "Group", ID: id, Fields: map[string]storage.Value{
						"Type": storage.StringValue("Queue"), "DeveloperName": storage.StringValue(name),
					}}
				}
				org.Objects["Group"] = groups
				leads := org.Objects["Lead"]
				leads.Definition.AssignmentRules = []storage.AssignmentRule{{
					ID: "01Q000000000071", Name: "FF_Context", Active: true,
					Entries: []storage.AssignmentRuleEntry{
						{AssignedTo: "FF_UserQueue", AssignedToType: "Queue", Formula: `AND(Company = "FF_USER", $User.Id = OwnerId)`},
						{AssignedTo: "FF_ProfileQueue", AssignedToType: "Queue", Formula: `AND(Company = "FF_PROFILE", $Profile.Name = "Standard User")`},
					},
				}}
				org.Objects["Lead"] = leads
				engine := NewEngine(&org)
				engine.Options.AssignmentRule = &storage.AssignmentRuleSelection{UseDefault: true}
				for i, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						expected, exists := want[api+"/"+tc.native]
						if !exists {
							t.Fatalf("missing native observation %s/%s", api, tc.native)
						}
						engine.UserID = tc.user
						id := storage.ID(fmt.Sprintf("00Q000000000%03d", i+1))
						input := storage.Record{Object: "Lead", ID: id, Fields: map[string]storage.Value{
							"LastName": storage.StringValue("Synthetic"), "Company": storage.StringValue(tc.company),
						}, System: storage.SystemFields{OwnerID: tc.owner}}
						leads := org.Objects["Lead"]
						leads.Records[id] = input
						org.Objects["Lead"] = leads
						var owner storage.ID
						var err error
						if route == "owner_resolver" {
							owner, err = engine.AssignmentRuleOwner(input)
						} else {
							err = engine.ApplyAssignmentRules(input)
							owner = org.Objects["Lead"].Records[id].System.OwnerID
						}
						if err != nil {
							t.Fatalf("native=%v; assignment error=%v", expected, err)
						}
						var queue any
						if group, ok := org.Objects["Group"].Records[owner]; ok {
							queue = group.Fields["DeveloperName"].String
						}
						observed := map[string]any{"status": "SUCCESS", "queue": queue}
						if !reflect.DeepEqual(observed, expected) {
							t.Fatalf("native=%v; Glade=%v", expected, observed)
						}
					})
				}
			})
		}
	}
}
