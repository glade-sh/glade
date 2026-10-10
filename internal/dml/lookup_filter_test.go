package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"strings"
	"testing"
)

func TestLookupFilterCriteriaAdmission(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		field, operation, logic      string
		inactive, optional, wantPass bool
		errorText                    string
	}{
		{name: "unknownField", field: "Account.MissingField", operation: "equals", errorText: "unknown lookup filter field"},
		{name: "unknownOperation", field: "Account.Name", operation: "unrecognized", errorText: "unsupported lookup filter operation"},
		{name: "invalidLogic", field: "Account.Name", operation: "equals", logic: "2", errorText: "unsupported lookup filter logic"},
		{name: "inactive", field: "Account.MissingField", operation: "equals", inactive: true, wantPass: true},
		{name: "optional", field: "Account.MissingField", operation: "equals", optional: true, wantPass: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			storage.EnsureStandardObject(&org, "Contact")
			object := org.Objects["Contact"]
			field := object.Definition.Fields["AccountId"]
			field.FilteredLookupInfo = storage.FilteredLookupInfo{Active: !tc.inactive, OptionalFilter: tc.optional, BooleanFilter: tc.logic, FilterItems: []storage.LookupFilterItem{{Field: tc.field, Operation: tc.operation, Value: "parent"}}}
			object.Definition.Fields["AccountId"] = field
			org.Objects["Contact"] = object
			engine := NewEngine(&org)
			parent := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("parent")}}})
			if !parent[0].Success {
				t.Fatal(parent)
			}
			result := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("owned"), "AccountId": storage.IDValue(parent[0].ID)}}})
			if result[0].Success != tc.wantPass {
				t.Fatalf("criteria admission = %#v; want success %v", result, tc.wantPass)
			}
			if !tc.wantPass && (len(result[0].Errors) == 0 || !strings.Contains(result[0].Errors[0].Message, tc.errorText)) {
				t.Fatalf("missing explicit criteria error: %#v", result)
			}
		})
	}
}

func TestLookupFilterRecordTypeLiteralUsesCriterionObject(t *testing.T) {
	for _, path := range []string{"$Source.RecordTypeId", "Account.RecordTypeId"} {
		for _, matching := range []bool{true, false} {
			t.Run(path+"/"+map[bool]string{true: "matching", false: "different"}[matching], func(t *testing.T) {
				org := testOrg()
				storage.EnsureStandardObject(&org, "Contact")
				for _, name := range []string{"Account", "Contact"} {
					object := org.Objects[name]
					object.Definition.RecordTypes = []storage.RecordTypeInfo{
						{DeveloperName: "Allowed", Name: "Allowed Display Label", Active: true, Available: true},
						{DeveloperName: "Other", Name: "Other Display Label", Active: true, Available: true},
					}
					storage.EnsureRecordTypeIDField(&object.Definition)
					org.Objects[name] = object
				}
				storage.EnsureDeterministicPlatformData(&org)
				contact := org.Objects["Contact"]
				field := contact.Definition.Fields["AccountId"]
				field.FilteredLookupInfo = storage.FilteredLookupInfo{Active: true, FilterItems: []storage.LookupFilterItem{{Field: path, Operation: "equals", Value: "Allowed"}}}
				contact.Definition.Fields["AccountId"] = field
				org.Objects["Contact"] = contact
				index := 0
				if !matching {
					index = 1
				}
				engine := NewEngine(&org)
				parent := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{
					"Name": storage.StringValue("target"), "RecordTypeId": storage.IDValue(org.Objects["Account"].Definition.RecordTypes[index].ID),
				}}})
				if !parent[0].Success {
					t.Fatal(parent)
				}
				result := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{
					"LastName": storage.StringValue("source"), "AccountId": storage.IDValue(parent[0].ID),
					"RecordTypeId": storage.IDValue(contact.Definition.RecordTypes[index].ID),
				}}})
				if result[0].Success != matching {
					t.Fatalf("insert = %#v; want success %v", result, matching)
				}
				if !matching && (len(result[0].Errors) != 1 || result[0].Errors[0].StatusCode != "FIELD_FILTER_VALIDATION_EXCEPTION") {
					t.Fatalf("unexpected rejection: %#v", result)
				}
				wantRecords := 0
				if matching {
					wantRecords = 1
				}
				if got := len(org.Objects["Contact"].Records); got != wantRecords {
					t.Fatalf("contact records = %d; want %d", got, wantRecords)
				}
			})
		}
	}
}

