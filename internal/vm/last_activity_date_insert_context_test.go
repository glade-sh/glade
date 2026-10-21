package vm

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Excluded inputs are local non-interference controls, not new native claims
// for clone, query, null JSON, reassignment, or other objects.
func TestLastActivityDateInsertContext(t *testing.T) {
	for _, cell := range []struct {
		name    string
		modify  func(*VM, *Value)
		payload func([]storage.Record, []*Value)
		filter  bool
	}{
		{name: "jsonDate", filter: true},
		{name: "ordinary", modify: func(_ *VM, row *Value) { delete(row.Fields, sobjectExplicitFieldsField) }},
		{name: "explicitWithoutJSON", modify: func(_ *VM, row *Value) {
			marker := row.Fields[sobjectExplicitFieldsField]
			marker.Runtime = ""
			row.Fields[sobjectExplicitFieldsField] = marker
		}},
		{name: "malformedMarker", modify: func(_ *VM, row *Value) { row.Fields[sobjectExplicitFieldsField] = Bool(true) }},
		{name: "notExplicit", modify: func(_ *VM, row *Value) { unmarkSObjectFieldMarker(row, sobjectExplicitFieldsField, "LastActivityDate") }},
		{name: "userAssignment", modify: func(_ *VM, row *Value) { row.Fields[sobjectUserSetFieldsField] = Map() }},
		{name: "queried", modify: func(_ *VM, row *Value) { row.Fields[sobjectQueriedFieldsField] = Map() }},
		{name: "cloneWithID", modify: func(_ *VM, row *Value) { row.Fields[sobjectCloneMarkerField] = Bool(true) }},
		{name: "cloneWithoutID", modify: func(_ *VM, row *Value) { row.Fields[sobjectCloneMarkerField] = Bool(false) }},
		{name: "classInstance", modify: func(_ *VM, row *Value) { row.classInstance = true }},
		{name: "nullDate", modify: func(_ *VM, row *Value) { row.Fields["LastActivityDate"] = typedNull("Date") }},
		{name: "stringDate", modify: func(_ *VM, row *Value) { row.Fields["LastActivityDate"] = String("2007-02-03") }},
		{name: "datetime", payload: func(_ []storage.Record, targets []*Value) {
			// Record conversion rejects Datetime for DATE before this helper.
			// Change the target after copying to exercise the helper's type guard.
			targets[0].Fields["LastActivityDate"] = platformScalar("Datetime", "2007-02-03T00:00:00.000Z")
		}},
		{name: "malformedDate", modify: func(_ *VM, row *Value) { row.Fields["LastActivityDate"] = platformScalar("Date", "invalid") }},
		{name: "noncanonicalDate", modify: func(_ *VM, row *Value) { row.Fields["LastActivityDate"] = platformScalar("Date", "2007-2-3") }},
		{name: "otherObject", modify: func(machine *VM, row *Value) {
			other := machine.Org.Objects["Account"]
			other.Definition.APIName = "Contact"
			machine.Org.Objects["Contact"] = other
			row.Type = "Contact"
		}},
		{name: "copiedDateMismatch", payload: func(records []storage.Record, _ []*Value) {
			records[0].Fields["LastActivityDate"] = storage.DateValue("2008-03-04")
		}},
		{name: "copiedTypeMismatch", payload: func(records []storage.Record, _ []*Value) {
			records[0].Fields["LastActivityDate"] = storage.StringValue("2007-02-03")
		}},
		{name: "copiedObjectMismatch", payload: func(records []storage.Record, _ []*Value) { records[0].Object = "Contact" }},
		{name: "contradictoryNull", payload: func(records []storage.Record, _ []*Value) { records[0].ExplicitNulls["LastActivityDate"] = true }},
		{name: "nilTarget", payload: func(_ []storage.Record, targets []*Value) { targets[0] = nil }},
	} {
		t.Run(cell.name, func(t *testing.T) {
			org := testDataOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["LastActivityDate"] = storage.Field{APIName: "LastActivityDate", Type: storage.FieldDate, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false), Permissionable: storage.BoolFlag(false)}
			machine := New(nil)
			machine.SetOrg(&org)
			row := Object("Account")
			row.Fields["Name"] = String("Owned provenance control")
			row.Fields["LastActivityDate"] = platformScalar("Date", "2007-02-03")
			row.Fields["LastModifiedDate"] = platformScalar("Datetime", "2001-02-03T04:05:06.000Z")
			markExplicitSObjectField(&row, "LastActivityDate")
			markExplicitSObjectField(&row, "LastModifiedDate")
			machine.markJSONDeserializedSObjectFields(&row, row.Type)
			if cell.modify != nil {
				cell.modify(machine, &row)
				machine.SetOrg(&org)
			}
			records, targets, err := machine.recordsFromValue(row)
			if err != nil {
				t.Fatal(err)
			}
			if cell.payload != nil {
				cell.payload(records, targets)
			}
			before := records[0].Clone()
			caller := cloneValuePreserveRefs(row)
			machine.filterJSONInsertLastActivityDate(records, targets)
			if cell.filter {
				delete(before.Fields, "LastActivityDate")
			}
			if !reflect.DeepEqual(before, records[0]) {
				t.Fatalf("copied payload changed outside the eligible date: want=%#v got=%#v", before, records[0])
			}
			if !reflect.DeepEqual(caller, row) || targets[0] != nil && !reflect.DeepEqual(caller, *targets[0]) {
				t.Fatal("caller fields or provenance changed")
			}
		})
	}
}

