package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Host route twins of upsert_duplicate_native/api{62,67}/istest/org.tsv:2-5.
// The controller uses the same synthetic matching and duplicate rules.
const upsertHostController = `public class AjaxController {
  private static String run(String email) {
    Contact row = new Contact(LastName='Block', Email=email, RouteKey__c=email);
    Database.UpsertResult saved = Database.upsert(row, Contact.RouteKey__c, false);
    String status = saved.isSuccess() ? 'ok' : String.valueOf(saved.getErrors()[0].getStatusCode());
    return status;
  }
  private static String runList(String email) {
    List<Contact> rows = new List<Contact>{new Contact(LastName='Block', Email=email, RouteKey__c=email)};
    List<Database.UpsertResult> saved = Database.upsert(rows, Contact.RouteKey__c, false);
    return saved[0].isSuccess() ? 'ok' : String.valueOf(saved[0].getErrors()[0].getStatusCode());
  }
  private static String runUpdate(String email) {
    insert new Contact(LastName='Block', Email='original@example.invalid', RouteKey__c='target');
    Contact row = new Contact(LastName='Block', Email=email, RouteKey__c='target');
    Database.UpsertResult saved = Database.upsert(row, Contact.RouteKey__c, false);
    return saved.isSuccess() ? 'ok' : String.valueOf(saved.getErrors()[0].getStatusCode());
  }
  private static String runUnchanged() {
    insert new Contact(LastName='Safe', Email='duplicate@example.invalid', RouteKey__c='target');
    Contact row = new Contact(LastName='Block', Email='duplicate@example.invalid', RouteKey__c='target');
    Database.UpsertResult saved = Database.upsert(row, Contact.RouteKey__c, false);
    return saved.isSuccess() ? 'ok' : String.valueOf(saved.getErrors()[0].getStatusCode());
  }
  private static String runReport() {
    insert new Contact(LastName='Safe', Email='report@example.invalid', RouteKey__c='report-base');
    Contact row = new Contact(LastName='Report', Email='report@example.invalid', RouteKey__c='report-new');
    Database.UpsertResult saved = Database.upsert(row, Contact.RouteKey__c, false);
    return saved.isSuccess() ? 'ok' : String.valueOf(saved.getErrors()[0].getStatusCode());
  }
  private static String runBlankPrior(Boolean explicitNull) {
    Contact target = new Contact(LastName='Safe', RouteKey__c='blank-target');
    if (explicitNull) target.Email = null;
    insert target;
    Contact row = new Contact(LastName='Block', Email='duplicate@example.invalid', RouteKey__c='blank-target');
    Database.UpsertResult saved = Database.upsert(row, Contact.RouteKey__c, false);
    return saved.isSuccess() ? 'ok' : String.valueOf(saved.getErrors()[0].getStatusCode());
  }
  @AuraEnabled public static String lightningBlankPrior(Boolean explicitNull) { return runBlankPrior(explicitNull); }
  @RemoteAction public static String remoteBlankPrior(Boolean explicitNull) { return runBlankPrior(explicitNull); }
  private static String runUncapturedScope() {
    Contact row = new Contact(LastName='Scope', Email='duplicate@example.invalid', Phone='5551234567', RouteKey__c='scope-new');
    Database.UpsertResult saved = Database.upsert(row, Contact.RouteKey__c, false);
    return saved.isSuccess() ? 'ok' : String.valueOf(saved.getErrors()[0].getStatusCode());
  }
  @AuraEnabled public static String lightningUncapturedScope() { return runUncapturedScope(); }
  @RemoteAction public static String remoteUncapturedScope() { return runUncapturedScope(); }
  private static String runFailureState() {
    Contact target = new Contact(LastName='Block', Email='original@example.invalid', RouteKey__c='target');
    insert target;
    Id targetId = target.Id;
    Contact row = new Contact(LastName='Block', Email='duplicate@example.invalid', RouteKey__c='target');
    Database.UpsertResult saved = Database.upsert(row, Contact.RouteKey__c, false);
    Contact stored = [SELECT Email FROM Contact WHERE Id=:targetId];
    return String.valueOf(saved.getErrors()[0].getStatusCode())+'|'+(row.Id==null)+'|'+stored.Email;
  }
  @AuraEnabled public static String lightningFailureState() { return runFailureState(); }
  @RemoteAction public static String remoteFailureState() { return runFailureState(); }
  private static String runAlert() {
    insert new Contact(LastName='Safe', Email='alert@example.invalid', RouteKey__c='alert-base');
    Contact row = new Contact(LastName='Alert', Email='alert@example.invalid', RouteKey__c='alert-new');
    Database.UpsertResult saved = Database.upsert(row, Contact.RouteKey__c, false);
    return saved.isSuccess() ? 'ok' : String.valueOf(saved.getErrors()[0].getStatusCode());
  }
  @AuraEnabled public static String lightningAlert() { return runAlert(); }
  @RemoteAction public static String remoteAlert() { return runAlert(); }
  @AuraEnabled public static String lightningReport() { return runReport(); }
  @RemoteAction public static String remoteReport() { return runReport(); }
  @AuraEnabled public static String lightningUnchanged() { return runUnchanged(); }
  @RemoteAction public static String remoteUnchanged() { return runUnchanged(); }
  @AuraEnabled public static String lightningUpdate(String email) { return runUpdate(email); }
  @RemoteAction public static String remoteUpdate(String email) { return runUpdate(email); }
  @AuraEnabled public static String lightning(String email) { return run(email); }
  @RemoteAction public static String remote(String email) { return run(email); }
  @AuraEnabled public static String lightningList(String email) { return runList(email); }
  @RemoteAction public static String remoteList(String email) { return runList(email); }
}`

