package dml

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/open-aer/oaer/internal/storage"
)

type Engine struct {
	Org    *storage.OrgState
	IDs    storage.IDGenerator
	Now    func() time.Time
	UserID storage.ID
}

type Result struct {
	ID                storage.ID   `json:"id,omitempty"`
	Success           bool         `json:"success"`
	Error             string       `json:"error,omitempty"`
	StatusCode        string       `json:"statusCode,omitempty"`
	Fields            []string     `json:"fields,omitempty"`
	Errors            []Error      `json:"errors,omitempty"`
	Created           bool         `json:"created,omitempty"`
	MergedRecordIDs   []storage.ID `json:"mergedRecordIds,omitempty"`
	UpdatedRelatedIDs []storage.ID `json:"updatedRelatedIds,omitempty"`
}

type Error struct {
	Message    string   `json:"message"`
	StatusCode string   `json:"statusCode,omitempty"`
	Fields     []string `json:"fields,omitempty"`
}

func NewEngine(org *storage.OrgState) Engine {
	prefixes := make(map[string]string, len(org.Objects))
	for name, object := range org.Objects {
		if object.Definition.KeyPrefix != "" {
			prefixes[name] = object.Definition.KeyPrefix
		}
	}
	ids := storage.NewIDGenerator(prefixes)
	ids.Sequences = copySequences(org.IDSequences)
	return Engine{Org: org, IDs: ids, Now: func() time.Time { return time.Now().UTC() }, UserID: "005000000000001"}
}

func (e Engine) systemTimestamp() string {
	now := time.Now().UTC()
	if e.Now != nil {
		now = e.Now().UTC()
	}
	return now.Format(time.RFC3339)
}

func (e Engine) systemUserID() storage.ID {
	if e.UserID != "" {
		return e.UserID
	}
	return "005000000000001"
}

func (e *Engine) Insert(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		id, err := e.insertOne(record)
		if err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		results[i] = Result{ID: id, Success: true}
	}
	e.Org.IDSequences = copySequences(e.IDs.Sequences)
	return results
}