func TestLastActivityDateInsertMetadataAndEngineBoundary(t *testing.T) {
	for _, cell := range []struct {
		name   string
		modify func(*storage.Field)
	}{
		{name: "createable", modify: func(f *storage.Field) { f.Createable = storage.BoolFlag(true) }},
		{name: "updateable", modify: func(f *storage.Field) { f.Updateable = storage.BoolFlag(true) }},
		{name: "permissionable", modify: func(f *storage.Field) { f.Permissionable = storage.BoolFlag(true) }},
		{name: "unknownCreateable", modify: func(f *storage.Field) { f.Createable = nil }},
		{name: "unknownUpdateable", modify: func(f *storage.Field) { f.Updateable = nil }},
		{name: "unknownPermissionable", modify: func(f *storage.Field) { f.Permissionable = nil }},
		{name: "nonDate", modify: func(f *storage.Field) { f.Type = storage.FieldString }},
	} {
		t.Run(cell.name, func(t *testing.T) {
			org := testDataOrg()
			field := storage.Field{APIName: "LastActivityDate", Type: storage.FieldDate, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false), Permissionable: storage.BoolFlag(false)}
			org.Objects["Account"].Definition.Fields["LastActivityDate"] = field
			machine := New(nil)
			machine.SetOrg(&org)
			row := Object("Account")
			row.Fields["LastActivityDate"] = platformScalar("Date", "2007-02-03")
			markExplicitSObjectField(&row, "LastActivityDate")
			machine.markJSONDeserializedSObjectFields(&row, row.Type)
			records, targets, err := machine.recordsFromValue(row)
			if err != nil {
				t.Fatal(err)
			}
			// Standard preparation fills unknown facets from the catalog.
			// Inject excluded metadata afterward to exercise the helper's guard.
			prepared := machine.describeDefCache["account"]
			field = prepared.Fields["LastActivityDate"]
			cell.modify(&field)
			prepared.Fields["LastActivityDate"] = field
			machine.describeDefCache["account"] = prepared
			before := records[0].Clone()
			machine.filterJSONInsertLastActivityDate(records, targets)
			if !reflect.DeepEqual(before, records[0]) {
				t.Fatal("filter changed a payload with excluded metadata")
			}
		})
	}
	org := testDataOrg()
	org.Objects["Account"].Definition.Fields["LastActivityDate"] = storage.Field{APIName: "LastActivityDate", Type: storage.FieldDate, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false), Permissionable: storage.BoolFlag(false)}
	engine := dml.NewEngine(&org)
	results := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Owned Engine control"), "LastActivityDate": storage.DateValue("2007-02-03")}}})
	if len(results) != 1 || results[0].Success || results[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" || !reflect.DeepEqual(results[0].Fields, []string{"LastActivityDate"}) {
		t.Fatalf("Engine writeability changed: %#v", results)
	}
}
