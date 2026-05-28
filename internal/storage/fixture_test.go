package storage

import (
	"errors"
	"strings"
	"testing"
)

func TestApplyFixtureResolvesAliasesAndRelationshipRefs(t *testing.T) {
	org := fixtureRelationshipOrg()

	err := ApplyFixture(&org, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{{Alias: "acme", Fields: map[string]Value{"Name": StringValue("Acme")}}}},
			{Name: "Contact", Records: []FixtureRecord{{Alias: "smith", FieldRefs: map[string]string{"AccountId": "acme"}}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["Account"].Records) != 1 || len(org.Objects["Contact"].Records) != 1 {
		t.Fatalf("records = %#v", org.Objects)
	}
	var accountID ID
	for id := range org.Objects["Account"].Records {
		accountID = id
	}
	for _, contact := range org.Objects["Contact"].Records {
		if got := contact.Fields["AccountId"].ID; got != accountID {
			t.Fatalf("AccountId = %q, want %q", got, accountID)
		}
	}
}

func TestEnsureStandardObjectFieldsUsesCustomRelationshipNameAsChildRelationship(t *testing.T) {
	definition := ObjectDefinition{
		APIName: "EducationDegree__c",
		Fields: map[string]Field{
			"Education__c": {APIName: "Education__c", Type: FieldReference, ReferenceTo: []string{"Education__c"}, RelationshipName: "EducationDegrees"},
		},
	}

	EnsureStandardObjectFields(&definition)
	relations := definition.Relations
	var childRelationship string
	for _, relation := range relations {
		if relation.Field == "Education__c" {
			childRelationship = relation.ChildRelationship
			break
		}
	}
	if got := childRelationship; got != "EducationDegrees__r" {
		t.Fatalf("ChildRelationship = %q, want EducationDegrees__r", got)
	}
}

func TestReadFixtureReportsUnsupportedVersionAsTypedError(t *testing.T) {
	_, err := ReadFixture(strings.NewReader(`{"version":"glade.storage.v0"}`))
	var versionErr UnsupportedFixtureVersionError
	if !errors.As(err, &versionErr) {
		t.Fatalf("error = %T %v, want UnsupportedFixtureVersionError", err, err)
	}
	if versionErr.Version != "glade.storage.v0" {
		t.Fatalf("version = %q", versionErr.Version)
	}
}

func TestApplyFixtureResolvesQualifiedAndPolymorphicAliases(t *testing.T) {
	org := fixtureRelationshipOrg()
	err := ApplyFixture(&org, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{{Alias: "shared", Fields: map[string]Value{"Name": StringValue("Acme")}}}},
			{Name: "Contact", Records: []FixtureRecord{{Alias: "shared", Fields: map[string]Value{"LastName": StringValue("Smith")}, FieldRefs: map[string]string{"AccountId": "Account.shared"}}}},
			{Name: "Task", Records: []FixtureRecord{{Alias: "task", FieldRefs: map[string]string{"WhatId": "Account.shared", "WhoId": "Contact.shared"}}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var accountID, contactID ID
	for id := range org.Objects["Account"].Records {
		accountID = id
	}
	for id := range org.Objects["Contact"].Records {
		contactID = id
	}
	for _, task := range org.Objects["Task"].Records {
		if got := task.Fields["WhatId"].ID; got != accountID {
			t.Fatalf("WhatId = %q, want %q", got, accountID)
		}
		if got := task.Fields["WhoId"].ID; got != contactID {
			t.Fatalf("WhoId = %q, want %q", got, contactID)
		}
	}
}

func TestApplyFixtureRejectsAmbiguousAndInvalidRelationshipAliases(t *testing.T) {
	ambiguous := fixtureRelationshipOrg()
	err := ApplyFixture(&ambiguous, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{{Alias: "shared", Fields: map[string]Value{"Name": StringValue("Acme")}}}},
			{Name: "Contact", Records: []FixtureRecord{{Alias: "shared", Fields: map[string]Value{"LastName": StringValue("Smith")}}}},
			{Name: "Task", Records: []FixtureRecord{{FieldRefs: map[string]string{"WhatId": "shared"}}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "ambiguous fixture alias") {
		t.Fatalf("ambiguous error = %v", err)
	}

	duplicateSameObject := fixtureRelationshipOrg()
	err = ApplyFixture(&duplicateSameObject, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{
				{Alias: "shared", Fields: map[string]Value{"Name": StringValue("Acme")}},
				{Alias: "shared", Fields: map[string]Value{"Name": StringValue("GloboCorp")}},
			}},
			{Name: "Contact", Records: []FixtureRecord{{FieldRefs: map[string]string{"AccountId": "shared"}}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "ambiguous fixture alias") {
		t.Fatalf("same-object duplicate error = %v", err)
	}

	wrongTarget := fixtureRelationshipOrg()
	err = ApplyFixture(&wrongTarget, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Contact", Records: []FixtureRecord{{Alias: "smith", Fields: map[string]Value{"LastName": StringValue("Smith")}}}},
			{Name: "Contact", Records: []FixtureRecord{{Alias: "jones", Fields: map[string]Value{"LastName": StringValue("Jones")}, FieldRefs: map[string]string{"AccountId": "Contact.smith"}}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot populate Contact.AccountId") {
		t.Fatalf("wrong target error = %v", err)
	}
}

func TestApplyFixtureInitializesNilExistingRecordMaps(t *testing.T) {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}},
	}
	err := ApplyFixture(&org, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{{Alias: "acme", Fields: map[string]Value{"Name": StringValue("Acme")}}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["Account"].Records) != 1 {
		t.Fatalf("records = %#v", org.Objects["Account"].Records)
	}
}

func TestApplyFixtureKeepsIDsForDuplicateObjectBlocks(t *testing.T) {
	org := fixtureRelationshipOrg()
	err := ApplyFixture(&org, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{
				{Alias: "acme", Fields: map[string]Value{"Name": StringValue("Acme")}},
				{Alias: "global", Fields: map[string]Value{"Name": StringValue("Global Media")}},
			}},
			{Name: "Account", Records: []FixtureRecord{
				{Alias: "edge", Fields: map[string]Value{"Name": StringValue("Edge")}},
			}},
			{Name: "Contact", Records: []FixtureRecord{
				{Fields: map[string]Value{"LastName": StringValue("Smith")}, FieldRefs: map[string]string{"AccountId": "Account.global"}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["Account"].Records) != 3 {
		t.Fatalf("Account records = %d", len(org.Objects["Account"].Records))
	}
	var globalID ID
	for id, account := range org.Objects["Account"].Records {
		if account.Fields["Name"].String == "Global Media" {
			globalID = id
		}
	}
	if globalID == "" {
		t.Fatalf("missing Global Media record: %#v", org.Objects["Account"].Records)
	}
	for _, contact := range org.Objects["Contact"].Records {
		if got := contact.Fields["AccountId"].ID; got != globalID {
			t.Fatalf("AccountId = %q, want %q", got, globalID)
		}
	}
}

func fixtureRelationshipOrg() OrgState {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}},
		Records:    make(map[ID]Record),
	}
	org.Objects["Contact"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]Field{
			"LastName":  {APIName: "LastName", Type: FieldString},
			"AccountId": {APIName: "AccountId", Type: FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
		}},
		Records: make(map[ID]Record),
	}
	org.Objects["Task"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Task", KeyPrefix: "00T", Fields: map[string]Field{
			"Subject": {APIName: "Subject", Type: FieldString},
			"WhoId":   {APIName: "WhoId", Type: FieldReference, ReferenceTo: []string{"Contact"}},
			"WhatId":  {APIName: "WhatId", Type: FieldReference, ReferenceTo: []string{"Account"}},
		}},
		Records: make(map[ID]Record),
	}
	return org
}

func TestEnsureDeterministicPlatformData(t *testing.T) {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}},
		Records:    make(map[ID]Record),
	}
	EnsureDeterministicPlatformData(&org)
	for _, objectName := range []string{"Organization", "UserRole", "User", "Profile", "UserLicense", "PermissionSet", "PermissionSetAssignment", "RecordType"} {
		want := 1
		if objectName == "Profile" {
			want = 7
		}
		if objectName == "User" {
			want = 2
		}
		if objectName == "PermissionSet" {
			want = 9
		}
		if objectName == "UserLicense" {
			want = 2
		}
		if objectName == "RecordType" {
			want = 5
		}
		if len(org.Objects[objectName].Records) != want {
			t.Fatalf("%s records = %#v", objectName, InspectOrg("", org))
		}
	}
	document, ok := org.Objects["Document"]
	if !ok || document.Definition.KeyPrefix != "015" || document.Definition.Fields["Body"].Type != FieldBlob || document.Definition.Fields["IsPublic"].Type != FieldBoolean {
		t.Fatalf("document definition = %#v", document.Definition)
	}
	if len(org.Objects["Account"].Definition.RecordTypes) != 1 {
		t.Fatalf("account record types = %#v", org.Objects["Account"].Definition.RecordTypes)
	}
	if field, ok := org.Objects["Contact"].Definition.Fields["Name"]; !ok || field.Type != FieldString {
		t.Fatalf("Contact.Name field = %#v, %v", field, ok)
	}
	if len(org.Objects["Opportunity"].Definition.RecordTypes) == 0 {
		t.Fatalf("Opportunity record types = %#v", org.Objects["Opportunity"].Definition.RecordTypes)
	}
	opportunityRecordTypeID := org.Objects["Opportunity"].Definition.RecordTypes[0].ID
	if opportunityRecordTypeID == "" {
		t.Fatalf("missing Opportunity record type ID")
	}
	if _, ok := org.Objects["RecordType"].Records[opportunityRecordTypeID]; !ok {
		t.Fatalf("missing Opportunity RecordType record %s: %#v", opportunityRecordTypeID, org.Objects["RecordType"].Records)
	}
	recordTypeID := org.Objects["Account"].Definition.RecordTypes[0].ID
	if recordTypeID == "" {
		t.Fatalf("missing Account record type ID")
	}
	if _, ok := org.Objects["RecordType"].Records[recordTypeID]; !ok {
		t.Fatalf("missing RecordType record %s: %#v", recordTypeID, org.Objects["RecordType"].Records)
	}
	if len(org.Objects["User"].Records) != 2 || len(org.Objects["Profile"].Records) != 7 || len(org.Objects["UserLicense"].Records) != 2 {
		t.Fatalf("platform records = %#v", InspectOrg("", org))
	}
	if _, ok := findRecordByStringField(org.Objects["Profile"].Records, "Name", "Customer Community Guest User"); !ok {
		t.Fatalf("missing guest profile: %#v", org.Objects["Profile"].Records)
	}
	user, ok := org.Objects["User"].Records["005000000000001"]
	if !ok {
		t.Fatalf("missing deterministic user: %#v", org.Objects["User"].Records)
	}
	if user.Fields["UserRoleId"].ID != "00E000000000001" || user.Fields["LocaleSidKey"].String != "en_US" || user.Fields["TimeZoneSidKey"].String != "UTC" {
		t.Fatalf("user fields = %#v", user.Fields)
	}
	automatedProcessUser, ok := org.Objects["User"].Records["005000000000002"]
	if !ok || automatedProcessUser.Fields["Name"].String != "Automated Process" || automatedProcessUser.Fields["UserType"].String != "AutomatedProcess" {
		t.Fatalf("automated process user = %#v, %v", automatedProcessUser, ok)
	}
	minimumAccessPermissionSet, ok := org.Objects["PermissionSet"].Records["0PS000000000004"]
	if !ok || !minimumAccessPermissionSet.Fields["IsOwnedByProfile"].Boolean || minimumAccessPermissionSet.Fields["ProfileId"].ID != "00e000000000002" {
		t.Fatalf("minimum access permission set = %#v, %v", minimumAccessPermissionSet, ok)
	}
	if refs := org.Objects["Attachment"].Definition.Fields["ParentId"].ReferenceTo; !containsStringFold(refs, "User") {
		t.Fatalf("Attachment.ParentId references = %#v, want User", refs)
	}
	if refs := org.Objects["Document"].Definition.Fields["FolderId"].ReferenceTo; !containsStringFold(refs, "User") {
		t.Fatalf("Document.FolderId references = %#v, want User", refs)
	}
	if org.Objects["ContentVersion"].Definition.Fields["ContentDocumentId"].Required {
		t.Fatalf("ContentVersion.ContentDocumentId should be optional for first-version inserts")
	}
	orgRecord := org.Objects["Organization"].Records["00D000000000001"]
	if orgRecord.Fields["IsSandbox"].Kind != ValueBoolean || !orgRecord.Fields["IsSandbox"].Boolean {
		t.Fatalf("organization fields = %#v", orgRecord.Fields)
	}
}

