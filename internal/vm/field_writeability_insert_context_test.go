package vm

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These local boundary controls preserve excluded provenance. They do not
// claim native parity for clone, query, or assignment handling elsewhere.
// LastActivityDate is a known adjacent mismatch: native JSON inserts ignore
// it, but this CreatedDate-only change deliberately leaves that payload intact.
func TestFieldWriteabilityRecoveryInsertContext(t *testing.T) {
	for _, cell := range []struct {
		name   string
		modify func(*VM, *Value)
		filter bool
	}{
		{name: "json", filter: true},
		{name: "jsonNull", filter: true, modify: func(_ *VM, row *Value) { row.Fields["CreatedDate"] = typedNull("Datetime") }},
		{name: "ordinary", modify: func(_ *VM, row *Value) { delete(row.Fields, sobjectExplicitFieldsField) }},
		{name: "explicitWithoutJSON", modify: func(_ *VM, row *Value) {
			marker := row.Fields[sobjectExplicitFieldsField]
			marker.Runtime = ""
			row.Fields[sobjectExplicitFieldsField] = marker
		}},
		{name: "jsonWithoutExplicitTimestamp", modify: func(_ *VM, row *Value) { unmarkSObjectFieldMarker(row, sobjectExplicitFieldsField, "CreatedDate") }},
		{name: "userSetMarker", modify: func(_ *VM, row *Value) { row.Fields[sobjectUserSetFieldsField] = Map() }},
		{name: "queriedMarker", modify: func(_ *VM, row *Value) { row.Fields[sobjectQueriedFieldsField] = Map() }},
		{name: "cloneWithId", modify: func(_ *VM, row *Value) { row.Fields[sobjectCloneMarkerField] = Bool(true) }},
		{name: "cloneWithoutId", modify: func(_ *VM, row *Value) { row.Fields[sobjectCloneMarkerField] = Bool(false) }},
		{name: "classInstance", modify: func(_ *VM, row *Value) { row.classInstance = true }},
		{name: "writableMetadata", modify: func(machine *VM, _ *Value) {
			account := machine.Org.Objects["Account"]
			field := account.Definition.Fields["CreatedDate"]
			field.Createable = storage.BoolFlag(true)
			account.Definition.Fields["CreatedDate"] = field
		}},
		{name: "nonTimestampMetadata", modify: func(machine *VM, _ *Value) {
			account := machine.Org.Objects["Account"]
			field := account.Definition.Fields["CreatedDate"]
			field.Type = storage.FieldString
			account.Definition.Fields["CreatedDate"] = field
		}},
	} {
		t.Run(cell.name, func(t *testing.T) {
			org := testDataOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["CreatedDate"] = storage.Field{APIName: "CreatedDate", Type: storage.FieldDateTime, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
			account.Definition.Fields["LastModifiedDate"] = storage.Field{APIName: "LastModifiedDate", Type: storage.FieldDateTime, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
			account.Definition.Fields["LastActivityDate"] = storage.Field{APIName: "LastActivityDate", Type: storage.FieldDate, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
			machine := New(nil)
			machine.SetOrg(&org)
			row := Object("Account")
			row.Fields["Name"] = String("Owned provenance control")
			row.Fields["CreatedDate"] = String("2001-02-03T04:05:06.000Z")
			row.Fields["LastModifiedDate"] = String("2002-03-04T05:06:07.000Z")
			row.Fields["LastActivityDate"] = String("2002-03-04")
			markExplicitSObjectField(&row, "CreatedDate")
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
			callerTimestamp := row.Fields["CreatedDate"]
			before := records[0].Clone()
			machine.filterJSONInsertCreationTimestamp(records, targets, "")
			if cell.filter {
				delete(before.Fields, "CreatedDate")
				delete(before.ExplicitNulls, "CreatedDate")
			}
			if !reflect.DeepEqual(before, records[0]) {
				t.Fatalf("copied payload changed outside selected timestamp: before=%#v after=%#v", before, records[0])
			}
			if !reflect.DeepEqual(callerTimestamp, row.Fields["CreatedDate"]) || !reflect.DeepEqual(callerTimestamp, targets[0].Fields["CreatedDate"]) {
				t.Fatal("filter changed the caller timestamp")
			}
		})
	}
}
