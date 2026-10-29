package sobject

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

// Native membership: standard_describe_profiles/P11/org.tsv physical lines
// 2577,2586,2602,12066,12075,12091; standard_recordtype_field_policy/org.tsv
// lines 2-4,9-11,16-25; capture notes are each profile's ORG-RUN.txt. Explicit
// metadata must survive the same conversion chain used by runtime/LWC hosts.
func TestStandardFieldPolicyProvenanceDescribeConversionRoutes(t *testing.T) {
	owned := schema.Field{Name: "OwnedUnrelated__c", Type: "Text", Label: "Owned unrelated field"}
	for _, tc := range []struct {
		name   string
		object schema.Object
		want   map[string]bool
	}{
		{
			name: "configured-case",
			object: schema.Object{Name: "Case", Fields: []schema.Field{owned,
				{Name: "HasCommentsUnreadByOwner", Type: "Checkbox"},
				{Name: "IsVisibleInSelfService", Type: "Checkbox"},
				{Name: "SourceId", Type: "Lookup", ReferenceTo: []string{"EmailMessage"}, RelationshipName: "Source"},
			}},
			want: map[string]bool{"HasCommentsUnreadByOwner": true, "IsVisibleInSelfService": true, "SourceId": true, "OwnedUnrelated__c": true},
		},
		{
			name:   "bare-case-with-unrelated-field",
			object: schema.Object{Name: "Case", Fields: []schema.Field{owned}},
			want:   map[string]bool{"HasCommentsUnreadByOwner": false, "IsVisibleInSelfService": false, "SourceId": false, "OwnedUnrelated__c": true},
		},
		{
			name: "configured-account-no-record-types",
			object: schema.Object{Name: "Account", Fields: []schema.Field{owned,
				{Name: "RecordTypeId", Type: "Lookup", ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
			}},
			want: map[string]bool{"RecordTypeId": true, "OwnedUnrelated__c": true},
		},
		{
			name:   "baseline-account-with-unrelated-field",
			object: schema.Object{Name: "Account", Fields: []schema.Field{owned}},
			want:   map[string]bool{"RecordTypeId": false, "OwnedUnrelated__c": true},
		},
		{
			name: "baseline-account-with-only-master-record-type",
			object: schema.Object{Name: "Account", Fields: []schema.Field{owned},
				RecordTypes: []schema.RecordType{{DeveloperName: "Master", Label: "Master", Active: true}}},
			want: map[string]bool{"RecordTypeId": false, "OwnedUnrelated__c": true},
		},
		{
			name: "account-with-native-non-master-record-type",
			object: schema.Object{Name: "Account", Fields: []schema.Field{owned},
				RecordTypes: []schema.RecordType{{DeveloperName: "OwnedPolicy", Label: "Owned policy", Active: true}}},
			want: map[string]bool{"RecordTypeId": true, "OwnedUnrelated__c": true},
		},
		{
			name: "custom-object-record-type-control",
			object: schema.Object{Name: "Policy__c", Fields: []schema.Field{owned},
				RecordTypes: []schema.RecordType{{DeveloperName: "OwnedPolicy", Label: "Owned policy", Active: true}}},
			want: map[string]bool{"RecordTypeId": true, "OwnedUnrelated__c": true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{tc.object}})
			describe, err := registry.Describe(tc.object.Name)
			if err != nil {
				t.Fatal(err)
			}
			checkDescribe := func(t *testing.T, value DescribeSObjectResult) {
				t.Helper()
				for field, want := range tc.want {
					if _, exists := value.Fields[field]; exists != want {
						t.Errorf("%s.%s membership=%t, want %t", tc.object.Name, field, exists, want)
					}
				}
			}
			t.Run("registry-output", func(t *testing.T) { checkDescribe(t, describe) })
			definition := ToObjectDefinition(describe)
			t.Run("storage-conversion", func(t *testing.T) {
				for field, want := range tc.want {
					if _, exists := storage.ResolveFieldName(definition, "", field); exists != want {
						t.Errorf("%s.%s membership=%t, want %t", tc.object.Name, field, exists, want)
					}
				}
			})
			raw, err := json.Marshal(definition)
			if err != nil {
				t.Fatal(err)
			}
			var restored storage.ObjectDefinition
			if err := json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			delete(restored.Fields, "Id")
			storage.EnsureStandardObjectFields(&restored)
			t.Run("json-refresh-describe-conversion", func(t *testing.T) {
				checkDescribe(t, FromObjectDefinition(restored))
			})
		})
	}
}