func TestEnsureDeterministicPlatformDataSkipsUsedRecordTypeIDs(t *testing.T) {
	org := NewOrgState()
	org.Objects["RecordType"] = ObjectState{
		Definition: ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]Field{}},
		Records: map[ID]Record{
			"012000000000001": {ID: "012000000000001", Object: "RecordType", Fields: map[string]Value{"SobjectType": StringValue("Contact")}},
		},
	}
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}},
		Records:    make(map[ID]Record),
	}
	EnsureDeterministicPlatformData(&org)
	info := org.Objects["Account"].Definition.RecordTypes[0]
	if info.ID == "012000000000001" || info.ID == "" {
		t.Fatalf("record type id = %q", info.ID)
	}
	record := org.Objects["RecordType"].Records[info.ID]
	if record.Fields["SobjectType"].String != "Account" {
		t.Fatalf("record type record = %#v", record)
	}
	if record.Fields["IsPersonType"].Kind != ValueBoolean || record.Fields["IsPersonType"].Boolean {
		t.Fatalf("account business record type person flag = %#v", record)
	}
}

func TestEnsureDeterministicPlatformDataSeedsCustomObjectRecordTypes(t *testing.T) {
	org := NewOrgState()
	org.Objects["Batch__c"] = ObjectState{
		Definition: ObjectDefinition{
			APIName:   "Batch__c",
			KeyPrefix: "a00",
			Fields:    map[string]Field{"Name": {APIName: "Name", Type: FieldString}},
			RecordTypes: []RecordTypeInfo{
				{DeveloperName: "Scheduled", Name: "Scheduled Batch", Active: true, Available: true, Default: true},
				{DeveloperName: "Ad_Hoc", Active: true, Available: true},
			},
		},
		Records: make(map[ID]Record),
	}
	EnsureDeterministicPlatformData(&org)

	recordTypes := org.Objects["Batch__c"].Definition.RecordTypes
	if len(recordTypes) != 2 {
		t.Fatalf("Batch__c record types = %#v", recordTypes)
	}
	if recordTypes[0].ID == "" || recordTypes[0].Name != "Scheduled Batch" {
		t.Fatalf("first record type = %#v", recordTypes[0])
	}
	if recordTypes[1].ID == "" || recordTypes[1].Name != "Ad_Hoc" {
		t.Fatalf("second record type = %#v", recordTypes[1])
	}
	recordTypeRecords := org.Objects["RecordType"].Records
	first := recordTypeRecords[recordTypes[0].ID]
	if first.Fields["SobjectType"].String != "Batch__c" || first.Fields["Name"].String != "Scheduled Batch" || !first.Fields["IsActive"].Boolean {
		t.Fatalf("first RecordType seed = %#v", first)
	}
	second := recordTypeRecords[recordTypes[1].ID]
	if second.Fields["SobjectType"].String != "Batch__c" || second.Fields["Name"].String != "Ad_Hoc" {
		t.Fatalf("second RecordType seed = %#v", second)
	}
}

