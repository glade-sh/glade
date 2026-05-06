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

func TestResolveFieldNameKeepsOtherNamespace(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account", Fields: map[string]Field{
		"other__Status__c": {APIName: "other__Status__c", Type: FieldString},
	}}

	resolved, ok := ResolveFieldName(definition, "pkg", "other__Status__c")
	if !ok || resolved != "other__Status__c" {
		t.Fatalf("ResolveFieldName(other__Status__c) = %q, %v", resolved, ok)
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
	if _, ok := definition.Fields["PersonMailingStreet"]; ok {
		t.Fatalf("PersonMailingStreet should be gated by PersonAccounts: %#v", definition.Fields["PersonMailingStreet"])
	}
}

func TestEnsureStandardObjectFieldsForFeaturesAddsPersonAccountShape(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account"}

	EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})

	if field, ok := definition.Fields["PersonMailingStreet"]; !ok || field.Type != FieldString {
		t.Fatalf("PersonMailingStreet field = %#v, %v", field, ok)
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

func TestEnsureStandardObjectAddsSalesCloudStandardObjectShape(t *testing.T) {
	org := NewOrgState()

	EnsureStandardObject(&org, "Lead")
	EnsureStandardObject(&org, "Opportunity")
	EnsureStandardObject(&org, "OrderItem")

	if org.Objects["Lead"].Definition.KeyPrefix != "00Q" {
		t.Fatalf("Lead key prefix = %q", org.Objects["Lead"].Definition.KeyPrefix)
	}
	if field, ok := org.Objects["Lead"].Definition.Fields["LastName"]; !ok || !field.Required {
		t.Fatalf("Lead.LastName field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["Opportunity"].Definition.Fields["StageName"]; !ok || !field.Required {
		t.Fatalf("Opportunity.StageName field = %#v, %v", field, ok)
	}
	if field, ok := org.Objects["OrderItem"].Definition.Fields["OrderId"]; !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Order" {
		t.Fatalf("OrderItem.OrderId field = %#v, %v", field, ok)
	}
}

func TestEnsureStandardObjectFieldsAddsCustomObjectRecordTypeId(t *testing.T) {
	definition := ObjectDefinition{APIName: "OrderItem__c"}

	EnsureStandardObjectFields(&definition)

	if field, ok := definition.Fields["Id"]; !ok || field.APIName != "Id" || field.Type != FieldID {
		t.Fatalf("Id field = %#v, %v", field, ok)
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
	for _, name := range []string{"ApiVersion", "Body", "DeveloperName", "HtmlValue", "IsActive", "Name", "NamespacePrefix", "Subject", "TemplateType"} {
		if field, ok := template.Definition.Fields[name]; !ok || field.APIName != name {
			t.Fatalf("%s field = %#v, %v", name, field, ok)
		}
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
