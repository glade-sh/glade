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

// businessAccountRecordType reports only a known, explicitly selected business
// record type. A VM-supplied default does not establish the same boundary.
func businessAccountRecordType(vm *VM, record storage.Record) bool {
	if vm == nil || vm.Org == nil || !strings.EqualFold(record.Object, "Account") || record.DMLRecordTypeDefaulted() {
		return false
	}
	object, ok := vm.Org.Objects["Account"]
	if !ok {
		return false
	}
	return dml.AccountHasExplicitBusinessRecordType(record.Object, object.Definition, record)
}

func (vm *VM) preflightBusinessAccountNameFailures(records []storage.Record, op, externalIDField string) []dml.Result {
	if op != "insert" && op != "update" && op != "upsert" {
		return nil
	}
	var failures []dml.Result
	for i, record := range records {
		if vm.accountPreflightInvalidIdentity(op, record, externalIDField) {
			if failures != nil {
				failures[i] = dml.Result{ID: record.ID, Success: true}
			}
			continue
		}
		_, supplied := record.GetField("FirstName")
		business := businessAccountRecordType(vm, record)
		person := false
		created := false
		old, found := vm.findOrgRecord(record.Object, record.ID)
		if op == "upsert" && externalIDField != "" && !strings.EqualFold(externalIDField, "Id") && strings.EqualFold(record.Object, "Account") {
			var valid bool
			old, found, valid = vm.accountExternalUpsertExisting(record, externalIDField)
			valid = valid && vm.accountExternalPreflightEligible(records, i, externalIDField)
			if !valid {
				if failures != nil {
					failures[i] = dml.Result{ID: record.ID, Success: true}
				}
				continue
			}
			created = !found
			if created {
				status, observed := vm.accountExternalCapturedNameShape(record, externalIDField)
				business = observed && status == "INVALID_FIELD_FOR_INSERT_UPDATE"
				if business && len(record.Fields) == 4 && len(record.ExplicitNulls) == 0 {
					selected, hasSelected := record.GetField("RecordTypeId")
					if hasSelected {
						for _, rt := range vm.Org.Objects["Account"].Definition.RecordTypes {
							if rt.PersonAccount && storage.IDsEqual(rt.ID, storage.ID(storageValueIDText(selected))) {
								if failures == nil {
									failures = make([]dml.Result, len(records))
									for j := 0; j < i; j++ {
										failures[j] = dml.Result{ID: records[j].ID, Success: true}
									}
								}
								message := "Unable to create/update fields: Name. Please check the security settings of this field and verify that it is read/write for your profile or permission set."
								failures[i] = dmlFailure(record.ID, message, status, []string{"Name"})
								failures[i].UpsertCreated = true
								break
							}
						}
					}
					if failures != nil && !failures[i].Success && failures[i].StatusCode != "" {
						continue
					}
				}
				if observed && status == "INVALID_CROSS_REFERENCE_KEY" {
					if failures == nil {
						failures = make([]dml.Result, len(records))
						for j := 0; j < i; j++ {
							failures[j] = dml.Result{ID: records[j].ID, Success: true}
						}
					}
					selected, _ := record.GetField("RecordTypeId")
					message := "Record Type ID: this ID value isn't valid for the user: " + storageValueIDText(selected)
					failures[i] = dmlFailure(record.ID, message, status, []string{"RecordTypeId"})
					failures[i].UpsertCreated = true
					continue
				}
			}
		}
		if strings.EqualFold(record.Object, "Account") {
			if found {
				flag, ok := old.GetField("IsPersonAccount")
				person = ok && flag.Kind == storage.ValueBoolean && flag.Boolean
				selected, hasSelected := record.GetField("RecordTypeId")
				if !business && (!hasSelected || storageValueIDText(selected) == "" || record.DMLRecordTypeDefaulted()) {
					business = businessAccountRecordType(vm, old)
				}
				if business {
					person = false
				}
			}
		}
		_, suppliedName := record.GetField("Name")
		badPersonName := person && (suppliedName || record.HasExplicitNull("Name"))
		if badPersonName || ((supplied || record.HasExplicitNull("FirstName")) && business) {
			if failures == nil {
				failures = make([]dml.Result, len(records))
				for j := 0; j < i; j++ {
					failures[j] = dml.Result{ID: records[j].ID, Success: true}
				}
			}
			field := "FirstName"
			if badPersonName {
				field = "Name"
			}
			message := "Account: bad field names on insert/update call: " + field
			if badPersonName && record.DMLNameUnassigned() {
				message = dml.QueriedPersonAccountNameError
			}
			failures[i] = dmlFailure(record.ID, message, "INVALID_FIELD_FOR_INSERT_UPDATE", []string{field})
			failures[i].UpsertCreated = created
		} else if failures != nil {
			failures[i] = dml.Result{ID: record.ID, Success: true}
		}
	}
	return failures
}

