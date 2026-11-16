package vm

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

const masterDetailFieldSecurityMessage = "Unable to create/update fields: %s. Please check the security settings of this field and verify that it is read/write for your profile or permission set."

// masterDetailReparentFailures applies the native rule for an update that
// carries a non-reparentable master-detail field. Below API 63 the platform
// ignores the field. From API 63 a changed value fails the row with
// INVALID_FIELD_FOR_INSERT_UPDATE. An update under user mode, the default from
// API 67, fails the row with the inaccessible-field error even when the value
// is unchanged, so a queried record that carries the field fails too. It
// returns one result per record, or nil when no row fails, and drops the
// ignored field from the records.
// NATIVE master-detail reparenting apex_release_gates/org.tsv anonymous 62.0 63.0 67.0
func (vm *VM) masterDetailReparentFailures(op string, records []storage.Record, targets []*Value, allOrNone bool, resolvedMode string) ([]dml.Result, error) {
	if op != "update" && op != "upsert" || vm.Org == nil {
		return nil, nil
	}
	api := vm.currentMethod.APIVersion
	var failures []dml.Result
	// Most objects have no non-reparentable master. Find that once per object
	// so a bulk update does not walk the field map for every row.
	lockedMasters := make(map[string][]string, 1)
	for i := range records {
		record := records[i]
		if record.ID == "" {
			continue
		}
		objectName, ok := vm.resolveObjectName(record.Object)
		if !ok {
			continue
		}
		object, exists := vm.Org.Objects[objectName]
		if !exists {
			continue
		}
		masters, seen := lockedMasters[objectName]
		if !seen {
			for name, field := range object.Definition.Fields {
				if field.MasterDetail && !field.ReparentableMasterDetail {
					masters = append(masters, name)
				}
			}
			lockedMasters[objectName] = masters
		}
		if len(masters) == 0 {
			continue
		}
		var locked []string
		for name := range record.Fields {
			if field, known := object.Definition.Fields[name]; known && field.MasterDetail && !field.ReparentableMasterDetail {
				if op == "update" && i < len(targets) && targets[i] != nil && savedMasterFieldUnchanged(*targets[i], name) {
					continue
				}
				locked = append(locked, name)
			}
		}
		for name := range record.ExplicitNulls {
			if _, dup := record.Fields[name]; dup {
				continue
			}
			if field, known := object.Definition.Fields[name]; known && field.MasterDetail && !field.ReparentableMasterDetail {
				locked = append(locked, name)
			}
		}
		userMode := op == "update" && resolvedMode == "USER_MODE"
		if userMode && i < len(targets) && targets[i] != nil {
			// The record handed to DML keeps only changed fields. A queried
			// master that was not touched still counts under user mode.
			for _, name := range masters {
				if slices.Contains(locked, name) {
					continue
				}
				if savedMasterFieldUnchanged(*targets[i], name) {
					continue
				}
				if _, _, present := objectFieldValue(*targets[i], name); present {
					locked = append(locked, name)
				}
			}
		}
		if len(locked) == 0 {
			continue
		}
		sort.Strings(locked)
		stored, found := vm.findOrgRecord(record.Object, record.ID)
		if !found {
			continue
		}
		changed := false
		for _, name := range locked {
			incoming, supplied := record.Fields[name]
			if !supplied && !record.HasExplicitNull(name) {
				continue
			}
			if !sameMasterDetailValue(stored.Fields[name], incoming, record.HasExplicitNull(name)) {
				changed = true
			}
		}
		if userMode || changed && apexversion.AtLeast(api, 63) {
			if failures == nil {
				failures = make([]dml.Result, len(records))
			}
			if userMode {
				if allOrNone && i < len(targets) && targets[i] != nil {
					return nil, userModeFieldDMLException(op, objectName, *targets[i], locked[:1], i)
				}
				failures[i] = userModeFieldDMLRowFailure(op, objectName, vm.recordValueForMessage(record, targets, i), locked[:1])
				continue
			}
			failures[i] = dml.Result{ID: record.ID, StatusCode: "INVALID_FIELD_FOR_INSERT_UPDATE", Error: fmt.Sprintf(masterDetailFieldSecurityMessage, locked[0]), Fields: locked[:1]}
			continue
		}
		// The platform ignores the field: update the rest of the row.
		fields := make(map[string]storage.Value, len(record.Fields))
		for name, value := range record.Fields {
			fields[name] = value
		}
		nulls := make(map[string]bool, len(record.ExplicitNulls))
		for name, value := range record.ExplicitNulls {
			nulls[name] = value
		}
		for _, name := range locked {
			delete(fields, name)
			delete(nulls, name)
		}
		record.Fields, record.ExplicitNulls = fields, nulls
		records[i] = record
	}
	return failures, nil
}

func sameMasterDetailValue(stored, incoming storage.Value, explicitNull bool) bool {
	if explicitNull || incoming.Kind == storage.ValueNull {
		return stored.Kind == storage.ValueNull
	}
	if stored.Kind == storage.ValueNull {
		return false
	}
	return storage.IDsEqual(stored.ID, incoming.ID)
}

// recordValueForMessage supplies the Apex value whose Id the user-mode row
// failure reports.
func (vm *VM) recordValueForMessage(record storage.Record, targets []*Value, i int) Value {
	if i < len(targets) && targets[i] != nil {
		return *targets[i]
	}
	value := Object(record.Object)
	value.Fields["Id"] = platformScalar("Id", string(record.ID))
	return value
}

// Natively an SObject that an insert just saved does not send its master-detail
// value again on the next update (rollup_trigger_timing r1_upd_* rows at 67.0:
// insert, change another field, update succeeds). A queried or put value still
// counts (apex_release_gates RP rows). The saved value is recorded in the
// defaulted-field marker under a key that carries the value, so it stands only
// while the value is unchanged.
func savedMasterMarker(value Value, name string) (string, bool) {
	_, current, present := objectFieldValue(value, name)
	if !present {
		return "", false
	}
	text, _ := idValueText(current)
	return "saved-master:" + strings.ToLower(name) + ":" + text, true
}

func markSavedMasterFields(value *Value, definition storage.ObjectDefinition) {
	if value == nil || value.Kind != ValueObject {
		return
	}
	for name, field := range definition.Fields {
		if !field.MasterDetail || field.ReparentableMasterDetail {
			continue
		}
		if key, ok := savedMasterMarker(*value, name); ok {
			markDefaultedSObjectField(value, key)
		}
	}
}

// savedMasterFieldUnchanged reports a master-detail value the object's insert
// saved and nothing has changed since.
func savedMasterFieldUnchanged(value Value, name string) bool {
	key, ok := savedMasterMarker(value, name)
	return ok && isDefaultedSObjectField(value, key)
}
