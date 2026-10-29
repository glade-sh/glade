package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// Native rows: ../apextest/testdata/conformance/org_object_availability/org.tsv.
// Keep catalog hydration from reviving a feature-off object after shape load.
func TestOrgObjectAvailabilityNativeVMRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ object, feature string }{
			{"Quote", "Quotes"}, {"QuoteLineItem", "Quotes"}, {"WebStore", "OrderManagement"},
		} {
			for _, enabled := range []bool{false, true} {
				t.Run(api+"/"+tc.object+"/"+map[bool]string{false: "off", true: "on"}[enabled], func(t *testing.T) {
					org := storage.NewOrgState()
					if enabled {
						storage.ApplyOrgShape(&org, []string{tc.feature})
					} else {
						storage.ApplyOrgShape(&org, nil)
					}
					// A host can hydrate a physical definition after loading the shape.
					storage.EnsureStandardObject(&org, tc.object)
					machine := New(nil)
					machine.SetOrg(&org)
					machine.currentMethod = Method{APIVersion: api}
					for i := 0; i < 2; i++ {
						_, described := machine.schemaGlobalDescribe().Map[mapKey(String(strings.ToLower(tc.object)))]
						if described != enabled {
							t.Errorf("describe=%t, want %t", described, enabled)
						}
					}
					if got := machine.typeForName("", "Schema."+tc.object, false).Kind != ValueNull; got != enabled {
						t.Errorf("reflection=%t, want %t", got, enabled)
					}
					if err := machine.enforceOrgShapeObjectAvailability(soql.Query{Object: tc.object}); (err == nil) != enabled {
						t.Errorf("query error=%v, enabled=%t", err, enabled)
					}
				})
			}
		}
	}
}

// VF/LWC record hosts call the same availability guard after security checks.
// These are local bridge contracts over the captured object-availability rows,
// not additional claims about Salesforce's hosted renderer.
func TestOrgObjectAvailabilityNativeRecordHosts(t *testing.T) {
	for _, object := range []string{"Quote", "WebStore"} {
		t.Run(object, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			storage.ApplyOrgShape(&org, nil)
			storage.EnsureStandardObject(&org, object)
			machine := New(nil)
			machine.SetOrg(&org)
			machine.currentMethod = Method{APIVersion: "67.0"}
			machine.SetCurrentUser(org.Objects["User"].Records[storage.ID(storage.DefaultUserID)])
			assertUnavailable := func(route string, err error) {
				t.Helper()
				if err == nil || !strings.Contains(err.Error(), "sObject type '"+object+"' is not supported") {
					t.Errorf("%s: expected feature-off query rejection, got %v", route, err)
				}
			}
			_, err := machine.AuthorizeVisualforceRecordFields(object, []string{"Name"})
			assertUnavailable("record fields", err)
			_, err = machine.ReadVisualforceRecords(object)
			assertUnavailable("record set", err)
			record := Object(object)
			record.Fields["Id"] = String(storage.StandardKeyPrefix(object) + "000000000001")
			_, err = machine.standardControllerFieldProjection(record, List(String("Name")))
			assertUnavailable("standard controller", err)
		})
	}
}
