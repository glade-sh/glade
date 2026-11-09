package dml

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) updateOne(record storage.Record) error {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return err
	}
	if storage.IsCustomMetadataDefinition(object.Definition) {
		return customMetadataReadOnlyError(objectName)
	}
	_, suppliedAccountName := record.GetField("Name")
	suppliedAccountName = suppliedAccountName || record.HasExplicitNull("Name")
	nullsFromFields := explicitNullsFromFieldValues(record)
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return err
	}
	normalizeNameFields(objectName, object.Definition, &record)
	if record.ID == "" {
		return errUpdateRequiresID
	}
	applyUpdateNullRecordTypeDefault(object.Definition, &record)
	stripImplicitReadOnlyDefaultFields(object.Definition, e.Org.Namespace, &record, false)
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return err
	}
	storedID, existing, ok := storage.LookupRecordByID(object.Records, record.ID)
	if !ok {
		return fmt.Errorf("dml: record %s does not exist", record.ID)
	}
	if existing.System.IsDeleted && !e.Options.AllowUpdateDeleted {
		return dmlErrorf("ENTITY_IS_DELETED", nil, "entity is deleted")
	}
	if err := e.validateApexRecordTypeUpdate(object.Definition, record, existing); err != nil {
		return err
	}
	if strings.EqualFold(objectName, "Account") && suppliedAccountName {
		if person, ok := existing.GetField("IsPersonAccount"); ok && person.Kind == storage.ValueBoolean && person.Boolean {
			if record.DMLNameUnassigned() {
				return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{"Name"}, QueriedPersonAccountNameError)
			}
			return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{"Name"}, "Account: bad field names on insert/update call: Name")
		}
	}
	if err := validateBusinessAccountContacts(objectName, object.Definition, record, existing); err != nil {
		return err
	}
	if existing.System.Locked && !e.canEditLockedRecord(objectName) {
		return lockedRecordError()
	}
	if err := e.validatePersonAccountCustomFieldWrites(record, existing); err != nil {
		return err
	}
	stripUnchangedNonUpdateableFields(object.Definition, e.Org.Namespace, &record, existing, nullsFromFields)
	if err := validateFieldWriteability(object.Definition, e.Org.Namespace, record, false); err != nil {
		return err
	}
	stripReadOnlyUpdateFields(object.Definition, e.Org.Namespace, &record)
	normalizeUserNameFields(objectName, object.Definition, &record)
	if err := e.applyStringLengthRules(object.Definition, &record); err != nil {
		return err
	}
	if err := validateFields(object.Definition, e.Org.Namespace, record, existing); err != nil {
		return err
	}
	if err := e.validateUpdateReferences(object.Definition, record); err != nil {
		return err
	}
	if err := e.validateActiveOwner(object.Definition, record.System.OwnerID, existing.System.OwnerID); err != nil {
		return err
	}
	if err := e.validateUnique(objectName, object.Definition, record, storedID); err != nil {
		return err
	}
	finalRecord := existing.Clone()
	if _, nameSupplied := record.GetField("Name"); nameSupplied || record.HasExplicitNull("Name") {
		finalRecord.HierarchyNameUsesRecordID = false
	}
	if _, ownerSupplied := record.GetField("SetupOwnerId"); ownerSupplied || record.HasExplicitNull("SetupOwnerId") {
		finalRecord.HierarchyNameUsesRecordID = false
	}
	if record.System.OwnerID != "" {
		finalRecord.System.OwnerID = record.System.OwnerID
	}
	if finalRecord.Fields == nil {
		finalRecord.Fields = make(map[string]storage.Value)
	}
	if finalRecord.ExplicitNulls == nil {
		finalRecord.ExplicitNulls = make(map[string]bool)
	}
	for field, value := range record.Fields {
		deleteCaseInsensitiveFieldAlias(object.Definition, e.Org.Namespace, finalRecord.Fields, field)
		deleteCaseInsensitiveNullAlias(object.Definition, e.Org.Namespace, finalRecord.ExplicitNulls, field)
		finalRecord.Fields[field] = value.Clone()
		delete(finalRecord.ExplicitNulls, field)
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			deleteCaseInsensitiveFieldAlias(object.Definition, e.Org.Namespace, finalRecord.Fields, field)
			deleteCaseInsensitiveNullAlias(object.Definition, e.Org.Namespace, finalRecord.ExplicitNulls, field)
			delete(finalRecord.Fields, field)
			finalRecord.ExplicitNulls[field] = true
		}
	}
	if record.ParentRelationships != nil {
		finalRecord.ParentRelationships = make(map[string]storage.Record, len(record.ParentRelationships))
		for name, parent := range record.ParentRelationships {
			finalRecord.ParentRelationships[name] = parent.Clone()
		}
	}
	if err := validateRequiredUpdate(object.Definition, record, finalRecord); err != nil {
		return err
	}
	if err := validatePersonAccountRequiredFields(objectName, finalRecord); err != nil {
		return err
	}
	e.applyTaskPriorityFlag(&finalRecord)
	e.applyActivityAccountFromWhat(&finalRecord)
	e.applyOrderItemDerivedFields(&finalRecord)
	e.applyRegulatoryCodeActive(&finalRecord)
	e.applyOpportunityLineItemDerivedFields(&finalRecord, record, existing)
	e.applyQuoteLineItemDerivedFields(&finalRecord)
	e.applyQuoteDerivedFields(&finalRecord)
	priorRecord := e.priorRecordForValidation(record.ID, existing)
	if err := e.validateValidationRules(objectName, object.Definition, finalRecord, &priorRecord, false); err != nil {
		return err
	}
	if err := e.validateLookupFilters(object.Definition, finalRecord); err != nil {
		return err
	}
	if err := e.enforceDuplicateRulesOnUpdate(finalRecord, priorRecord); err != nil {
		return err
	}
	_, hasPrivateShares := privateOwnerShareObject(e.Org, object.Definition)
	needsRollback := hasPrivateShares || !e.DeferAutomation && hasObjectAutomation(object.Definition)
	rollback := e.beginRollbackPoint(needsRollback)
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, objectName); cloned {
		object = e.Org.Objects[objectName]
		storedID, existing, _ = storage.LookupRecordByID(object.Records, record.ID)
	}
	if existing.Fields == nil {
		existing.Fields = make(map[string]storage.Value)
	}
	if existing.ExplicitNulls == nil {
		existing.ExplicitNulls = make(map[string]bool)
	}
	stamp := e.systemTimestamp()
	oldRecord := existing.Clone()
	existing.ForgetChangedClearedLookups(record)
	existing.System.OwnerID = finalRecord.System.OwnerID
	existing.HierarchyNameUsesRecordID = finalRecord.HierarchyNameUsesRecordID
	for field, value := range record.Fields {
		deleteCaseInsensitiveFieldAlias(object.Definition, e.Org.Namespace, existing.Fields, field)
		deleteCaseInsensitiveNullAlias(object.Definition, e.Org.Namespace, existing.ExplicitNulls, field)
		existing.Fields[field] = value.Clone()
		delete(existing.ExplicitNulls, field)
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			deleteCaseInsensitiveFieldAlias(object.Definition, e.Org.Namespace, existing.Fields, field)
			deleteCaseInsensitiveNullAlias(object.Definition, e.Org.Namespace, existing.ExplicitNulls, field)
			delete(existing.Fields, field)
			existing.ExplicitNulls[field] = true
		}
	}
	e.applyTaskPriorityFlag(&existing)
	e.applyActivityAccountFromWhat(&existing)
	stripStoredCalculatedFields(object.Definition, &existing)
	e.applyOrderItemDerivedFields(&existing)
	e.applyOpportunityLineItemDerivedFields(&existing, record, oldRecord)
	e.applyQuoteLineItemDerivedFields(&existing)
	e.applyQuoteDerivedFields(&existing)
	materializeEmailMessageName(&existing)
	existing.System.LastModifiedDate = stamp
	existing.System.SystemModstamp = stamp
	existing.System.LastModifiedByID = e.systemUserID()
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordUpdate(objectName, storedID, oldRecord)
	}
	existing.ClearDMLNameUnassigned()
	object.Records[storedID] = existing
	e.Org.Objects[objectName] = object
	e.removeUniqueIndexRecord(objectName, object.Definition, oldRecord)
	e.addUniqueIndexRecord(objectName, object.Definition, existing)
	e.recalculateSummaryFieldsForChildren(objectName, oldRecord, finalRecord)
	e.refreshActivityAccounts(objectName, object.Definition, storedID, oldRecord, existing)
	if !storage.IDsEqual(oldRecord.System.OwnerID, existing.System.OwnerID) {
		if err := e.reconcilePrivateOwnerShares(objectName, storedID); err != nil {
			if rollbackErr := e.restoreRollbackPoint(rollback); rollbackErr != nil {
				return rollbackErr
			}
			return err
		}
	}
	if syncPersonContactAfterUpdate(objectName) {
		if err := e.syncPersonContact(existing); err != nil {
			if rollbackErr := e.restoreRollbackPoint(rollback); rollbackErr != nil {
				return rollbackErr
			}
			return err
		}
	}
	if !e.DeferAutomation {
		e.flushActivityRefresh()
		if err := e.withPriorRecordForAutomation(storedID, oldRecord, func() error {
			_, err := e.ApplyAutomation(objectName, storedID, record)
			return err
		}); err != nil {
			if rollbackErr := e.restoreRollbackPoint(rollback); rollbackErr != nil {
				return rollbackErr
			}
			return err
		}
	}
	if e.ChangeObserver != nil {
		e.ChangeObserver("UPDATE", oldRecord, e.Org.Objects[objectName].Records[storedID])
	}
	return nil
}

