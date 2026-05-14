package storage

import (
	"errors"
	"testing"

	"github.com/open-aer/oaer/internal/schema"
)

func TestResolveFieldNameResolvesIdCaseInsensitive(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}}

	resolved, ok := ResolveFieldName(definition, "", "id")
	if !ok || resolved != "Id" {
		t.Fatalf("ResolveFieldName(id) = %q, %v", resolved, ok)
	}
}

func TestResolveFieldNameMapsUnqualifiedCustomFieldToOrgNamespace(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account", Fields: map[string]Field{
		"pkg__UpdatePrimaryLocation__c": {APIName: "pkg__UpdatePrimaryLocation__c", Type: FieldBoolean},
		"UpdatePrimaryLocation__c":      {APIName: "UpdatePrimaryLocation__c", Type: FieldString},
	}}

	resolved, ok := ResolveFieldName(definition, "pkg", "UpdatePrimaryLocation__c")
	if !ok || resolved != "pkg__UpdatePrimaryLocation__c" {
		t.Fatalf("ResolveFieldName(UpdatePrimaryLocation__c) = %q, %v", resolved, ok)
	}
}

func TestResolveFieldNameMapsCustomFieldCaseInsensitiveToOrgNamespace(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account", Fields: map[string]Field{
		"pkg__UpdatePrimaryLocation__c": {APIName: "pkg__UpdatePrimaryLocation__c", Type: FieldBoolean},
	}}

	resolved, ok := ResolveFieldName(definition, "pkg", "updateprimarylocation__C")
	if !ok || resolved != "pkg__UpdatePrimaryLocation__c" {
		t.Fatalf("ResolveFieldName(updateprimarylocation__C) = %q, %v", resolved, ok)
	}
}

func TestResolveFieldNameStripsNamespaceCaseInsensitive(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account", Fields: map[string]Field{
		"UpdatePrimaryLocation__c": {APIName: "UpdatePrimaryLocation__c", Type: FieldBoolean},
	}}

	resolved, ok := ResolveFieldName(definition, "pkg", "PKG__UpdatePrimaryLocation__c")
	if !ok || resolved != "UpdatePrimaryLocation__c" {
		t.Fatalf("ResolveFieldName(PKG__UpdatePrimaryLocation__c) = %q, %v", resolved, ok)
	}
}

func TestResolveFieldNameKeepsOtherNamespace(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account", Fields: map[string]Field{
		"other__Status__c": {APIName: "other__Status__c", Type: FieldString},
	}}

	resolved, ok := ResolveFieldName(definition, "pkg", "other__Status__c")
	if !ok || resolved != "other__Status__c" {
		t.Fatalf("ResolveFieldName(other__Status__c) = %q, %v", resolved, ok)
	}
}

func TestResolveFieldNameMapsLocationComponentFields(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account", Fields: map[string]Field{
		"pkg__PrimaryLocation__c": {APIName: "pkg__PrimaryLocation__c", Type: FieldLocation},
	}}

	resolved, ok := ResolveFieldName(definition, "pkg", "PrimaryLocation__Latitude__s")
	if !ok || resolved != "pkg__PrimaryLocation__Latitude__s" {
		t.Fatalf("ResolveFieldName(PrimaryLocation__Latitude__s) = %q, %v", resolved, ok)
	}
	resolved, ok = ResolveFieldName(definition, "pkg", "pkg__PrimaryLocation__Longitude__s")
	if !ok || resolved != "pkg__PrimaryLocation__Longitude__s" {
		t.Fatalf("ResolveFieldName(pkg__PrimaryLocation__Longitude__s) = %q, %v", resolved, ok)
	}

	resolved, ok = ResolveFieldName(definition, "", "PrimaryLocation__Latitude__s")
	if !ok || resolved != "pkg__PrimaryLocation__Latitude__s" {
		t.Fatalf("ResolveFieldName(PrimaryLocation__Latitude__s no namespace) = %q, %v", resolved, ok)
	}
}

func TestDefaultValueForFieldUnquotesStringExpressions(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "single quoted", raw: "'resetcss'", want: "resetcss"},
		{name: "escaped single quote", raw: "'Bob''s'", want: "Bob's"},
		{name: "double quoted", raw: `"resetcss"`, want: "resetcss"},
		{name: "bare", raw: "resetcss", want: "resetcss"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DefaultValueForField(Field{APIName: "DefaultCSS__c", Type: FieldString, DefaultValue: tt.raw})
			if !ok || got.Kind != ValueString || got.String != tt.want {
				t.Fatalf("DefaultValueForField(%q) = %#v, %v; want %q", tt.raw, got, ok, tt.want)
			}
		})
	}
}

func TestDefaultValueForFieldUsesPicklistDefaultEntry(t *testing.T) {
	got, ok := DefaultValueForField(Field{
		APIName: "Status__c",
		Type:    FieldPicklist,
		PicklistValues: []PicklistValue{
			{Value: "Active", Default: true, Active: true},
			{Value: "Inactive", Active: true},
		},
	})
	if !ok || got.Kind != ValueString || got.String != "Active" {
		t.Fatalf("picklist default = %#v, %v; want Active", got, ok)
	}
}

