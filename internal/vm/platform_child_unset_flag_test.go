package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native: standard_platform_child_fields/org.tsv:27,30,36 (API 67).
func TestPlatformChildUnsetFlagFirstAndCachedVMDescribeRoutes(t *testing.T) {
	for _, tc := range []struct{ object, field, flag string }{
		{"Order", "OpportunityId", "accessible"},
		{"FinanceBalanceSnapshot", "AccountId", "createable"},
		{"ExtlRecShrStagedRecord", "ParentRecordId", "nillable"},
	} {
		t.Run(tc.object+"/"+tc.flag, func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = "67.0"
			storage.EnsureStandardObject(&org, tc.object)
			state := org.Objects[tc.object]
			state.Definition = state.Definition.Clone()
			f := state.Definition.Fields[tc.field]
			switch tc.flag {
			case "accessible":
				f.Accessible = nil
			case "createable":
				f.Createable = nil
			default:
				f.Nillable = nil
			}
			state.Definition.Fields[tc.field] = f
			org.Objects[tc.object] = state
			machine := New(nil)
			machine.SetOrg(&org)
			for _, route := range []string{"first-describe", "cached-describe"} {
				t.Run(route, func(t *testing.T) {
					d, err := machine.describeFieldValue(tc.object, tc.field)
					if err != nil {
						t.Fatal(err)
					}
					if d.Fields[tc.flag].Bool {
						t.Errorf("%s.%s %s=true; native row requires false", tc.object, tc.field, tc.flag)
					}
				})
			}
		})
	}
}