func TestEnsureDeterministicPlatformDataSeparatesDuplicateRecordTypeIDsOnSameObject(t *testing.T) {
	org := NewOrgState()
	org.Objects["Batch__c"] = ObjectState{
		Definition: ObjectDefinition{
			APIName:   "Batch__c",
			KeyPrefix: "a00",
			Fields:    map[string]Field{"Name": {APIName: "Name", Type: FieldString}},
			RecordTypes: []RecordTypeInfo{
				{ID: "012000000000003", DeveloperName: "Automatic", Name: "Automatic", Active: true, Available: true},
				{ID: "012000000000003", DeveloperName: "Manual", Name: "Manual", Active: true, Available: true},
			},
		},
		Records: make(map[ID]Record),
	}
	EnsureDeterministicPlatformData(&org)

	recordTypes := org.Objects["Batch__c"].Definition.RecordTypes
	if recordTypes[0].ID == "" || recordTypes[1].ID == "" || recordTypes[0].ID == recordTypes[1].ID {
		t.Fatalf("record type ids = %#v", recordTypes)
	}
	recordTypeRecords := org.Objects["RecordType"].Records
	for _, info := range recordTypes {
		record := recordTypeRecords[info.ID]
		if record.Fields["SobjectType"].String != "Batch__c" || record.Fields["DeveloperName"].String != info.DeveloperName || record.Fields["Name"].String != info.Name {
			t.Fatalf("record type record for %s = %#v", info.DeveloperName, record)
		}
	}
}