func (e *Engine) Update(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if err := e.updateOne(record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) Delete(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if err := e.deleteOne(record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) Upsert(records []storage.Record) []Result {
	return e.upsert(records, "")
}

func (e *Engine) UpsertWithExternalID(records []storage.Record, externalIDField string) []Result {
	return e.upsert(records, externalIDField)
}

func (e *Engine) upsert(records []storage.Record, externalIDField string) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if record.ID == "" {
			id, created, err := e.upsertByExternalID(record, externalIDField)
			if err != nil {
				results[i] = resultFromError(record.ID, err)
				continue
			}
			results[i] = Result{ID: id, Success: true, Created: created}
			continue
		}
		if err := e.updateOne(record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		results[i] = Result{ID: record.ID, Success: true, Created: false}
	}
	e.Org.IDSequences = copySequences(e.IDs.Sequences)
	return results
}

func (e *Engine) Undelete(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if record.ID == "" {
			results[i] = Result{Success: false, Error: "dml: undelete requires id", StatusCode: "MISSING_ARGUMENT", Fields: []string{"Id"}}
			continue
		}
		object, objectName, err := e.object(record.Object)
		if err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		if err := e.validateObjectID(object.Definition, record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		stored, ok := object.Records[record.ID]
		if !ok {
			results[i] = resultFromError(record.ID, fmt.Errorf("dml: record %s does not exist", record.ID))
			continue
		}
		if !stored.System.IsDeleted {
			results[i] = failedResult(record.ID, fmt.Sprintf("dml: record %s is not deleted", record.ID), "ENTITY_IS_NOT_DELETED", nil)
			continue
		}
		stamp := e.systemTimestamp()
		stored.System.IsDeleted = false
		stored.System.LastModifiedDate = stamp
		stored.System.SystemModstamp = stamp
		stored.System.LastModifiedByID = e.systemUserID()
		object.Records[record.ID] = stored
		e.Org.Objects[objectName] = object
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) EmptyRecycleBin(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if record.ID == "" {
			results[i] = Result{Success: false, Error: "dml: emptyRecycleBin requires id", StatusCode: "MISSING_ARGUMENT", Fields: []string{"Id"}}
			continue
		}
		object, objectName, err := e.object(record.Object)
		if err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		if err := e.validateObjectID(object.Definition, record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		stored, ok := object.Records[record.ID]
		if !ok {
			results[i] = resultFromError(record.ID, fmt.Errorf("dml: record %s does not exist", record.ID))
			continue
		}
		if !stored.System.IsDeleted {
			results[i] = Result{
				ID:         record.ID,
				Success:    false,
				Error:      fmt.Sprintf("dml: record %s is not in the recycle bin", record.ID),
				StatusCode: "ENTITY_IS_NOT_IN_RECYCLE_BIN",
			}
			continue
		}
		delete(object.Records, record.ID)
		e.Org.Objects[objectName] = object
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) Lock(records []storage.Record) []Result {
	return e.setLock(records, true)
}

func (e *Engine) Unlock(records []storage.Record) []Result {
	return e.setLock(records, false)
}

func (e *Engine) Merge(master storage.Record, duplicates []storage.Record) []Result {
	results := make([]Result, len(duplicates))
	object, objectName, err := e.object(master.Object)
	if err != nil {
		for i := range results {
			results[i] = resultFromError(master.ID, err)
		}
		return results
	}
	if master.ID == "" {
		for i := range results {
			results[i] = Result{Success: false, Error: "dml: merge master requires id", StatusCode: "MISSING_ARGUMENT", Fields: []string{"Id"}}
		}
		return results
	}
	storedMaster, ok := object.Records[master.ID]
	if !ok || storedMaster.System.IsDeleted {
		for i := range results {
			results[i] = resultFromError(master.ID, fmt.Errorf("dml: merge master %s does not exist", master.ID))
		}
		return results
	}
	if len(master.Fields) > 0 || len(master.ExplicitNulls) > 0 {
		if err := e.updateOne(master); err != nil {
			for i := range results {
				results[i] = resultFromError(master.ID, err)
			}
			return results
		}
		object = e.Org.Objects[objectName]
	}
	for i, duplicate := range duplicates {
		if duplicate.ID == "" {
			results[i] = Result{Success: false, Error: "dml: merge duplicate requires id", StatusCode: "MISSING_ARGUMENT", Fields: []string{"Id"}}
			continue
		}
		if duplicate.Object != "" && duplicate.Object != objectName {
			results[i] = resultFromError(duplicate.ID, fmt.Errorf("dml: merge duplicate object %s does not match master %s", duplicate.Object, objectName))
			continue
		}
		if duplicate.ID == master.ID {
			results[i] = resultFromError(duplicate.ID, fmt.Errorf("dml: merge duplicate cannot be master"))
			continue
		}
		storedDuplicate, ok := object.Records[duplicate.ID]
		if !ok || storedDuplicate.System.IsDeleted {
			results[i] = resultFromError(duplicate.ID, fmt.Errorf("dml: merge duplicate %s does not exist", duplicate.ID))
			continue
		}
		updatedRelatedIDs := e.reparentLookups(objectName, duplicate.ID, master.ID)
		stamp := e.systemTimestamp()
		storedDuplicate.System.IsDeleted = true
		storedDuplicate.System.LastModifiedDate = stamp
		storedDuplicate.System.SystemModstamp = stamp
		storedDuplicate.System.LastModifiedByID = e.systemUserID()
		object.Records[duplicate.ID] = storedDuplicate
		e.Org.Objects[objectName] = object
		results[i] = Result{ID: master.ID, Success: true, MergedRecordIDs: []storage.ID{duplicate.ID}, UpdatedRelatedIDs: updatedRelatedIDs}
	}
	return results
}

func (e *Engine) setLock(records []storage.Record, locked bool) []Result {
	results := make([]Result, len(records))
	op := "lock"
	if !locked {
		op = "unlock"
	}
	for i, record := range records {
		if record.ID == "" {
			results[i] = Result{Success: false, Error: "dml: " + op + " requires id", StatusCode: "MISSING_ARGUMENT", Fields: []string{"Id"}}
			continue
		}
		object, objectName, err := e.object(record.Object)
		if err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		if err := e.validateObjectID(object.Definition, record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		stored, ok := object.Records[record.ID]
		if !ok || stored.System.IsDeleted {
			results[i] = resultFromError(record.ID, fmt.Errorf("dml: record %s does not exist", record.ID))
			continue
		}
		stored.System.Locked = locked
		object.Records[record.ID] = stored
		e.Org.Objects[objectName] = object
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) reparentLookups(parentObject string, oldID, newID storage.ID) []storage.ID {
	seen := make(map[storage.ID]struct{})
	for childObjectName, childObject := range e.Org.Objects {
		changed := false
		for _, relation := range childObject.Definition.Relations {
			if !containsString(relation.ParentObjects, parentObject) {
				continue
			}
			for id, record := range childObject.Records {
				value, ok := record.Fields[relation.Field]
				if !ok || idFromStorageValue(value) != oldID {
					continue
				}
				record.Fields[relation.Field] = storage.IDValue(newID)
				childObject.Records[id] = record
				seen[id] = struct{}{}
				changed = true
			}
		}
		if changed {
			e.Org.Objects[childObjectName] = childObject
		}
	}
	ids := make([]storage.ID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (e *Engine) WithTransaction(fn func(*Engine) error) error {
	before := e.Org.Clone()
	if err := fn(e); err != nil {
		*e.Org = before
		e.IDs.Sequences = copySequences(before.IDSequences)
		return err
	}
	return nil
}

func (e *Engine) insertOne(record storage.Record) (storage.ID, error) {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return "", err
	}
	if storage.IsCustomMetadataDefinition(object.Definition) {
		return "", customMetadataReadOnlyError(objectName)
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return "", err
	}
	if err := validateFields(object.Definition, e.Org.Namespace, record); err != nil {
		return "", err
	}
	if err := validateRequired(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return "", err
	}
	if err := validateValidationRules(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateUnique(objectName, object.Definition, record, ""); err != nil {
		return "", err
	}
	if record.ID == "" {
		id, err := e.IDs.Next(objectName)
		if err != nil {
			return "", err
		}
		record.ID = id
	}
	if err := storage.ValidateID(record.ID); err != nil {
		return "", err
	}
	if _, exists := object.Records[record.ID]; exists {
		return "", fmt.Errorf("dml: duplicate id %s", record.ID)
	}
	stamp := e.systemTimestamp()
	userID := e.systemUserID()
	if record.System.CreatedDate == "" {
		record.System.CreatedDate = stamp
	}
	if record.System.LastModifiedDate == "" {
		record.System.LastModifiedDate = stamp
	}
	if record.System.SystemModstamp == "" {
		record.System.SystemModstamp = stamp
	}
	if record.System.CreatedByID == "" {
		record.System.CreatedByID = userID
	}
	if record.System.LastModifiedByID == "" {
		record.System.LastModifiedByID = userID
	}
	if record.System.OwnerID == "" {
		record.System.OwnerID = userID
	}
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	object.Records[record.ID] = record.Clone()
	e.Org.Objects[objectName] = object
	return record.ID, nil
}

func (e *Engine) updateOne(record storage.Record) error {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return err
	}
	if storage.IsCustomMetadataDefinition(object.Definition) {
		return customMetadataReadOnlyError(objectName)
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return err
	}
	if record.ID == "" {
		return fmt.Errorf("dml: update requires id")
	}
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return err
	}
	if err := validateFields(object.Definition, e.Org.Namespace, record); err != nil {
		return err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return err
	}
	if err := e.validateUnique(objectName, object.Definition, record, record.ID); err != nil {
		return err
	}
	existing, ok := object.Records[record.ID]
	if !ok {
		return fmt.Errorf("dml: record %s does not exist", record.ID)
	}
	if existing.System.IsDeleted {
		return fmt.Errorf("dml: record %s is deleted", record.ID)
	}
	finalRecord := existing.Clone()
	if finalRecord.Fields == nil {
		finalRecord.Fields = make(map[string]storage.Value)
	}
	if finalRecord.ExplicitNulls == nil {
		finalRecord.ExplicitNulls = make(map[string]bool)
	}
	for field, value := range record.Fields {
		finalRecord.Fields[field] = value.Clone()
		delete(finalRecord.ExplicitNulls, field)
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			delete(finalRecord.Fields, field)
			finalRecord.ExplicitNulls[field] = true
		}
	}
	if err := validateValidationRules(object.Definition, finalRecord); err != nil {
		return err
	}
	if existing.Fields == nil {
		existing.Fields = make(map[string]storage.Value)
	}
	if existing.ExplicitNulls == nil {
		existing.ExplicitNulls = make(map[string]bool)
	}
	stamp := e.systemTimestamp()
	for field, value := range record.Fields {
		existing.Fields[field] = value.Clone()
		delete(existing.ExplicitNulls, field)
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			delete(existing.Fields, field)
			existing.ExplicitNulls[field] = true
		}
	}
	existing.System.LastModifiedDate = stamp
	existing.System.SystemModstamp = stamp
	existing.System.LastModifiedByID = e.systemUserID()
	object.Records[record.ID] = existing
	e.Org.Objects[objectName] = object
	return nil
}

func (e *Engine) deleteOne(record storage.Record) error {
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
	stored, ok := object.Records[record.ID]
	if !ok {
		return fmt.Errorf("dml: record %s does not exist", record.ID)
	}
	if stored.System.IsDeleted {
		return fmt.Errorf("dml: record %s is deleted", record.ID)
	}
	if err := e.validateDeleteReferences(objectName, record.ID); err != nil {
		return err
	}
	return e.deleteRecord(objectName, record.ID, make(map[string]bool))
}

func (e *Engine) deleteRecord(objectName string, id storage.ID, seen map[string]bool) error {
	key := objectName + ":" + string(id)
	if seen[key] {
		return nil
	}
	seen[key] = true
	object := e.Org.Objects[objectName]
	stored, ok := object.Records[id]
	if !ok {
		return fmt.Errorf("dml: record %s does not exist", id)
	}
	if stored.System.IsDeleted {
		return fmt.Errorf("dml: record %s is deleted", id)
	}
	if err := e.validateDeleteReferences(objectName, id); err != nil {
		return err
	}
	stamp := e.systemTimestamp()
	stored.System.IsDeleted = true
	stored.System.LastModifiedDate = stamp
	stored.System.SystemModstamp = stamp
	stored.System.LastModifiedByID = e.systemUserID()
	object.Records[id] = stored
	e.Org.Objects[objectName] = object
	return e.cascadeDeleteChildren(objectName, id, seen)
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
	field, value, ok, err := upsertExternalID(object.Definition, e.Org.Namespace, record, externalIDField)
	if err != nil {
		return "", false, err
	}
	if !ok {
		id, err := e.insertOne(record)
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
		id, err := e.insertOne(record)
		return id, true, err
	}
	record.ID = matches[0]
	if err := e.updateOne(record); err != nil {
		return "", false, err
	}
	return record.ID, false, nil
}

func (e *Engine) object(name string) (storage.ObjectState, string, error) {
	objectName, ok := storage.ResolveObjectName(*e.Org, name)
	if !ok {
		return storage.ObjectState{}, "", fmt.Errorf("dml: unknown object %s", name)
	}
	object := e.Org.Objects[objectName]
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	return object, objectName, nil
}

func canonicalizeRecord(namespace string, definition storage.ObjectDefinition, objectName string, record storage.Record) (storage.Record, error) {
	record.Object = objectName
	fields := make(map[string]storage.Value, len(record.Fields))
	for field, value := range record.Fields {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			fields[field] = value
			continue
		}
		if _, exists := fields[canonical]; exists && canonical != field {
			return storage.Record{}, fmt.Errorf("dml: duplicate field alias %s.%s", objectName, field)
		}
		fields[canonical] = value
	}
	record.Fields = fields
	nulls := make(map[string]bool, len(record.ExplicitNulls))
	for field, value := range record.ExplicitNulls {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			nulls[field] = value
			continue
		}
		nulls[canonical] = value
	}
	record.ExplicitNulls = nulls
	return record, nil
}

func validateFields(definition storage.ObjectDefinition, namespace string, record storage.Record) error {
	for field := range record.Fields {
		if field == "Id" {
			continue
		}
		if _, ok := storage.ResolveFieldName(definition, namespace, field); !ok {
			return fmt.Errorf("dml: unknown field %s.%s", record.Object, field)
		}
	}
	for field := range record.ExplicitNulls {
		if _, ok := storage.ResolveFieldName(definition, namespace, field); !ok {
			return fmt.Errorf("dml: unknown field %s.%s", record.Object, field)
		}
	}
	return nil
}

func validateRequired(definition storage.ObjectDefinition, record storage.Record) error {
	for name, field := range definition.Fields {
		if !field.Required {
			continue
		}
		if _, ok := record.Fields[name]; ok {
			continue
		}
		if record.ExplicitNulls[name] {
			return fmt.Errorf("dml: required field %s.%s is null", record.Object, name)
		}
		return fmt.Errorf("dml: missing required field %s.%s", record.Object, name)
	}
	return nil
}

func (e *Engine) validateObjectID(definition storage.ObjectDefinition, record storage.Record) error {
	if record.ID == "" || definition.KeyPrefix == "" {
		return nil
	}
	if !strings.HasPrefix(string(record.ID), definition.KeyPrefix) {
		return dmlErrorf("INVALID_FIELD", []string{"Id"}, "dml: id %s does not belong to %s", record.ID, record.Object)
	}
	return nil
}

func (e *Engine) validateReferences(definition storage.ObjectDefinition, record storage.Record) error {
	for name, field := range definition.Fields {
		if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
			continue
		}
		value, ok := record.Fields[name]
		if !ok || value.Kind == storage.ValueNull {
			continue
		}
		id := idFromStorageValue(value)
		if id == "" {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{name}, "dml: invalid reference %s.%s", record.Object, name)
		}
		found := false
		for _, targetName := range field.ReferenceTo {
			canonical, ok := storage.ResolveObjectName(*e.Org, targetName)
			if !ok {
				continue
			}
			target := e.Org.Objects[canonical]
			parent, ok := target.Records[id]
			if ok && !parent.System.IsDeleted {
				found = true
				break
			}
		}
		if !found {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{name}, "dml: reference %s.%s points to missing record %s", record.Object, name, id)
		}
	}
	return nil
}