func upsertHostServer(t *testing.T, api string) *Server {
	t.Helper()
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	base := filepath.Join(root, "force-app/main/default")
	writeServerTestFile(t, filepath.Join(base, "classes/AjaxController.cls"), upsertHostController)
	writeServerTestFile(t, filepath.Join(base, "classes/AjaxController.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	writeServerTestFile(t, filepath.Join(base, "pages/Remote.page"), `<apex:page controller="AjaxController"></apex:page>`)
	for name, dst := range map[string]string{
		"Contact.matchingRule-meta.xml":                  "matchingRules/Contact.matchingRule-meta.xml",
		"Contact.Synthetic_Alert.duplicateRule-meta.xml": "duplicateRules/Contact.Synthetic_Alert.duplicateRule-meta.xml",
		"Contact.Synthetic_Block.duplicateRule-meta.xml": "duplicateRules/Contact.Synthetic_Block.duplicateRule-meta.xml",
		"RouteKey__c.field-meta.xml":                     "objects/Contact/fields/RouteKey__c.field-meta.xml",
	} {
		data, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/upsert_duplicate_native/metadata", name))
		if err != nil {
			t.Fatal(err)
		}
		writeServerTestFile(t, filepath.Join(base, dst), string(data))
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	policies, err := automation.LoadProject(p)
	if err != nil || len(policies.Diagnostics) != 0 {
		t.Fatalf("policies: %v %v", err, policies.Diagnostics)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	contact := org.Objects["Contact"]
	contact.Definition.Fields["RouteKey__c"] = storage.Field{APIName: "RouteKey__c", Type: storage.FieldString, ExternalID: true}
	org.Objects["Contact"] = contact
	automation.ApplyToOrg(&org, policies)
	engine := dml.NewEngine(&org)
	inserted := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Block"), "Email": storage.StringValue("duplicate@example.invalid"), "RouteKey__c": storage.StringValue("seed")}}})
	if len(inserted) != 1 || !inserted[0].Success {
		t.Fatalf("seed: %+v", inserted)
	}
	srv := newLightningTestServer(t, &org, source)
	configureVisualforceTestPrincipal(t, srv)
	srv.SetProjectIndex(typesys.Build(p, schema))
	if srv.runtimeErr != nil {
		t.Fatalf("compile: %v", srv.runtimeErr)
	}
	return srv
}

func upsertHostNativeExpected(t *testing.T, api, label, shape string) string {
	t.Helper()
	path := filepath.Join("../apextest/testdata/conformance/upsert_duplicate_native", "api"+strings.TrimSuffix(api, ".0"), "istest", "org.tsv")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	key := shape + "_database_partial_false"
	if label == "duplicate" {
		key = shape + "_database_partial_true"
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		row := strings.Split(line, "\t")
		if len(row) == 4 && row[0] == key {
			return row[1]
		}
	}
	t.Fatalf("missing native row %s in %s", key, path)
	return ""
}

func TestUpsertDuplicateLightningControllerNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ label, email string }{{"distinct", "distinct@example.invalid"}, {"duplicate", "duplicate@example.invalid"}} {
			t.Run(api+"/"+tc.label, func(t *testing.T) {
				srv := upsertHostServer(t, api)
				want := upsertHostNativeExpected(t, api, tc.label, "custom_insert")
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/lightning", strings.NewReader(`{"email":"`+tc.email+`"}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
				}
				var out lwcbrowser.WireResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if out.Error != nil || out.Data != want {
					t.Fatalf("response: %+v; want %s", out, want)
				}
			})
		}
	}
}

func TestUpsertDuplicateVisualforceControllerNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ label, email string }{{"distinct", "distinct@example.invalid"}, {"duplicate", "duplicate@example.invalid"}} {
			t.Run(api+"/"+tc.label, func(t *testing.T) {
				srv := upsertHostServer(t, api)
				want := upsertHostNativeExpected(t, api, tc.label, "custom_insert")
				body := `[{"action":"AjaxController","method":"remote","data":["` + tc.email + `"],"type":"rpc","tid":1}]`
				result := postVisualforceRemotingWithViewState(t, srv, body)
				if len(result) != 1 || !result[0].Status || result[0].Result != want {
					t.Fatalf("response: %+v; want %s", result, want)
				}
			})
		}
	}
}

// These list-route controls preserve shipping behavior and make no native claim.
func TestUpsertDuplicateListLightningPreservesShipping(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			srv := upsertHostServer(t, api)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/lightningList", strings.NewReader(`{"email":"duplicate@example.invalid"}`))
			req.Header.Set("Content-Type", "application/json")
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			var out lwcbrowser.WireResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if out.Error != nil || out.Data != "ok" {
				t.Fatalf("response: %+v", out)
			}
		})
	}
}

func TestUpsertDuplicateListVisualforcePreservesShipping(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			srv := upsertHostServer(t, api)
			body := `[{"action":"AjaxController","method":"remoteList","data":["duplicate@example.invalid"],"type":"rpc","tid":1}]`
			result := postVisualforceRemotingWithViewState(t, srv, body)
			if len(result) != 1 || !result[0].Status || result[0].Result != "ok" {
				t.Fatalf("response: %+v", result)
			}
		})
	}
}

func TestUpsertDuplicateUpdateLightningNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ label, email string }{{"distinct", "distinct@example.invalid"}, {"duplicate", "duplicate@example.invalid"}} {
			t.Run(api+"/"+tc.label, func(t *testing.T) {
				srv := upsertHostServer(t, api)
				want := upsertHostNativeExpected(t, api, tc.label, "custom_update")
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/lightningUpdate", strings.NewReader(`{"email":"`+tc.email+`"}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
				}
				var out lwcbrowser.WireResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if out.Error != nil || out.Data != want {
					t.Fatalf("response: %+v; want %s", out, want)
				}
			})
		}
	}
}

func TestUpsertDuplicateUpdateVisualforceNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ label, email string }{{"distinct", "distinct@example.invalid"}, {"duplicate", "duplicate@example.invalid"}} {
			t.Run(api+"/"+tc.label, func(t *testing.T) {
				srv := upsertHostServer(t, api)
				want := upsertHostNativeExpected(t, api, tc.label, "custom_update")
				body := `[{"action":"AjaxController","method":"remoteUpdate","data":["` + tc.email + `"],"type":"rpc","tid":1}]`
				result := postVisualforceRemotingWithViewState(t, srv, body)
				if len(result) != 1 || !result[0].Status || result[0].Result != want {
					t.Fatalf("response: %+v; want %s", result, want)
				}
			})
		}
	}
}

