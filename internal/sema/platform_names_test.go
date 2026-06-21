package sema

import "testing"

func TestSemaCanonicalPlatformAliasCoversDocumentedSystemNamespace(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"System.Blob", "Blob"},
		{"System.Boolean", "Boolean"},
		{"System.Database", "Database"},
		{"System.Date", "Date"},
		{"System.Datetime", "Datetime"},
		{"System.Decimal", "Decimal"},
		{"System.Exception", "Exception"},
		{"System.HttpRequest", "HttpRequest"},
		{"System.HttpResponse", "HttpResponse"},
		{"System.JSON", "JSON"},
		{"System.Limits", "Limits"},
		{"System.Math", "Math"},
		{"System.RestContext", "RestContext"},
		{"System.RestRequest", "RestRequest"},
		{"System.RestResponse", "RestResponse"},
		{"System.String", "String"},
		{"System.System", "System"},
		{"System.URL", "URL"},
		{"List<System.RestRequest>", "List<RestRequest>"},
		{"Map<String,System.HttpResponse>", "Map<String,HttpResponse>"},
	} {
		if got := semaCanonicalPlatformAlias(tc.in); got != tc.want {
			t.Fatalf("semaCanonicalPlatformAlias(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSemaCanonicalPlatformAliasCoversDocumentedSchemaImplicitImports(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"ChildRelationship", "Schema.ChildRelationship"},
		{"DataCategory", "Schema.DataCategory"},
		{"DataCategoryGroupSobjectTypePair", "Schema.DataCategoryGroupSobjectTypePair"},
		{"DescribeColorResult", "Schema.DescribeColorResult"},
		{"DescribeDataCategoryGroupResult", "Schema.DescribeDataCategoryGroupResult"},
		{"DescribeDataCategoryGroupStructureResult", "Schema.DescribeDataCategoryGroupStructureResult"},
		{"DescribeFieldResult", "Schema.DescribeFieldResult"},
		{"DescribeIconResult", "Schema.DescribeIconResult"},
		{"DescribeSObjectResult", "Schema.DescribeSObjectResult"},
		{"DisplayType", "Schema.DisplayType"},
		{"FieldDescribeOptions", "Schema.FieldDescribeOptions"},
		{"FieldSet", "Schema.FieldSet"},
		{"FieldSetMember", "Schema.FieldSetMember"},
		{"FilteredLookupInfo", "Schema.FilteredLookupInfo"},
		{"PicklistEntry", "Schema.PicklistEntry"},
		{"RecordTypeInfo", "Schema.RecordTypeInfo"},
		{"SObjectDescribeOptions", "Schema.SObjectDescribeOptions"},
		{"SObjectField", "Schema.SObjectField"},
		{"SObjectType", "Schema.SObjectType"},
		{"SObjectTypeFields", "Schema.SObjectTypeFields"},
		{"SObjectTypeFieldSets", "Schema.SObjectTypeFieldSets"},
		{"SoapType", "Schema.SoapType"},
		{"List<FieldDescribeOptions>", "List<Schema.FieldDescribeOptions>"},
	} {
		if got := semaCanonicalPlatformAlias(tc.in); got != tc.want {
			t.Fatalf("semaCanonicalPlatformAlias(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