func (e *Engine) validateUnique(objectName string, definition storage.ObjectDefinition, record storage.Record, currentID storage.ID) error {
	for fieldName, field := range definition.Fields {
		if !field.Unique {
			continue
		}
		value, ok := record.Fields[fieldName]
		if !ok || value.Kind == storage.ValueNull {
			continue
		}
		object := e.Org.Objects[objectName]
		for id, stored := range object.Records {
			if id == currentID || stored.System.IsDeleted {
				continue
			}
			storedValue, ok := stored.Fields[fieldName]
			if !ok {
				continue
			}
			if storageValuesEqual(field, storedValue, value) {
				return dmlErrorf("DUPLICATE_VALUE", []string{fieldName}, "dml: duplicate value %s.%s", objectName, fieldName)
			}
		}
	}
	return nil
}

func validateValidationRules(definition storage.ObjectDefinition, record storage.Record) error {
	for _, rule := range definition.ValidationRules {
		if !rule.Active {
			continue
		}
		matches, ok := evaluateValidationFormula(rule.ErrorConditionFormula, record)
		if !ok || !matches {
			continue
		}
		message := rule.ErrorMessage
		if message == "" {
			message = fmt.Sprintf("dml: validation rule %s failed", rule.Name)
		}
		fields := []string(nil)
		if rule.ErrorDisplayField != "" {
			fields = []string{rule.ErrorDisplayField}
		}
		return dmlErrorf("FIELD_CUSTOM_VALIDATION_EXCEPTION", fields, "%s", message)
	}
	return nil
}