func TestDefaultValueForRecordFieldEvaluatesRecordTypeIF(t *testing.T) {
	definition := ObjectDefinition{
		APIName: "Product__c",
		RecordTypes: []RecordTypeInfo{
			{ID: "012000000000001AAA", DeveloperName: "Merchandise", Name: "Merchandise"},
			{ID: "012000000000002AAA", DeveloperName: "Membership", Name: "Membership"},
		},
	}
	field := Field{APIName: "QuantityMax__c", Type: FieldDecimal, DefaultValue: "IF($RecordType.Name == 'Merchandise', 999, 1)"}

	merch, ok := DefaultValueForRecordField(definition, Record{Fields: map[string]Value{"RecordTypeId": IDValue("012000000000001AAA")}}, field)
	if !ok || merch.Kind != ValueDecimal || merch.Decimal != "999" {
		t.Fatalf("merchandise default = %#v, %v; want 999", merch, ok)
	}
	membership, ok := DefaultValueForRecordField(definition, Record{Fields: map[string]Value{"RecordTypeId": IDValue("012000000000002AAA")}}, field)
	if !ok || membership.Kind != ValueDecimal || membership.Decimal != "1" {
		t.Fatalf("membership default = %#v, %v; want 1", membership, ok)
	}
}

func TestResolveObjectNameMapsUnqualifiedCustomObjectToOrgNamespace(t *testing.T) {
	org := NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Thing__c"] = ObjectState{Definition: ObjectDefinition{APIName: "pkg__Thing__c"}}

	resolved, ok := ResolveObjectName(org, "Thing__c")
	if !ok || resolved != "pkg__Thing__c" {
		t.Fatalf("ResolveObjectName(Thing__c) = %q, %v", resolved, ok)
	}
}

func TestResolveObjectNameMapsCustomObjectCaseInsensitiveToOrgNamespace(t *testing.T) {
	org := NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Thing__c"] = ObjectState{Definition: ObjectDefinition{APIName: "pkg__Thing__c"}}

	resolved, ok := ResolveObjectName(org, "thing__C")
	if !ok || resolved != "pkg__Thing__c" {
		t.Fatalf("ResolveObjectName(thing__C) = %q, %v", resolved, ok)
	}
}

func TestResolveObjectNameStripsNamespaceCaseInsensitive(t *testing.T) {
	org := NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Thing__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Thing__c"}}

	resolved, ok := ResolveObjectName(org, "PKG__Thing__c")
	if !ok || resolved != "Thing__c" {
		t.Fatalf("ResolveObjectName(PKG__Thing__c) = %q, %v", resolved, ok)
	}
}

func TestResolveObjectNameKeepsOtherNamespace(t *testing.T) {
	org := NewOrgState()
	org.Namespace = "pkg"
	org.Objects["other__Thing__c"] = ObjectState{Definition: ObjectDefinition{APIName: "other__Thing__c"}}

	resolved, ok := ResolveObjectName(org, "other__Thing__c")
	if !ok || resolved != "other__Thing__c" {
		t.Fatalf("ResolveObjectName(other__Thing__c) = %q, %v", resolved, ok)
	}
}

func TestResolveObjectNameKeepsExactCustomObjectMatch(t *testing.T) {
	org := NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Thing__c"] = ObjectState{Definition: ObjectDefinition{APIName: "pkg__Thing__c"}}
	org.Objects["Thing__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Thing__c"}}

	resolved, ok := ResolveObjectName(org, "Thing__c")
	if !ok || resolved != "Thing__c" {
		t.Fatalf("ResolveObjectName(Thing__c) = %q, %v", resolved, ok)
	}
}

func TestEnsureStandardObjectFieldsAddsAccountWebsiteWithoutClobber(t *testing.T) {
	definition := ObjectDefinition{
		APIName: "Account",
		Fields: map[string]Field{
			"Website": {APIName: "Website", Label: "Custom label", Type: FieldAny},
		},
	}

	EnsureStandardObjectFields(&definition)

	if definition.Fields["Website"].Type != FieldAny || definition.Fields["Website"].Label != "Custom label" {
		t.Fatalf("Website field was clobbered: %#v", definition.Fields["Website"])
	}
	if field, ok := definition.Fields["Phone"]; !ok || field.Type != FieldString {
		t.Fatalf("Phone field = %#v, %v", field, ok)
	}
	if field, ok := definition.Fields["Name"]; !ok || !field.Required {
		t.Fatalf("Name field = %#v, %v", field, ok)
	}
	if field, ok := definition.Fields["TickerSymbol"]; !ok || field.Type != FieldString {
		t.Fatalf("TickerSymbol field = %#v, %v", field, ok)
	}
	if _, ok := definition.Fields["PersonMailingStreet"]; ok {
		t.Fatalf("PersonMailingStreet should be gated by PersonAccounts: %#v", definition.Fields["PersonMailingStreet"])
	}
	if _, ok := definition.Fields["PersonContactId"]; ok {
		t.Fatalf("PersonContactId should be gated by PersonAccounts: %#v", definition.Fields["PersonContactId"])
	}
	if _, ok := definition.Fields["BillingCountryCode"]; ok {
		t.Fatalf("BillingCountryCode should be gated by StateAndCountryPicklist: %#v", definition.Fields["BillingCountryCode"])
	}
}

