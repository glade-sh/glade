package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSetupFormulaUsesQueryExecutionUser(t *testing.T) {
	org := storage.NewOrgState()
	org.OrgID = "00D000000000mH2"
	user := storage.ID(storage.DefaultUserID)
	actor := storage.ID("005000000000002")
	setting := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
	org.Objects[setting.APIName] = storage.ObjectState{Definition: setting, Records: map[storage.ID]storage.Record{
		"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(user), "Flag__c": storage.BooleanValue(true)}},
		"a02000000000002": {ID: "a02000000000002", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Flag__c": storage.BooleanValue(false)}},
	}}
	account := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"SetupFlag__c": {APIName: "SetupFlag__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Setup.Prefs__c.Flag__c"}}}
	account.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldCalculated, DisplayType: "DOUBLE", Formula: "IF($Setup.Prefs__c.Flag__c, 1, 2)"}
	org.Objects["Account"] = storage.ObjectState{Definition: account, Records: map[storage.ID]storage.Record{"001000000000001": {ID: "001000000000001", Object: "Account"}}}
	contact := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}, Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account"}}}
	contact.Fields["Computed__c"] = storage.Field{APIName: "Computed__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Setup.Prefs__c.Flag__c"}
	org.Objects["Contact"] = storage.ObjectState{Definition: contact, Records: map[storage.ID]storage.Record{"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue("001000000000001")}}}}
	related, err := Parse("SELECT Account.SetupFlag__c FROM Contact")
	if err != nil {
		t.Fatal(err)
	}
	semi, err := Parse("SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE Computed__c = false)")
	if err != nil {
		t.Fatal(err)
	}
	q, err := Parse("SELECT SetupFlag__c FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	where, err := Parse("SELECT Id FROM Account WHERE SetupFlag__c = true")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := Parse("SELECT SUM(Amount__c) total FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	for _, tc := range []struct {
		actor storage.ID
		want  bool
	}{{user, true}, {actor, false}, {user, true}} {
		result, err := ExecuteWithCacheAndContext(org, q, cache, ExecutionContext{UserID: tc.actor})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Records) != 1 {
			t.Fatalf("rows=%+v", result)
		}
		got := result.Records[0].Fields["SetupFlag__c"]
		if got.Kind != storage.ValueBoolean || got.Boolean != tc.want {
			t.Fatalf("actor=%s got=%#v want=%v", tc.actor, got, tc.want)
		}
		semiResult, err := ExecuteWithCacheAndContext(org, semi, cache, ExecutionContext{UserID: tc.actor})
		if err != nil {
			t.Fatal(err)
		}
		if (len(semiResult.Records) == 1) == tc.want {
			t.Fatalf("actor=%s semijoin=%#v", tc.actor, semiResult)
		}
		parent, err := ExecuteWithCacheAndContext(org, related, cache, ExecutionContext{UserID: tc.actor})
		if err != nil {
			t.Fatal(err)
		}
		if len(parent.Records) != 1 || parent.Records[0].Fields["Account.SetupFlag__c"].Boolean != tc.want {
			t.Fatalf("actor=%s relationship=%#v want=%v", tc.actor, parent, tc.want)
		}
		aggregated, err := ExecuteWithCacheAndContext(org, sum, cache, ExecutionContext{UserID: tc.actor})
		if err != nil {
			t.Fatal(err)
		}
		wantSum := "2.0"
		if tc.want {
			wantSum = "1.0"
		}
		if len(aggregated.Records) != 1 || aggregated.Records[0].Fields["total"].Decimal != wantSum {
			t.Fatalf("actor=%s aggregate=%#v want=%s", tc.actor, aggregated, wantSum)
		}
		filtered, err := ExecuteWithCacheAndContext(org, where, cache, ExecutionContext{UserID: tc.actor})
		if err != nil {
			t.Fatal(err)
		}
		if (len(filtered.Records) == 1) != tc.want {
			t.Fatalf("actor=%s filtered=%#v want match=%v", tc.actor, filtered, tc.want)
		}
	}
}

func TestSetupFormulaOrderingUsesQueryExecutionUser(t *testing.T) {
	org := storage.NewOrgState()
	actor := storage.ID("005000000000002")
	prefs := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
	org.Objects[prefs.APIName] = storage.ObjectState{Definition: prefs, Records: map[storage.ID]storage.Record{
		"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(storage.ID(storage.DefaultUserID)), "Flag__c": storage.BooleanValue(true)}},
		"a02000000000002": {ID: "a02000000000002", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Flag__c": storage.BooleanValue(false)}},
	}}
	account := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "Other__c": {APIName: "Other__c", Type: storage.FieldString}, "Sort__c": {APIName: "Sort__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "IF($Setup.Prefs__c.Flag__c,Name,Other__c)"}}}
	org.Objects["Account"] = storage.ObjectState{Definition: account, Records: map[storage.ID]storage.Record{
		"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("A"), "Other__c": storage.StringValue("Z")}},
		"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Z"), "Other__c": storage.StringValue("A")}},
	}}
	query, err := Parse("SELECT Id FROM Account ORDER BY Sort__c")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id    storage.ID
		first storage.ID
	}{{storage.ID(storage.DefaultUserID), "001000000000001"}, {actor, "001000000000002"}} {
		result, err := ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{UserID: tc.id})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Records) != 2 || result.Records[0].ID != tc.first {
			t.Fatalf("actor=%s rows=%#v want first=%s", tc.id, result.Records, tc.first)
		}
	}
}

func TestSetupFormulaChildOrderingUsesQueryExecutionUser(t *testing.T) {
	org := storage.NewOrgState()
	actor := storage.ID("005000000000002")
	parent := storage.ID("001000000000001")
	prefs := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
	org.Objects[prefs.APIName] = storage.ObjectState{Definition: prefs, Records: map[storage.ID]storage.Record{
		"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(storage.ID(storage.DefaultUserID)), "Flag__c": storage.BooleanValue(true)}},
		"a02000000000002": {ID: "a02000000000002", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Flag__c": storage.BooleanValue(false)}},
	}}
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account"}, Records: map[storage.ID]storage.Record{parent: {ID: parent, Object: "Account"}}}
	contact := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}, "Name": {APIName: "Name", Type: storage.FieldString}, "Other__c": {APIName: "Other__c", Type: storage.FieldString}, "Sort__c": {APIName: "Sort__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "IF($Setup.Prefs__c.Flag__c,Name,Other__c)"}}, Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account", ChildRelationship: "Contacts"}}}
	org.Objects["Contact"] = storage.ObjectState{Definition: contact, Records: map[storage.ID]storage.Record{
		"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(parent), "Name": storage.StringValue("A"), "Other__c": storage.StringValue("Z")}},
		"003000000000002": {ID: "003000000000002", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(parent), "Name": storage.StringValue("Z"), "Other__c": storage.StringValue("A")}},
	}}
	query, err := Parse("SELECT Id,(SELECT Id FROM Contacts ORDER BY Sort__c) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id    storage.ID
		first storage.ID
	}{{storage.ID(storage.DefaultUserID), "003000000000001"}, {actor, "003000000000002"}} {
		result, err := ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{UserID: tc.id})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Records) != 1 {
			t.Fatalf("parents=%#v", result)
		}
		rows := result.Records[0].Children["Contacts"]
		if len(rows) != 2 || rows[0].ID != tc.first {
			t.Fatalf("actor=%s rows=%#v want first=%s", tc.id, rows, tc.first)
		}
	}
}