// Native person- and business-default rows cover the simple Name/FirstName
// candidate and one additional selector, LastName or PersonTitle at a time.
func (vm *VM) accountExternalCapturedNameShape(record storage.Record, externalIDField string) (string, bool) {
	if vm == nil || vm.Org == nil {
		return "", false
	}
	hasCity := false
	for field := range record.ExplicitNulls {
		if !strings.EqualFold(field, "BillingCity") {
			return "", false
		}
		hasCity = true
	}
	object, ok := vm.Org.Objects["Account"]
	if !ok {
		return "", false
	}
	if _, enabled := storage.ResolveFieldName(object.Definition, "", "IsPersonAccount"); !enabled {
		return "", false
	}
	name, hasName := record.GetField("Name")
	first, hasFirst := record.GetField("FirstName")
	if !hasName || name.Kind != storage.ValueString || strings.TrimSpace(name.String) == "" || !hasFirst || first.Kind != storage.ValueString || strings.TrimSpace(first.String) == "" {
		return "", false
	}
	hasType, hasLast, hasTitle := false, false, false
	for field, value := range record.Fields {
		if strings.EqualFold(field, "Name") || strings.EqualFold(field, "FirstName") || strings.EqualFold(field, externalIDField) {
			continue
		}
		if strings.EqualFold(field, "LastName") || strings.EqualFold(field, "PersonTitle") || strings.EqualFold(field, "BillingCity") {
			switch {
			case strings.EqualFold(field, "LastName"):
				hasLast = true
			case strings.EqualFold(field, "PersonTitle"):
				hasTitle = true
			default:
				hasCity = true
			}
			if strings.EqualFold(field, "BillingCity") && value.Kind == storage.ValueNull {
				continue
			}
			if value.Kind != storage.ValueString || strings.TrimSpace(value.String) == "" {
				return "", false
			}
			continue
		}
		if strings.EqualFold(field, "RecordTypeId") {
			hasType = true
			continue
		}
		return "", false
	}
	selectors := 0
	for _, present := range []bool{hasType, hasCity, hasLast, hasTitle} {
		if present {
			selectors++
		}
	}
	if hasCity && !hasType {
		return "", false
	}
	if selectors > 1 && !(hasType && hasCity && !hasLast && !hasTitle) && !(hasType && !hasCity && hasLast && hasTitle) {
		return "", false
	}
	if value, present := record.GetField("RecordTypeId"); present {
		id := storageValueIDText(value)
		if (len(id) != 15 && len(id) != 18) || !strings.HasPrefix(id, "012") {
			return "", false
		}
		for _, rt := range object.Definition.RecordTypes {
			if storage.IDsEqual(storage.ID(id), rt.ID) {
				if !rt.Active || !rt.Available {
					return "", false
				}
				// The captured person error covers only the four-field input.
				// Preserve the previous path for additional selectors and nulls.
				if rt.PersonAccount && (len(record.Fields) != 4 || len(record.ExplicitNulls) != 0) {
					return "", false
				}
				return "INVALID_FIELD_FOR_INSERT_UPDATE", true
			}
		}
		return "INVALID_CROSS_REFERENCE_KEY", true
	}
	return "INVALID_FIELD_FOR_INSERT_UPDATE", true
}