func TestLookupFilterRecordTypeDeveloperNameWinsLabelCollision(t *testing.T) {
	org := testOrg()
	storage.EnsureStandardObject(&org, "Contact")
	for _, name := range []string{"Account", "Contact"} {
		object := org.Objects[name]
		object.Definition.RecordTypes = []storage.RecordTypeInfo{
			{DeveloperName: "LabelAlias", Name: "Allowed", Active: true, Available: true},
			{DeveloperName: "Allowed", Name: "Other", Active: true, Available: true},
		}
		storage.EnsureRecordTypeIDField(&object.Definition)
		org.Objects[name] = object
	}
	storage.EnsureDeterministicPlatformData(&org)
	contact := org.Objects["Contact"]
	field := contact.Definition.Fields["AccountId"]
	field.FilteredLookupInfo = storage.FilteredLookupInfo{
		Active:      true,
		FilterItems: []storage.LookupFilterItem{{Field: "Account.RecordTypeId", Operation: "equals", Value: "Allowed"}},
	}
	contact.Definition.Fields["AccountId"] = field
	org.Objects["Contact"] = contact
	engine := NewEngine(&org)
	accountRecordTypeID := org.Objects["Account"].Definition.RecordTypes[1].ID
	parent := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{
		"Name":         storage.StringValue("target"),
		"RecordTypeId": storage.IDValue(accountRecordTypeID),
	}}})
	if !parent[0].Success {
		t.Fatal(parent)
	}
	result := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{
		"LastName":  storage.StringValue("source"),
		"AccountId": storage.IDValue(parent[0].ID),
	}}})
	if !result[0].Success {
		t.Fatalf("insert = %#v; developer-name collision selected the wrong record type", result)
	}
}

func TestLookupFilterBooleanLogic(t *testing.T) {
	for _, tc := range []struct {
		expression  string
		values      []bool
		want, valid bool
	}{
		{"(1 AND 2) OR 3", []bool{true, true, false}, true, true},
		{"(1 AND 2) OR 3", []bool{false, true, false}, false, true},
		{"(1 AND 2) OR 3", []bool{false, false, true}, true, true},
		{"", []bool{true, false}, false, true},
		{"", []bool{true, true}, true, true},
		{"1 OR 4", []bool{true}, false, false},
		{"1 AND", []bool{true}, false, false},
		{"0", []bool{true}, false, false},
		{"1 OR TRUE", []bool{false}, false, false},
	} {
		got, valid := evaluateLookupFilterLogic(tc.expression, tc.values)
		if got != tc.want || valid != tc.valid {
			t.Fatalf("logic %q on %v = %v,%v; want %v,%v", tc.expression, tc.values, got, valid, tc.want, tc.valid)
		}
	}
}

func TestLookupFilterSparseUpdateUsesRetainedFields(t *testing.T) {
	org := testOrg()
	storage.EnsureStandardObject(&org, "Contact")
	object := org.Objects["Contact"]
	field := object.Definition.Fields["AccountId"]
	field.FilteredLookupInfo = storage.FilteredLookupInfo{Active: true, FilterItems: []storage.LookupFilterItem{{Field: "Account.Name", Operation: "equals", Value: "allowed"}}}
	object.Definition.Fields["AccountId"] = field
	org.Objects["Contact"] = object
	engine := NewEngine(&org)
	parent := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("allowed")}}})
	if !parent[0].Success {
		t.Fatal(parent)
	}
	child := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("original"), "AccountId": storage.IDValue(parent[0].ID)}}})
	if !child[0].Success {
		t.Fatal(child)
	}
	changed := engine.Update([]storage.Record{{Object: "Account", ID: parent[0].ID, Fields: map[string]storage.Value{"Name": storage.StringValue("inactive")}}})
	if !changed[0].Success {
		t.Fatal(changed)
	}
	result := engine.Update([]storage.Record{{Object: "Contact", ID: child[0].ID, Fields: map[string]storage.Value{"LastName": storage.StringValue("attempted")}}})
	if result[0].Success {
		t.Fatal("sparse update bypassed lookup filter")
	}
	if got := org.Objects["Contact"].Records[child[0].ID].Fields["LastName"].String; got != "original" {
		t.Fatalf("failed sparse update persisted %q", got)
	}
}