func evaluateValidationFormula(formula string, record storage.Record) (bool, bool) {
	formula = strings.TrimSpace(formula)
	if formula == "" {
		return false, false
	}
	upper := strings.ToUpper(formula)
	if strings.HasPrefix(upper, "NOT(") && strings.HasSuffix(formula, ")") {
		value, ok := evaluateValidationFormula(formula[4:len(formula)-1], record)
		return !value, ok
	}
	if strings.HasPrefix(upper, "ISBLANK(") && strings.HasSuffix(formula, ")") {
		field := strings.TrimSpace(formula[len("ISBLANK(") : len(formula)-1])
		return validationFieldBlank(record, field), true
	}
	for _, op := range []string{"<>", "!="} {
		if left, right, ok := splitValidationComparison(formula, op); ok {
			return !validationFieldEquals(record, left, right), true
		}
	}
	if left, right, ok := splitValidationComparison(formula, "="); ok {
		return validationFieldEquals(record, left, right), true
	}
	return false, false
}

func splitValidationComparison(formula, op string) (string, string, bool) {
	index := strings.Index(formula, op)
	if index < 0 {
		return "", "", false
	}
	left := strings.TrimSpace(formula[:index])
	right := strings.TrimSpace(formula[index+len(op):])
	if left == "" || right == "" {
		return "", "", false
	}
	return left, trimFormulaLiteral(right), true
}

