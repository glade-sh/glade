package soql

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// picklist_order_native.json K/U rows observe configured ordering, a single
// unlisted value, secondary keys and ordinary-text controls at API 62/67.
func TestPicklistOrderUsesDefinitionSequence(t *testing.T) {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{APIName: "WorkItem__c", Fields: map[string]storage.Field{
		"Id":   {APIName: "Id", Type: storage.FieldID},
		"Name": {APIName: "Name", Type: storage.FieldString},
		"Phase__c": {APIName: "Phase__c", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{
			{Value: "Zulu", Label: "Last alphabetically", Active: true},
			{Value: "Alpha", Label: "First alphabetically", Active: true},
			{Value: "Middle", Label: "Middle", Active: true},
		}},
		"Text__c":         {APIName: "Text__c", Type: storage.FieldString},
		"Unconfigured__c": {APIName: "Unconfigured__c", Type: storage.FieldPicklist},
		"Partial__c": {APIName: "Partial__c", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{
			{Value: "Zulu", Active: true}, {Value: "Alpha", Active: true},
		}},
		"Multiple__c": {APIName: "Multiple__c", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{
			{Value: "Zulu", Active: true},
		}},
		"Restricted__c": {APIName: "Restricted__c", Type: storage.FieldPicklist, RestrictedPicklist: true, PicklistValues: []storage.PicklistValue{
			{Value: "Zulu", Active: true}, {Value: "Alpha", Active: true},
		}},
	}}
	state := storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
	for i, value := range []string{"Alpha", "Zulu", "Middle", "Zulu", ""} {
		id := storage.ID([]string{"a30000000000001", "a30000000000002", "a30000000000003", "a30000000000004", "a30000000000005"}[i])
		name := []string{"a", "z1", "m", "z2", "null"}[i]
		fields := map[string]storage.Value{"Name": storage.StringValue(name)}
		if value != "" {
			fields["Phase__c"] = storage.StringValue(value)
			fields["Text__c"] = storage.StringValue(value)
			fields["Unconfigured__c"] = storage.StringValue(value)
			fields["Partial__c"] = storage.StringValue(value)
			fields["Multiple__c"] = storage.StringValue(value)
			fields["Restricted__c"] = storage.StringValue(value)
		}
		state.Records[id] = storage.Record{ID: id, Object: "WorkItem__c", Fields: fields}
	}
	org.Objects["WorkItem__c"] = state
	for _, tc := range []struct {
		name, order string
		want        []string
	}{
		{"ascending with nulls and secondary key", "Phase__c ASC NULLS FIRST, Name ASC", []string{"null", "z1", "z2", "a", "m"}},
		{"descending with nulls and secondary key", "Phase__c DESC NULLS LAST, Name DESC", []string{"m", "a", "z2", "z1", "null"}},
		{"text field control", "Text__c ASC NULLS LAST, Name ASC", []string{"a", "m", "z1", "z2", "null"}},
		{"single unlisted value follows configured values", "Partial__c ASC NULLS LAST, Name ASC", []string{"z1", "z2", "a", "m", "null"}},
		// Preserve existing behavior outside the captured unrestricted singleton
		// boundary. These are local controls, not native ordering claims.
		{"missing metadata boundary control", "Unconfigured__c ASC NULLS LAST, Name ASC", []string{"a", "m", "z1", "z2", "null"}},
		{"multiple unlisted values boundary control", "Multiple__c ASC NULLS LAST, Name ASC", []string{"a", "m", "z1", "z2", "null"}},
		{"restricted unlisted value boundary control", "Restricted__c ASC NULLS LAST, Name ASC", []string{"a", "m", "z1", "z2", "null"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseAndExecute(org, "SELECT Name FROM WorkItem__c ORDER BY "+tc.order)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, row := range result.Records {
				got = append(got, row.Fields["Name"].String)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ordered names=%v; want=%v", got, tc.want)
			}
		})
	}
	t.Run("child query configured order", func(t *testing.T) {
		const parentID storage.ID = "a31000000000001"
		state.Definition.Fields["Bucket__c"] = storage.Field{APIName: "Bucket__c", Type: storage.FieldReference, ReferenceTo: []string{"WorkBucket__c"}}
		state.Definition.Relations = []storage.Relationship{{Field: "Bucket__c", ParentObjects: []string{"WorkBucket__c"}, ParentRelationship: "Bucket__r", ChildRelationship: "Items__r"}}
		for _, record := range state.Records {
			record.Fields["Bucket__c"] = storage.IDValue(parentID)
		}
		org.Objects["WorkItem__c"] = state
		org.Objects["WorkBucket__c"] = storage.ObjectState{
			Definition: storage.ObjectDefinition{APIName: "WorkBucket__c", Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}}},
			Records:    map[storage.ID]storage.Record{parentID: {ID: parentID, Object: "WorkBucket__c"}},
		}
		result, err := ParseAndExecute(org, "SELECT Id, (SELECT Name FROM Items__r ORDER BY Phase__c ASC NULLS LAST, Name ASC) FROM WorkBucket__c")
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Records) != 1 {
			t.Fatalf("parent rows=%d", len(result.Records))
		}
		var got []string
		for _, row := range result.Records[0].Children["Items__r"] {
			got = append(got, row.Fields["Name"].String)
		}
		want := []string{"z1", "z2", "a", "m", "null"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ordered child names=%v; want=%v", got, want)
		}
	})
}

