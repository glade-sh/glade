package vm

import (
	"fmt"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native outcomes and before-trigger boundaries: case349_external/org.tsv:5-14,19-28.
func TestCase349ExternalAccountUpsertPreflight(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", Active: true, Available: true}, {ID: "012000000000002", Active: true, Available: true, PersonAccount: true}}
	business := storage.ID("001000000000101")
	person := storage.ID("001000000000102")
	account.Records[business] = storage.Record{Object: "Account", ID: business, Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue("business"), "RecordTypeId": storage.IDValue("012000000000001"), "IsPersonAccount": storage.BooleanValue(false)}}
	account.Records[person] = storage.Record{Object: "Account", ID: person, Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue("person"), "RecordTypeId": storage.IDValue("012000000000002"), "IsPersonAccount": storage.BooleanValue(true)}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, tc := range []struct {
		name, key, field, value, status string
		created                         bool
	}{
		{"businessFirst", "business", "FirstName", "Synthetic", "INVALID_FIELD_FOR_INSERT_UPDATE", false},
		{"businessBadEmail", "business", "PersonEmail", "invalid", "INVALID_EMAIL_ADDRESS", false},
		{"personName", "person", "Name", "Synthetic", "INVALID_FIELD_FOR_INSERT_UPDATE", false},
		{"unmatchedFirst", "unmatched", "FirstName", "Synthetic", "INVALID_FIELD_FOR_INSERT_UPDATE", true},
		{"unmatchedBadEmail", "unmatched", "PersonEmail", "invalid", "INVALID_EMAIL_ADDRESS", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]storage.Value{"ProbeKey__c": storage.StringValue(tc.key), tc.field: storage.StringValue(tc.value)}
			if tc.created {
				fields["Name"] = storage.StringValue("Unmatched")
			}
			record := []storage.Record{{Object: "Account", Fields: fields}}
			got := machine.preflightBusinessAccountNameFailures(record, "upsert", "ProbeKey__c")
			if tc.field == "PersonEmail" {
				got = machine.preflightAccountPersonEmailFailures(record, "upsert", "ProbeKey__c")
			}
			if len(got) != 1 || got[0].Success || got[0].StatusCode != tc.status || got[0].UpsertCreated != tc.created {
				t.Fatalf("got %#v, want %s created=%t", got, tc.status, tc.created)
			}
		})
	}
	for _, key := range []string{"business", "unmatched"} {
		record := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue(key), "PersonEmail": storage.StringValue("   ")}}}
		if got := machine.preflightAccountPersonEmailFailures(record, "upsert", "ProbeKey__c"); len(got) != 0 && !got[0].Success {
			t.Errorf("blank email %s: %#v", key, got)
		}
	}
}

// A duplicate key is left to the regular upsert path even after a prior row fails.
func TestCase349ExternalPreflightLeavesDuplicateForDML(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	for _, id := range []storage.ID{"001000000000101", "001000000000102"} {
		account.Records[id] = storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue("duplicate")}}
	}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	records := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue("new"), "PersonEmail": storage.StringValue("invalid")}}, {Object: "Account", Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue("duplicate"), "PersonEmail": storage.StringValue("invalid")}}}
	got := machine.preflightAccountPersonEmailFailures(records, "upsert", "ProbeKey__c")
	if len(got) != 2 || got[0].Success || !got[1].Success {
		t.Fatalf("preflight: %#v", got)
	}
}

// An explicit Id with an external key remains on the existing upsert path;
// the native external-key capture contains Id-less candidates only.
func TestCase349ExternalPreflightSkipsExplicitId(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	candidate := []storage.Record{{Object: "Account", ID: "001000000000101", Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue("unmatched"), "PersonEmail": storage.StringValue("invalid")}}}
	if got := machine.preflightAccountPersonEmailFailures(candidate, "upsert", "ProbeKey__c"); len(got) != 0 && !got[0].Success {
		t.Fatalf("explicit Id: %#v", got)
	}
}

// Selector and duplicate-input validation must remain on the existing DML path.
func TestCase349ExternalPreflightPreservesInputValidation(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.Fields["Ordinary__c"] = storage.Field{APIName: "Ordinary__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, tc := range []struct {
		name, key string
		values    []string
	}{{"nonexternal", "Ordinary__c", []string{"x"}}, {"empty", "ProbeKey__c", []string{""}}, {"duplicates", "ProbeKey__c", []string{"same", "same"}}} {
		t.Run(tc.name, func(t *testing.T) {
			var records []storage.Record
			for _, v := range tc.values {
				records = append(records, storage.Record{Object: "Account", Fields: map[string]storage.Value{tc.key: storage.StringValue(v), "PersonEmail": storage.StringValue("invalid")}})
			}
			if got := machine.preflightAccountPersonEmailFailures(records, "upsert", tc.key); len(got) != 0 {
				for _, r := range got {
					if !r.Success {
						t.Fatalf("preflight bypassed input validation: %#v", got)
					}
				}
			}
		})
	}
}