func trimFormulaLiteral(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		quote := value[0]
		if (quote == '\'' || quote == '"') && value[len(value)-1] == quote {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func validationFieldBlank(record storage.Record, field string) bool {
	if field == "Id" {
		return record.ID == ""
	}
	if record.ExplicitNulls[field] {
		return true
	}
	value, ok := record.Fields[field]
	if !ok || value.Kind == storage.ValueNull {
		return true
	}
	return value.Kind == storage.ValueString && value.String == ""
}

func validationFieldEquals(record storage.Record, field, want string) bool {
	if field == "Id" {
		return string(record.ID) == want
	}
	if record.ExplicitNulls[field] {
		return want == "" || strings.EqualFold(want, "NULL")
	}
	value, ok := record.Fields[field]
	if !ok || value.Kind == storage.ValueNull {
		return want == "" || strings.EqualFold(want, "NULL")
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal:
		return value.String == want
	case storage.ValueID:
		return string(value.ID) == want
	case storage.ValueInteger:
		return fmt.Sprintf("%d", value.Integer) == want
	case storage.ValueBoolean:
		return strings.EqualFold(fmt.Sprintf("%t", value.Boolean), want)
	default:
		return false
	}
}

func (e *Engine) validateDeleteReferences(objectName string, id storage.ID) error {
	for childObjectName, childObject := range e.Org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if !relation.RestrictedDelete || !containsString(relation.ParentObjects, objectName) {
				continue
			}
			for _, child := range childObject.Records {
				if child.System.IsDeleted {
					continue
				}
				value, ok := child.Fields[relation.Field]
				if ok && idFromStorageValue(value) == id {
					return dmlErrorf("DELETE_FAILED", []string{relation.Field}, "dml: cannot delete %s %s because %s records reference it", objectName, id, childObjectName)
				}
			}
		}
	}
	return nil
}

