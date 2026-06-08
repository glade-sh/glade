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
	nullsFromFields := explicitNullsFromFieldValues(record)
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return err
	}
	normalizeNameFields(objectName, object.Definition, &record)
	if record.ID == "" {
		return fmt.Errorf("dml: update requires id")
	}
	stripImplicitReadOnlyDefaultFields(object.Definition, e.Org.Namespace, &record, false)
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return err
	}
	storedID, existing, ok := storage.LookupRecordByID(object.Records, record.ID)
	if !ok {
		return fmt.Errorf("dml: record %s does not exist", record.ID)
	}
	if existing.System.IsDeleted && !e.Options.AllowUpdateDeleted {
		return fmt.Errorf("dml: record %s is deleted", record.ID)
	}
	stripUnchangedNonUpdateableFields(object.Definition, e.Org.Namespace, &record, existing, nullsFromFields)
	if err := validateFieldWriteability(object.Definition, e.Org.Namespace, record, false); err != nil {
		return err
	}
	stripReadOnlyUpdateFields(object.Definition, e.Org.Namespace, &record)
	if err := e.applyStringLengthRules(object.Definition, &record); err != nil {
		return err
	}
	if err := validateFields(object.Definition, e.Org.Namespace, record); err != nil {
		return err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return err
	}
	if err := e.validateUnique(objectName, object.Definition, record, storedID); err != nil {
		return err
	}
	finalRecord := existing.Clone()
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
	if err := validateRequiredUpdate(object.Definition, record); err != nil {
		return err
	}
	if err := validatePersonAccountRequiredFields(objectName, finalRecord); err != nil {
		return err
	}
	priorRecord := e.priorRecordForValidation(record.ID, existing)
	if err := e.validateValidationRules(objectName, object.Definition, finalRecord, &priorRecord, false); err != nil {
		return err
	}
	needsRollback := !e.DeferAutomation && hasObjectAutomation(object.Definition)
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
	stripStoredCalculatedFields(object.Definition, &existing)
	existing.System.LastModifiedDate = stamp
	existing.System.SystemModstamp = stamp
	existing.System.LastModifiedByID = e.systemUserID()
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordUpdate(objectName, storedID, oldRecord)
	}
	object.Records[storedID] = existing
	e.Org.Objects[objectName] = object
	e.removeUniqueIndexRecord(objectName, object.Definition, oldRecord)
	e.addUniqueIndexRecord(objectName, object.Definition, existing)
	e.recalculateSummaryFieldsForChildren(objectName, oldRecord, finalRecord)
	if syncPersonContactAfterUpdate(objectName) {
		if err := e.syncPersonContact(existing); err != nil {
			e.restoreRollbackPoint(rollback)
			return err
		}
	}
	if !e.DeferAutomation {
		if err := e.withPriorRecordForAutomation(storedID, oldRecord, func() error {
			_, err := e.ApplyAutomation(objectName, storedID)
			return err
		}); err != nil {
			e.restoreRollbackPoint(rollback)
			return err
		}
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
		if isFormulaBackedField(fieldDef) {
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
		return fmt.Errorf("dml: record %s is deleted", record.ID)
	}
	return e.deleteRecord(objectName, storedID, make(map[string]bool), ctx)
}

func (e *Engine) deleteRecord(objectName string, id storage.ID, seen map[string]bool, ctx *deleteContext) error {
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
		return fmt.Errorf("dml: record %s is deleted", id)
	}
	if err := e.validateDeleteReferences(objectName, id, ctx); err != nil {
		return err
	}
	if err := e.applyBeforeDeleteFlows(objectName, stored); err != nil {
		return err
	}
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, objectName); cloned {
		object = e.Org.Objects[objectName]
		storedID, stored, _ = storage.LookupRecordByID(object.Records, id)
	}
	stamp := e.systemTimestamp()
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordUpdate(objectName, storedID, stored)
	}
	stored.System.IsDeleted = true
	stored.System.LastModifiedDate = stamp
	stored.System.SystemModstamp = stamp
	stored.System.LastModifiedByID = e.systemUserID()
	object.Records[storedID] = stored
	e.Org.Objects[objectName] = object
	e.removeUniqueIndexRecord(objectName, object.Definition, stored)
	e.recalculateSummaryFieldsForChildren(objectName, stored)
	return e.cascadeDeleteChildren(objectName, storedID, seen, ctx)
}

func (e *Engine) upsertByExternalID(record storage.Record, externalIDField string) (storage.ID, bool, error) {
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
			id, err := e.insertOne(record, nil)
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
	field, value, ok, err := upsertExternalID(object.Definition, e.Org.Namespace, record, externalIDField)
	if err != nil {
		return "", false, err
	}
	if !ok {
		id, err := e.insertOne(record, nil)
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
		return "", false, dmlErrorf("DUPLICATE_VALUE", []string{field}, "dml: external id %s.%s matched multiple records", objectName, field)
	}
	if len(matches) == 0 {
		id, err := e.insertOne(record, nil)
		return id, true, err
	}
	record.ID = matches[0]
	if err := e.updateOne(record); err != nil {
		return "", false, err
	}
	return record.ID, false, nil
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
