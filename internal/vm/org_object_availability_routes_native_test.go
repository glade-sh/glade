package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const availabilityRoutesFixture = "../apextest/testdata/conformance/org_object_availability_routes"

func availabilityNativeRows(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open(filepath.Join(availabilityRoutesFixture, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.LazyQuotes = '\t', true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func availabilityRouteOrg(phase string) storage.OrgState {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	features := map[string][]string{"plain": nil, "quotes_on": {"Quotes"}, "order_management": {"OrderManagement"}}
	storage.ApplyOrgShape(&org, features[phase])
	// Exercise the VM policy even if a host hydrates a disabled definition.
	for _, object := range []string{"Quote", "QuoteLineItem", "WebStore", "Account"} {
		storage.EnsureStandardObject(&org, object)
	}
	return org
}

// Invoke public overloads rather than the internal guard. This also covers
// late hydration, where physical presence alone must not grant availability.
func TestOrgObjectAvailabilityPublicNativeRoutes(t *testing.T) {
	rows := availabilityNativeRows(t, "full-org.tsv")
	if len(rows) != 13 {
		t.Fatal("incomplete native route matrix")
	}
	routes := availabilityNativeRows(t, "full-cases.tsv")
	for _, row := range rows[1:] {
		var observed map[string]map[string]any
		if err := json.Unmarshal([]byte(row[3]), &observed); err != nil {
			t.Fatal(err)
		}
		for _, object := range []string{"Quote", "QuoteLineItem", "WebStore", "Account"} {
			for _, route := range routes[1:] {
				t.Run(row[0]+"/"+row[1]+"/"+row[2]+"/"+object+"/"+route[0], func(t *testing.T) {
					want, ok := observed[object][route[0]]
					if !ok {
						t.Fatal("missing native facet")
					}
					source := "String n='" + object + "'; Id missing=null; String q='SELECT Id FROM '+n+' WHERE Id = :missing'; String c='SELECT COUNT() FROM '+n+' WHERE Id = :missing'; Map<String,Object> b=new Map<String,Object>{'missing'=>missing};" + route[1]
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
					if err != nil {
						t.Fatal(err)
					}
					org := availabilityRouteOrg(row[1])
					machine := New(nil)
					machine.SetOrg(&org)
					machine.SetCurrentUser(org.Objects["User"].Records[storage.ID(storage.DefaultUserID)])
					if row[2] == "isTest" {
						machine.EnableTestContext()
					}
					got, err := machine.callMethod(Method{Name: "AvailabilityRoute.read", ClassName: "AvailabilityRoute", IsStatic: true, APIVersion: row[0], SourceContextBound: true, Program: program}, nil, &Result{})
					if err != nil {
						t.Fatal(err)
					}
					actual := any(got.Text)
					if got.Kind == ValueBool {
						actual = got.Bool
					}
					if actual != want {
						t.Errorf("native=%v; Glade=%v", want, actual)
					}
				})
			}
		}
	}
}

// These app/module maps are local aliases, not native Schema APIs. They share
// the global availability projection and must not revive gated definitions.
func TestOrgObjectAvailabilityDescribeAliases(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, phase := range []string{"plain", "quotes_on", "order_management"} {
			for _, named := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/isTest=%t", api, phase, named), func(t *testing.T) {
					org := availabilityRouteOrg(phase)
					machine := New(nil)
					machine.SetOrg(&org)
					if named {
						machine.EnableTestContext()
					}
					for i := 0; i < 2; i++ {
						for _, expression := range []string{"Schema.getGlobalDescribe()", "Schema.getAppDescribe('Sales')", "Schema.getModuleDescribe()", "Schema.getModuleDescribe('Sales')"} {
							for _, object := range []string{"Quote", "QuoteLineItem", "WebStore", "Account"} {
								want := object == "Account" || phase == "quotes_on" && object != "WebStore" || phase == "order_management" && object == "WebStore"
								source := fmt.Sprintf("System.assertEquals(%t,%s.containsKey('%s'));System.assertEquals(%t,%s.get('%s')!=null);System.assertEquals(%t,%s.keySet().contains('%s'));", want, expression, object, want, expression, object, want, expression, strings.ToLower(object))
								program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								_, err = machine.callMethod(Method{Name: "AvailabilityAlias.read", ClassName: "AvailabilityAlias", IsStatic: true, APIVersion: api, SourceContextBound: true, Program: program}, nil, &Result{})
								if err != nil {
									t.Fatalf("%s/%s: %v", expression, object, err)
								}
							}
						}
					}
				})
			}
		}
	}
}