// The native Account email rows reject malformed values before the candidate
// trigger for inserts, updates and Id-based upserts, including person records.
func (vm *VM) preflightAccountPersonEmailFailures(records []storage.Record, op, externalIDField string) []dml.Result {
	if !vm.accountPersonAccountsEnabled() || (op != "insert" && op != "update" && op != "upsert") {
		return nil
	}
	var failures []dml.Result
	for i, record := range records {
		if vm.accountPreflightInvalidIdentity(op, record, externalIDField) {
			if failures != nil {
				failures[i] = dml.Result{ID: record.ID, Success: true}
			}
			continue
		}
		value, supplied := record.GetField("PersonEmail")
		if strings.EqualFold(record.Object, "Account") && supplied && dml.InvalidEmailValue(value) {
			created := false
			if op == "upsert" && externalIDField != "" && !strings.EqualFold(externalIDField, "Id") {
				_, found, valid := vm.accountExternalUpsertExisting(record, externalIDField)
				valid = valid && vm.accountExternalPreflightEligible(records, i, externalIDField)
				if !valid {
					if failures != nil {
						failures[i] = dml.Result{ID: record.ID, Success: true}
					}
					continue
				}
				created = !found
			}
			if failures == nil {
				failures = make([]dml.Result, len(records))
				for j := 0; j < i; j++ {
					failures[j] = dml.Result{ID: records[j].ID, Success: true}
				}
			}
			failures[i] = dmlFailure(record.ID, "Email: invalid email address: "+strings.TrimSpace(value.String), "INVALID_EMAIL_ADDRESS", []string{"PersonEmail"})
			failures[i].UpsertCreated = created
		} else if failures != nil {
			failures[i] = dml.Result{ID: record.ID, Success: true}
		}
	}
	return failures
}

// Combined invalid identity cases stay on the existing identity-validation path.
func (vm *VM) accountPreflightInvalidIdentity(op string, record storage.Record, externalIDField string) bool {
	if op == "insert" {
		return record.ID != ""
	}
	if op == "update" {
		_, failed := vm.preflightUpdateIdentityFailure(record)
		return failed
	}
	if op == "upsert" && (externalIDField == "" || strings.EqualFold(externalIDField, "Id")) {
		// The captured Id routes use existing records; Id-less or invalid Ids
		// retain the prior insert/identity path and its isCreated result.
		_, failed := vm.preflightUpdateIdentityFailure(record)
		return failed
	}
	return false
}

// Leave unprobed selectors, empty values and duplicate input keys to normal DML validation.
func (vm *VM) accountExternalPreflightEligible(records []storage.Record, index int, selector string) bool {
	if vm == nil || vm.Org == nil || index < 0 || index >= len(records) {
		return false
	}
	object, ok := vm.Org.Objects["Account"]
	if !ok {
		return false
	}
	field, value, ok := upsertMatchField(object.Definition, vm.Org.Namespace, records[index], selector)
	definition, known := object.Definition.Fields[field]
	if !ok || !known || !definition.ExternalID || !definition.Unique || definition.Type != storage.FieldString || (value.Kind == storage.ValueString && strings.TrimSpace(value.String) == "") {
		return false
	}
	for j, other := range records {
		if j == index || !strings.EqualFold(other.Object, "Account") {
			continue
		}
		otherField, otherValue, present := upsertMatchField(object.Definition, vm.Org.Namespace, other, selector)
		if present && strings.EqualFold(otherField, field) && storageValuesEqualForVM(definition, value, otherValue) {
			return false
		}
	}
	return true
}

// Resolve a unique external-key target without overriding duplicate-key errors.
func (vm *VM) accountExternalUpsertExisting(record storage.Record, externalIDField string) (storage.Record, bool, bool) {
	if vm == nil || vm.Org == nil || record.ID != "" {
		return storage.Record{}, false, false
	}
	object, ok := vm.Org.Objects["Account"]
	if !ok {
		return storage.Record{}, false, false
	}
	field, value, ok := upsertMatchField(object.Definition, vm.Org.Namespace, record, externalIDField)
	if !ok {
		return storage.Record{}, false, false
	}
	var found storage.Record
	count := 0
	for _, stored := range object.Records {
		if stored.System.IsDeleted {
			continue
		}
		if v, present := stored.GetField(field); present && storageValuesEqualForVM(object.Definition.Fields[field], v, value) {
			found = stored
			count++
		}
	}
	return found, count == 1, count < 2
}

func (vm *VM) accountPersonAccountsEnabled() bool {
	if vm == nil || vm.Org == nil {
		return false
	}
	name, ok := storage.ResolveObjectName(*vm.Org, "Account")
	if !ok {
		return false
	}
	_, enabled := storage.ResolveFieldName(vm.Org.Objects[name].Definition, "", "IsPersonAccount")
	return enabled
}

func applyAccountBeforeDMLDerivedFields(vm *VM, record *storage.Record) {
	if vm == nil || vm.Org == nil || record == nil || !accountRecordHasPersonAccountSignal(*record) {
		return
	}
	// The explicit business record type determines the trigger-visible kind,
	// even when a person-only field will be rejected after the trigger.
	if businessAccountRecordType(vm, *record) {
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
	return dml.AccountPersonSignalForBeforeTrigger(record)
}
