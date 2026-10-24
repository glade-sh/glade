package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The API-independent catalog projects the API 67 native ParentRecords edges
// from standard_platform_child_fields on loaded and cached runtime paths.
func TestStagedParentChildRelationshipLoadedAndCachedVMRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, parent := range []string{"Account", "Case", "Contact", "Lead", "Opportunity", "Product2"} {
			t.Run(api+"/"+parent+"/loaded-child", func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				storage.EnsureStandardObject(&org, parent)
				storage.EnsureStandardObject(&org, "ExtlRecShrStagedRecord")
				machine := New(nil)
				machine.SetOrg(&org)
				for _, describeRoute := range []string{"first-describe", "cached-describe"} {
					t.Run(describeRoute, func(t *testing.T) {
						matches := 0
						for _, value := range machine.describeChildRelationships(parent) {
							if value.Fields["relationshipName"].Text != "ParentRecords" {
								continue
							}
							matches++
							if value.Fields["childSObject"].Fields["object"].Text != "ExtlRecShrStagedRecord" || value.Fields["field"].Fields["field"].Text != "ParentRecordId" {
								t.Fatalf("native describe edge = %+v", value)
							}
						}
						if matches != 1 {
							t.Fatalf("ParentRecords describe edges = %d; want 1", matches)
						}
					})
				}
			})
		}
	}
}