func TestEnsureStandardObjectFieldsForFeaturesAddsPersonAccountShape(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account"}

	EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})

	if field, ok := definition.Fields["PersonMailingStreet"]; !ok || field.Type != FieldString {
		t.Fatalf("PersonMailingStreet field = %#v, %v", field, ok)
	}
	if _, ok := definition.Fields["PersonMailingStateCode"]; ok {
		t.Fatalf("PersonMailingStateCode should be gated by StateAndCountryPicklist: %#v", definition.Fields["PersonMailingStateCode"])
	}
	if field, ok := definition.Fields["IsPersonAccount"]; !ok || field.Type != FieldBoolean {
		t.Fatalf("IsPersonAccount field = %#v, %v", field, ok)
	}
	if len(definition.RecordTypes) == 0 {
		t.Fatalf("record types = %#v", definition.RecordTypes)
	}
	foundPersonAccount := false
	for _, recordType := range definition.RecordTypes {
		if recordType.DeveloperName == "PersonAccount" {
			foundPersonAccount = true
		}
	}
	if !foundPersonAccount {
		t.Fatalf("record types missing PersonAccount: %#v", definition.RecordTypes)
	}
}

func TestEnsureStandardObjectFieldsForFeaturesAddsStateAndCountryPicklists(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account"}

	EnsureStandardObjectFieldsForFeatures(&definition, []string{"StateAndCountryPicklist"})

	if field, ok := definition.Fields["BillingCountryCode"]; !ok || field.Type == "" {
		t.Fatalf("BillingCountryCode field = %#v, %v", field, ok)
	}
	if field, ok := definition.Fields["ShippingStateCode"]; !ok || field.Type == "" {
		t.Fatalf("ShippingStateCode field = %#v, %v", field, ok)
	}
}

func TestEnsureStandardObjectFieldsIncludesStubOverlayFields(t *testing.T) {
	definition := ObjectDefinition{APIName: "AsyncApexJob"}

	EnsureStandardObjectFields(&definition)

	if field, ok := definition.Fields["CreatedDate"]; !ok || field.Type != FieldDateTime {
		t.Fatalf("CreatedDate field = %#v, %v", field, ok)
	}
	if field, ok := definition.Fields["ApexClassId"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "ApexClass" {
		t.Fatalf("ApexClassId field = %#v, %v", field, ok)
	}
}

func TestApplyOrgShapeAddsOptionalFeatureObjectsAndRecords(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)

	ApplyOrgShape(&org, []string{
		"PersonAccounts",
		"Communities",
		"Sites",
		"ContactsToMultipleAccounts",
		"PlatformCache",
		"StateAndCountryPicklist",
		"EnableSetPasswordInApi",
		"AddCustomApps:3",
		"AnalyticsAdminPerms",
		"HealthCloudUser",
	})

	account := org.Objects["Account"]
	if _, ok := account.Definition.Fields["PersonContactId"]; !ok {
		t.Fatalf("PersonContactId missing from Account shape")
	}
	if _, ok := account.Definition.Fields["PersonMailingStateCode"]; !ok {
		t.Fatalf("PersonMailingStateCode missing from Account shape")
	}
	if _, ok := org.Objects["AccountContactRelation"]; !ok {
		t.Fatalf("AccountContactRelation object missing")
	}
	if site := org.Objects["Site"]; len(site.Records) == 0 || site.Definition.Fields["GuestUserId"].Type != FieldReference {
		t.Fatalf("Site shape/records = %#v", site)
	}
	if network := org.Objects["Network"]; len(network.Records) == 0 {
		t.Fatalf("Network records = %#v", network.Records)
	}
	if cache := org.Objects["PlatformCachePartition"]; len(cache.Records) == 0 {
		t.Fatalf("PlatformCachePartition records = %#v", cache.Records)
	}
	if apps := org.Objects["CustomApplication"]; len(apps.Records) != 3 {
		t.Fatalf("CustomApplication records = %d, want 3", len(apps.Records))
	}
	orgRecord := org.Objects["Organization"].Records["00D000000000001"]
	if !orgRecord.Fields["IsSetPasswordInApiEnabled"].Boolean || !orgRecord.Fields["HasAnalyticsAdminPerms"].Boolean || !orgRecord.Fields["IsHealthCloudEnabled"].Boolean {
		t.Fatalf("scratch feature flags were not applied: %#v", orgRecord.Fields)
	}
}