func (e *Engine) priorRecordForValidation(id storage.ID, fallback storage.Record) storage.Record {
	if e != nil && e.PriorRecords != nil && id != "" {
		if _, prior, ok := storage.LookupRecordByID(e.PriorRecords, id); ok {
			return prior.Clone()
		}
	}
	return fallback.Clone()
}

func (e *Engine) withPriorRecordForAutomation(id storage.ID, prior storage.Record, fn func() error) error {
	if e == nil || id == "" {
		return fn()
	}
	if e.PriorRecords == nil {
		e.PriorRecords = map[storage.ID]storage.Record{id: prior.Clone()}
		defer func() {
			e.PriorRecords = nil
		}()
		return fn()
	}
	if _, _, ok := storage.LookupRecordByID(e.PriorRecords, id); ok {
		return fn()
	}
	e.PriorRecords[id] = prior.Clone()
	defer delete(e.PriorRecords, id)
	return fn()
}

func stripStoredCalculatedFields(definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil || record.Fields == nil {
		return
	}
	for field := range record.Fields {
		fieldDef, ok := definition.Fields[field]
		if !ok {
			continue
		}
		if isFormulaBackedField(fieldDef) && !quoteRollupField(record.Object, field) {
			delete(record.Fields, field)
		}
	}
}