// Native adjacent/org.tsv:6: an unchanged matching email is accepted.
func TestUpsertDuplicateUnchangedLightningAndVisualforce(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/lightning", func(t *testing.T) {
			srv := upsertHostServer(t, api)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/lightningUnchanged", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			srv.ServeHTTP(rec, req)
			var out lwcbrowser.WireResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || out.Error != nil || out.Data != "ok" {
				t.Fatalf("status %d response %+v", rec.Code, out)
			}
		})
		t.Run(api+"/visualforce", func(t *testing.T) {
			srv := upsertHostServer(t, api)
			body := `[{"action":"AjaxController","method":"remoteUnchanged","data":[],"type":"rpc","tid":1}]`
			result := postVisualforceRemotingWithViewState(t, srv, body)
			if len(result) != 1 || !result[0].Status || result[0].Result != "ok" {
				t.Fatalf("response %+v", result)
			}
		})
	}
}

// Host commit behavior is uncaptured; preserve the existing host transaction behavior.
func TestUpsertDuplicateReportLightningAndVisualforce(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"lightning", "visualforce"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				srv := upsertHostServer(t, api)
				contact := srv.Org.Objects["Contact"]
				found := false
				for _, rule := range contact.Definition.DuplicateRules {
					if strings.EqualFold(rule.Name, "Synthetic_Block") {
						rule.Name = "Synthetic_Report"
						rule.ActionOnInsert = "Allow"
						rule.OperationsOnInsert = []string{"Report"}
						rule.Filter.Items = append([]storage.DuplicateRuleFilterItem(nil), rule.Filter.Items...)
						rule.Filter.Items[0].Value = "Report"
						rule.SortOrder = 5
						contact.Definition.DuplicateRules = append(contact.Definition.DuplicateRules, rule)
						found = true
						break
					}
				}
				if !found {
					t.Fatal("missing synthetic blocking rule")
				}
				srv.Org.Objects["Contact"] = contact
				if route == "lightning" {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/lightningReport", strings.NewReader(`{}`))
					req.Header.Set("Content-Type", "application/json")
					srv.ServeHTTP(rec, req)
					var out lwcbrowser.WireResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
						t.Fatal(err)
					}
					if rec.Code != http.StatusOK || out.Error != nil || out.Data != "ok" {
						t.Fatalf("status %d response %+v", rec.Code, out)
					}
				} else {
					result := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"remoteReport","data":[],"type":"rpc","tid":1}]`)
					if len(result) != 1 || !result[0].Status || result[0].Result != "ok" {
						t.Fatalf("response %+v", result)
					}
				}
				sets, items := len(srv.Org.Objects["DuplicateRecordSet"].Records), len(srv.Org.Objects["DuplicateRecordItem"].Records)
				if sets != 0 || items != 0 {
					t.Fatalf("shipping host behavior: got %d sets and %d items", sets, items)
				}
			})
		}
	}
}

// API62/67 adjacent/org.tsv:4 captures rejection of an alert-rule insert.
func TestUpsertDuplicateAlertLightningAndVisualforce(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/lightning", func(t *testing.T) {
			srv := upsertHostServer(t, api)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/lightningAlert", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			srv.ServeHTTP(rec, req)
			var out lwcbrowser.WireResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || out.Error != nil || out.Data != "DUPLICATES_DETECTED" {
				t.Fatalf("status %d response %+v", rec.Code, out)
			}
		})
		t.Run(api+"/visualforce", func(t *testing.T) {
			srv := upsertHostServer(t, api)
			result := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"remoteAlert","data":[],"type":"rpc","tid":1}]`)
			if len(result) != 1 || !result[0].Status || result[0].Result != "DUPLICATES_DETECTED" {
				t.Fatalf("response %+v", result)
			}
		})
	}
}

