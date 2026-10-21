package vm

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These copied-payload boundaries exclude unobserved provenance and preserve
// unrelated readonly fields. They do not assert native behavior for excluded
// query, clone, null, malformed or non-Account assignment/update inputs.
func TestCreatedDateJSONAssignmentPayloadBoundaries(t *testing.T) {
	for _, cell := range []struct {
		name, op, object string
		modify           func(*Value, *storage.Record)
		filter           bool
	}{
		{name: "insertWritable", op: "insert", filter: true},
		{name: "updateAfterInsert", op: "update", filter: true},
		{name: "updateFreshJSON", op: "update", filter: true, modify: func(row *Value, _ *storage.Record) {
			delete(row.Fields, sobjectQueriedFieldsField)
			delete(row.Fields, sobjectDMLAccessibleField)
			delete(row.Fields, sobjectUserSetFieldsField)
		}},
		{name: "contactAssignment", op: "insert", object: "Contact"},
		{name: "customAssignment", op: "insert", object: "Owned__c"},
		{name: "contactUpdate", op: "update", object: "Contact"},
		{name: "cloneWithoutId", op: "insert", modify: func(row *Value, _ *storage.Record) { row.Fields[sobjectCloneMarkerField] = Bool(false) }},
		{name: "ordinaryQuery", op: "update", modify: func(row *Value, _ *storage.Record) {
			marker := row.Fields[sobjectExplicitFieldsField]
			marker.Runtime = ""
			row.Fields[sobjectExplicitFieldsField] = marker
		}},
		{name: "queryWithoutDMLAccess", op: "update", modify: func(row *Value, _ *storage.Record) { row.Fields[sobjectDMLAccessibleField] = Bool(false) }},
		{name: "queriedInsert", op: "insert", modify: func(row *Value, _ *storage.Record) {
			row.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(row.Type, nil)
			row.Fields[sobjectDMLAccessibleField] = Bool(true)
		}},
		{name: "emptyAssignments", op: "insert", modify: func(row *Value, _ *storage.Record) { row.Fields[sobjectUserSetFieldsField] = Map() }},
		{name: "falseAssignment", op: "insert", modify: func(row *Value, _ *storage.Record) {
			row.Fields[sobjectUserSetFieldsField].Map[mapKey(String("name"))] = Bool(false)
		}},
		{name: "missingAssignmentKey", op: "insert", modify: func(row *Value, _ *storage.Record) {
			delete(row.Fields[sobjectUserSetFieldsField].MapKeys, mapKey(String("name")))
		}},
		{name: "manualCreatedDate", op: "update", modify: func(row *Value, _ *storage.Record) { markUserSetSObjectField(row, "CreatedDate") }},
		{name: "manualId", op: "insert", modify: func(row *Value, _ *storage.Record) { markUserSetSObjectField(row, "Id") }},
		{name: "otherReadonlyAssignment", op: "insert", modify: func(row *Value, _ *storage.Record) { markUserSetSObjectField(row, "LastModifiedDate") }},
		{name: "nullAssignment", op: "insert", modify: func(row *Value, record *storage.Record) {
			row.Fields["CreatedDate"] = typedNull("Datetime")
			delete(record.Fields, "CreatedDate")
			record.ExplicitNulls["CreatedDate"] = true
		}},
		{name: "nullUpdate", op: "update", modify: func(row *Value, record *storage.Record) {
			row.Fields["CreatedDate"] = typedNull("Datetime")
			delete(record.Fields, "CreatedDate")
			record.ExplicitNulls["CreatedDate"] = true
		}},
		{name: "malformedDatetime", op: "update", modify: func(row *Value, _ *storage.Record) { row.Fields["CreatedDate"] = platformScalar("Datetime", "invalid") }},
		{name: "copiedValueMismatch", op: "update", modify: func(_ *Value, record *storage.Record) {
			record.Fields["CreatedDate"] = storage.DateTimeValue("2002-03-04T05:06:07.000Z")
		}},
		{name: "emptyQueryMarker", op: "update", modify: func(row *Value, _ *storage.Record) { row.Fields[sobjectQueriedFieldsField] = Map() }},
		{name: "wrongQueryObject", op: "update", modify: func(row *Value, _ *storage.Record) {
			row.Fields[sobjectQueriedFieldsField].Map[mapKey(String("object"))] = String("Contact")
		}},
		{name: "malformedQueryFlag", op: "update", modify: func(row *Value, _ *storage.Record) {
			row.Fields[sobjectQueriedFieldsField].Map[mapKey(String("name"))] = String("true")
		}},
		{name: "upsertUnchanged", op: "upsert"},
		{name: "legacyContactNullInsert", op: "insert", object: "Contact", filter: true, modify: func(row *Value, record *storage.Record) {
			delete(row.Fields, sobjectUserSetFieldsField)
			row.Fields["CreatedDate"] = typedNull("Datetime")
			delete(record.Fields, "CreatedDate")
			record.ExplicitNulls["CreatedDate"] = true
		}},
	} {
		t.Run(cell.name, func(t *testing.T) {
			org := testDataOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["CreatedDate"] = storage.Field{APIName: "CreatedDate", Type: storage.FieldDateTime, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
			account.Definition.Fields["LastModifiedDate"] = storage.Field{APIName: "LastModifiedDate", Type: storage.FieldDateTime, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
			account.Definition.Fields["Id"] = storage.Field{APIName: "Id", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
			org.Objects["Account"] = account
			object := cell.object
			if object == "" {
				object = "Account"
			}
			if object != "Account" {
				other := account
				other.Definition.APIName = object
				org.Objects[object] = other
			}
			machine := New(nil)
			machine.SetOrg(&org)
			row := Object(object)
			row.Fields["Name"] = String("Owned assignment boundary")
			row.Fields["CreatedDate"] = platformScalar("Datetime", "2001-02-03T04:05:06.000Z")
			row.Fields["LastModifiedDate"] = platformScalar("Datetime", "2002-03-04T05:06:07.000Z")
			markExplicitSObjectField(&row, "CreatedDate")
			markExplicitSObjectField(&row, "LastModifiedDate")
			machine.markJSONDeserializedSObjectFields(&row, row.Type)
			markUserSetSObjectField(&row, "Name")
			if cell.op == "update" {
				row.Fields["Id"] = platformScalar("Id", "001000000000001AAA")
				markExplicitSObjectField(&row, "Id")
				row.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(object, map[string]bool{"name": true})
				row.Fields[sobjectDMLAccessibleField] = Bool(true)
			}
			records, _, err := machine.recordsFromValue(row)
			if err != nil {
				t.Fatal(err)
			}
			if cell.modify != nil {
				cell.modify(&row, &records[0])
			}
			before := records[0].Clone()
			callerTimestamp := row.Fields["CreatedDate"]
			machine.filterJSONCreationTimestamp(cell.op, records, []*Value{&row}, "")
			if cell.filter {
				delete(before.Fields, "CreatedDate")
				delete(before.ExplicitNulls, "CreatedDate")
			}
			if !reflect.DeepEqual(before, records[0]) {
				t.Fatalf("copied payload outside permitted CreatedDate change: want=%#v got=%#v", before, records[0])
			}
			if !reflect.DeepEqual(callerTimestamp, row.Fields["CreatedDate"]) {
				t.Fatal("changed caller CreatedDate")
			}
		})
	}
}
