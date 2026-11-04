package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// Native rows: ../apextest/testdata/conformance/org_account_contact_availability/org.tsv.
// Keep catalog hydration from reviving a feature-off object after shape load.
func TestOrgAccountContactAvailabilityNativeVMRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ object, feature string }{
			{"AccountContactRelation", "ContactsToMultipleAccounts"},
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
func TestOrgAccountContactAvailabilityNativeRecordHosts(t *testing.T) {
	for _, object := range []string{"AccountContactRelation"} {
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
			_, err := machine.AuthorizeVisualforceRecordFields(object, []string{"Id"})
			assertUnavailable("record fields", err)
			_, err = machine.ReadVisualforceRecords(object)
			assertUnavailable("record set", err)
			record := Object(object)
			record.Fields["Id"] = String(storage.StandardKeyPrefix(object) + "000000000001")
			_, err = machine.standardControllerFieldProjection(record, List(String("Id")))
			assertUnavailable("standard controller", err)
		})
	}
}

// This exercises the local static-query path after late hydration. The native
// dynamic-query observations in org_account_contact_availability/org.tsv
// establish object availability, not a native static-compilation claim.
func TestOrgAccountContactAvailabilityStaticHydrated(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, enabled := range []bool{false, true} {
			for _, named := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/enabled=%t/isTest=%t", api, enabled, named), func(t *testing.T) {
					org := storage.NewOrgState()
					if enabled {
						storage.ApplyOrgShape(&org, []string{"ContactsToMultipleAccounts"})
					} else {
						storage.ApplyOrgShape(&org, nil)
					}
					storage.EnsureStandardObject(&org, "AccountContactRelation")
					machine := New(nil)
					machine.SetOrg(&org)
					if named {
						machine.EnableTestContext()
					}
					source := "try { List<SObject> rows=[SELECT Id FROM AccountContactRelation]; return true; } catch (Exception e) { return e.getTypeName(); }"
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					got, err := machine.callMethod(Method{Name: "AvailabilityStatic.read", ClassName: "AvailabilityStatic", IsStatic: true, APIVersion: api, SourceContextBound: true, Program: program}, nil, &Result{})
					if err != nil {
						t.Fatal(err)
					}
					if enabled && (got.Kind != ValueBool || !got.Bool) {
						t.Fatalf("enabled got %v", got)
					}
					if !enabled && got.Text != "System.QueryException" {
						t.Fatalf("disabled got %v", got)
					}
				})
			}
		}
	}
}
