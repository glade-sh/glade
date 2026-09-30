package vm

import "github.com/glade-sh/glade/internal/storage"

// The snapshot is private typed state, outside Apex/JSON-addressable Fields.
// It is immutable after SOQL publishes it, so ordinary Value copies may share it.
type sobjectLoadedReferenceSnapshot struct {
	recordID   storage.ID
	references map[string]storage.ID
}

// Only an unchanged SOQL view of a live, SetNull lookup contributes this input
// provenance. Visibility markers alone also occur after DML and on trigger rows.
func (vm *VM) markSOQLLoadedReferences(value *Value, record storage.Record) {
	if vm == nil || vm.Org == nil || value == nil || value.Kind != ValueObject || record.ID == "" {
		return
	}
	objectName, ok := vm.resolveObjectName(record.Object)
	if !ok {
		return
	}
	definition := vm.Org.Objects[objectName].Definition
	snapshot := &sobjectLoadedReferenceSnapshot{recordID: record.ID, references: make(map[string]storage.ID)}
	for _, relation := range definition.Relations {
		if !relation.SetNullOnDelete || !vm.queriedSObjectFieldsIncludes(*value, relation.Field) {
			continue
		}
		field, ok := storage.ResolveFieldName(definition, vm.OrgNamespace(), relation.Field)
		if !ok {
			continue
		}
		raw, ok := record.GetField(field)
		if !ok {
			continue
		}
		id := storage.ID(storageValueIDText(raw))
		if id == "" {
			continue
		}
		for _, targetName := range definition.Fields[field].ReferenceTo {
			target, ok := storage.ResolveObjectName(*vm.Org, targetName)
			if !ok {
				continue
			}
			_, parent, ok := storage.LookupRecordByID(vm.Org.Objects[target].Records, id)
			if ok && !parent.System.IsDeleted {
				snapshot.references[field] = id
				break
			}
		}
	}
	if len(snapshot.references) > 0 {
		value.loadedReferenceSnapshot = snapshot
	}
}

func (vm *VM) captureLoadedReferenceInput(value Value, record *storage.Record) {
	if record == nil || record.ID == "" || isUserSetSObjectFieldAlias(value, "Id") {
		return
	}
	for _, marker := range []string{sobjectDMLAccessibleField, sobjectTriggerField, sobjectCloneMarkerField} {
		if _, exists := value.Fields[marker]; exists {
			return
		}
	}
	snapshot := value.loadedReferenceSnapshot
	if snapshot == nil {
		return
	}
	if !storage.IDsEqual(record.ID, snapshot.recordID) {
		return
	}
	for field, raw := range record.Fields {
		if isUserSetSObjectFieldAlias(value, field) || !vm.queriedSObjectFieldsIncludes(value, field) {
			continue
		}
		loaded := snapshot.references[field]
		id := storage.ID(storageValueIDText(raw))
		if loaded == "" || id == "" || !storage.IDsEqual(loaded, id) {
			continue
		}
		if record.LoadedReferences == nil {
			record.LoadedReferences = make(map[string]storage.ID)
		}
		record.LoadedReferences[field] = id
	}
}

// Before-trigger records are views of the original DML input. Retain its
// provenance only for fields the trigger did not explicitly assign or change.
func preserveLoadedReferenceInput(record *storage.Record, original storage.Record, value Value) {
	if record == nil || !storage.IDsEqual(record.ID, original.ID) || isUserSetSObjectFieldAlias(value, "Id") {
		return
	}
	for field, id := range original.LoadedReferences {
		if isUserSetSObjectFieldAlias(value, field) || record.HasExplicitNull(field) {
			continue
		}
		current, ok := record.GetField(field)
		if !ok || !storage.IDsEqual(storage.ID(storageValueIDText(current)), id) {
			continue
		}
		if record.LoadedReferences == nil {
			record.LoadedReferences = make(map[string]storage.ID)
		}
		record.LoadedReferences[field] = id
	}
}
