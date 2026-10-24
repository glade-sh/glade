package dml

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Each route reuses the captured Account/Contact observations; upsert exercises
// the same insert/update paths without introducing another expected behavior.
func TestPersonAccountCustomFieldsNativeDMLRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/person_account_custom_fields/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		wants := map[string]any{}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
			row := strings.SplitN(line, "\t", 4)
			if len(row) != 4 {
				t.Fatal(line)
			}
			if row[0] == api && row[1] == "anonymous" {
				var v any
				if err := json.Unmarshal([]byte(row[3]), &v); err != nil {
					t.Fatal(err)
				}
				wants[row[2]] = v
			}
		}
		for _, route := range []string{"insert_update", "upsert"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				storage.EnsureDeterministicPlatformData(&org)
				contact := org.Objects["Contact"]
				contact.Definition = contact.Definition.Clone()
				contact.Definition.Fields["OptIn__c"] = storage.Field{APIName: "OptIn__c", Type: storage.FieldBoolean, DefaultValue: "false"}
				contact.Definition.Fields["Note__c"] = storage.Field{APIName: "Note__c", Type: storage.FieldString, Length: 40}
				contact.Definition.Fields["Sponsor__c"] = storage.Field{APIName: "Sponsor__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}
				org.Objects["Contact"] = contact
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				engine := NewEngine(&org)
				insert := func(r storage.Record) storage.ID {
					t.Helper()
					var results []Result
					if route == "upsert" {
						results = engine.Upsert([]storage.Record{r})
					} else {
						results = engine.Insert([]storage.Record{r})
					}
					if len(results) != 1 || !results[0].Success {
						t.Fatalf("insert: %+v", results)
					}
					return results[0].ID
				}
				update := func(r storage.Record) Result {
					t.Helper()
					var results []Result
					if route == "upsert" {
						results = engine.Upsert([]storage.Record{r})
					} else {
						results = engine.Update([]storage.Record{r})
					}
					if len(results) != 1 {
						t.Fatalf("update: %+v", results)
					}
					return results[0]
				}
				mustUpdate := func(r storage.Record) {
					t.Helper()
					if result := update(r); !result.Success {
						t.Fatalf("update: %+v", result)
					}
				}
				check := func(id string, got any) {
					t.Helper()
					want, ok := wants[id]
					if !ok {
						t.Fatalf("missing native row %s", id)
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("%s: native=%v; Glade=%v", id, want, got)
					}
				}
				sponsorID := insert(storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Garden Sponsor")}})
				accountID := insert(storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(personAccountRecordTypeID(t, org)), "LastName": storage.StringValue("Gardener"), "Note__pc": storage.StringValue("seed"), "Sponsor__pc": storage.IDValue(sponsorID)}})
				account := org.Objects["Account"].Records[accountID]
				contactID := account.Fields["PersonContactId"].ID
				if contactID == "" {
					t.Fatal("missing PersonContactId")
				}
				readContact := func() storage.Record { return org.Objects["Contact"].Records[contactID] }
				check("contact_after_insert", []any{readContact().Fields["Note__c"].String, readContact().Fields["Sponsor__c"].ID == sponsorID})
				snapshot := storage.SnapshotRuntimeOrg(&org)
				beforeAccount := snapshot.Objects["Account"].Records[accountID].Clone()
				beforeContact := snapshot.Objects["Contact"].Records[contactID].Clone()
				t.Cleanup(func() {
					if !reflect.DeepEqual(snapshot.Objects["Account"].Records[accountID], beforeAccount) || !reflect.DeepEqual(snapshot.Objects["Contact"].Records[contactID], beforeContact) {
						t.Error("Person Account synchronization changed the frozen snapshot")
					}
				})
				mustUpdate(storage.Record{Object: "Account", ID: accountID, Fields: map[string]storage.Value{"Note__pc": storage.StringValue("account")}})
				check("contact_after_account_update", readContact().Fields["Note__c"].String)
				mustUpdate(storage.Record{Object: "Contact", ID: contactID, Fields: map[string]storage.Value{"Note__c": storage.StringValue("contact")}})
				check("account_after_contact_update", org.Objects["Account"].Records[accountID].Fields["Note__pc"].String)
				mustUpdate(storage.Record{Object: "Account", ID: accountID, Fields: map[string]storage.Value{"Note__pc": storage.StringValue("dynamic")}})
				check("dynamic_dml", readContact().Fields["Note__c"].String)
				for _, clear := range []storage.Record{
					{Object: "Account", ID: accountID, Fields: map[string]storage.Value{"Note__pc": storage.NullValue()}},
					{Object: "Account", ID: accountID, ExplicitNulls: map[string]bool{"Note__pc": true}},
				} {
					mustUpdate(storage.Record{Object: "Account", ID: accountID, Fields: map[string]storage.Value{"Note__pc": storage.StringValue("dynamic")}})
					mustUpdate(clear)
					value, present := readContact().GetField("Note__c")
					var got any
					if present && value.Kind != storage.ValueNull {
						got = value.String
					}
					check("contact_after_clear", got)
				}
				result := update(storage.Record{Object: "Account", ID: sponsorID, Fields: map[string]storage.Value{"Note__pc": storage.StringValue("business")}})
				check("business_write", result.StatusCode)
				value, present := org.Objects["Account"].Records[sponsorID].GetField("Note__pc")
				var got any
				if present && value.Kind != storage.ValueNull {
					got = value.String
				}
				check("business_read", got)
			})
		}
	}
}
