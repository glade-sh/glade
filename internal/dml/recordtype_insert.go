package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func hasActiveCustomRecordTypes(definition storage.ObjectDefinition) bool {
	if !strings.HasSuffix(strings.ToLower(definition.APIName), "__c") || storage.IsCustomSettingDefinition(definition) {
		return false
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.Active && !isDescribeOnlyMasterRecordTypeID(recordType.ID) {
			return true
		}
	}
	return false
}

// UsesImplicitMasterRecordType distinguishes a custom object's omitted record
// type from choosing its sole real type without an explicit default mapping.
func UsesImplicitMasterRecordType(definition storage.ObjectDefinition) bool {
	if !hasActiveCustomRecordTypes(definition) {
		return false
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.Default {
			return false
		}
	}
	return true
}

// hasActiveRealRecordTypes: the object has an active record type besides
// Master, on custom and standard objects alike (record_type_visibility rows
// for Account and Opportunity). Custom settings have none, and an Account with
// person-account record types keeps its person-account contracts.
func hasActiveRealRecordTypes(definition storage.ObjectDefinition) bool {
	if storage.IsCustomSettingDefinition(definition) {
		return false
	}
	real := false
	for _, recordType := range definition.RecordTypes {
		if recordType.PersonAccount {
			return false
		}
		if recordType.Active && !isDescribeOnlyMasterRecordTypeID(recordType.ID) && !strings.EqualFold(recordType.DeveloperName, "Master") {
			real = true
		}
	}
	return real
}

func hasExplicitNullRecordTypeInput(record storage.Record) bool {
	value, present := record.GetField("RecordTypeId")
	return record.HasExplicitNull("RecordTypeId") || (present && value.Kind == storage.ValueNull)
}

func (e *Engine) validateApexRecordTypeInsert(definition storage.ObjectDefinition, record storage.Record) error {
	mode := e.Options.ApexRecordTypeInsertMode
	// person_account_profile_routes/org.tsv captures statement insert/upsert
	// and scalar/list Database.insert, including options and explicit modes.
	if e.Options.ApexRejectNullPersonRecordType && strings.EqualFold(definition.APIName, "Account") && isPersonAccountRecord(record) && hasExplicitNullRecordTypeInput(record) {
		return dmlErrorf("INVALID_CROSS_REFERENCE_KEY", []string{"RecordTypeId"}, "Record Type ID: this ID value isn't valid for the user: ")
	}
	if (mode != "SYSTEM_MODE" && mode != "USER_MODE") || !hasActiveRealRecordTypes(definition) {
		return nil
	}
	value, present := record.GetField("RecordTypeId")
	explicitNull := hasExplicitNullRecordTypeInput(record)
	if !present && !explicitNull {
		return nil
	}
	id := idFromStorageValue(value)
	// Owned 61/62/67 rows and 61/67 mode twins (record_type_id_null_insert,
	// record_type_visibility for Account and Opportunity): without a default,
	// explicit null is accepted only in USER_MODE. A default rejects it in
	// every mode (validateNullRecordTypeWithDefault). The describe-only Master
	// token is never an insertable record-type reference, and USER_MODE
	// rejects a record type the running user's profile does not see.
	if explicitNull {
		if mode == "SYSTEM_MODE" && !hasRealDefaultRecordType(definition) {
			return dmlErrorf("INVALID_CROSS_REFERENCE_KEY", []string{"RecordTypeId"}, "Record Type ID: this ID value isn't valid for the user: ")
		}
		return nil
	}
	if isDescribeOnlyMasterRecordTypeID(id) || (mode == "USER_MODE" && recordTypeUnavailable(definition, id)) {
		return dmlErrorf("INVALID_CROSS_REFERENCE_KEY", []string{"RecordTypeId"}, "Record Type ID: this ID value isn't valid for the user: %s", id)
	}
	return nil
}