// Exercise the connected DML route, including the native before-trigger boundary.
// Rows: testdata/conformance/case349_external/org.tsv:5-6,8-9,11-12 and API67 twins.
func TestCase349ExternalUpsertTriggerBoundary(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, key, assignment, status string
			hits                          int
			created                       bool
		}{
			{"businessFirst", "business", "a.FirstName='Synthetic';", "INVALID_FIELD_FOR_INSERT_UPDATE", 0, false},
			{"businessEmail", "business", "a.PersonEmail='invalid';", "INVALID_EMAIL_ADDRESS", 0, false},
			{"personName", "person", "a.Name='Synthetic'; a.PersonHomePhone='4155550123';", "INVALID_FIELD_FOR_INSERT_UPDATE", 0, false},
			{"unmatchedFirst", "unmatched", "a.Name='Unmatched'; a.FirstName='Synthetic';", "INVALID_FIELD_FOR_INSERT_UPDATE", 0, true},
			{"unmatchedEmail", "unmatched", "a.Name='Unmatched'; a.PersonEmail='invalid';", "INVALID_EMAIL_ADDRESS", 0, true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				account := org.Objects["Account"]
				account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
				account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", Active: true, Available: true}, {ID: "012000000000002", Active: true, Available: true, PersonAccount: true}}
				for _, seed := range []struct {
					id      storage.ID
					key, rt string
					person  bool
				}{{"001000000000101", "business", "012000000000001", false}, {"001000000000102", "person", "012000000000002", true}} {
					account.Records[seed.id] = storage.Record{Object: "Account", ID: seed.id, Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue(seed.key), "RecordTypeId": storage.IDValue(storage.ID(seed.rt)), "IsPersonAccount": storage.BooleanValue(seed.person)}}
				}
				org.Objects["Account"] = account
				machine := New(nil)
				machine.SetOrg(&org)
				if err := machine.RegisterClass(Class{Name: "ExternalCounter", StaticFields: map[string]Field{"hits": {Name: "hits", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)}}}); err != nil {
					t.Fatal(err)
				}
				trigger, err := CompileAnonymousWithOptions("ExternalCounter.hits++;", CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				for _, op := range []string{"insert", "update"} {
					if err = machine.RegisterTrigger(Trigger{Name: "ExternalObserver" + op, Object: "Account", Timing: "before", Operation: op, APIVersion: api, Program: trigger}); err != nil {
						t.Fatal(err)
					}
				}
				source := fmt.Sprintf("Account a=new Account(ProbeKey__c='%s'); %s Database.UpsertResult r=Database.upsert(a,Account.Fields.ProbeKey__c,false); System.assertEquals(false,r.isSuccess()); System.assertEquals('%s',String.valueOf(r.getErrors()[0].getStatusCode())); System.assertEquals(%t,r.isCreated()); System.assertEquals(%d,ExternalCounter.hits);", tc.key, tc.assignment, tc.status, tc.created, tc.hits)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Uncaptured combined failures retain the existing identity-validation route.
func TestCase349AccountPreflightPreservesInvalidIdentity(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", Active: true, Available: true}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, tc := range []struct {
		op string
		id storage.ID
	}{{"insert", "001000000000101"}, {"update", ""}, {"update", "001000000000102"}, {"update", "003000000000101"}} {
		rec := []storage.Record{{Object: "Account", ID: tc.id, Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000001"), "FirstName": storage.StringValue("Synthetic"), "PersonEmail": storage.StringValue("invalid")}}}
		if got := machine.preflightBusinessAccountNameFailures(rec, tc.op, ""); len(got) > 0 && !got[0].Success {
			t.Errorf("%s name: %#v", tc.op, got)
		}
		if got := machine.preflightAccountPersonEmailFailures(rec, tc.op, ""); len(got) > 0 && !got[0].Success {
			t.Errorf("%s email: %#v", tc.op, got)
		}
	}
}

// No native row covers combined-invalid all-or-none batches; retain the
// existing identity validation instead of reporting another row as successful.
func TestCase349AccountAllOrNoneIdentityPrecedence(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	id := storage.ID("001000000000101")
	account.Records[id] = storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Business")}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	source := `Boolean caught=false; try { Database.update(new List<Account>{new Account(Id='001000000000199'),new Account(Id='001000000000101',PersonEmail='invalid')},true); } catch(DmlException e) { caught=true; System.assertEquals('INVALID_CROSS_REFERENCE_KEY',String.valueOf(e.getDmlStatusCode(0))); } System.assertEquals(true,caught);`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// Nonunique and non-text selectors have no capture and retain the old path.
func TestCase349ExternalPreflightLeavesUncapturedSelectorClasses(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["Nonunique__c"] = storage.Field{APIName: "Nonunique__c", Type: storage.FieldString, ExternalID: true}
	account.Definition.Fields["Numeric__c"] = storage.Field{APIName: "Numeric__c", Type: storage.FieldDecimal, ExternalID: true, Unique: true}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, tc := range []struct {
		field string
		value storage.Value
	}{{"Nonunique__c", storage.StringValue("synthetic")}, {"Numeric__c", storage.DecimalValue("1")}} {
		rec := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{tc.field: tc.value, "PersonEmail": storage.StringValue("invalid")}}}
		if got := machine.preflightAccountPersonEmailFailures(rec, "upsert", tc.field); len(got) > 0 && !got[0].Success {
			t.Errorf("%s: %#v", tc.field, got)
		}
	}
}

// Preserve both already-detected row failures in all-or-none batch results.
func TestCase349AccountAllOrNoneCollectsFieldFailures(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", Active: true, Available: true}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	source := `Boolean caught=false; try { Database.insert(new List<Account>{new Account(Name='First',PersonEmail='invalid'),new Account(Name='Second',RecordTypeId='012000000000001',FirstName='Synthetic')},true); } catch(DmlException e) { caught=true; System.assertEquals(2,e.getNumDml()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(e.getDmlStatusCode(0))); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(e.getDmlStatusCode(1))); } System.assertEquals(true,caught);`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// Supplemental routes for the native all-or-none batch rows at
// testdata/conformance/case349_external/org.tsv:56-67.
func TestCase349AccountMixedBatchNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, op := range []string{"insert", "update", "upsert"} {
			t.Run(api+"/"+op, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				account := org.Objects["Account"]
				account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
				account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", Active: true, Available: true}}
				for _, seed := range []struct {
					id  storage.ID
					key string
				}{{"001000000000101", "first"}, {"001000000000102", "second"}} {
					account.Records[seed.id] = storage.Record{Object: "Account", ID: seed.id, Fields: map[string]storage.Value{"Name": storage.StringValue("Business"), "ProbeKey__c": storage.StringValue(seed.key), "RecordTypeId": storage.IDValue("012000000000001"), "IsPersonAccount": storage.BooleanValue(false)}}
				}
				org.Objects["Account"] = account
				machine := New(nil)
				machine.SetOrg(&org)
				if err := machine.RegisterClass(Class{Name: "BatchCounter", StaticFields: map[string]Field{"hits": {Name: "hits", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)}}}); err != nil {
					t.Fatal(err)
				}
				trigger, err := CompileAnonymousWithOptions("BatchCounter.hits++;", CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				for _, kind := range []string{"insert", "update"} {
					if err = machine.RegisterTrigger(Trigger{Name: "BatchObserver" + kind, Object: "Account", Timing: "before", Operation: kind, APIVersion: api, Program: trigger}); err != nil {
						t.Fatal(err)
					}
				}
				rows := "new Account(Name='First',PersonEmail='invalid'),new Account(Name='Second',RecordTypeId='012000000000001',FirstName='Synthetic')"
				call := "Database.insert(values,true)"
				if op == "update" {
					rows = "new Account(Id='001000000000101',PersonEmail='invalid'),new Account(Id='001000000000102',FirstName='Synthetic')"
					call = "Database.update(values,true)"
				}
				if op == "upsert" {
					rows = "new Account(ProbeKey__c='first',PersonEmail='invalid'),new Account(ProbeKey__c='second',FirstName='Synthetic')"
					call = "Database.upsert(values,Account.Fields.ProbeKey__c,true)"
				}
				source := fmt.Sprintf("List<Account> values=new List<Account>{%s}; Boolean caught=false; try { %s; } catch(DmlException e) { caught=true; System.assertEquals(2,e.getNumDml()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(e.getDmlStatusCode(0))); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(e.getDmlStatusCode(1))); } System.assertEquals(true,caught); System.assertEquals(0,BatchCounter.hits);", rows, call)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Preserve duplicate external-key validation for mixed all-or-none batches.
func TestCase349ExternalBatchDuplicatePrecedence(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	for _, id := range []storage.ID{"001000000000101", "001000000000102"} {
		account.Records[id] = storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Business"), "ProbeKey__c": storage.StringValue("duplicate")}}
	}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	source := `Boolean caught=false; try { Database.upsert(new List<Account>{new Account(ProbeKey__c='duplicate'),new Account(ProbeKey__c='new',Name='Other',PersonEmail='invalid')},Account.Fields.ProbeKey__c,true); } catch(DmlException e) { caught=true; System.assertEquals('DUPLICATE_EXTERNAL_ID',String.valueOf(e.getDmlStatusCode(0))); } System.assertEquals(true,caught);`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// Combined invalid fields are outside the capture and retain the old DML path.
func TestCase349AccountCombinedFieldFailuresUseExistingPath(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", Active: true, Available: true}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "CombinedCounter", StaticFields: map[string]Field{"hits": {Name: "hits", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)}}}); err != nil {
		t.Fatal(err)
	}
	trigger, err := CompileAnonymous("CombinedCounter.hits++;")
	if err != nil {
		t.Fatal(err)
	}
	if err = machine.RegisterTrigger(Trigger{Name: "CombinedObserver", Object: "Account", Timing: "before", Operation: "insert", Program: trigger}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`Account a=new Account(Name='Business',RecordTypeId='012000000000001',FirstName='Synthetic',PersonEmail='invalid'); Database.SaveResult r=Database.insert(a,false); System.assertEquals(false,r.isSuccess()); System.assertEquals(1,CombinedCounter.hits);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