func TestEnsureDeterministicPlatformDataSeedsCommonSalesObjects(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)

	for _, objectName := range []string{"Account", "Contact", "Opportunity", "Product2"} {
		if resolved, ok := ResolveObjectName(org, objectName); !ok || resolved != objectName {
			t.Fatalf("ResolveObjectName(%s) = %q, %v", objectName, resolved, ok)
		}
	}
	for objectName, wantPrefix := range map[string]string{
		"Contact":                "003",
		"Opportunity":            "006",
		"OpportunityLineItem":    "00k",
		"OpportunityContactRole": "00K",
		"PricebookEntry":         "01u",
		"Product2":               "01t",
	} {
		EnsureStandardObject(&org, objectName)
		if got := org.Objects[objectName].Definition.KeyPrefix; got != wantPrefix {
			t.Fatalf("%s key prefix = %q, want %q", objectName, got, wantPrefix)
		}
	}

	account := org.Objects["Account"].Definition
	if field, ok := account.Fields["NumberOfEmployees"]; !ok || field.Type != FieldInteger {
		t.Fatalf("Account.NumberOfEmployees field = %#v, %v", field, ok)
	}
	if field, ok := account.Fields["AnnualRevenue"]; !ok || field.Type != FieldDecimal {
		t.Fatalf("Account.AnnualRevenue field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["Contact"].Definition.Fields["AccountId"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Account" {
		t.Fatalf("Contact.AccountId field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["Lead"].Definition.Fields["NumberOfEmployees"]; !ok || field.Type != FieldInteger {
		t.Fatalf("Lead.NumberOfEmployees field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["Opportunity"].Definition.Fields["StageName"]; !ok || !field.Required {
		t.Fatalf("Opportunity.StageName field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["Product2"].Definition.Fields["IsActive"]; !ok || field.Type != FieldBoolean {
		t.Fatalf("Product2.IsActive field = %#v, %v", field, ok)
	}
	if _, ok := org.Objects["Pricebook2"].Records["01s000000000001"]; !ok {
		t.Fatalf("standard Pricebook2 record was not seeded: %#v", org.Objects["Pricebook2"].Records)
	}
}

func TestEnsureStandardObjectCaseBusinessHoursDoesNotBlockInsert(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "Case")
	if org.Objects["Case"].Definition.Fields["BusinessHoursId"].Required {
		t.Fatalf("Case.BusinessHoursId should not block local inserts")
	}
}

func TestEnsureStandardObjectAddsSalesCloudStandardObjectShape(t *testing.T) {
	org := NewOrgState()

	EnsureStandardObject(&org, "Account")
	EnsureStandardObject(&org, "Lead")
	EnsureStandardObject(&org, "Opportunity")
	EnsureStandardObject(&org, "CampaignMember")
	EnsureStandardObject(&org, "OpportunityContactRole")
	EnsureStandardObject(&org, "OrderItem")
	EnsureStandardObject(&org, "OpportunityLineItem")
	EnsureStandardObject(&org, "PricebookEntry")

	if org.Objects["Lead"].Definition.KeyPrefix != "00Q" {
		t.Fatalf("Lead key prefix = %q", org.Objects["Lead"].Definition.KeyPrefix)
	}
	if !IsKnownStandardObject("OpportunityContactRole") {
		t.Fatalf("OpportunityContactRole should be recognized as a generated standard object")
	}
	if !IsKnownStandardObject("CampaignMember") {
		t.Fatalf("CampaignMember should be recognized as a generated standard object")
	}
	if field, ok := org.Objects["Lead"].Definition.Fields["LastName"]; !ok || !field.Required {
		t.Fatalf("Lead.LastName field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["Account"].Definition.Fields["AccountNumber"]; !ok || field.Type != FieldString {
		t.Fatalf("Account.AccountNumber field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["CampaignMember"].Definition.Fields["CampaignId"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Campaign" || !field.Required {
		t.Fatalf("CampaignMember.CampaignId field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["CampaignMember"].Definition.Fields["ContactId"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Contact" {
		t.Fatalf("CampaignMember.ContactId field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["CampaignMember"].Definition.Fields["LeadId"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Lead" {
		t.Fatalf("CampaignMember.LeadId field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["CampaignMember"].Definition.Fields["HasResponded"]; !ok || field.Type != FieldBoolean {
		t.Fatalf("CampaignMember.HasResponded field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["CampaignMember"].Definition.Fields["Status"]; !ok || field.Type != FieldPicklist {
		t.Fatalf("CampaignMember.Status field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["Opportunity"].Definition.Fields["StageName"]; !ok || !field.Required {
		t.Fatalf("Opportunity.StageName field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["Opportunity"].Definition.Fields["RecordTypeId"]; !ok || field.Type != FieldReference {
		t.Fatalf("Opportunity.RecordTypeId field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["OpportunityContactRole"].Definition.Fields["ContactId"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Contact" {
		t.Fatalf("OpportunityContactRole.ContactId field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["OpportunityContactRole"].Definition.Fields["OpportunityId"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Opportunity" || !field.Required {
		t.Fatalf("OpportunityContactRole.OpportunityId field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["OpportunityContactRole"].Definition.Fields["Role"]; !ok || field.Type != FieldPicklist || len(field.PicklistValues) == 0 {
		t.Fatalf("OpportunityContactRole.Role field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["OpportunityContactRole"].Definition.Fields["IsPrimary"]; !ok || field.Type != FieldBoolean {
		t.Fatalf("OpportunityContactRole.IsPrimary field = %#v, %v", field, ok)
	}
	if !hasRelationship(org.Objects["OpportunityContactRole"].Definition.Relations, "OpportunityId", "Opportunity", "Opportunity") {
		t.Fatalf("OpportunityContactRole relations missing OpportunityId: %#v", org.Objects["OpportunityContactRole"].Definition.Relations)
	}
	if !hasRelationship(org.Objects["OpportunityContactRole"].Definition.Relations, "ContactId", "Contact", "Contact") {
		t.Fatalf("OpportunityContactRole relations missing ContactId: %#v", org.Objects["OpportunityContactRole"].Definition.Relations)
	}
	if field, ok := org.Objects["OrderItem"].Definition.Fields["OrderId"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Order" {
		t.Fatalf("OrderItem.OrderId field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["OpportunityLineItem"].Definition.Fields["PricebookEntryId"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "PricebookEntry" {
		t.Fatalf("OpportunityLineItem.PricebookEntryId field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["PricebookEntry"].Definition.Fields["Product2Id"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Product2" {
		t.Fatalf("PricebookEntry.Product2Id field = %#v, %v", field, ok)
	}
}

func TestEnsureStandardObjectAddsGeneratedSObjectStubOverlayShape(t *testing.T) {
	org := NewOrgState()

	EnsureStandardObject(&org, "AIApplication")
	EnsureStandardObject(&org, "AIInsightAction")
	EnsureStandardObject(&org, "AccountContactRelation")

	if !IsKnownStandardObject("AIApplication") {
		t.Fatalf("AIApplication should be recognized from generated SObject stubs")
	}
	if org.Objects["AIApplication"].Definition.PluralLabel != "AI Applications" {
		t.Fatalf("AIApplication plural label = %q", org.Objects["AIApplication"].Definition.PluralLabel)
	}
	if field, ok := org.Objects["AIApplication"].Definition.Fields["DeveloperName"]; !ok || field.Type != FieldString {
		t.Fatalf("AIApplication.DeveloperName field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["AIApplication"].Definition.Fields["CreatedById"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "User" {
		t.Fatalf("AIApplication.CreatedById field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["AIInsightAction"].Definition.Fields["AiRecordInsightId"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "AIRecordInsight" || field.ChildRelationshipName != "AIInsightActions" {
		t.Fatalf("AIInsightAction.AiRecordInsightId field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["AccountContactRelation"].Definition.Fields["Roles"]; !ok || field.Label != "Roles" || field.Type != FieldString {
		t.Fatalf("AccountContactRelation.Roles field = %#v, %v", field, ok)
	}
}

func TestEnsureStandardObjectAddsGeneratedSObjectStubAccessorFlags(t *testing.T) {
	org := NewOrgState()

	EnsureStandardObject(&org, "UserAppInfo")
	fields := org.Objects["UserAppInfo"].Definition.Fields
	if field := fields["Id"]; FieldFlagValue(field.Createable, true) || FieldFlagValue(field.Updateable, true) {
		t.Fatalf("UserAppInfo.Id flags = %#v", field)
	}
	if field := fields["CreatedDate"]; FieldFlagValue(field.Createable, true) || FieldFlagValue(field.Updateable, true) {
		t.Fatalf("UserAppInfo.CreatedDate flags = %#v", field)
	}
}

func TestEnsureStandardObjectPreservesStubChildRelationshipsForSharedFields(t *testing.T) {
	org := NewOrgState()

	EnsureStandardObject(&org, "Contact")
	EnsureStandardObject(&org, "Task")

	task := org.Objects["Task"].Definition
	if !hasChildRelationship(task.Relations, "WhoId", "Contact", "Who", "Tasks") {
		t.Fatalf("Task.WhoId relations missing Contact.Tasks: %#v", task.Relations)
	}
	if hasChildRelationship(task.Relations, "WhoId", "Account", "Who", "PersonTasks") {
		t.Fatalf("Task.WhoId relations should not expose feature-gated Account.PersonTasks: %#v", task.Relations)
	}
	if hasChildRelationship(task.Relations, "WhoId", "Contact", "Who", "PersonTasks") {
		t.Fatalf("Task.WhoId relations should not expose Contact.PersonTasks: %#v", task.Relations)
	}
}

func TestEnsureStandardObjectMergesStubPolymorphicReferenceBreadth(t *testing.T) {
	definition := ObjectDefinition{
		APIName: "Task",
		Fields: map[string]Field{
			"WhatId": {APIName: "WhatId", Label: "Local label", Type: FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "What"},
		},
	}

	EnsureStandardObjectFields(&definition)

	field := definition.Fields["WhatId"]
	if field.Label != "Local label" {
		t.Fatalf("WhatId label was clobbered: %#v", field)
	}
	for _, target := range []string{"Account", "Opportunity", "WorkOrder"} {
		if !containsTestString(field.ReferenceTo, target) {
			t.Fatalf("Task.WhatId ReferenceTo missing %s: %#v", target, field.ReferenceTo)
		}
	}
	if !hasPolymorphicRelationship(definition.Relations, "WhatId", "Opportunity", "What", "Tasks") {
		t.Fatalf("Task.WhatId relations missing polymorphic Opportunity.Tasks: %#v", definition.Relations)
	}
}

func TestEnsureStandardObjectMapsCompoundStubFieldTypes(t *testing.T) {
	definition := ObjectDefinition{APIName: "Contact"}

	EnsureStandardObjectFields(&definition)

	if field, ok := definition.Fields["MailingAddress"]; !ok || field.Type != FieldAddress || field.DisplayType != "ADDRESS" {
		t.Fatalf("Contact.MailingAddress field = %#v, %v", field, ok)
	}
}

func TestEnsureStandardObjectAddsRecordTypeIdFromGeneratedRecordTypes(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account"}

	EnsureStandardObjectFields(&definition)

	if len(definition.RecordTypes) == 0 {
		t.Fatalf("Account record types = %#v", definition.RecordTypes)
	}
	field, ok := definition.Fields["RecordTypeId"]
	if !ok || field.Type != FieldReference || field.RelationshipName != "RecordType" {
		t.Fatalf("Account.RecordTypeId field = %#v, %v", field, ok)
	}
	if !hasRelationship(definition.Relations, "RecordTypeId", "RecordType", "RecordType") {
		t.Fatalf("Account.RecordTypeId relation missing: %#v", definition.Relations)
	}
}

func hasRelationship(relations []Relationship, fieldName, parentObject, parentRelationship string) bool {
	for _, relation := range relations {
		if relation.Field != fieldName || relation.ParentRelationship != parentRelationship {
			continue
		}
		for _, candidate := range relation.ParentObjects {
			if candidate == parentObject {
				return true
			}
		}
	}
	return false
}

func hasChildRelationship(relations []Relationship, fieldName, parentObject, parentRelationship, childRelationship string) bool {
	for _, relation := range relations {
		if relation.Field != fieldName || relation.ParentRelationship != parentRelationship || relation.ChildRelationship != childRelationship {
			continue
		}
		for _, candidate := range relation.ParentObjects {
			if candidate == parentObject {
				return true
			}
		}
	}
	return false
}

func hasPolymorphicRelationship(relations []Relationship, fieldName, parentObject, parentRelationship, childRelationship string) bool {
	for _, relation := range relations {
		if relation.Field != fieldName || relation.ParentRelationship != parentRelationship || relation.ChildRelationship != childRelationship || !relation.Polymorphic {
			continue
		}
		for _, candidate := range relation.ParentObjects {
			if candidate == parentObject {
				return true
			}
		}
	}
	return false
}

func containsTestString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestEnsureStandardObjectFieldsAddsCustomObjectNameAndRecordTypeId(t *testing.T) {
	definition := ObjectDefinition{APIName: "OrderItem__c"}

	EnsureStandardObjectFields(&definition)

	if field, ok := definition.Fields["Id"]; !ok || field.APIName != "Id" || field.Type != FieldID {
		t.Fatalf("Id field = %#v, %v", field, ok)
	}
	if field, ok := definition.Fields["Name"]; !ok || field.Type != FieldString {
		t.Fatalf("Name field = %#v, %v", field, ok)
	}
	if field, ok := definition.Fields["LastActivityDate"]; !ok || field.Type != FieldDate {
		t.Fatalf("LastActivityDate field = %#v, %v", field, ok)
	}
	field, ok := definition.Fields["RecordTypeId"]
	if !ok || field.Type != FieldReference || field.RelationshipName != "RecordType" {
		t.Fatalf("RecordTypeId field = %#v, %v", field, ok)
	}
	if len(definition.Relations) != 1 || definition.Relations[0].ParentRelationship != "RecordType" {
		t.Fatalf("relations = %#v", definition.Relations)
	}
}

func TestEnsureStandardObjectFieldsAddsCustomMetadataIdentityFields(t *testing.T) {
	definition := ObjectDefinition{APIName: "Feature__mdt"}

	EnsureStandardObjectFields(&definition)

	for _, name := range []string{"DeveloperName", "Label", "Language", "MasterLabel", "NamespacePrefix", "QualifiedApiName"} {
		field, ok := definition.Fields[name]
		if !ok || field.Type != FieldString {
			t.Fatalf("%s field = %#v, %v", name, field, ok)
		}
	}
	if _, ok := definition.Fields["RecordTypeId"]; ok {
		t.Fatalf("custom metadata should not get custom object RecordTypeId: %#v", definition.Fields["RecordTypeId"])
	}
}

func TestEnsureStandardObjectFieldsDerivesCustomMetadataRelationship(t *testing.T) {
	definition := ObjectDefinition{
		APIName: "Binding__mdt",
		Fields: map[string]Field{
			"Target__c": {APIName: "Target__c", Type: FieldReference, ReferenceTo: []string{"Target__mdt"}},
		},
	}

	EnsureStandardObjectFields(&definition)

	field := definition.Fields["Target__c"]
	if field.RelationshipName != "" {
		t.Fatalf("field relationship name should preserve source metadata: %#v", field)
	}
	if len(definition.Relations) != 1 {
		t.Fatalf("relations = %#v", definition.Relations)
	}
	relation := definition.Relations[0]
	if relation.Field != "Target__c" || relation.ParentRelationship != "Target__r" || len(relation.ParentObjects) != 1 || relation.ParentObjects[0] != "Target__mdt" {
		t.Fatalf("relation = %#v", relation)
	}
}

func TestApplyCustomMetadataRecordsMaterializesDeterministicRows(t *testing.T) {
	org := NewOrgState()
	org.Namespace = "pkg"
	targetDefinition := ObjectDefinition{
		APIName:   "Target__mdt",
		KeyPrefix: "a10",
		Metadata:  map[string]string{"kind": "customMetadata"},
		Fields: map[string]Field{
			"Description__c": {APIName: "Description__c", Type: FieldString},
		},
	}
	EnsureStandardObjectFields(&targetDefinition)
	featureDefinition := ObjectDefinition{
		APIName:   "Feature__mdt",
		KeyPrefix: "a11",
		Metadata:  map[string]string{"kind": "customMetadata"},
		Fields: map[string]Field{
			"Enabled__c": {APIName: "Enabled__c", Type: FieldBoolean},
			"Target__c":  {APIName: "Target__c", Type: FieldReference, ReferenceTo: []string{"Target__mdt"}},
		},
	}
	EnsureStandardObjectFields(&featureDefinition)
	org.Objects["Target__mdt"] = ObjectState{Definition: targetDefinition, Records: map[ID]Record{}}
	org.Objects["Feature__mdt"] = ObjectState{Definition: featureDefinition, Records: map[ID]Record{}}

	err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{
		{FullName: "Feature.Default", ObjectName: "Feature__mdt", DeveloperName: "Default", Label: "Default Label", Values: []schema.CustomMetadataValue{{Field: "Enabled__c", Value: "true"}, {Field: "Target__c", Value: "Target"}}},
		{FullName: "Target.Target", ObjectName: "Target__mdt", DeveloperName: "Target", Values: []schema.CustomMetadataValue{{Field: "Description__c", Value: "Target row"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	target := onlyRecord(t, org.Objects["Target__mdt"].Records)
	if target.Fields["QualifiedApiName"].String != "Target" || target.Fields["Description__c"].String != "Target row" {
		t.Fatalf("target record = %#v", target)
	}
	feature := onlyRecord(t, org.Objects["Feature__mdt"].Records)
	if feature.Fields["DeveloperName"].String != "Default" || feature.Fields["MasterLabel"].String != "Default Label" || !feature.Fields["Enabled__c"].Boolean {
		t.Fatalf("feature record = %#v", feature)
	}
	if feature.Fields["Target__c"].Kind != ValueID || feature.Fields["Target__c"].ID != target.ID {
		t.Fatalf("relationship value = %#v, target id %s", feature.Fields["Target__c"], target.ID)
	}
}

func TestApplyCustomMetadataRecordsKeepsEntityDefinitionRelationshipsByName(t *testing.T) {
	org := NewOrgState()
	definition := ObjectDefinition{
		APIName:   "TriggerStep__mdt",
		KeyPrefix: "a12",
		Metadata:  map[string]string{"kind": "customMetadata"},
		Fields: map[string]Field{
			"Object__c": {APIName: "Object__c", Type: FieldReference, ReferenceTo: []string{"EntityDefinition"}},
		},
	}
	EnsureStandardObjectFields(&definition)
	org.Objects["TriggerStep__mdt"] = ObjectState{Definition: definition, Records: map[ID]Record{}}

	err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{
		{FullName: "TriggerStep.MembershipUpdateAccountStep", ObjectName: "TriggerStep__mdt", DeveloperName: "MembershipUpdateAccountStep", Values: []schema.CustomMetadataValue{{Field: "Object__c", Value: "Membership__c"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	record := onlyRecord(t, org.Objects["TriggerStep__mdt"].Records)
	if value := record.Fields["Object__c"]; value.Kind != ValueString || value.String != "Membership__c" {
		t.Fatalf("entity relationship value = %#v", value)
	}
}

func onlyRecord(t *testing.T, records map[ID]Record) Record {
	t.Helper()
	if len(records) != 1 {
		t.Fatalf("records = %#v", records)
	}
	for _, record := range records {
		return record
	}
	return Record{}
}

func TestApplyCustomMetadataRecordsReportsPreciseUnsupportedMetadata(t *testing.T) {
	org := NewOrgState()
	definition := ObjectDefinition{
		APIName:   "Feature__mdt",
		KeyPrefix: "a11",
		Metadata:  map[string]string{"kind": "customMetadata"},
		Fields:    map[string]Field{},
	}
	EnsureStandardObjectFields(&definition)
	org.Objects["Feature__mdt"] = ObjectState{Definition: definition, Records: map[ID]Record{}}

	err := ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{{
		FullName:      "Feature.Default",
		ObjectName:    "Feature__mdt",
		DeveloperName: "Default",
		File:          "customMetadata/Feature.Default.md",
		Values:        []schema.CustomMetadataValue{{Field: "Missing__c", Value: "x"}},
	}})
	var unsupported UnsupportedMetadataError
	if !errors.As(err, &unsupported) || unsupported.File != "customMetadata/Feature.Default.md" || unsupported.Feature != "custom metadata field" {
		t.Fatalf("error = %#v", err)
	}
}

func TestEnsureStandardObjectAddsEmailTemplateRenderAndProbeFields(t *testing.T) {
	org := NewOrgState()

	EnsureStandardObject(&org, "EmailTemplate")

	template := org.Objects["EmailTemplate"]
	if template.Definition.KeyPrefix != "00X" {
		t.Fatalf("EmailTemplate key prefix = %q", template.Definition.KeyPrefix)
	}
	for _, name := range []string{"ApiVersion", "Body", "DeveloperName", "HtmlValue", "IsActive", "Name", "NamespacePrefix", "Subject", "TemplateStyle", "TemplateType"} {
		if field, ok := template.Definition.Fields[name]; !ok || field.APIName != name {
			t.Fatalf("%s field = %#v, %v", name, field, ok)
		}
	}
	if got := template.Definition.Fields["TemplateStyle"].DefaultValue; got != "none" {
		t.Fatalf("TemplateStyle default = %q", got)
	}
	if got := template.Definition.Fields["TemplateType"].DefaultValue; got != "text" {
		t.Fatalf("TemplateType default = %q", got)
	}
}

func TestEnsureStandardObjectFieldsAddsKnowledgeArticleLanguage(t *testing.T) {
	definition := ObjectDefinition{APIName: "FAQ__kav"}

	EnsureStandardObjectFields(&definition)

	field, ok := definition.Fields["Language"]
	if !ok || field.Type != FieldString {
		t.Fatalf("Language field = %#v, %v", field, ok)
	}
	if title, ok := definition.Fields["Title"]; !ok || title.Type != FieldString {
		t.Fatalf("Title field = %#v, %v", title, ok)
	}
}

func TestEnsureStandardObjectFieldsDoesNotAddDeveloperNameToCustomObjectsOrSettings(t *testing.T) {
	for _, definition := range []ObjectDefinition{
		{APIName: "Event_Log__c"},
		{APIName: "List_Setting__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "List"}},
	} {
		EnsureStandardObjectFields(&definition)
		if _, ok := definition.Fields["DeveloperName"]; ok {
			t.Fatalf("%s unexpectedly has DeveloperName", definition.APIName)
		}
	}
}

func TestCloneRecordDoesNotShareMutableFieldState(t *testing.T) {
	original := Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]Value{
			"Name":    StringValue("Acme"),
			"Tags__c": ListValue(StringValue("a"), StringValue("b")),
		},
		ExplicitNulls: map[string]bool{"Description": true},
	}

	clone := original.Clone()
	clone.Fields["Name"] = StringValue("Changed")
	clone.Fields["Tags__c"].List[0] = StringValue("changed")
	clone.ExplicitNulls["Description"] = false

	if original.Fields["Name"].String != "Acme" {
		t.Fatalf("original name changed: %#v", original.Fields["Name"])
	}
	if original.Fields["Tags__c"].List[0].String != "a" {
		t.Fatalf("original list changed: %#v", original.Fields["Tags__c"])
	}
	if !original.ExplicitNulls["Description"] {
		t.Fatalf("original explicit null changed: %#v", original.ExplicitNulls)
	}
}

func TestCloneOrgStateDoesNotShareRecordsOrIndexes(t *testing.T) {
	org := NewOrgState()
	org.IDSequences["Account"] = 1
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]Field{
				"Name": {APIName: "Name", Type: FieldString},
			},
			Indexes: []IndexDefinition{{Name: "Account.Name", Object: "Account", Fields: []string{"Name"}}},
		},
		Records: map[ID]Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]Value{"Name": StringValue("Acme")},
			},
		},
		Indexes: map[string]IndexSet{
			"Account.Name": {
				Definition: IndexDefinition{Name: "Account.Name", Object: "Account", Fields: []string{"Name"}},
				Entries:    map[string][]ID{"Acme": {"001000000000001"}},
			},
		},
	}

	clone := org.Clone()
	account := clone.Objects["Account"]
	account.Records["001000000000001"] = Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]Value{"Name": StringValue("Changed")},
	}
	account.Definition.Fields["Name"] = Field{APIName: "Name", Type: FieldBoolean}
	account.Definition.Indexes[0].Fields[0] = "OtherName"
	account.Indexes["Account.Name"].Entries["Acme"][0] = "001000000000002"
	clone.Objects["Account"] = account
	clone.IDSequences["Account"] = 2

	originalAccount := org.Objects["Account"]
	if org.IDSequences["Account"] != 1 {
		t.Fatalf("original sequence changed: %#v", org.IDSequences)
	}
	if originalAccount.Records["001000000000001"].Fields["Name"].String != "Acme" {
		t.Fatalf("original record changed: %#v", originalAccount.Records)
	}
	if originalAccount.Definition.Fields["Name"].Type != FieldString {
		t.Fatalf("original definition field changed: %#v", originalAccount.Definition.Fields)
	}
	if originalAccount.Definition.Indexes[0].Fields[0] != "Name" {
		t.Fatalf("original definition index changed: %#v", originalAccount.Definition.Indexes)
	}
	if originalAccount.Indexes["Account.Name"].Entries["Acme"][0] != "001000000000001" {
		t.Fatalf("original index changed: %#v", originalAccount.Indexes)
	}
}

func TestCloneTransactionFrameDoesNotShareMutationSnapshots(t *testing.T) {
	before := Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]Value{"Name": StringValue("Before")},
	}
	after := Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]Value{"Name": StringValue("After")},
	}
	frame := TransactionFrame{
		ID:    "tx-1",
		Depth: 1,
		Mutations: []Mutation{{
			Op:     MutationUpdate,
			Object: "Account",
			ID:     "001000000000001",
			Before: &before,
			After:  &after,
		}},
	}

	clone := frame.Clone()
	clone.Mutations[0].Before.Fields["Name"] = StringValue("Changed")
	clone.Mutations[0].After.Fields["Name"] = StringValue("Changed")

	if frame.Mutations[0].Before.Fields["Name"].String != "Before" {
		t.Fatalf("original before snapshot changed: %#v", frame.Mutations[0].Before)
	}
	if frame.Mutations[0].After.Fields["Name"].String != "After" {
		t.Fatalf("original after snapshot changed: %#v", frame.Mutations[0].After)
	}
}

func TestNormalizeRESTAPIVersion(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"65.0", "65.0"},
		{" v62.3 ", "62.3"},
		{"v59.5", "59.5"},
		{"", ""},
	}
	for _, tc := range cases {
		got := NormalizeRESTAPIVersion(tc.in)
		if got != tc.want {
			t.Fatalf("NormalizeRESTAPIVersion(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestEffectiveRESTAPIVersion(t *testing.T) {
	if got := EffectiveRESTAPIVersion(""); got != DefaultRESTAPIVersion {
		t.Fatalf("blank -> %q want %s", got, DefaultRESTAPIVersion)
	}
	if got := EffectiveRESTAPIVersion("   "); got != DefaultRESTAPIVersion {
		t.Fatalf("whitespace-only -> %q want %s", got, DefaultRESTAPIVersion)
	}
	if got := EffectiveRESTAPIVersion("v70.0"); got != "70.0" {
		t.Fatalf("explicit -> %q want 70.0", got)
	}
}