// validateApexRecordTypeUpdate: a USER_MODE update to a record type the
// running user's profile does not see fails (record_type_visibility update
// rows); SYSTEM_MODE keeps it.
func (e *Engine) validateApexRecordTypeUpdate(definition storage.ObjectDefinition, record, existing storage.Record) error {
	if e.Options.ApexRecordTypeUpdateMode != "USER_MODE" || !hasActiveRealRecordTypes(definition) {
		return nil
	}
	value, present := record.GetField("RecordTypeId")
	if !present || value.Kind == storage.ValueNull {
		return nil
	}
	id := idFromStorageValue(value)
	if previous, ok := existing.GetField("RecordTypeId"); ok && storage.IDsEqual(idFromStorageValue(previous), id) {
		return nil
	}
	if recordTypeUnavailable(definition, id) {
		return dmlErrorf("INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY", nil, "invalid record type")
	}
	return nil
}

func recordTypeUnavailable(definition storage.ObjectDefinition, id storage.ID) bool {
	for _, recordType := range definition.RecordTypes {
		if recordType.ID != "" && storage.IDsEqual(recordType.ID, id) {
			return recordType.Unavailable
		}
	}
	return false
}

func hasRealDefaultRecordType(definition storage.ObjectDefinition) bool {
	for _, recordType := range definition.RecordTypes {
		if recordType.Default && recordType.Active && !isDescribeOnlyMasterRecordTypeID(recordType.ID) {
			return true
		}
	}
	return false
}

// applyUpdateNullRecordTypeDefault turns an update to a null RecordTypeId into
// the profile default record type (record_type_id_null_insert_default;
// record_type_visibility profile rows for Account).
func applyUpdateNullRecordTypeDefault(definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil || !hasActiveRealRecordTypes(definition) || !hasExplicitNullRecordTypeInput(*record) {
		return
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.Default && recordType.Active && !isDescribeOnlyMasterRecordTypeID(recordType.ID) {
			if record.Fields == nil {
				record.Fields = make(map[string]storage.Value)
			}
			record.Fields["RecordTypeId"] = storage.IDValue(recordType.ID)
			delete(record.ExplicitNulls, "RecordTypeId")
			return
		}
	}
}

// validateNullRecordTypeWithDefault: with a profile default record type, an
// explicit null RecordTypeId is rejected at every API and access mode
// (record_type_id_null_insert_default; record_type_visibility profile rows
// for Account). It runs after the required-field
// checks; the native order between the two is not captured.
func validateNullRecordTypeWithDefault(definition storage.ObjectDefinition, record storage.Record) error {
	if hasActiveRealRecordTypes(definition) && hasExplicitNullRecordTypeInput(record) && hasRealDefaultRecordType(definition) {
		return dmlErrorf("INVALID_CROSS_REFERENCE_KEY", []string{"RecordTypeId"}, "Record Type ID: this ID value isn't valid for the user: ")
	}
	return nil
}

// RecordTypeFieldAbsent: an object has RecordTypeId, and the RecordType
// relationship, only once it has a record type besides Master
// (record_type_field_presence, record_type_shape and record_type_visibility
// rows): get and put raise SObjectException and SOQL raises QueryException.
// Unspecified shapes keep the field, and so do an object whose scratch
// definition sets a defaultRecordType and a definition that declares it.
func RecordTypeFieldAbsent(org *storage.OrgState, definition storage.ObjectDefinition) bool {
	if org == nil || org.HasFeature(project.UnspecifiedStandardFieldShape) ||
		org.HasFeature(project.ObjectDefaultRecordTypeFeatureFor(definition.APIName)) {
		return false
	}
	if _, declared := storage.ResolveFieldName(definition, org.Namespace, "RecordTypeId"); declared {
		return false
	}
	for _, recordType := range definition.RecordTypes {
		if !strings.EqualFold(recordType.DeveloperName, "Master") {
			return false
		}
	}
	return true
}