func isFormulaBackedField(field storage.Field) bool {
	return field.Type == storage.FieldCalculated || strings.TrimSpace(field.Formula) != ""
}

func isCalculatedOrSummaryField(field storage.Field) bool {
	return field.Type == storage.FieldSummary || isFormulaBackedField(field)
}

func (e *Engine) deleteOne(record storage.Record) error {
	return e.deleteOneWithContext(record, nil)
}

func (e *Engine) deleteOneWithContext(record storage.Record, ctx *deleteContext) error {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return err
	}
	if storage.IsCustomMetadataDefinition(object.Definition) {
		return customMetadataReadOnlyError(objectName)
	}
	if record.ID == "" {
		return fmt.Errorf("dml: delete requires id")
	}
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return err
	}
	storedID, stored, ok := storage.LookupRecordByID(object.Records, record.ID)
	if !ok {
		return fmt.Errorf("dml: record %s does not exist", record.ID)
	}
	if stored.System.IsDeleted {
		return dmlErrorf("ENTITY_IS_DELETED", nil, "entity is deleted")
	}
	if stored.System.Locked && !e.canEditLockedRecord(objectName) {
		return lockedRecordError()
	}
	return e.deleteRecord(objectName, storedID, make(map[string]bool), ctx, nil)
}

type cascadeDeleteParent struct {
	object string
	id     storage.ID
}

func (e *Engine) deleteRecord(objectName string, id storage.ID, seen map[string]bool, ctx *deleteContext, cascadeParent *cascadeDeleteParent) error {
	key := objectName + ":" + string(id)
	if seen[key] {
		return nil
	}
	seen[key] = true
	object := e.Org.Objects[objectName]
	storedID, stored, ok := storage.LookupRecordByID(object.Records, id)
	if !ok {
		return fmt.Errorf("dml: record %s does not exist", id)
	}
	if stored.System.IsDeleted {
		return dmlErrorf("ENTITY_IS_DELETED", nil, "entity is deleted")
	}
	if err := e.validateDeleteReferences(objectName, id, ctx); err != nil {
		return err
	}
	if err := e.applyBeforeDeleteFlows(objectName, stored); err != nil {
		return err
	}
	object = e.Org.Objects[objectName]
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, objectName); cloned {
		object = e.Org.Objects[objectName]
		storedID, stored, _ = storage.LookupRecordByID(object.Records, id)
	}
	stamp := e.systemTimestamp()
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordUpdate(objectName, storedID, stored)
	}
	stored.System.IsDeleted = true
	stored.System.RecycleBinEmptied = false
	if cascadeParent == nil {
		stored.System.CascadeDeletedByObject = ""
		stored.System.CascadeDeletedByID = ""
	} else {
		stored.System.CascadeDeletedByObject = cascadeParent.object
		stored.System.CascadeDeletedByID = cascadeParent.id
	}
	stored.System.LastModifiedDate = stamp
	stored.System.SystemModstamp = stamp
	stored.System.LastModifiedByID = e.systemUserID()
	object.Records[storedID] = stored
	e.Org.Objects[objectName] = object
	e.removeUniqueIndexRecord(objectName, object.Definition, stored)
	e.recalculateSummaryFieldsForChildren(objectName, stored)
	e.clearDeletedParentReferences(objectName, storedID, ctx)
	if err := e.cascadeDeleteChildren(objectName, storedID, seen, ctx); err != nil {
		return err
	}
	if objectName == "PermissionSetGroupComponent" {
		e.markPermissionSetGroupComponentChanged(stored)
	}
	if e.ChangeObserver != nil {
		e.ChangeObserver("DELETE", stored, storage.Record{})
	}
	return nil
}