func (e *Engine) cascadeDeleteChildren(objectName string, id storage.ID, seen map[string]bool) error {
	for childObjectName, childObject := range e.Org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if !relation.CascadeDelete || !containsString(relation.ParentObjects, objectName) {
				continue
			}
			for childID, child := range childObject.Records {
				if child.System.IsDeleted {
					continue
				}
				value, ok := child.Fields[relation.Field]
				if ok && idFromStorageValue(value) == id {
					if err := e.deleteRecord(childObjectName, childID, seen); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func upsertExternalID(definition storage.ObjectDefinition, namespace string, record storage.Record, externalIDField string) (string, storage.Value, bool, error) {
	if externalIDField != "" {
		fieldName := externalIDField
		if canonical, ok := storage.ResolveFieldName(definition, namespace, fieldName); ok {
			fieldName = canonical
		}
		field, ok := definition.Fields[fieldName]
		if !ok {
			return "", storage.Value{}, false, dmlErrorf("INVALID_FIELD", []string{externalIDField}, "dml: unknown external id field %s.%s", record.Object, externalIDField)
		}
		if !field.ExternalID {
			return "", storage.Value{}, false, dmlErrorf("INVALID_FIELD", []string{fieldName}, "dml: field %s.%s is not an external id", record.Object, fieldName)
		}
		value, ok := record.Fields[fieldName]
		if !ok || value.Kind == storage.ValueNull {
			return fieldName, storage.Value{}, false, dmlErrorf("MISSING_ARGUMENT", []string{fieldName}, "dml: external id field %s.%s is missing", record.Object, fieldName)
		}
		return fieldName, value, true, nil
	}
	for name, field := range definition.Fields {
		if !field.ExternalID {
			continue
		}
		value, ok := record.Fields[name]
		if ok && value.Kind != storage.ValueNull {
			return name, value, true, nil
		}
	}
	return "", storage.Value{}, false, nil
}

func idFromStorageValue(value storage.Value) storage.ID {
	switch value.Kind {
	case storage.ValueID:
		return value.ID
	case storage.ValueString:
		return storage.ID(value.String)
	default:
		return ""
	}
}

func storageValuesEqual(field storage.Field, left, right storage.Value) bool {
	if left.Kind == storage.ValueString && right.Kind == storage.ValueString && !field.CaseSensitive {
		return strings.EqualFold(left.String, right.String)
	}
	if left.Kind != right.Kind {
		if left.Kind == storage.ValueID && right.Kind == storage.ValueString {
			return string(left.ID) == right.String
		}
		if left.Kind == storage.ValueString && right.Kind == storage.ValueID {
			return left.String == string(right.ID)
		}
		return false
	}
	switch left.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal:
		return left.String == right.String
	case storage.ValueInteger:
		return left.Integer == right.Integer
	case storage.ValueBoolean:
		return left.Boolean == right.Boolean
	case storage.ValueID:
		return left.ID == right.ID
	default:
		return false
	}
}

type dmlError struct {
	code    string
	fields  []string
	message string
}

func (e dmlError) Error() string {
	return e.message
}

func dmlErrorf(code string, fields []string, format string, args ...any) error {
	return dmlError{code: code, fields: append([]string(nil), fields...), message: fmt.Sprintf(format, args...)}
}

func customMetadataReadOnlyError(objectName string) error {
	return dmlErrorf("INVALID_TYPE", nil, "dml: custom metadata object %s is read-only", objectName)
}

func resultFromError(id storage.ID, err error) Result {
	var typed dmlError
	if errors.As(err, &typed) {
		return failedResult(id, typed.message, typed.code, typed.fields)
	}
	msg := err.Error()
	code := "FIELD_CUSTOM_VALIDATION_EXCEPTION"
	var fields []string
	switch {
	case contains(msg, "unknown object"):
		code = "INVALID_TYPE"
	case contains(msg, "unknown field"):
		code = "INVALID_FIELD_FOR_INSERT_UPDATE"
		fields = extractField(msg)
	case contains(msg, "required field"):
		code = "REQUIRED_FIELD_MISSING"
		fields = extractField(msg)
	case contains(msg, "duplicate id"):
		code = "DUPLICATE_VALUE"
	case contains(msg, "deleted"):
		code = "ENTITY_IS_DELETED"
	case contains(msg, "update requires id") || contains(msg, "delete requires id") || contains(msg, "undelete requires id"):
		code = "MISSING_ARGUMENT"
		fields = []string{"Id"}
	case contains(msg, "does not exist"):
		code = "ENTITY_IS_DELETED"
	}
	return failedResult(id, msg, code, fields)
}

func failedResult(id storage.ID, message, statusCode string, fields []string) Result {
	copiedFields := append([]string(nil), fields...)
	return Result{
		ID:         id,
		Success:    false,
		Error:      message,
		StatusCode: statusCode,
		Fields:     copiedFields,
		Errors: []Error{{
			Message:    message,
			StatusCode: statusCode,
			Fields:     append([]string(nil), copiedFields...),
		}},
	}
}

func extractField(msg string) []string {
	// Extract field name from "dml: ... field Object.Field" or "dml: ... field Object.Field is null"
	parts := strings.Split(msg, ".")
	if len(parts) >= 2 {
		field := strings.TrimSpace(parts[len(parts)-1])
		field = strings.TrimSuffix(field, " is null")
		if field != "" {
			return []string{field}
		}
	}
	return nil
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), substr)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func copySequences(in map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