// Hosted rendering is outside this contract. Native query/describe off/on
// rows supply object availability; these tests pin the local record bridges.
func TestOrgObjectAvailabilityRecordHostTwins(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, phase := range []string{"plain", "quotes_on", "order_management"} {
			for _, named := range []bool{false, true} {
				for _, object := range []string{"Quote", "QuoteLineItem", "WebStore", "Account"} {
					t.Run(fmt.Sprintf("%s/%s/isTest=%t/%s", api, phase, named, object), func(t *testing.T) {
						org := availabilityRouteOrg(phase)
						machine := New(nil)
						machine.SetOrg(&org)
						machine.currentMethod = Method{APIVersion: api}
						if named {
							machine.EnableTestContext()
						}
						machine.SetCurrentUser(org.Objects["User"].Records[storage.ID(storage.DefaultUserID)])
						enabled := object == "Account" || phase == "quotes_on" && object != "WebStore" || phase == "order_management" && object == "WebStore"
						id := storage.ID(storage.StandardKeyPrefix(object) + "000000000001")
						check := func(route string, err error) {
							t.Helper()
							if named && route != "standard controller" {
								if err == nil || !strings.Contains(err.Error(), "Visualforce record read does not accept an active test user context") {
									t.Errorf("%s: expected existing host test-context boundary, got %v", route, err)
								}
								return
							}
							if !enabled {
								if err == nil || !strings.Contains(err.Error(), "sObject type '"+object+"' is not supported") {
									t.Errorf("%s: expected feature-off rejection, got %v", route, err)
								}
							} else if err != nil {
								t.Errorf("%s: available object: %v", route, err)
							}
						}
						for i := 0; i < 2; i++ {
							_, err := machine.AuthorizeVisualforceRecordFields(object, []string{"Id"})
							check("record authorization", err)
							_, err = machine.ReadVisualforceRecords(object)
							check("record set", err)
							_, found, err := machine.ReadVisualforceRecordFields(object, id, []string{"Id"})
							check("record fields", err)
							if found {
								t.Error("unexpected row in empty synthetic object")
							}
							partial := Object(object)
							partial.Fields["Id"] = String(string(id))
							_, err = machine.standardControllerFieldProjection(partial, List(String("Id")))
							if enabled && err != nil && strings.Contains(err.Error(), "List has no rows for assignment to SObject") {
								err = nil // existing empty-row local bridge contract
							}
							check("standard controller", err)
						}
					})
				}
			}
		}
	}
}

// The native static rows reject unavailable object types during compilation.
// After host hydration the VM can execute already compiled code; it must still
// enforce the same availability boundary instead of exposing the definition.
func TestOrgObjectAvailabilityStaticHydratedRoutes(t *testing.T) {
	rows := availabilityNativeRows(t, "full-org.tsv")
	for _, row := range rows[1:] {
		var observed map[string]map[string]any
		if err := json.Unmarshal([]byte(row[3]), &observed); err != nil {
			t.Fatal(err)
		}
		for _, object := range []string{"Quote", "QuoteLineItem", "WebStore", "Account"} {
			t.Run(row[0]+"/"+row[1]+"/"+row[2]+"/"+object, func(t *testing.T) {
				source := "try { List<SObject> rows=[SELECT Id FROM " + object + "]; return true; } catch (Exception e) { return e.getTypeName(); }"
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				org := availabilityRouteOrg(row[1])
				machine := New(nil)
				machine.SetOrg(&org)
				if row[2] == "isTest" {
					machine.EnableTestContext()
				}
				got, err := machine.callMethod(Method{Name: "AvailabilityStatic.read", ClassName: "AvailabilityStatic", IsStatic: true, APIVersion: row[0], SourceContextBound: true, Program: program}, nil, &Result{})
				if err != nil {
					t.Fatal(err)
				}
				actual := any(got.Text)
				if got.Kind == ValueBool {
					actual = got.Bool
				}
				if want := observed[object]["qd"]; actual != want {
					t.Errorf("hydrated static route: native availability=%v; Glade=%v", want, actual)
				}
			})
		}
	}
}

// Both shapes matter: project setup removes disabled definitions, whereas a
// record host can hydrate them later. The relationship guard must cover both.
func TestOrgObjectAvailabilityChildrenNativeDefinitionRoutes(t *testing.T) {
	rows := availabilityNativeRows(t, "../org_object_availability_children/org.tsv")
	queries := map[string]string{
		"quotes":      "SELECT Id,(SELECT Id FROM Quotes) FROM Opportunity",
		"quotesLower": "SELECT Id,(SELECT Id FROM quotes) FROM opportunity",
		"lines":       "SELECT Id,(SELECT Id FROM QuoteLineItems) FROM Quote",
		"linesLower":  "SELECT Id,(SELECT Id FROM quotelineitems) FROM quote",
		"boundary":    "SELECT Id,(SELECT Id FROM Contacts) FROM Account",
	}
	for _, row := range rows[1:] {
		var observed map[string]any
		if err := json.Unmarshal([]byte(row[3]), &observed); err != nil {
			t.Fatal(err)
		}
		for name, query := range queries {
			for _, hydrated := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/%s/%s/hydrated=%t", row[0], row[1], row[2], name, hydrated), func(t *testing.T) {
					org := availabilityRouteOrg(row[1])
					if !hydrated {
						for _, object := range []string{"Quote", "QuoteLineItem", "WebStore"} {
							if !org.AllowsStandardObject(object) {
								storage.DeleteObject(&org, object)
							}
						}
					}
					machine := New(nil)
					machine.SetOrg(&org)
					if row[2] == "isTest" {
						machine.EnableTestContext()
					}
					source := "try { Database.query('" + query + "'); return true; } catch (Exception e) { return e.getTypeName(); }"
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
					if err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 2; i++ {
						got, err := machine.callMethod(Method{Name: "AvailabilityChildren.read", ClassName: "AvailabilityChildren", IsStatic: true, APIVersion: row[0], SourceContextBound: true, Program: program}, nil, &Result{})
						if err != nil {
							t.Fatal(err)
						}
						actual := any(got.Text)
						if got.Kind == ValueBool {
							actual = got.Bool
						}
						if want := observed[name]; actual != want {
							t.Errorf("native=%v; Glade=%v", want, actual)
						}
					}
				})
			}
		}
	}
}
