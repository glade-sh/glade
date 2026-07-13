package dbmanager_test

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dbmanager"
	"github.com/glade-sh/glade/internal/storage"
)

func TestManagerListsObjectsWithCounts(t *testing.T) {
	org := testManagerOrg()
	manager := dbmanager.New(&org)

	got := manager.ListObjects(dbmanager.ListObjectsOptions{Query: "acc"})

	if len(got.Objects) != 1 || got.Objects[0].Name != "Account" || got.Objects[0].Records != 1 {
		t.Fatalf("objects = %#v", got.Objects)
	}
}

func TestManagerListObjectsReturnsCategoryAndCapabilities(t *testing.T) {
	org := testManagerOrg()
	org.Objects["Project__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Project__c",
			Label:   "Project",
			Fields: map[string]storage.Field{
				"Id":   {APIName: "Id", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Profile"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Profile",
			Label:   "Profile",
			Fields: map[string]storage.Field{
				"Id":   {APIName: "Id", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Rule__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:  "Rule__mdt",
			Label:    "Rule",
			Metadata: map[string]string{"kind": "customMetadata"},
			Fields: map[string]storage.Field{
				"Id":            {APIName: "Id", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
				"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	setupNames := []string{"ApexPage", "CurrencyType", "DatedConversionRate", "Network", "OpportunityStage", "PermissionSetAssignment", "StaticResource", "UserLicense", "UserLogin"}
	for _, name := range setupNames {
		org.Objects[name] = storage.ObjectState{
			Definition: storage.ObjectDefinition{
				APIName: name,
				Label:   name,
				Fields: map[string]storage.Field{
					"Id":   {APIName: "Id", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
					"Name": {APIName: "Name", Type: storage.FieldString},
				},
			},
			Records: map[storage.ID]storage.Record{},
		}
	}
	manager := dbmanager.New(&org)

	got := manager.ListObjects(dbmanager.ListObjectsOptions{})

	account := objectSummaryByName(t, got, "Account")
	if account.Category != "standard" || !account.Capabilities.Createable || !account.Capabilities.Updateable || !account.Capabilities.Deletable {
		t.Fatalf("Account summary = %#v", account)
	}
	custom := objectSummaryByName(t, got, "Project__c")
	if custom.Category != "custom" || !custom.Capabilities.Createable {
		t.Fatalf("Project__c summary = %#v", custom)
	}
	profile := objectSummaryByName(t, got, "Profile")
	if profile.Category != "setup" || profile.Capabilities.Createable || profile.Capabilities.Reason == "" {
		t.Fatalf("Profile summary = %#v", profile)
	}
	metadata := objectSummaryByName(t, got, "Rule__mdt")
	if metadata.Category != "setup" || metadata.Capabilities.Createable || metadata.Capabilities.Reason == "" {
		t.Fatalf("Rule__mdt summary = %#v", metadata)
	}
	for _, name := range setupNames {
		setup := objectSummaryByName(t, got, name)
		if setup.Category != "setup" || setup.Capabilities.Createable || setup.Capabilities.Reason == "" {
			t.Fatalf("%s summary = %#v", name, setup)
		}
	}
}

func TestManagerObjectDetailReturnsCategoryAndCapabilities(t *testing.T) {
	org := testManagerOrg()
	manager := dbmanager.New(&org)

	got, err := manager.ObjectDetail("Account")
	if err != nil {
		t.Fatal(err)
	}

	if got.Category != "standard" || !got.Capabilities.Createable {
		t.Fatalf("detail = %#v", got)
	}
}

func TestManagerObjectDetailBuildsSalesforceLikeFieldEditors(t *testing.T) {
	org := testManagerOrg()
	manager := dbmanager.New(&org)

	got, err := manager.ObjectDetail("Account")
	if err != nil {
		t.Fatal(err)
	}

	assertFieldEditor(t, got.Fields, "Industry", "picklist", true)
	assertFieldEditor(t, got.Fields, "Services__c", "multipicklist", true)
	assertFieldEditor(t, got.Fields, "OwnerId", "lookup", true)
	assertFieldEditor(t, got.Fields, "IsActive__c", "checkbox", true)
	assertFieldEditor(t, got.Fields, "AnnualRevenue", "number", true)
	assertFieldEditor(t, got.Fields, "CloseDate__c", "date", true)
	assertFieldEditor(t, got.Fields, "LastContacted__c", "datetime", true)
}

func TestManagerCareProgramDetailAndParentLookupUseStandardCatalog(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "CareProgram")
	state := org.Objects["CareProgram"]
	state.Records["0kP000000000001"] = storage.Record{
		ID:     "0kP000000000001",
		Object: "CareProgram",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Parent Program")},
	}
	org.Objects["CareProgram"] = state
	manager := dbmanager.New(&org)

	detail, err := manager.ObjectDetail("CareProgram")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Label != "Care Program" {
		t.Fatalf("CareProgram detail label = %q", detail.Label)
	}
	assertFieldEditor(t, detail.Fields, "ParentProgramId", "lookup", true)

	lookup, err := manager.Lookup(dbmanager.LookupOptions{Object: "CareProgram", Field: "ParentProgramId", Query: "Parent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(lookup.Records) != 1 || lookup.Records[0].Object != "CareProgram" || lookup.Records[0].Title != "Parent Program" {
		t.Fatalf("CareProgram parent lookup = %#v", lookup.Records)
	}
}

func TestManagerListRecordsSkipsDeletedByDefault(t *testing.T) {
	org := testManagerOrg()
	manager := dbmanager.New(&org)

	got, err := manager.ListRecords("Account", dbmanager.ListRecordsOptions{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || got.Records[0].Title != "Acme" {
		t.Fatalf("records = %#v", got)
	}
}

func TestManagerListRecordsIncludesDeletedWhenRequested(t *testing.T) {
	org := testManagerOrg()
	manager := dbmanager.New(&org)

	got, err := manager.ListRecords("Account", dbmanager.ListRecordsOptions{IncludeDeleted: true, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 {
		t.Fatalf("total = %d, want 2", got.Total)
	}
}

func TestManagerRecordDetailIncludesExplicitNulls(t *testing.T) {
	org := testManagerOrg()
	manager := dbmanager.New(&org)

	got, err := manager.RecordDetail("Account", "001000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Fields["Description"] != nil {
		t.Fatalf("Description = %#v, want nil explicit null", got.Fields["Description"])
	}
	if got.Fields["Name"] != "Acme" {
		t.Fatalf("Name = %#v", got.Fields["Name"])
	}
}

func assertFieldEditor(t *testing.T, fields []dbmanager.FieldEditor, name, control string, editable bool) {
	t.Helper()
	for _, field := range fields {
		if field.Name != name {
			continue
		}
		if field.Control != control {
			t.Fatalf("%s control = %q, want %q", name, field.Control, control)
		}
		if editable && (!field.Createable || !field.Updateable) {
			t.Fatalf("%s create/update flags = %t/%t", name, field.Createable, field.Updateable)
		}
		return
	}
	t.Fatalf("field %s missing from %#v", name, fields)
}

func objectSummaryByName(t *testing.T, list dbmanager.ObjectList, name string) dbmanager.ObjectSummary {
	t.Helper()
	for _, object := range list.Objects {
		if object.Name == name {
			return object
		}
	}
	t.Fatalf("object %s missing from %#v", name, list.Objects)
	return dbmanager.ObjectSummary{}
}

func testManagerOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["User"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "User",
			Label:     "User",
			KeyPrefix: "005",
			Fields: map[string]storage.Field{
				"Id":       {APIName: "Id", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
				"Name":     {APIName: "Name", Label: "Name", Type: storage.FieldString},
				"Username": {APIName: "Username", Label: "Username", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"005000000000001": {
				ID:     "005000000000001",
				Object: "User",
				Fields: map[string]storage.Value{
					"Name":     storage.StringValue("System User"),
					"Username": storage.StringValue("system@example.invalid"),
				},
			},
		},
	}
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Account",
			Label:       "Account",
			PluralLabel: "Accounts",
			KeyPrefix:   "001",
			Fields: map[string]storage.Field{
				"Id":               {APIName: "Id", Label: "Record ID", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
				"Name":             {APIName: "Name", Label: "Account Name", Type: storage.FieldString, Required: true, Nillable: storage.BoolFlag(false)},
				"Description":      {APIName: "Description", Label: "Description", Type: storage.FieldBlob},
				"Industry":         {APIName: "Industry", Label: "Industry", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Technology", Label: "Technology", Active: true}, {Value: "Media", Label: "Media", Active: true}}},
				"Services__c":      {APIName: "Services__c", Label: "Services", Type: storage.FieldMultiPicklist, PicklistValues: []storage.PicklistValue{{Value: "Implementation", Label: "Implementation", Active: true}, {Value: "Support", Label: "Support", Active: true}}},
				"OwnerId":          {APIName: "OwnerId", Label: "Owner", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner"},
				"IsActive__c":      {APIName: "IsActive__c", Label: "Active", Type: storage.FieldBoolean},
				"AnnualRevenue":    {APIName: "AnnualRevenue", Label: "Annual Revenue", Type: storage.FieldDecimal},
				"Employees":        {APIName: "Employees", Label: "Employees", Type: storage.FieldInteger},
				"CloseDate__c":     {APIName: "CloseDate__c", Label: "Close Date", Type: storage.FieldDate},
				"LastContacted__c": {APIName: "LastContacted__c", Label: "Last Contacted", Type: storage.FieldDateTime},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":        storage.StringValue("Acme"),
					"Industry":    storage.StringValue("Technology"),
					"OwnerId":     storage.IDValue("005000000000001"),
					"Employees":   storage.IntegerValue(7),
					"IsActive__c": storage.BooleanValue(true),
				},
				ExplicitNulls: map[string]bool{"Description": true},
			},
			"001000000000002": {
				ID:     "001000000000002",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Deleted Account"),
				},
				System: storage.SystemFields{IsDeleted: true},
			},
		},
	}
	storage.EnsureDeterministicPlatformData(&org)
	return org
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}