func TestEnsureDeterministicPlatformDataSeedsAccountPersonTypeRecordTypes(t *testing.T) {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields:    map[string]Field{"Name": {APIName: "Name", Type: FieldString}},
			RecordTypes: []RecordTypeInfo{
				{DeveloperName: "Organization", Name: "Organization", Active: true, Available: true},
				{DeveloperName: "Individual", Name: "Individual", Active: true, Available: true},
			},
		},
		Records: make(map[ID]Record),
	}
	EnsureDeterministicPlatformData(&org)

	for _, info := range org.Objects["Account"].Definition.RecordTypes {
		record := org.Objects["RecordType"].Records[info.ID]
		if info.DeveloperName == "Individual" && !record.Fields["IsPersonType"].Boolean {
			t.Fatalf("individual record type = %#v", record)
		}
		if info.DeveloperName == "Organization" && record.Fields["IsPersonType"].Boolean {
			t.Fatalf("organization record type = %#v", record)
		}
	}
}

func TestEnsureDeterministicPlatformDataAdvancesRecordTypeSequenceToMaxID(t *testing.T) {
	org := NewOrgState()
	org.Objects["RecordType"] = ObjectState{
		Definition: ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]Field{}},
		Records: map[ID]Record{
			"012000000000010": {ID: "012000000000010", Object: "RecordType", Fields: map[string]Value{"SobjectType": StringValue("Contact")}},
		},
	}
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}},
		Records:    make(map[ID]Record),
	}
	EnsureDeterministicPlatformData(&org)
	if got := org.IDSequences["RecordType"]; got < 36 {
		t.Fatalf("RecordType sequence = %d, want at least 36", got)
	}
	generator := NewIDGenerator(prefixesForOrg(org))
	generator.Sequences = copySequences(org.IDSequences)
	next, err := generator.Next("RecordType")
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := org.Objects["RecordType"].Records[next]; exists {
		t.Fatalf("next RecordType ID %s collides with existing records", next)
	}
}

func containsStringFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func findRecordByStringField(records map[ID]Record, fieldName, want string) (Record, bool) {
	for _, record := range records {
		if value, ok := record.Fields[fieldName]; ok && strings.EqualFold(value.String, want) {
			return record, true
		}
	}
	return Record{}, false
}
