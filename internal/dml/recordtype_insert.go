package dml

import (
	"strings"

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
	if (mode != "SYSTEM_MODE" && mode != "USER_MODE") || !UsesImplicitMasterRecordType(definition) {
		return nil
	}
	value, present := record.GetField("RecordTypeId")
	explicitNull := hasExplicitNullRecordTypeInput(record)
	if !present && !explicitNull {
		return nil
	}
	id := idFromStorageValue(value)
	// Owned 61/62/67 rows and 61/67 mode twins: omission retains the implicit Master,
	// explicit null is accepted only in USER_MODE, and the describe-only
	// Master token is not an insertable custom record-type reference.
	if (explicitNull && mode == "SYSTEM_MODE") || isDescribeOnlyMasterRecordTypeID(id) {
		return dmlErrorf("INVALID_CROSS_REFERENCE_KEY", []string{"RecordTypeId"}, "Record Type ID: this ID value isn't valid for the user: %s", id)
	}
	return nil
}
