package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

var accountDMLPolicy = objectDMLPolicy{
	beforeDMLDerivedFields: applyAccountBeforeDMLDerivedFields,
	defaultRecordTypeID:    accountDefaultRecordTypeID,
}

func applyAccountBeforeDMLDerivedFields(vm *VM, record *storage.Record) {
	if vm == nil || vm.Org == nil || record == nil || !accountRecordHasPersonAccountSignal(*record) {
		return
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, record.Object)
	if !ok {
		return
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return
	}
	if _, enabled := storage.ResolveFieldName(object.Definition, "", "IsPersonAccount"); !enabled {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["IsPersonAccount"] = storage.BooleanValue(true)
}

func accountDefaultRecordTypeID(vm *VM, definition storage.ObjectDefinition, record storage.Record) storage.ID {
	recordType, ok := dml.AccountDefaultRecordType(vm.Org, storage.ID(vm.currentUserID()), definition, record)
	if !ok {
		return ""
	}
	return recordType.ID
}

func accountRecordHasPersonAccountSignal(record storage.Record) bool {
	for field, value := range record.Fields {
		if !strings.HasPrefix(field, "Person") && field != "FirstName" && field != "LastName" {
			continue
		}
		if storageValueHasNonZeroContent(value) {
			return true
		}
	}
	return false
}

func storageValueHasNonZeroContent(value storage.Value) bool {
	switch value.Kind {
	case storage.ValueNull, "":
		return false
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal:
		return value.String != ""
	case storage.ValueID:
		return value.ID != ""
	case storage.ValueInteger:
		return value.Integer != 0
	default:
		return true
	}
}