// API62/67 istest/org.tsv:11: rejected custom-key update keeps the input Id null and stored email unchanged.
func TestUpsertDuplicateFailureStateLightningAndVisualforce(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		want := "DUPLICATES_DETECTED|true|original@example.invalid"
		t.Run(api+"/lightning", func(t *testing.T) {
			srv := upsertHostServer(t, api)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/lightningFailureState", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			srv.ServeHTTP(rec, req)
			var out lwcbrowser.WireResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || out.Error != nil || out.Data != want {
				t.Fatalf("status %d response %+v; want %s", rec.Code, out, want)
			}
		})
		t.Run(api+"/visualforce", func(t *testing.T) {
			srv := upsertHostServer(t, api)
			result := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"remoteFailureState","data":[],"type":"rpc","tid":1}]`)
			if len(result) != 1 || !result[0].Status || result[0].Result != want {
				t.Fatalf("response %+v; want %s", result, want)
			}
		})
	}
}

// Local shipping-preservation controls: other-field and multi-field matches are uncaptured.
func TestUpsertDuplicateUncapturedScopeLightningAndVisualforce(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"other_field", "multi_field"} {
			for _, route := range []string{"lightning", "visualforce"} {
				t.Run(api+"/"+shape+"/"+route, func(t *testing.T) {
					srv := upsertHostServer(t, api)
					object := srv.Org.Objects["Contact"]
					for id, row := range object.Records {
						row.Fields["Phone"] = storage.StringValue("5551234567")
						object.Records[id] = row
					}
					match := storage.MatchingRule{Object: "Contact", Name: "Synthetic_Exact", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Phone", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}
					if shape == "multi_field" {
						match.Items = append(match.Items, storage.MatchingRuleItem{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"})
					}
					object.Definition.DuplicateRules = []storage.DuplicateRule{{Name: "ScopeRule", Object: "Contact", Active: true, ActionOnInsert: "Block", MatchRules: []storage.MatchingRule{match}}}
					srv.Org.Objects["Contact"] = object
					if route == "lightning" {
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/lightningUncapturedScope", strings.NewReader(`{}`))
						req.Header.Set("Content-Type", "application/json")
						srv.ServeHTTP(rec, req)
						var out lwcbrowser.WireResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
							t.Fatal(err)
						}
						if rec.Code != http.StatusOK || out.Error != nil || out.Data != "ok" {
							t.Fatalf("status %d response %+v", rec.Code, out)
						}
					} else {
						result := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"remoteUncapturedScope","data":[],"type":"rpc","tid":1}]`)
						if len(result) != 1 || !result[0].Status || result[0].Result != "ok" {
							t.Fatalf("response %+v", result)
						}
					}
				})
			}
		}
	}
}

// Missing and null prior keys are uncaptured local preservation controls.
func TestUpsertDuplicateUncapturedBlankPriorLightningAndVisualforce(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, prior := range []string{"false", "true"} {
			for _, route := range []string{"lightning", "visualforce"} {
				t.Run(api+"/"+prior+"/"+route, func(t *testing.T) {
					srv := upsertHostServer(t, api)
					if route == "lightning" {
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/lightningBlankPrior", strings.NewReader(`{"explicitNull":`+prior+`}`))
						req.Header.Set("Content-Type", "application/json")
						srv.ServeHTTP(rec, req)
						var out lwcbrowser.WireResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
							t.Fatal(err)
						}
						if rec.Code != http.StatusOK || out.Error != nil || out.Data != "ok" {
							t.Fatalf("status %d response %+v", rec.Code, out)
						}
					} else {
						result := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"remoteBlankPrior","data":[`+prior+`],"type":"rpc","tid":1}]`)
						if len(result) != 1 || !result[0].Status || result[0].Result != "ok" {
							t.Fatalf("response %+v", result)
						}
					}
				})
			}
		}
	}
}