func (e *Engine) upsertByExternalID(record storage.Record, externalIDField string, statementStamp *string, singleRecord bool) (storage.ID, bool, error) {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return "", false, err
	}
	if storage.IsCustomMetadataDefinition(object.Definition) {
		return "", false, customMetadataReadOnlyError(objectName)
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return "", false, err
	}
	if isExplicitIDUpsertField(object.Definition, e.Org.Namespace, externalIDField) {
		record = recordWithExplicitIDField(object.Definition, e.Org.Namespace, record)
		if record.ID == "" {
			id, err := e.insertUpsertID(record, statementStamp, false)
			return id, true, err
		}
		if err := e.validateUpsertIDReference(record); err != nil {
			return "", false, err
		}
		if err := e.updateOne(record); err != nil {
			return "", false, err
		}
		return record.ID, false, nil
	}
	field, value, ok, err := upsertExternalID(object.Definition, e.Org.Namespace, record, externalIDField, e.Options.AllowApexIDLookupUpsert)
	if err != nil {
		return "", false, err
	}
	if !ok {
		id, err := e.insertUpsertID(record, statementStamp, false)
		return id, true, err
	}
	matches := make([]storage.ID, 0, 1)
	for id, stored := range object.Records {
		if stored.System.IsDeleted {
			continue
		}
		storedValue, storedOK := stored.Fields[field]
		if !storedOK || !storageValuesEqual(object.Definition.Fields[field], storedValue, value) {
			continue
		}
		matches = append(matches, id)
	}
	if len(matches) > 1 {
		return "", false, dmlErrorf("DUPLICATE_EXTERNAL_ID", []string{field}, "dml: external id %s.%s matched multiple records", objectName, field)
	}
	if len(matches) == 0 {
		// The native rows cover a single record. Preserve the existing behavior
		// for multi-record custom-key upserts until their semantics are captured.
		if !singleRecord {
			id, err := e.insertOne(record, nil)
			return id, true, err
		}
		id, err := e.insertUpsertID(record, statementStamp, true)
		return id, true, err
	}
	record.ID = matches[0]
	if err := e.updateOne(record); err != nil {
		return "", false, err
	}
	return record.ID, false, nil
}

func (e *Engine) insertUpsertID(record storage.Record, statementStamp *string, capturedRulesOnly bool) (storage.ID, error) {
	previous := e.Options.EnforceDuplicateRules
	previousScope := e.Options.UpsertCapturedInsertRulesOnly
	if e.Options.EnforceUpsertIDInsertDuplicateRules {
		e.Options.EnforceDuplicateRules = true
	}
	e.Options.UpsertCapturedInsertRulesOnly = capturedRulesOnly
	defer func() {
		e.Options.EnforceDuplicateRules = previous
		e.Options.UpsertCapturedInsertRulesOnly = previousScope
	}()
	return e.insertOne(record, statementStamp)
}

func isExplicitIDUpsertField(definition storage.ObjectDefinition, namespace, fieldName string) bool {
	fieldName = strings.TrimSpace(fieldName)
	if fieldName == "" {
		return false
	}
	if canonical, ok := storage.ResolveFieldName(definition, namespace, fieldName); ok {
		return strings.EqualFold(canonical, "Id")
	}
	return strings.EqualFold(fieldName, "Id")
}

func recordWithExplicitIDField(definition storage.ObjectDefinition, namespace string, record storage.Record) storage.Record {
	if record.ID != "" {
		return record
	}
	for fieldName, value := range record.Fields {
		if !isExplicitIDUpsertField(definition, namespace, fieldName) {
			continue
		}
		if id := idFromStorageValue(value); id != "" {
			record.ID = id
		}
		return record
	}
	return record
}

func (e *Engine) validateUpsertIDReference(record storage.Record) error {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return err
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return err
	}
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return err
	}
	storedID, existing, ok := storage.LookupRecordByID(object.Records, record.ID)
	if !ok || existing.System.IsDeleted || storedID == "" {
		return dmlErrorf("INVALID_CROSS_REFERENCE_KEY", []string{"Id"}, "invalid cross reference id")
	}
	return nil
}
