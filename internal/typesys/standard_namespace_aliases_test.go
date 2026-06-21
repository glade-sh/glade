package typesys

import (
	"strings"
	"testing"
)

func TestStandardSystemNamespaceTypeNamesIncludeGeneratedSystemDocs(t *testing.T) {
	names := StandardSystemNamespaceTypeNames()
	required := []string{
		"Blob",
		"Boolean",
		"Database",
		"Date",
		"Datetime",
		"Decimal",
		"Exception",
		"HttpRequest",
		"HttpResponse",
		"JSON",
		"Limits",
		"Math",
		"RestContext",
		"RestRequest",
		"RestResponse",
		"String",
		"System",
		"URL",
	}
	for _, name := range required {
		if !containsStringFold(names, name) {
			t.Fatalf("StandardSystemNamespaceTypeNames missing %q in %v", name, names)
		}
	}
	if len(names) < 223 {
		t.Fatalf("StandardSystemNamespaceTypeNames count = %d, want at least 223", len(names))
	}
}

func TestStandardSchemaNamespaceTypeNamesIncludeGeneratedSchemaDocs(t *testing.T) {
	names := StandardSchemaNamespaceTypeNames()
	required := []string{
		"ChildRelationship",
		"DataCategory",
		"DataCategoryGroupSobjectTypePair",
		"DescribeColorResult",
		"DescribeDataCategoryGroupResult",
		"DescribeDataCategoryGroupStructureResult",
		"DescribeFieldResult",
		"DescribeIconResult",
		"DescribeSObjectResult",
		"DisplayType",
		"FieldDescribeOptions",
		"FieldSet",
		"FieldSetMember",
		"FilteredLookupInfo",
		"PicklistEntry",
		"RecordTypeInfo",
		"SObjectDescribeOptions",
		"SObjectField",
		"SObjectType",
		"SObjectTypeFields",
		"SObjectTypeFieldSets",
		"SoapType",
	}
	for _, name := range required {
		if !containsStringFold(names, name) {
			t.Fatalf("StandardSchemaNamespaceTypeNames missing %q in %v", name, names)
		}
	}
	if len(names) < len(required) {
		t.Fatalf("StandardSchemaNamespaceTypeNames count = %d, want at least %d", len(names), len(required))
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
