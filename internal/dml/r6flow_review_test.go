package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestR6FlowFlatGlobalDecisionRoutes(t *testing.T) {
	org := testOrg()
	storage.EnsureStandardObject(&org, "User")
	storage.EnsureStandardObject(&org, "Profile")
	profileID := storage.ID("00e000000000001")
	profile := org.Objects["Profile"]
	profile.Records[profileID] = storage.Record{ID: profileID, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}}
	org.Objects["Profile"] = profile
	user := org.Objects["User"]
	user.Records[storage.DefaultUserID] = storage.Record{ID: storage.DefaultUserID, Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue(profileID)}}
	org.Objects["User"] = user
	engine := NewEngine(&org)
	for _, tc := range []struct{ name, field, value string }{
		{"profile", "$Profile.Name", "System Administrator"},
		{"user", "$User.Id", "{!$User.Id}"},
		{"permission", "$Permission.R6_Bypass", "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: tc.field, Operation: "equals", Value: tc.value}}}
			matched, supported := engine.evaluateFlowBranch(branch, storage.Record{}, storage.ObjectDefinition{}, true)
			if !supported || !matched {
				t.Fatalf("matched=%v supported=%v", matched, supported)
			}
		})
	}
}

func TestR6FlowBeforeDeleteLoopCannotHideLookupFromCreateGuard(t *testing.T) {
	loop := storage.FlowStep{Kind: "loop", Loop: storage.FlowLoop{Steps: []storage.FlowStep{
		{Kind: "recordLookup"},
		{Kind: "decision", Branches: []storage.FlowBranch{{Steps: []storage.FlowStep{{Kind: "recordCreate"}}}}},
	}}}
	if flowStepsBeforeSaveSafeWithTargets([]storage.FlowStep{loop}, true, true, nil) {
		t.Fatal("loop lookup bypassed nested create guard")
	}
}