// restricted_picklist_null_order.tsv rows 2-13: anonymous Apex API 62/67,
// restricted picklist with configured Zulu, Alpha, Middle and one null.
// ROAD-451 restricted picklist native capture uses configured Zulu, Alpha,
// Middle and null values. Source 62 DESC with no NULLS clause places null last.
func TestRestrictedPicklistLegacyDescendingImplicitNulls(t *testing.T) {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{APIName: "Item__c", Fields: map[string]storage.Field{
		"Name":    {APIName: "Name", Type: storage.FieldString},
		"Rank__c": {APIName: "Rank__c", Type: storage.FieldPicklist, RestrictedPicklist: true, PicklistValues: []storage.PicklistValue{{Value: "Zulu"}, {Value: "Alpha"}, {Value: "Middle"}}},
	}}
	records := []storage.Record{
		{Fields: map[string]storage.Value{"Name": storage.StringValue("null")}},
		{Fields: map[string]storage.Value{"Name": storage.StringValue("z"), "Rank__c": storage.StringValue("Zulu")}},
		{Fields: map[string]storage.Value{"Name": storage.StringValue("a"), "Rank__c": storage.StringValue("Alpha")}},
		{Fields: map[string]storage.Value{"Name": storage.StringValue("m"), "Rank__c": storage.StringValue("Middle")}},
	}
	org.Objects["Item__c"] = storage.ObjectState{Definition: definition}
	for _, tc := range []struct {
		name         string
		desc, legacy bool
		nulls        string
		want         []string
	}{
		{"source62 asc default", false, true, "", []string{"null", "z", "a", "m"}},
		{"source62 desc default", true, true, "", []string{"m", "a", "z", "null"}},
		{"source62 asc first", false, true, "FIRST", []string{"null", "z", "a", "m"}},
		{"source62 asc last", false, true, "LAST", []string{"null", "z", "a", "m"}},
		{"source62 desc first", true, true, "FIRST", []string{"m", "a", "z", "null"}},
		{"source62 desc last", true, true, "LAST", []string{"m", "a", "z", "null"}},
		{"source67 asc default", false, false, "", []string{"null", "z", "a", "m"}},
		{"source67 desc default", true, false, "", []string{"null", "m", "a", "z"}},
		{"source67 asc first", false, false, "FIRST", []string{"null", "z", "a", "m"}},
		{"source67 asc last", false, false, "LAST", []string{"z", "a", "m", "null"}},
		{"source67 desc first", true, false, "FIRST", []string{"null", "m", "a", "z"}},
		{"source67 desc last", true, false, "LAST", []string{"m", "a", "z", "null"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := append([]storage.Record(nil), records...)
			sortRecordsByOrder(org, definition, rows, []OrderSpec{{Field: "Rank__c", Desc: tc.desc, Nulls: tc.nulls}}, tc.legacy)
			var got []string
			for _, record := range rows {
				got = append(got, record.Fields["Name"].String)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
